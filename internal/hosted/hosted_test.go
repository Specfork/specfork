package hosted

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/specfork/specfork/internal/index"
	"github.com/specfork/specfork/internal/mcpserver"
	"github.com/specfork/specfork/internal/stats"
	"github.com/specfork/specfork/internal/testfx"
)

var clock = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

type header struct{ h map[string]string }

func (r header) RoundTrip(q *http.Request) (*http.Response, error) {
	for k, v := range r.h {
		q.Header.Set(k, v)
	}
	return http.DefaultTransport.RoundTrip(q)
}

func setup(t *testing.T, st stats.Store, h map[string]string) (*httptest.Server, *mcp.ClientSession) {
	t.Helper()
	ix, fetch := testfx.Seeds(t)
	ts := httptest.NewServer(Handler(mcpserver.Deps{LoadIndex: func() (*index.Index, error) { return ix, nil }, Fetcher: fetch, Version: "test"}, st, func() time.Time { return clock }))
	t.Cleanup(ts.Close)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "1"}, nil).Connect(context.Background(),
		&mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp", HTTPClient: &http.Client{Transport: header{h}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return ts, cs
}

func call(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// MCP-6
func TestHostedHandshake(t *testing.T) {
	_, cs := setup(t, stats.NewMemory(), nil)
	if got := cs.InitializeResult().ServerInfo.Name; got != "specfork" {
		t.Fatalf("server %q", got)
	}
}

// MCP-6
func TestHostedTools(t *testing.T) {
	_, cs := setup(t, stats.NewMemory(), nil)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
	}
	if strings.Join(names, ",") != "get_spec,resolve_spec,search_specs" {
		t.Fatalf("tools %v", names)
	}
	r := call(t, cs, "resolve_spec", map[string]any{"name": "@specfork/scheduling"})
	if r.IsError || !strings.HasPrefix(r.Content[0].(*mcp.TextContent).Text, "# Bundle: @specfork/scheduling") {
		t.Fatalf("resolve: %+v", r)
	}
}

// MCP-7
func TestCounterKey(t *testing.T) {
	st := stats.NewMemory()
	_, cs := setup(t, st, nil)
	call(t, cs, "search_specs", map[string]any{"query": "booking"})
	call(t, cs, "get_spec", map[string]any{"name": "@specfork/scheduling"})
	call(t, cs, "resolve_spec", map[string]any{"name": "@specfork/scheduling"})
	call(t, cs, "resolve_spec", map[string]any{"name": "@specfork/scheduling"})
	if r := call(t, cs, "resolve_spec", map[string]any{"name": "@nobody/nothing"}); !r.IsError {
		t.Fatal("expected a tool error")
	}
	call(t, cs, "get_spec", map[string]any{"name": "@nobody/other"})
	want := []stats.Row{
		{Day: "2026-10-27", Tool: "get_spec", Spec: "@specfork/scheduling", Count: 1},
		{Day: "2026-10-27", Tool: "get_spec", Spec: "unknown", Count: 1},
		{Day: "2026-10-27", Tool: "resolve_spec", Spec: "@specfork/scheduling", Count: 2},
		{Day: "2026-10-27", Tool: "resolve_spec", Spec: "unknown", Count: 1},
		{Day: "2026-10-27", Tool: "search_specs", Spec: "", Count: 1},
	}
	got := st.Rows(clock)
	if len(got) != len(want) {
		t.Fatalf("rows %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d: %+v, want %+v", i, got[i], want[i])
		}
	}
}

// MCP-7
func TestCounterNoPersonalData(t *testing.T) {
	var logs bytes.Buffer
	oldL, oldS := log.Writer(), slog.Default()
	log.SetOutput(&logs)
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { log.SetOutput(oldL); slog.SetDefault(oldS) })

	path := filepath.Join(t.TempDir(), "stats.json")
	st, err := stats.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, cs := setup(t, st, map[string]string{"X-Forwarded-For": "203.0.113.77", "User-Agent": "secret-agent/9", "Authorization": "Bearer tok-123"})
	call(t, cs, "search_specs", map[string]any{"query": "needle-query-text"})
	call(t, cs, "resolve_spec", map[string]any{"name": "@specfork/scheduling"})
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	all := string(stored) + logs.String()
	for _, s := range []string{"203.0.113.77", "127.0.0.1", "secret-agent", "tok-123", "needle-query-text", cs.ID()} {
		if s != "" && strings.Contains(all, s) {
			t.Errorf("stored or logged %q", s)
		}
	}
	var rows []map[string]any
	if err := json.Unmarshal(stored, &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if len(r) != 4 {
			t.Errorf("row has fields beyond day, tool, spec, count: %v", r)
		}
	}
}

// MCP-8
func TestStatsJSON(t *testing.T) {
	st := stats.NewMemory()
	st.Add(stats.NewKey(clock.AddDate(0, 0, -95), "resolve_spec", "@old/spec"))
	st.Add(stats.NewKey(clock.AddDate(0, 0, -3), "resolve_spec", "@specfork/scheduling"))
	st.Add(stats.NewKey(clock.AddDate(0, 0, -3), "resolve_spec", "@specfork/scheduling"))
	ts, _ := setup(t, st, nil)
	res, err := http.Get(ts.URL + "/stats.json")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if cc := res.Header.Get("Cache-Control"); cc != "public, max-age=300" {
		t.Fatalf("Cache-Control %q", cc)
	}
	b, _ := io.ReadAll(res.Body)
	var rows []map[string]any
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	if len(rows) != 1 || rows[0]["day"] != "2026-10-24" || rows[0]["tool"] != "resolve_spec" || rows[0]["spec"] != "@specfork/scheduling" || rows[0]["count"] != float64(2) {
		t.Fatalf("rows %s", b)
	}
	if res, err := http.Post(ts.URL+"/stats.json", "application/json", nil); err != nil || res.StatusCode == 200 {
		t.Fatalf("POST should not be served: %v %v", res, err)
	}
}

// downStore is a counter store whose database is gone.
type downStore struct{ *stats.Memory }

func (downStore) Add(stats.Key) error                    { return errors.New("db down") }
func (downStore) RowsErr(time.Time) ([]stats.Row, error) { return nil, errors.New("db down") }

// MCP-12
func TestToolsWithoutDB(t *testing.T) {
	ts, cs := setup(t, downStore{stats.NewMemory()}, nil)
	if r := call(t, cs, "resolve_spec", map[string]any{"name": "@specfork/scheduling"}); r.IsError {
		t.Fatalf("tool call failed with the database down: %+v", r)
	}
	res, err := http.Get(ts.URL + "/stats.json")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("stats.json status %d", res.StatusCode)
	}
}
