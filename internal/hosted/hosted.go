// Package hosted serves the MCP tools over streamable HTTP and counts calls per day,
// tool and spec name, nothing else (MCP-6..8).
package hosted

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/specfork/specfork/internal/index"
	"github.com/specfork/specfork/internal/mcpserver"
	"github.com/specfork/specfork/internal/source"
	"github.com/specfork/specfork/internal/stats"
)

// Deps are the CLI's (MCP-3) plus the counter store.
type Deps struct {
	LoadIndex func() (*index.Index, error)
	Fetcher   source.Fetcher
	Version   string
	Store     stats.Store
	Now       func() time.Time // nil: time.Now
}

// New returns the handler for /mcp, /stats.json and /healthz. It keeps no sessions and
// writes no logs; request headers and bodies are never stored (MCP-7).
func New(d Deps) http.Handler {
	if d.Now == nil {
		d.Now = time.Now
	}
	srv := mcpserver.New(mcpserver.Deps{
		LoadIndex: d.LoadIndex, Fetcher: d.Fetcher, Version: d.Version,
		OnCall: func(tool, spec string, err error) { d.count(tool, spec, err) },
	})
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
	mux.HandleFunc("GET /stats.json", d.statsJSON)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	return mux
}

// count adds one call. Failed calls and names outside the index are counted as unknown.
func (d Deps) count(tool, spec string, err error) {
	if tool == "search_specs" {
		spec = ""
	} else if err != nil || !d.indexed(spec) {
		spec = stats.Unknown
	}
	d.Store.Add(stats.Day(d.Now()), tool, spec)
}

func (d Deps) indexed(name string) bool {
	ix, err := d.LoadIndex()
	if err != nil {
		return false
	}
	_, ok := ix.Find(name)
	return ok
}

func (d Deps) statsJSON(w http.ResponseWriter, _ *http.Request) {
	rows, err := d.Store.Rows(stats.Day(d.Now().AddDate(0, 0, 1-stats.Window)))
	if err != nil {
		http.Error(w, "stats unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	json.NewEncoder(w).Encode(rows)
}
