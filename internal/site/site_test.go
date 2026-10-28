package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/specfork/specfork/internal/index"
	"github.com/specfork/specfork/internal/stats"
	"github.com/specfork/specfork/internal/testfx"
)

func build(t *testing.T, mutate func(*index.Index)) (string, func(string) string) {
	t.Helper()
	ix, fetch := testfx.Seeds(t)
	for i := range ix.Specs {
		switch ix.Specs[i].Name {
		case "@specfork/auth", "@specfork/teams", "@specfork/webhooks":
			ix.Specs[i].Tags = []string{"base"}
		case "@specfork/scheduling":
			ix.Specs[i].Tags = []string{"saas", "calendar"}
			ix.Specs[i].Implementations = []index.Impl{{Repo: "someone/scheduling-go", Stack: []string{"go"}, Version: "0.1.0", URL: "https://example.com"}}
			ix.Specs[i].PushedAt = "2026-10-20T10:00:00Z"
		}
	}
	if mutate != nil {
		mutate(ix)
	}
	dir := t.TempDir()
	if err := Generate(ix, dir, Options{Fetcher: fetch, DocsDir: "../../docs", Install: "../../install.sh"}); err != nil {
		t.Fatal(err)
	}
	return dir, func(p string) string {
		b, err := os.ReadFile(filepath.Join(dir, p))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
}

func must(t *testing.T, page, label string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(page, w) {
			t.Errorf("%s missing %q", label, w)
		}
	}
}

// WEB-1, WEB-2, WEB-11, WEB-23, WEB-24
func TestSitePages(t *testing.T) {
	_, read := build(t, nil)
	for _, p := range []string{"index.html", "specfork/scheduling/index.html", "specfork/auth/index.html", "docs/index.html", "docs/publish/index.html", "docs/claim/index.html", "docs/feedback/index.html", "404.html", "index.json", "sitemap.xml", "robots.txt", "llms.txt", "og.png", "style.css", "app.js", ".nojekyll"} {
		read(p)
	}
	must(t, read("index.html"), "home", "Recipes for software", "Pick a recipe", "Fork and adjust", "Build with your agent", `href="/docs/"`, `href="/docs/publish/"`, `href="#specs"`)
	docs := read("docs/index.html")
	must(t, docs, "docs", "<h1", "Use Specfork with your agent", `href="/docs/publish/"`, `aria-current="page"`, `href="/#specs"`)
	if strings.Contains(docs, "blob/main/docs/publish.md") {
		t.Error("doc links not rewritten to site paths")
	}

	// WEB-24: npx first, then the other ways to install.
	i := strings.Index(docs, "npx specfork")
	if i < 0 || i > strings.Index(docs, "go install") {
		t.Error("docs do not lead with npx specfork")
	}
	must(t, docs, "docs", "npm i -g specfork", "brew install specfork/tap/specfork", "curl -fsSL https://specfork.dev/install.sh | sh", "go install github.com/specfork/specfork/cmd/specfork@latest", "claude mcp add specfork -- npx -y specfork mcp")
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	if i := strings.Index(string(readme), "npx specfork"); i < 0 || i > strings.Index(string(readme), "go install") {
		t.Error("README does not lead with npx specfork")
	}

	// WEB-23: the install script is served byte for byte.
	want, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	if read("install.sh") != string(want) || !strings.HasPrefix(read("install.sh"), "#!/bin/sh\n") {
		t.Error("install.sh differs from the file in the repo")
	}
}

// WEB-3, WEB-4
func TestSiteCatalogue(t *testing.T) {
	_, read := build(t, nil)
	home := read("index.html")
	must(t, home, "home", `data-tag="base"`, `base <span>3</span>`, `href="/specfork/scheduling/"`, "requirements</span>", `<span class="tag">calendar</span>`, `id="q"`)
	// Area titles are searchable.
	if !regexp.MustCompile(`data-search="[^"]*@specfork/scheduling[^"]*slots`).MatchString(home) {
		t.Error("area titles not in search data")
	}
	// Filtered-out cards must really disappear: card styles set display, which beats the hidden attribute.
	if !strings.Contains(read("style.css"), "[hidden]{display:none !important}") {
		t.Error("hidden cards stay visible")
	}
	if !regexp.MustCompile(`href="/style\.css\?v=[0-9a-f]{8}"`).MatchString(home) || !regexp.MustCompile(`src="/app\.js\?v=[0-9a-f]{8}"`).MatchString(home) {
		t.Error("assets not fingerprinted")
	}
	// Base specs come after the others.
	if strings.Index(home, `href="/specfork/auth/"`) < strings.Index(home, `href="/specfork/scheduling/"`) {
		t.Error("base specs should be listed last")
	}
}

// WEB-5, WEB-9, WEB-10
func TestSpecPage(t *testing.T) {
	_, read := build(t, nil)
	p := read("specfork/scheduling/index.html")
	must(t, p, "scheduling", "npx specfork resolve @specfork/scheduling", "claude mcp add specfork -- npx -y specfork mcp", `class="copy"`,
		`href="/specfork/auth/"`, `href="/specfork/teams/"`, `href="/specfork/webhooks/"`, "requirements in total",
		"someone/scheduling-go", "https://example.com", "github.com/calcom/cal.com", "0.1.0 · ")
	must(t, read("specfork/auth/index.html"), "auth", "Required by", `href="/specfork/scheduling/"`)
}

// IDX-12: the spec page and the home card say what builds it.
func TestSiteBuildsWith(t *testing.T) {
	_, read := build(t, nil)
	must(t, read("specfork/scheduling/index.html"), "spec page", "not yet measured")
	must(t, read("index.html"), "home", "not yet measured")
	_, read = build(t, func(ix *index.Index) {
		for i := range ix.Specs {
			if ix.Specs[i].Name == "@specfork/scheduling" {
				v := &ix.Specs[i].Versions[0]
				index.ApplyBuilds(ix, []string{"haiku"}, []index.BuildRecord{{Spec: ix.Specs[i].Name, VersionSHA: v.SHA, Tier: "haiku", ChecksPassed: 3, ChecksTotal: 3, CostUSD: 0.5}})
			}
		}
	})
	must(t, read("specfork/scheduling/index.html"), "spec page", "builds with: <strong>haiku</strong>")
	must(t, read("index.html"), "home", "builds with haiku")
}

// WEB-6, WEB-7
func TestSpecContent(t *testing.T) {
	_, read := build(t, nil)
	p := read("specfork/scheduling/index.html")
	must(t, p, "content",
		`id="area-slots"`, `<tr id="SLT-4">`, `href="#area-slots"`,
		`href="/specfork/teams/#TEAM-10"`,
		"From <a href=\"https://github.com/specfork/scheduling/tree/v0.1.0-sha\">")
	if strings.Contains(p, "../../teams") {
		t.Error("cross-repo relative link leaked")
	}
	_, read2 := build(t, func(ix *index.Index) {
		for i := range ix.Specs {
			if ix.Specs[i].Name == "@specfork/scheduling" {
				ix.Specs[i].License = "LicenseRef-Proprietary"
			}
		}
	})
	closed := read2("specfork/scheduling/index.html")
	if strings.Contains(closed, `<tr id="SLT-4">`) || !strings.Contains(closed, "The text of this spec is on") {
		t.Error("non-open licence must not show content")
	}
}

// WEB-8
func TestRenderSafety(t *testing.T) {
	l := &linker{file: "specs/a.md", ids: map[string]string{}, areaAnchor: map[string]string{}, pageOf: func(string) string { return "" }, blob: "https://github.com/o/r/blob/sha/"}
	html, err := l.render([]byte("<script>alert(1)</script>\n\n[x](javascript:alert(1)) [y](https://example.com) [z](vbscript:msgbox) ![i](data:text/html,hi)\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"<script>", "javascript:", "vbscript:", "data:text/html"} {
		if strings.Contains(html, bad) {
			t.Errorf("unsafe %q rendered: %s", bad, html)
		}
	}
	if !strings.Contains(html, `rel="nofollow ugc noopener"`) {
		t.Errorf("external link without rel: %s", html)
	}
}

// WEB-12, WEB-13, WEB-14, WEB-15
func TestSEO(t *testing.T) {
	dir, read := build(t, nil)
	titles := map[string]string{}
	re := regexp.MustCompile(`<title>([^<]+)</title>`)
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if strings.HasSuffix(p, ".html") {
			b, _ := os.ReadFile(p)
			m := re.FindSubmatch(b)
			if m == nil {
				t.Errorf("%s has no title", p)
				return nil
			}
			if other, dup := titles[string(m[1])]; dup {
				t.Errorf("duplicate title %q in %s and %s", m[1], p, other)
			}
			titles[string(m[1])] = p
			for _, w := range []string{`<meta name="description"`, `<link rel="canonical" href="https://specfork.dev/`, `og:image" content="https://specfork.dev/og.png"`, `twitter:card`} {
				if !strings.Contains(string(b), w) {
					t.Errorf("%s missing %s", p, w)
				}
			}
		}
		return nil
	})
	ld := regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	var home map[string]any
	if m := ld.FindStringSubmatch(read("index.html")); m == nil || json.Unmarshal([]byte(m[1]), &home) != nil || home["@type"] != "WebSite" || home["potentialAction"] == nil {
		t.Errorf("home JSON-LD: %v", home)
	}
	var sp map[string]any
	if m := ld.FindStringSubmatch(read("specfork/scheduling/index.html")); m == nil || json.Unmarshal([]byte(m[1]), &sp) != nil || sp["@type"] != "CreativeWork" || sp["version"] != "0.1.0" || sp["isBasedOn"] == nil {
		t.Errorf("spec JSON-LD: %v", sp)
	}
	must(t, read("sitemap.xml"), "sitemap", "<loc>https://specfork.dev/specfork/scheduling/</loc><lastmod>2026-10-20</lastmod>", "<loc>https://specfork.dev/docs/claim/</loc>")
	must(t, read("robots.txt"), "robots", "Sitemap: https://specfork.dev/sitemap.xml")
	must(t, read("llms.txt"), "llms", "# Specfork", "npx specfork resolve", "npm i -g specfork", "brew install specfork/tap/specfork", "curl -fsSL https://specfork.dev/install.sh | sh", "go install github.com/specfork/specfork/cmd/specfork@latest", "claude mcp add specfork -- npx -y specfork mcp", "[@specfork/scheduling](https://specfork.dev/specfork/scheduling/)")
}

// WEB-17
func TestFeedbackLinks(t *testing.T) {
	_, read := build(t, nil)
	home := read("index.html")
	must(t, home, "home", `href="/docs/feedback/">Feedback</a>`, "issues/new?template=spec-request.yml")
	must(t, read("specfork/scheduling/index.html"), "spec", "issues/new?template=spec-problem.yml&amp;spec=%40specfork%2fscheduling")
	must(t, read("docs/feedback/index.html"), "feedback doc", "template=feedback.yml", "template=spec-request.yml", "template=spec-problem.yml", "/discussions", `href="/docs/publish/"`)
	for _, f := range []string{"config.yml", "feedback.yml", "spec-request.yml", "spec-problem.yml"} {
		if _, err := os.Stat(filepath.Join("../../.github/ISSUE_TEMPLATE", f)); err != nil {
			t.Errorf("issue form %s: %v", f, err)
		}
	}
}

// MCP-9
func TestSiteCounters(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	gen := func(rows []stats.Row) func(string) string {
		ix, fetch := testfx.Seeds(t)
		dir := t.TempDir()
		if err := Generate(ix, dir, Options{Fetcher: fetch, DocsDir: "../../docs", Install: "../../install.sh", Stats: rows, Now: now}); err != nil {
			t.Fatal(err)
		}
		return func(p string) string {
			b, err := os.ReadFile(filepath.Join(dir, p))
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		}
	}
	read := gen([]stats.Row{
		{Day: "2026-10-20", Tool: "resolve_spec", Spec: "@specfork/scheduling", Count: 4},
		{Day: "2026-08-10", Tool: "resolve_spec", Spec: "@specfork/scheduling", Count: 6},
	})
	must(t, read("specfork/scheduling/index.html"), "spec page", "<strong>4</strong> resolves in 30 days, <strong>10</strong> in 90")
	must(t, read("index.html"), "home", "4 resolves in 30 days, 10 in 90")
	if strings.Contains(read("specfork/auth/index.html"), "resolves in") {
		t.Error("spec without resolves shows counters")
	}
	home := read("index.html")
	if strings.Count(home, "resolves in 30 days") != 1 {
		t.Error("home: only the resolved spec should show counters")
	}
	none := gen(nil)
	for _, p := range []string{"specfork/scheduling/index.html", "index.html"} {
		if s := none(p); strings.Contains(s, "resolves in") {
			t.Errorf("%s shows counters without data", p)
		}
	}
}
