package hosted

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/specfork/specfork/internal/index"
	"github.com/specfork/specfork/internal/stats"
	"github.com/specfork/specfork/internal/testfx"
)

var now = time.Date(2026, 10, 26, 23, 59, 0, 0, time.UTC)

func serve(t *testing.T, store stats.Store) (*httptest.Server, *mcp.ClientSession) {
	t.Helper()
	ix, fetch := testfx.Seeds(t)
	h := New(Deps{LoadIndex: func() (*index.Index, error) { return ix, nil }, Fetcher: fetch, Version: "test", Store: store, Now: func() time.Time { return now }})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "1"}, nil).Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return ts, cs
}

// MCP-6
func TestHostedHandshake(t *testing.T) {
	_, cs := serve(t, &stats.Mem{})
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
func TestHostedTools(t *testing.T) {
	_, cs := serve(t, &stats.Mem{})
	r := call(t, cs, "search_specs", map[string]any{"query": "booking"})
	if !strings.Contains(r.Content[0].(*mcp.TextContent).Text, "@specfork/scheduling") {
		t.Fatalf("search %v", r.Content)
	}
	r = call(t, cs, "get_spec", map[string]any{"name": "@specfork/auth"})
	if r.IsError || !strings.Contains(r.Content[0].(*mcp.TextContent).Text, `"version":"0.1.0"`) {
		t.Fatalf("get %v", r.Content)
	}
	r = call(t, cs, "resolve_spec", map[string]any{"name": "@specfork/scheduling"})
	if doc := r.Content[0].(*mcp.TextContent).Text; !strings.HasPrefix(doc, "# Bundle: @specfork/scheduling@0.1.0") {
		t.Fatalf("bundle starts %q", doc[:40])
	}
	if !call(t, cs, "resolve_spec", map[string]any{"name": "@nobody/x"}).IsError {
		t.Fatal("unknown spec should be a tool error")
	}
}

// MCP-7
func TestCounterKey(t *testing.T) {
	store := &stats.Mem{}
	_, cs := serve(t, store)
	call(t, cs, "resolve_spec", map[string]any{"name": "@specfork/scheduling"})
	call(t, cs, "search_specs", map[string]any{"query": "a secret search"})
	call(t, cs, "resolve_spec", map[string]any{"name": "@nobody/x"})
	call(t, cs, "get_spec", map[string]any{"name": "@specfork/auth", "version": "9.9.9"}) // indexed name, failed call
	rows, _ := store.Rows("")
	want := []stats.Row{
		{Day: "2026-10-26", Tool: "get_spec", Spec: "unknown", Count: 1},
		{Day: "2026-10-26", Tool: "resolve_spec", Spec: "@specfork/scheduling", Count: 1},
		{Day: "2026-10-26", Tool: "resolve_spec", Spec: "unknown", Count: 1},
		{Day: "2026-10-26", Tool: "search_specs", Spec: "", Count: 1},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows %+v", rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
}

// MCP-7
func TestCounterNoPersonalData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.json")
	store, err := stats.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ts, _ := serve(t, store)
	body := `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"search_specs","arguments":{"query":"zebra-query-text"}}}`
	req, _ := http.NewRequest("POST", ts.URL+"/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("X-Forwarded-For", "203.0.113.77")
	req.Header.Set("User-Agent", "secret-agent/9")
	req.Header.Set("Mcp-Session-Id", "session-xyz")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"203.0.113.77", "127.0.0.1", "secret-agent", "session-xyz", "zebra-query-text"} {
		if strings.Contains(string(data), s) {
			t.Errorf("stored data contains %q", s)
		}
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil || len(rows) == 0 {
		t.Fatalf("stored %s (%v)", data, err)
	}
	for _, r := range rows {
		if len(r) != 4 {
			t.Errorf("row has fields beyond day, tool, spec, count: %v", r)
		}
	}
}

// MCP-8
func TestStatsJSON(t *testing.T) {
	store := &stats.Mem{}
	days := []string{"2026-07-01", "2026-07-29", "2026-07-28"} // 2026-07-29 is the first of 90 days ending 2026-10-26
	for _, d := range days {
		store.Add(d, "resolve_spec", "@specfork/auth")
	}
	store.Add("2026-10-26", "search_specs", "")
	ts, _ := serve(t, store)
	resp, err := http.Get(ts.URL + "/stats.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=300" {
		t.Errorf("Cache-Control %q", cc)
	}
	var rows []stats.Row
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Day != "2026-07-29" || rows[1].Day != "2026-10-26" || rows[1].Tool != "search_specs" {
		t.Fatalf("rows %+v", rows)
	}
}
