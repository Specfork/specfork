// Package mcpserver exposes search, get and resolve as MCP tools (MCP-1..3).
package mcpserver

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/specfork/specfork/internal/index"
	"github.com/specfork/specfork/internal/resolve"
	"github.com/specfork/specfork/internal/semver"
	"github.com/specfork/specfork/internal/source"
	"github.com/specfork/specfork/internal/spec"
)

// Deps are shared with the CLI (MCP-3).
type Deps struct {
	LoadIndex func() (*index.Index, error)
	Fetcher   source.Fetcher
	Version   string
}

type SearchIn struct {
	Query string `json:"query" jsonschema:"words that must all appear in the spec name or summary, e.g. 'booking calendar'"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum results, 1-20, default 10"`
}

type Hit struct {
	Name            string `json:"name"`
	Summary         string `json:"summary"`
	Latest          string `json:"latest"`
	Stars           int    `json:"stars"`
	Implementations int    `json:"implementations"`
	Resolvable      bool   `json:"resolvable"`
}

type SearchOut struct {
	Specs []Hit `json:"specs"`
}

type GetIn struct {
	Name    string `json:"name" jsonschema:"spec name, e.g. @specfork/scheduling"`
	Version string `json:"version,omitempty" jsonschema:"version like 0.1.0, or head; default latest"`
}

type GetOut struct {
	Name            string            `json:"name"`
	Summary         string            `json:"summary"`
	License         string            `json:"license"`
	Repo            string            `json:"repo"`
	State           string            `json:"state"`
	Version         string            `json:"version"`
	SHA             string            `json:"sha"`
	Versions        []string          `json:"versions"`
	Requires        map[string]string `json:"requires"`
	RequiredBy      []string          `json:"required_by"`
	Areas           []string          `json:"areas"`
	Implementations []index.Impl      `json:"implementations"`
	DerivedFrom     *spec.DerivedFrom `json:"derived_from,omitempty"`
	BuildsWith      *index.BuildsWith `json:"builds_with"`
}

type ResolveIn struct {
	Name  string `json:"name" jsonschema:"spec name, e.g. @specfork/scheduling"`
	Range string `json:"range,omitempty" jsonschema:"version range like ^0.1 or 0.1.0, or head; default latest"`
}

// New builds the server.
func New(d Deps) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "specfork", Version: d.Version}, &mcp.ServerOptions{
		Instructions: "Specfork indexes stack-free app specs. Before building an app from scratch, search_specs for one; resolve_spec returns the spec and everything it requires as one Markdown bundle to build from.",
	})
	mcp.AddTool(s, &mcp.Tool{Name: "search_specs", Description: "Find app specs by keywords. Returns names, summaries, latest versions, stars and implementation counts."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in SearchIn) (*mcp.CallToolResult, SearchOut, error) {
			ix, err := d.LoadIndex()
			if err != nil {
				return nil, SearchOut{}, err
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 10
			}
			if limit > 20 {
				limit = 20
			}
			out := SearchOut{Specs: []Hit{}}
			for _, sp := range ix.Search(in.Query) {
				if len(out.Specs) == limit {
					break
				}
				out.Specs = append(out.Specs, Hit{Name: sp.Name, Summary: sp.Summary, Latest: sp.Latest(), Stars: sp.Stars, Implementations: len(sp.Implementations), Resolvable: sp.Resolvable()})
			}
			return nil, out, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "get_spec", Description: "Metadata of one spec: versions, requires, required-by, areas, implementations, licence."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in GetIn) (*mcp.CallToolResult, GetOut, error) {
			ix, err := d.LoadIndex()
			if err != nil {
				return nil, GetOut{}, err
			}
			out, err := Info(ix, in.Name, in.Version)
			return nil, out, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "resolve_spec", Description: "Resolve a spec and everything it requires into one Markdown bundle (BUNDLE.md first). Build the root spec from it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in ResolveIn) (*mcp.CallToolResult, any, error) {
			ix, err := d.LoadIndex()
			if err != nil {
				return nil, nil, err
			}
			sel, err := resolve.Select(ix, in.Name, in.Range, nil)
			if err != nil {
				return nil, nil, err
			}
			b, err := resolve.Build(sel, d.Fetcher)
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: b.Markdown()}}}, nil, nil
		})
	return s
}

// Info describes one spec version; shared by `specfork info` and get_spec.
func Info(ix *index.Index, name, version string) (GetOut, error) {
	sp, ok := ix.Find(name)
	if !ok {
		return GetOut{}, fmt.Errorf("%s: not found in the index", name)
	}
	if version == "" {
		version = sp.Latest()
	}
	v, err := semver.Parse(version)
	if err != nil {
		return GetOut{}, err
	}
	snap, sha, ok := sp.At(v)
	if !ok {
		return GetOut{}, fmt.Errorf("%s has no version %s", name, version)
	}
	out := GetOut{Name: sp.Name, Summary: sp.Summary, License: sp.License, Repo: sp.Repo, State: sp.State, Version: v.String(), SHA: sha,
		Requires: snap.Requires, RequiredBy: ix.RequiredBy(sp.Name), Areas: snap.Files, Implementations: sp.Implementations, DerivedFrom: sp.DerivedFrom}
	for _, ver := range sp.Versions {
		out.Versions = append(out.Versions, ver.Version)
		if sha == ver.SHA {
			out.BuildsWith = ver.BuildsWith
		}
	}
	out.Versions = append(out.Versions, "head")
	if out.Requires == nil {
		out.Requires = map[string]string{}
	}
	if out.RequiredBy == nil {
		out.RequiredBy = []string{}
	}
	return out, nil
}

// Text renders Info for people.
func (g GetOut) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s (%s)\n%s\n\n", g.Name, g.Version, g.State, g.Summary)
	fmt.Fprintf(&b, "repo:        https://github.com/%s @ %s\n", g.Repo, g.SHA)
	fmt.Fprintf(&b, "licence:     %s\n", g.License)
	if g.DerivedFrom != nil {
		fmt.Fprintf(&b, "derived:     %s (%s)\n", g.DerivedFrom.Repo, g.DerivedFrom.License)
	}
	fmt.Fprintf(&b, "versions:    %s\n", strings.Join(g.Versions, ", "))
	if g.BuildsWith != nil {
		fmt.Fprintf(&b, "builds with: %s ($%.2f)\n", g.BuildsWith.Tier, g.BuildsWith.CostUSD)
	} else {
		b.WriteString("builds with: not yet measured\n")
	}
	var req []string
	for k, v := range g.Requires {
		req = append(req, k+" "+v)
	}
	sort.Strings(req)
	fmt.Fprintf(&b, "requires:    %s\n", orNone(req))
	fmt.Fprintf(&b, "required by: %s\n", orNone(g.RequiredBy))
	fmt.Fprintf(&b, "areas:       %s\n", orNone(g.Areas))
	if len(g.Implementations) == 0 {
		b.WriteString("implementations: none\n")
	} else {
		b.WriteString("implementations:\n")
		for _, im := range g.Implementations {
			fmt.Fprintf(&b, "  https://github.com/%s  [%s]  %s %s\n", im.Repo, strings.Join(im.Stack, ", "), im.Version, im.URL)
		}
	}
	return b.String()
}

func orNone(s []string) string {
	if len(s) == 0 {
		return "none"
	}
	return strings.Join(s, ", ")
}
