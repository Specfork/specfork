// Package site renders https://specfork.dev from index.json and the specs at their
// indexed commits (WEB).
package site

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/specfork/specfork/internal/index"
	"github.com/specfork/specfork/internal/resolve"
	"github.com/specfork/specfork/internal/scores"
	"github.com/specfork/specfork/internal/source"
	"github.com/specfork/specfork/internal/spec"
	"github.com/specfork/specfork/internal/stats"
)

//go:embed templates/*.html templates/style.css templates/app.js templates/og.png
var files embed.FS

// Repo is where Specfork's own code lives.
const Repo = "specfork/specfork"

// BaseURL is the canonical site origin (WEB-12).
const BaseURL = "https://specfork.dev"

// APIURL is the only host the site's script talks to (WEB-21).
const APIURL = "https://api.specfork.dev"

// openLicenses may be shown on the site (WEB-7).
var openLicenses = map[string]bool{
	"CC-BY-4.0": true, "CC-BY-SA-4.0": true, "CC0-1.0": true, "MIT": true, "Apache-2.0": true,
	"BSD-2-Clause": true, "BSD-3-Clause": true, "ISC": true, "MPL-2.0": true, "Unlicense": true,
}

// Options configure a build.
type Options struct {
	Fetcher source.Fetcher // spec files at a commit; nil skips content
	DocsDir string         // Markdown docs (WEB-11); "" skips docs pages
	Install string         // install.sh, served as is (WEB-23); "" skips it
	Warn    func(string)
	Scores  []scores.Score // from the API's /scores.json (WEB-18); nil shows no scores
	Stats   []stats.Row    // counters from the hosted server's /stats.json (MCP-9); nil shows no numbers
	Now     time.Time      // zero means time.Now
}

var tmpl = template.Must(template.New("").Funcs(template.FuncMap{
	"short": func(s string) string {
		if len(s) > 7 {
			return s[:7]
		}
		return s
	},
	"page":  pagePath,
	"asset": assetURL,
	"join":  strings.Join,
	"base":  filepath.Base,
	"safe":  func(s string) template.HTML { return template.HTML(s) },
	"json": func(v any) template.JS {
		b, _ := json.Marshal(v)
		return template.JS(b)
	},
}).ParseFS(files, "templates/*.html"))

// assetURL adds a content fingerprint so browsers fetch new CSS and JS after a deploy.
func assetURL(name string) string {
	data, err := files.ReadFile("templates/" + name)
	if err != nil {
		return "/" + name
	}
	sum := sha256.Sum256(data)
	return "/" + name + "?v=" + hex.EncodeToString(sum[:4])
}

// pagePath is a spec's site path: "@specfork/scheduling" → "/specfork/scheduling/" (WEB-1).
func pagePath(name string) string { return "/" + resolve.Dir(name) + "/" }

// Area is one rendered area section.
type Area struct {
	Anchor, Title, File, HTML string
	Requirements              int
}

// Content is a spec's rendered text at one commit.
type Content struct {
	Shown        bool
	ReadmeHTML   string
	Areas        []Area
	Requirements int
	Version, SHA string
	SearchText   string
}

// BundleItem is one spec in a resolved bundle (WEB-9).
type BundleItem struct {
	Name, Version string
	Requirements  int
}

type card struct {
	index.Spec
	Requirements int
	Search       string
	Base         bool
	Resolves     *stats.Resolves
	Score        *scores.Score
}

type head struct {
	Title, Description, Canonical string
	JSONLD                        any
}

type homePage struct {
	Head  head
	Cards []card
	Tags  []tagCount
	Index *index.Index
	Repo  string
	Total int
}

type tagCount struct {
	Tag   string
	Count int
}

type specPage struct {
	Head       head
	Spec       index.Spec
	Content    Content
	Bundle     []BundleItem
	BundleSum  int
	BundleErr  string
	RequiredBy []string
	Repo       string
	Resolves   *stats.Resolves
	Score      *scores.Score
	API        string
}

type docPage struct {
	Head  head
	Title string
	HTML  template.HTML
	Repo  string
	Docs  []docLink
	Path  string
	API   string
	Claim bool // the page has the one-click claim box (WEB-20)
}

type docLink struct{ Path, Title string }

var docOrder = []struct{ file, path string }{{"index.md", "/docs/"}, {"publish.md", "/docs/publish/"}, {"claim.md", "/docs/claim/"}, {"feedback.md", "/docs/feedback/"}, {"privacy.md", "/docs/privacy/"}}

// builder carries state for one build.
type builder struct {
	ix      *index.Index
	opt     Options
	dir     string
	content map[string]Content // by repo@sha
	known   map[string]bool
}

// Generate writes the site into dir.
func Generate(ix *index.Index, dir string, opt Options) error {
	if opt.Warn == nil {
		opt.Warn = func(string) {}
	}
	b := &builder{ix: ix, opt: opt, dir: dir, content: map[string]Content{}, known: map[string]bool{}}
	for _, s := range ix.Specs {
		b.known[s.Name] = true
	}
	now := opt.Now
	if now.IsZero() {
		now = time.Now()
	}
	resolves := stats.ResolvesBySpec(opt.Stats, now)
	resolvesOf := func(name string) *stats.Resolves {
		if r, ok := resolves[name]; ok {
			return &r
		}
		return nil
	}
	scoreBy := scores.BySpec(opt.Scores)
	scoreOf := func(name string) *scores.Score {
		if sc, ok := scoreBy[name]; ok {
			return &sc
		}
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := b.static(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ix, "", "  ")
	if err != nil {
		return err
	}
	if err := b.write("index.json", append(data, '\n')); err != nil {
		return err
	}

	cards := make([]card, 0, len(ix.Specs))
	total := 0
	for _, s := range ix.Specs {
		c := b.latest(s)
		p := specPage{Spec: s, Content: c, RequiredBy: ix.RequiredBy(s.Name), Repo: Repo, Resolves: resolvesOf(s.Name), Score: scoreOf(s.Name), API: APIURL}
		p.Bundle, p.BundleSum, p.BundleErr = b.bundle(s)
		p.Head = head{
			Title:       specTitle(s),
			Description: clip(s.Summary, 160),
			Canonical:   BaseURL + pagePath(s.Name),
			JSONLD:      specJSONLD(s, c),
		}
		if err := b.render(path.Join(resolve.Dir(s.Name), "index.html"), "spec.html", p); err != nil {
			return err
		}
		total += c.Requirements
		cards = append(cards, card{
			Spec: s, Requirements: c.Requirements, Base: hasTag(s.Tags, "base"), Resolves: resolvesOf(s.Name), Score: scoreOf(s.Name),
			Search: strings.ToLower(strings.Join([]string{s.Name, s.Summary, strings.Join(s.Tags, " "), c.SearchText}, " ")),
		})
	}
	sort.SliceStable(cards, func(i, j int) bool {
		if cards[i].Base != cards[j].Base {
			return !cards[i].Base
		}
		return cards[i].Name < cards[j].Name
	})
	home := homePage{
		Head: head{
			Title:       "Specfork: recipes for software",
			Description: "Stack-free app specs your coding agent can build from. Fork a recipe, change what you need, and build it with Claude Code or any MCP-capable agent.",
			Canonical:   BaseURL + "/",
			JSONLD: map[string]any{
				"@context": "https://schema.org", "@type": "WebSite", "name": "Specfork", "url": BaseURL + "/",
				"description": "Recipes for software: stack-free app specs agents can find, compose and build from.",
				"potentialAction": map[string]any{
					"@type": "SearchAction", "target": BaseURL + "/?q={search_term_string}",
					"query-input": "required name=search_term_string",
				},
			},
		},
		Cards: cards, Tags: tagCounts(ix.Specs), Index: ix, Repo: Repo, Total: total,
	}
	if err := b.render("index.html", "home.html", home); err != nil {
		return err
	}
	docs, err := b.docs()
	if err != nil {
		return err
	}
	nf := docPage{Head: head{Title: "Not found · Specfork", Description: "This page doesn't exist.", Canonical: BaseURL + "/404.html"}, Repo: Repo}
	if err := b.render("404.html", "404.html", nf); err != nil {
		return err
	}
	return b.seo(docs)
}

func (b *builder) static() error {
	for _, f := range []string{"style.css", "app.js", "og.png"} {
		data, err := files.ReadFile("templates/" + f)
		if err != nil {
			return err
		}
		if err := b.write(f, data); err != nil {
			return err
		}
	}
	if b.opt.Install != "" {
		data, err := os.ReadFile(b.opt.Install)
		if err != nil {
			return err
		}
		if err := b.write("install.sh", data); err != nil {
			return err
		}
	}
	return b.write(".nojekyll", nil)
}

func (b *builder) write(rel string, data []byte) error {
	full := filepath.Join(b.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

func (b *builder) render(rel, name string, data any) error {
	var sb strings.Builder
	if err := tmpl.ExecuteTemplate(&sb, name, data); err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	return b.write(rel, []byte(sb.String()))
}

func (b *builder) pageOf(name string) string {
	if b.known[name] {
		return pagePath(name)
	}
	return ""
}

// latest renders a spec at its newest version, or head when untagged (WEB-6, WEB-7).
func (b *builder) latest(s index.Spec) Content {
	snap, sha, ver := s.Head, s.HeadSHA, "head"
	if len(s.Versions) > 0 {
		snap, sha, ver = s.Versions[0].Snapshot, s.Versions[0].SHA, s.Versions[0].Version
	}
	return b.contentAt(s, snap, sha, ver)
}

func (b *builder) contentAt(s index.Spec, snap index.Snapshot, sha, ver string) Content {
	key := s.Repo + "@" + sha
	if c, ok := b.content[key]; ok {
		return c
	}
	c := Content{Version: ver, SHA: sha}
	defer func() { b.content[key] = c }()
	if b.opt.Fetcher == nil || sha == "" {
		return c
	}
	readme, err := b.opt.Fetcher.File(s.Repo, sha, "README.md")
	if err != nil {
		b.opt.Warn(fmt.Sprintf("%s: README.md: %v", s.Name, err))
		return c
	}
	l := &linker{file: "README.md", ids: map[string]string{}, areaAnchor: map[string]string{}, pageOf: b.pageOf,
		blob: fmt.Sprintf("https://github.com/%s/blob/%s/", s.Repo, sha)}
	type raw struct {
		file string
		data []byte
		area spec.Area
	}
	var areas []raw
	var titles []string
	for _, f := range snap.Files {
		data, err := b.opt.Fetcher.File(s.Repo, sha, f)
		if err != nil {
			b.opt.Warn(fmt.Sprintf("%s: %s: %v", s.Name, f, err))
			continue
		}
		a := spec.ParseArea(f, data)
		areas = append(areas, raw{f, data, a})
		l.areaAnchor[f] = "area-" + strings.TrimSuffix(path.Base(f), ".md")
		titles = append(titles, a.Title)
		for _, r := range a.Rows {
			l.ids[r.ID] = f
			if !r.Removed() {
				c.Requirements++
			}
		}
	}
	c.SearchText = strings.Join(titles, " ")
	if !openLicenses[s.License] {
		return c
	}
	if html, err := l.render(l.idLinks(stripFrontMatter(readme)), 1); err == nil {
		c.ReadmeHTML = html
	}
	for _, a := range areas {
		l.file = a.file
		html, err := l.render(l.idLinks(a.data), 1)
		if err != nil {
			b.opt.Warn(fmt.Sprintf("%s: %s: %v", s.Name, a.file, err))
			continue
		}
		req := 0
		for _, r := range a.area.Rows {
			if !r.Removed() {
				req++
			}
		}
		c.Areas = append(c.Areas, Area{Anchor: l.areaAnchor[a.file], Title: a.area.Title, File: a.file, HTML: html, Requirements: req})
	}
	c.Shown = true
	return c
}

// bundle lists what `specfork resolve` would bring in for the latest version (WEB-9).
func (b *builder) bundle(s index.Spec) ([]BundleItem, int, string) {
	if !s.Resolvable() {
		return nil, 0, "This spec is " + s.State + " and can't be resolved."
	}
	sel, err := resolve.Select(b.ix, s.Name, "", nil)
	if err != nil {
		return nil, 0, err.Error()
	}
	var items []BundleItem
	total := 0
	for _, n := range sel.Order {
		c := b.contentAt(*n.Spec, n.Snapshot, n.SHA, n.Version.String())
		items = append(items, BundleItem{Name: n.Name, Version: n.Version.String(), Requirements: c.Requirements})
		total += c.Requirements
	}
	return items, total, ""
}

func (b *builder) docs() ([]docLink, error) {
	if b.opt.DocsDir == "" {
		return nil, nil
	}
	type doc struct {
		link docLink
		html string
		desc string
	}
	var all []doc
	var links []docLink
	for _, d := range docOrder {
		src, err := os.ReadFile(filepath.Join(b.opt.DocsDir, d.file))
		if err != nil {
			return nil, err
		}
		title, desc := docMeta(src)
		l := &linker{file: "docs/" + d.file, ids: map[string]string{}, areaAnchor: map[string]string{}, pageOf: b.pageOf,
			blob: "https://github.com/" + Repo + "/blob/main/"}
		html, err := l.render(src, 0)
		if err != nil {
			return nil, err
		}
		for _, o := range docOrder {
			html = strings.ReplaceAll(html, `href="https://github.com/`+Repo+`/blob/main/docs/`+o.file+`"`, `href="`+o.path+`"`)
		}
		lk := docLink{Path: d.path, Title: title}
		links = append(links, lk)
		all = append(all, doc{lk, html, desc})
	}
	for _, d := range all {
		p := docPage{
			Head:  head{Title: d.link.Title + " · Specfork", Description: clip(d.desc, 160), Canonical: BaseURL + d.link.Path},
			Title: d.link.Title, HTML: template.HTML(d.html), Repo: Repo, Docs: links, Path: d.link.Path, API: APIURL, Claim: d.link.Path == "/docs/claim/",
		}
		if err := b.render(strings.TrimPrefix(d.link.Path, "/")+"index.html", "doc.html", p); err != nil {
			return nil, err
		}
	}
	return links, nil
}

// docMeta reads the first heading and the first paragraph line of a doc.
func docMeta(src []byte) (title, desc string) {
	for _, line := range strings.Split(string(src), "\n") {
		t := strings.TrimSpace(line)
		switch {
		case title == "" && strings.HasPrefix(t, "# "):
			title = strings.TrimPrefix(t, "# ")
		case title != "" && desc == "" && t != "" && !strings.HasPrefix(t, "#"):
			desc = strings.NewReplacer("**", "", "`", "").Replace(t)
		}
	}
	return
}

// seo writes sitemap.xml, robots.txt and llms.txt (WEB-14, WEB-15).
func (b *builder) seo(docs []docLink) error {
	type entry struct{ loc, lastmod string }
	urls := []entry{{BaseURL + "/", b.ix.GeneratedAt}}
	for _, d := range docs {
		urls = append(urls, entry{BaseURL + d.Path, ""})
	}
	for _, s := range b.ix.Specs {
		urls = append(urls, entry{BaseURL + pagePath(s.Name), s.PushedAt})
	}
	var sm strings.Builder
	sm.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, u := range urls {
		sm.WriteString("  <url><loc>" + u.loc + "</loc>")
		if t, err := time.Parse(time.RFC3339, u.lastmod); err == nil {
			sm.WriteString("<lastmod>" + t.UTC().Format("2006-01-02") + "</lastmod>")
		}
		sm.WriteString("</url>\n")
	}
	sm.WriteString("</urlset>\n")
	if err := b.write("sitemap.xml", []byte(sm.String())); err != nil {
		return err
	}
	if err := b.write("robots.txt", []byte("User-agent: *\nAllow: /\n\nSitemap: "+BaseURL+"/sitemap.xml\n")); err != nil {
		return err
	}
	var lt strings.Builder
	lt.WriteString("# Specfork\n\n> Recipes for software: stack-free app specs that coding agents can find, compose and build from.\n\n")
	lt.WriteString("Each spec is a set of numbered, testable requirements with acceptance checks. Specs can require other specs (bases such as auth or teams); resolving one gives a single bundle to build from.\n\n")
	lt.WriteString("## Use\n\n- CLI: `npx specfork search <words>` and `npx specfork resolve <name>` (writes specfork-bundle/BUNDLE.md).\n")
	lt.WriteString("- Install the CLI: `npm i -g specfork`, `brew install specfork/tap/specfork`, `curl -fsSL " + BaseURL + "/install.sh | sh`, or `go install github.com/" + Repo + "/cmd/specfork@latest`.\n")
	lt.WriteString("- MCP: `claude mcp add specfork -- npx -y specfork mcp` gives agents `search_specs`, `get_spec`, `resolve_spec`.\n")
	lt.WriteString("- Catalogue as JSON: " + BaseURL + "/index.json\n")
	lt.WriteString("- Docs: " + BaseURL + "/docs/\n\n## Specs\n\n")
	for _, s := range b.ix.Specs {
		if s.Resolvable() {
			fmt.Fprintf(&lt, "- [%s](%s%s): %s\n", s.Name, BaseURL, pagePath(s.Name), s.Summary)
		}
	}
	return b.write("llms.txt", []byte(lt.String()))
}

func specTitle(s index.Spec) string {
	parts := strings.SplitN(strings.TrimPrefix(s.Name, "@"), "/", 2)
	n := strings.ReplaceAll(parts[len(parts)-1], "-", " ")
	return strings.ToUpper(n[:1]) + n[1:] + " app spec (" + s.Name + ") · Specfork"
}

func specJSONLD(s index.Spec, c Content) map[string]any {
	owner := strings.SplitN(s.Repo, "/", 2)[0]
	ld := map[string]any{
		"@context": "https://schema.org", "@type": "CreativeWork",
		"name": s.Name, "description": s.Summary, "url": BaseURL + pagePath(s.Name),
		"license":  "https://spdx.org/licenses/" + s.License + ".html",
		"author":   map[string]any{"@type": "Organization", "name": owner, "url": "https://github.com/" + owner},
		"keywords": strings.Join(s.Tags, ", "),
	}
	if c.Version != "" && c.Version != "head" {
		ld["version"] = c.Version
	}
	if d := s.DerivedFrom; d != nil && d.Repo != "" {
		ld["isBasedOn"] = d.Repo
	}
	return ld
}

func tagCounts(specs []index.Spec) []tagCount {
	m := map[string]int{}
	for _, s := range specs {
		for _, t := range s.Tags {
			m[t]++
		}
	}
	var out []tagCount
	for t, n := range m {
		out = append(out, tagCount{t, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	return out
}

func hasTag(tags []string, t string) bool {
	for _, x := range tags {
		if x == t {
			return true
		}
	}
	return false
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}
