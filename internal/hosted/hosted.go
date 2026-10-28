// Package hosted serves the MCP tools over streamable HTTP and counts their use (MCP-6..8).
package hosted

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/specfork/specfork/internal/mcpserver"
	"github.com/specfork/specfork/internal/stats"
)

var tools = map[string]bool{"search_specs": true, "get_spec": true, "resolve_spec": true}

// Handler serves /mcp and /stats.json. It reads no header, address or session of a
// request for counting, and logs nothing (MCP-7).
func Handler(d mcpserver.Deps, st stats.Store, now func() time.Time) http.Handler {
	srv := mcpserver.New(d)
	srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			res, err := next(ctx, method, req)
			if method != "tools/call" {
				return res, err
			}
			p, ok := req.GetParams().(*mcp.CallToolParamsRaw)
			if !ok {
				return res, err
			}
			failed := err != nil
			if r, ok := res.(*mcp.CallToolResult); ok && r != nil && r.IsError {
				failed = true
			}
			st.Add(stats.NewKey(now(), tool(p.Name), specOf(d, p, failed)))
			return res, err
		}
	})
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil))
	mux.HandleFunc("GET /stats.json", func(w http.ResponseWriter, _ *http.Request) {
		rows := st.Rows(now())
		if c, ok := st.(interface {
			RowsErr(time.Time) ([]stats.Row, error)
		}); ok {
			var err error
			if rows, err = c.RowsErr(now()); err != nil { // MCP-12
				http.Error(w, `{"error":"unavailable"}`, http.StatusServiceUnavailable)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		json.NewEncoder(w).Encode(rows)
	})
	return mux
}

// tool bounds the tool label to the known tools so callers cannot grow the table.
func tool(name string) string {
	if tools[name] {
		return name
	}
	return stats.Unknown
}

// specOf is the spec name a call is counted under (MCP-7).
func specOf(d mcpserver.Deps, p *mcp.CallToolParamsRaw, failed bool) string {
	if failed {
		return stats.Unknown
	}
	if p.Name == "search_specs" {
		return ""
	}
	var a struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(p.Arguments, &a) != nil {
		return stats.Unknown
	}
	ix, err := d.LoadIndex()
	if err != nil {
		return stats.Unknown
	}
	if _, ok := ix.Find(a.Name); !ok {
		return stats.Unknown
	}
	return a.Name
}
