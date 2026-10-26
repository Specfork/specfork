// Command specfork finds, lints and resolves Specfork specs (CLI), serves MCP (MCP),
// and runs the index and claim workflows.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/specfork/specfork/internal/crawl"
	"github.com/specfork/specfork/internal/ghapi"
	"github.com/specfork/specfork/internal/hosted"
	"github.com/specfork/specfork/internal/index"
	"github.com/specfork/specfork/internal/mcpserver"
	"github.com/specfork/specfork/internal/registry"
	"github.com/specfork/specfork/internal/resolve"
	"github.com/specfork/specfork/internal/site"
	"github.com/specfork/specfork/internal/source"
	"github.com/specfork/specfork/internal/spec"
	"github.com/specfork/specfork/internal/stats"
)

// Exit codes (CLI-9).
const (
	exitOK      = 0
	exitFail    = 1
	exitUsage   = 2
	exitNetwork = 3
)

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

type netError struct{ err error }

func (e netError) Error() string { return e.err.Error() }
func (e netError) Unwrap() error { return e.err }

// failure is a non-zero exit whose message was already printed.
type failure struct{}

func (failure) Error() string { return "" }

const usage = `specfork: find, compose and build from app specs

  specfork search <query> [--json]          find specs
  specfork info <name>[@version] [--json]   versions, requires, implementations
  specfork resolve <name>[@range] [--out dir] [--update]
                                           write the bundle and specfork.lock
  specfork lint [dir] [--cheap]             check a spec repo (--cheap: also the cheap-buildable profile)
  specfork mcp                              MCP server on stdio
  specfork serve [--addr :8080] [--stats-db stats.json]
                                           hosted MCP over HTTP at /mcp, counters at /stats.json
  specfork version

Index: $SPECFORK_INDEX or ` + index.DefaultURL + `
`

// app holds swappable dependencies for tests.
type app struct {
	stdout, stderr io.Writer
	loadIndex      func() (*index.Index, error)
	fetcher        source.Fetcher
}

func main() {
	a := &app{stdout: os.Stdout, stderr: os.Stderr, fetcher: source.GitHub{}}
	a.loadIndex = func() (*index.Index, error) {
		ix, err := index.Load(index.Source())
		if err != nil {
			return nil, netError{err}
		}
		return ix, nil
	}
	os.Exit(a.run(os.Args[1:]))
}

func (a *app) run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.stderr, usage)
		return exitUsage
	}
	var err error
	switch args[0] {
	case "search":
		err = a.search(args[1:])
	case "info":
		err = a.info(args[1:])
	case "resolve":
		err = a.resolve(args[1:])
	case "lint":
		err = a.lint(args[1:])
	case "mcp":
		err = a.mcp()
	case "serve":
		err = a.serve(args[1:])
	case "crawl":
		err = a.crawl(args[1:])
	case "claim-check":
		err = a.claimCheck(args[1:])
	case "version", "--version":
		fmt.Fprintln(a.stdout, version())
	case "help", "-h", "--help":
		fmt.Fprint(a.stdout, usage)
	default:
		err = usageError{"unknown command " + args[0]}
	}
	var ue usageError
	var ne netError
	switch {
	case err == nil:
		return exitOK
	case errors.As(err, &ue):
		fmt.Fprintf(a.stderr, "specfork: %s\n\n%s", err, usage)
		return exitUsage
	case errors.As(err, &ne):
		fmt.Fprintf(a.stderr, "specfork: %s\n", err)
		return exitNetwork
	case errors.Is(err, failure{}):
		return exitFail
	default:
		fmt.Fprintf(a.stderr, "specfork: %s\n", err)
		return exitFail
	}
}

func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return "specfork " + bi.Main.Version
	}
	return "specfork (devel)"
}

// parse splits flags from positional args so flags may follow them.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	fs.SetOutput(io.Discard)
	var pos []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, usageError{err.Error()}
		}
		args = fs.Args()
		if len(args) > 0 {
			pos = append(pos, args[0])
			args = args[1:]
		}
	}
	return pos, nil
}

func (a *app) search(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return usageError{"search needs a query"}
	}
	ix, err := a.loadIndex()
	if err != nil {
		return err
	}
	hits := ix.Search(strings.Join(pos, " "))
	if *asJSON {
		out := []mcpserver.Hit{}
		for _, s := range hits {
			out = append(out, mcpserver.Hit{Name: s.Name, Summary: s.Summary, Latest: s.Latest(), Stars: s.Stars, Implementations: len(s.Implementations), Resolvable: s.Resolvable()})
		}
		return json.NewEncoder(a.stdout).Encode(out)
	}
	if len(hits) == 0 {
		fmt.Fprintln(a.stderr, "no specs match")
		return failure{}
	}
	tw := tabwriter.NewWriter(a.stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tLATEST\tSTARS\tIMPLS\tSUMMARY")
	for _, s := range hits {
		name := s.Name
		if !s.Resolvable() {
			name += " (" + s.State + ")"
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n", name, s.Latest(), s.Stars, len(s.Implementations), s.Summary)
	}
	return tw.Flush()
}

func (a *app) info(args []string) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usageError{"info needs one spec name"}
	}
	ix, err := a.loadIndex()
	if err != nil {
		return err
	}
	name, ver := spec.SplitNameVersion(pos[0])
	out, err := mcpserver.Info(ix, name, ver)
	if err != nil {
		return err
	}
	if *asJSON {
		return json.NewEncoder(a.stdout).Encode(out)
	}
	fmt.Fprint(a.stdout, out.Text())
	return nil
}

func (a *app) resolve(args []string) error {
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	out := fs.String("out", "specfork-bundle", "")
	update := fs.Bool("update", false, "")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usageError{"resolve needs one spec name"}
	}
	name, rng := spec.SplitNameVersion(pos[0])
	ix, err := a.loadIndex()
	if err != nil {
		return err
	}
	var lock *resolve.Lock
	if !*update {
		if lock, err = resolve.ReadLock("specfork.lock"); err != nil {
			return err
		}
		if lock != nil && lock.Root != name {
			lock = nil
		}
		if lock != nil && rng != "" {
			return usageError{"specfork.lock pins " + name + "; drop the range or pass --update"}
		}
	}
	sel, err := resolve.Select(ix, name, rng, lock)
	if err != nil {
		return err
	}
	b, err := resolve.Build(sel, a.fetcher)
	if err != nil {
		return netError{err}
	}
	if err := b.Write(*out); err != nil {
		return err
	}
	if err := resolve.WriteLock("specfork.lock", sel.Lock()); err != nil {
		return err
	}
	for _, n := range sel.Order {
		fmt.Fprintf(a.stdout, "%s@%s  %s\n", n.Name, n.Version, short(n.SHA))
	}
	fmt.Fprintf(a.stdout, "wrote %s/ (%d files) and specfork.lock; start at %s/BUNDLE.md\n", *out, len(b.Paths), *out)
	return nil
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func (a *app) lint(args []string) error {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	cheap := fs.Bool("cheap", false, "")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	dir := "."
	if len(pos) > 1 {
		return usageError{"lint takes at most one directory"}
	}
	if len(pos) == 1 {
		dir = pos[0]
	}
	r, err := spec.LoadDir(dir)
	if err != nil {
		return err
	}
	issues := spec.Lint(r)
	if *cheap {
		issues = append(issues, spec.LintCheap(r)...)
	}
	errs := 0
	for _, i := range issues {
		fmt.Fprintln(a.stdout, i)
		if !i.Warning {
			errs++
		}
	}
	if errs > 0 {
		fmt.Fprintf(a.stderr, "%d error(s)\n", errs)
		return failure{}
	}
	fmt.Fprintln(a.stderr, "ok")
	return nil
}

func (a *app) mcp() error {
	srv := mcpserver.New(mcpserver.Deps{LoadIndex: a.loadIndex, Fetcher: a.fetcher, Version: version()})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return srv.Run(ctx, &mcp.StdioTransport{})
}

// serve runs the hosted MCP server (MCP-6..8). It logs nothing about requests.
func (a *app) serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", ":8080", "")
	db := fs.String("stats-db", "stats.json", "")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	store, err := stats.OpenFile(*db)
	if err != nil {
		return err
	}
	h := hosted.New(hosted.Deps{LoadIndex: a.loadIndex, Fetcher: a.fetcher, Version: version(), Store: store})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	srv := &http.Server{Addr: *addr, Handler: h, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	fmt.Fprintf(a.stderr, "specfork: serving MCP on %s/mcp\n", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// crawl is run by the index workflow (IDX-1, IDX-9).
func (a *app) crawl(args []string) error {
	fs := flag.NewFlagSet("crawl", flag.ContinueOnError)
	names := fs.String("names", "registry/names.json", "")
	out := fs.String("out", "_site", "")
	prevURL := fs.String("prev", index.DefaultURL, "")
	docs := fs.String("docs", "docs", "")
	buildsPath := fs.String("builds", "registry/builds.json", "")
	tiersPath := fs.String("tiers", "registry/tiers.json", "")
	statsURL := fs.String("stats", "", "")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	reg, err := registry.Load(*names)
	if err != nil {
		return err
	}
	if probs := reg.Validate(); len(probs) > 0 {
		return errors.New("names.json: " + strings.Join(probs, "; "))
	}
	var prev *index.Index
	if *prevURL != "" {
		if p, err := index.Load(*prevURL); err == nil {
			prev = p
		} else {
			fmt.Fprintf(a.stderr, "no previous index (%v); reading everything\n", err)
		}
	}
	gh := ghapi.New(os.Getenv("GITHUB_TOKEN"))
	ix, st := crawl.Run(gh, reg, prev, time.Now())
	tiers, err := index.ReadTiers(*tiersPath)
	if err != nil {
		return err
	}
	recs, err := index.ReadBuilds(*buildsPath)
	if err != nil {
		return err
	}
	if n := index.ApplyBuilds(ix, tiers, recs); n > 0 {
		fmt.Fprintf(a.stderr, "warning: %d build record(s) match no indexed version or tier; dropped\n", n)
	}
	for _, e := range st.Errors {
		fmt.Fprintln(a.stderr, "warning:", e)
	}
	fmt.Fprintf(a.stderr, "specs %d, unclaimed %d, read %d, reused %d, skipped %d, API calls %d, rate limited %v\n",
		len(ix.Specs), len(ix.Unclaimed), st.Read, st.Reused, st.Skipped, gh.Calls, st.RateLimited)
	// Counters are optional: a missing or down stats server never fails the build (MCP-9).
	var rows []stats.Row
	if *statsURL != "" {
		if rows, err = stats.Fetch(*statsURL); err != nil {
			fmt.Fprintf(a.stderr, "warning: no usage counters (%v)\n", err)
			rows = nil
		}
	}
	return site.Generate(ix, *out, site.Options{Fetcher: source.GitHub{}, DocsDir: *docs, Stats: rows, Warn: func(s string) { fmt.Fprintln(a.stderr, "warning:", s) }})
}

// claimCheck is run by the claim workflow on a registry PR (REG-2).
func (a *app) claimCheck(args []string) error {
	fs := flag.NewFlagSet("claim-check", flag.ContinueOnError)
	repo := fs.String("repo", os.Getenv("GITHUB_REPOSITORY"), "")
	pr := fs.Int("pr", 0, "")
	base := fs.String("base", "registry/names.json", "")
	merge := fs.Bool("merge", false, "")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if *pr == 0 || *repo == "" {
		return usageError{"claim-check needs --pr and --repo"}
	}
	gh := ghapi.New(os.Getenv("GITHUB_TOKEN"))
	p, err := gh.PullRequest(*repo, *pr)
	if err != nil {
		return err
	}
	files, err := gh.PRFiles(*repo, *pr)
	if err != nil {
		return err
	}
	baseNames, err := registry.Load(*base)
	if err != nil {
		return err
	}
	var problems []string
	for _, f := range files {
		if f != "registry/names.json" {
			problems = append(problems, "a claim PR may only change registry/names.json, not "+f)
		}
	}
	verdict := registry.Verdict{}
	if len(problems) == 0 {
		raw, err := gh.Raw(p.Head.Repo.FullName, p.Head.SHA, "registry/names.json")
		if err != nil {
			return err
		}
		headNames, err := registry.Parse(raw)
		if err != nil {
			problems = append(problems, err.Error())
		} else {
			verdict = registry.CheckClaim(baseNames, headNames, p.User.Login, gh)
			problems = append(problems, verdict.Problems...)
		}
	}
	if len(problems) == 0 && verdict.OK() {
		fmt.Fprintf(a.stdout, "claim ok: %s\n", strings.Join(verdict.Added, ", "))
		if *merge {
			if err := gh.Merge(*repo, *pr, "chore(registry): claim "+strings.Join(verdict.Added, ", ")); err != nil {
				return err
			}
		}
		return nil
	}
	body := "Specfork claim check didn't pass:\n\n- " + strings.Join(problems, "\n- ")
	if verdict.Manual {
		body += "\n\nA maintainer needs to review this PR."
	}
	body += "\n\nSee https://github.com/" + site.Repo + "#claim-a-name"
	fmt.Fprintln(a.stdout, body)
	if *merge {
		if err := gh.Comment(*repo, *pr, body); err != nil {
			return err
		}
	}
	return failure{}
}
