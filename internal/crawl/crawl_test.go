package crawl

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/specfork/specfork/internal/ghapi"
	"github.com/specfork/specfork/internal/index"
	"github.com/specfork/specfork/internal/registry"
)

type fake struct {
	repos    map[int64]ghapi.Repo
	search   map[string][]int64
	heads    map[string]string
	tags     map[string][]ghapi.Tag
	files    map[string]string // full@sha/path → content
	trees    map[string][]string
	calls    map[string]int
	budget   int // API calls before ErrRateLimited; 0 = unlimited
	apiCalls int
}

func newFake() *fake {
	return &fake{repos: map[int64]ghapi.Repo{}, search: map[string][]int64{}, heads: map[string]string{}, tags: map[string][]ghapi.Tag{},
		files: map[string]string{}, trees: map[string][]string{}, calls: map[string]int{}}
}

func (f *fake) api(kind string) error {
	f.calls[kind]++
	f.apiCalls++
	if f.budget > 0 && f.apiCalls > f.budget {
		return ghapi.ErrRateLimited
	}
	return nil
}

func (f *fake) SearchRepos(q string) ([]ghapi.Repo, error) {
	if err := f.api("search"); err != nil {
		return nil, err
	}
	var out []ghapi.Repo
	for _, id := range f.search[q] {
		out = append(out, f.repos[id])
	}
	return out, nil
}
func (f *fake) RepoByID(id int64) (ghapi.Repo, error) {
	if err := f.api("repo"); err != nil {
		return ghapi.Repo{}, err
	}
	r, ok := f.repos[id]
	if !ok {
		return r, ghapi.ErrNotFound
	}
	return r, nil
}
func (f *fake) BranchSHA(full, branch string) (string, error) {
	if err := f.api("branch"); err != nil {
		return "", err
	}
	return f.heads[full], nil
}
func (f *fake) Tags(full string) ([]ghapi.Tag, error) {
	if err := f.api("tags"); err != nil {
		return nil, err
	}
	return f.tags[full], nil
}
func (f *fake) TreeFiles(full, sha string) ([]string, error) {
	if err := f.api("tree"); err != nil {
		return nil, err
	}
	return f.trees[full+"@"+sha], nil
}
func (f *fake) Raw(full, sha, p string) ([]byte, error) {
	f.calls["raw"]++
	s, ok := f.files[full+"@"+sha+"/"+p]
	if !ok {
		return nil, ghapi.ErrNotFound
	}
	return []byte(s), nil
}

// addSpec adds a spec repo with a head commit and optional tags (all with the same front matter).
func (f *fake) addSpec(id, owner int64, full, name, requires string, tags ...string) {
	login, _, _ := strings.Cut(full, "/")
	f.repos[id] = ghapi.Repo{ID: id, FullName: full, Owner: ghapi.Owner{ID: owner, Login: login, Type: "User"}, Topics: []string{"specfork"}, DefaultBranch: "main", Stars: int(id)}
	f.search[SpecQuery] = append(f.search[SpecQuery], id)
	readme := fmt.Sprintf("---\nspecfork: 1\nname: %q\nsummary: s %s\nlicense: CC-BY-4.0\n%s---\n", name, name, requires)
	shas := append([]string{"head-" + full}, tags...)
	f.heads[full] = "head-" + full
	for _, sha := range shas {
		f.files[full+"@"+sha+"/README.md"] = readme
		f.trees[full+"@"+sha] = []string{"README.md", "specs/a.md", "specs/_template.md", "specs/README.md", "other/x.md"}
	}
	for _, t := range tags {
		f.tags[full] = append(f.tags[full], ghapi.Tag{Name: t, SHA: t + "-sha"})
		f.files[full+"@"+t+"-sha/README.md"] = readme
		f.trees[full+"@"+t+"-sha"] = []string{"README.md", "specs/a.md"}
	}
}

func names(m map[string]registry.Entry) *registry.Names { return &registry.Names{Names: m} }

func active(id, owner int64, repo string) registry.Entry {
	return registry.Entry{RepoID: id, OwnerID: owner, Repo: repo, ClaimedAt: "2026-10-22", State: "active"}
}

var now = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

// IDX-1: search results plus every registered repo, even without the topic.
func TestCrawlSources(t *testing.T) {
	f := newFake()
	f.addSpec(1, 100, "ann/a", "@ann/a", "")
	f.addSpec(2, 200, "bob/b", "@bob/b", "")
	f.search[SpecQuery] = []int64{1} // bob/b lost its topic
	ix, _ := Run(f, names(map[string]registry.Entry{"@ann/a": active(1, 100, "ann/a"), "@bob/b": active(2, 200, "bob/b")}), nil, now)
	if len(ix.Specs) != 2 || ix.Specs[1].Name != "@bob/b" {
		t.Fatalf("specs %+v", ix.Specs)
	}
}

// IDX-3, IDX-6
func TestFieldsRead(t *testing.T) {
	f := newFake()
	f.addSpec(1, 100, "ann/a", "@ann/a", "requires:\n  \"@ann/b\": \"^0.1\"\n", "v0.1.0", "v0.2.0", "not-a-version", "v1.0")
	ix, st := Run(f, names(map[string]registry.Entry{"@ann/a": active(1, 100, "ann/a")}), nil, now)
	s := ix.Specs[0]
	if s.HeadSHA != "head-ann/a" || s.OwnerID != 100 || s.Stars != 1 || s.Summary != "s @ann/a" || s.License != "CC-BY-4.0" {
		t.Fatalf("spec %+v", s)
	}
	if len(s.Versions) != 2 || s.Versions[0].Version != "0.2.0" || s.Versions[0].SHA != "v0.2.0-sha" {
		t.Fatalf("versions %+v", s.Versions)
	}
	if strings.Join(s.Head.Files, ",") != "specs/a.md" || s.Head.Requires["@ann/b"] != "^0.1" {
		t.Fatalf("head %+v", s.Head)
	}
	if st.Read != 1 {
		t.Fatalf("stats %+v", st)
	}
}

func TestIndexSchema(t *testing.T) {
	f := newFake()
	f.addSpec(1, 100, "ann/a", "@ann/a", "", "v0.1.0")
	ix, _ := Run(f, names(map[string]registry.Entry{"@ann/a": active(1, 100, "ann/a")}), nil, now)
	b, _ := json.Marshal(ix)
	var raw map[string]any
	json.Unmarshal(b, &raw)
	spec := raw["specs"].([]any)[0].(map[string]any)
	for _, k := range []string{"name", "repo", "repo_id", "owner_id", "state", "summary", "license", "stars", "pushed_at", "head_sha", "head", "versions", "implementations"} {
		if _, ok := spec[k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
	v := spec["versions"].([]any)[0].(map[string]any)
	for _, k := range []string{"version", "sha", "files", "builds_with", "builds"} {
		if _, ok := v[k]; !ok {
			t.Errorf("version missing %s", k)
		}
	}
	for _, k := range []string{"score", "scores", "average", "resolves", "count"} { // IDX-14
		if _, ok := spec[k]; ok {
			t.Errorf("index.json carries %s", k)
		}
	}
	if raw["generated_at"] != "2026-10-22T12:00:00Z" || raw["unclaimed"] == nil {
		t.Errorf("top level %v", raw)
	}
}

// IDX-4
func TestUnclaimed(t *testing.T) {
	f := newFake()
	f.addSpec(1, 100, "ann/a", "@ann/a", "")
	f.addSpec(2, 200, "bob/b", "@bob/b", "")
	f.files["eve/notspec@head-eve/notspec/README.md"] = "# no front matter"
	f.repos[3] = ghapi.Repo{ID: 3, FullName: "eve/notspec", DefaultBranch: "main"}
	f.search[SpecQuery] = append(f.search[SpecQuery], 3)
	f.heads["eve/notspec"] = "head-eve/notspec"
	ix, _ := Run(f, names(map[string]registry.Entry{"@ann/a": active(1, 100, "ann/a")}), nil, now)
	if len(ix.Specs) != 1 || strings.Join(ix.Unclaimed, ",") != "bob/b" {
		t.Fatalf("specs %d unclaimed %v", len(ix.Specs), ix.Unclaimed)
	}
}

// IDX-5
func TestSkipUnchanged(t *testing.T) {
	f := newFake()
	f.addSpec(1, 100, "ann/a", "@ann/a", "", "v0.1.0")
	reg := names(map[string]registry.Entry{"@ann/a": active(1, 100, "ann/a")})
	first, _ := Run(f, reg, nil, now)
	f.calls = map[string]int{}
	f.repos[1] = func() ghapi.Repo { r := f.repos[1]; r.Stars = 42; return r }()
	second, st := Run(f, reg, first, now)
	if f.calls["tree"] != 0 || f.calls["raw"] != 0 || st.Reused != 1 {
		t.Fatalf("re-read an unchanged spec: %v %+v", f.calls, st)
	}
	if second.Specs[0].Stars != 42 {
		t.Fatal("stars not refreshed")
	}
	f.heads["ann/a"] = "head-2"
	f.files["ann/a@head-2/README.md"] = f.files["ann/a@head-ann/a/README.md"]
	f.trees["ann/a@head-2"] = []string{"specs/a.md", "specs/b.md"}
	third, _ := Run(f, reg, second, now)
	if third.Specs[0].HeadSHA != "head-2" || len(third.Specs[0].Head.Files) != 2 {
		t.Fatalf("changed head not re-read: %+v", third.Specs[0])
	}
}

// IDX-7
func TestImplAttach(t *testing.T) {
	f := newFake()
	f.addSpec(1, 100, "ann/a", "@ann/a", "", "v0.1.0")
	for id, full := range map[int64]string{7: "zed/go-a", 8: "zed/other"} {
		f.repos[id] = ghapi.Repo{ID: id, FullName: full, DefaultBranch: "main"}
		f.search[ImplQuery] = append(f.search[ImplQuery], id)
		f.heads[full] = "h"
	}
	f.files["zed/go-a@h/README.md"] = "---\nimplements: \"@ann/a@0.1.0\"\nstack: [go]\nurl: https://a.example\n---\n"
	f.files["zed/other@h/README.md"] = "---\nimplements: \"@nobody/x@1.0.0\"\n---\n"
	ix, _ := Run(f, names(map[string]registry.Entry{"@ann/a": active(1, 100, "ann/a")}), nil, now)
	im := ix.Specs[0].Implementations
	if len(im) != 1 || im[0].Repo != "zed/go-a" || im[0].Version != "0.1.0" || im[0].URL != "https://a.example" || im[0].Stack[0] != "go" {
		t.Fatalf("impls %+v", im)
	}
}

// IDX-8: once the budget is spent, unread specs keep their previous entry.
func TestRateLimitStop(t *testing.T) {
	f := newFake()
	f.addSpec(1, 100, "ann/a", "@ann/a", "")
	f.addSpec(2, 200, "bob/b", "@bob/b", "")
	reg := names(map[string]registry.Entry{"@ann/a": active(1, 100, "ann/a"), "@bob/b": active(2, 200, "bob/b")})
	first, _ := Run(f, reg, nil, now)
	f.heads["bob/b"] = "changed"
	f.apiCalls = 0
	f.budget = 4 // two lookups, one search, one branch read
	ix, st := Run(f, reg, first, now)
	if !st.RateLimited || len(ix.Specs) != 2 || ix.Specs[1].HeadSHA != "head-bob/b" {
		t.Fatalf("stats %+v specs %+v", st, ix.Specs)
	}
}

// REG-5
func TestFreezeOnIDChange(t *testing.T) {
	f := newFake()
	f.addSpec(1, 999, "ann/a", "@ann/a", "") // transferred: owner id changed
	reg := names(map[string]registry.Entry{"@ann/a": active(1, 100, "ann/a"), "@gone/x": active(5, 500, "gone/x")})
	ix, _ := Run(f, reg, nil, now)
	states := map[string]string{}
	for _, s := range ix.Specs {
		states[s.Name] = s.State
	}
	if states["@ann/a"] != "frozen" || states["@gone/x"] != "frozen" {
		t.Fatalf("states %v", states)
	}
	sp, _ := (&index.Index{Specs: ix.Specs}).Find("@ann/a")
	if sp.Resolvable() {
		t.Fatal("frozen spec resolvable")
	}
}

// REG-6
func TestRenameFollows(t *testing.T) {
	f := newFake()
	f.addSpec(1, 100, "ann/renamed", "@ann/a", "")
	ix, _ := Run(f, names(map[string]registry.Entry{"@ann/a": active(1, 100, "ann/a")}), nil, now)
	if ix.Specs[0].Repo != "ann/renamed" || ix.Specs[0].State != "active" {
		t.Fatalf("%+v", ix.Specs[0])
	}
}

// IDX-3: tags from before a rename are ignored.
func TestOldNameTagsIgnored(t *testing.T) {
	f := newFake()
	f.addSpec(1, 100, "ann/a", "@ann/a", "", "v0.1.0", "v0.2.0")
	f.files["ann/a@v0.1.0-sha/README.md"] = "---\nspechub: 1\nname: \"@old/a\"\n---\n"
	ix, _ := Run(f, names(map[string]registry.Entry{"@ann/a": active(1, 100, "ann/a")}), nil, now)
	if v := ix.Specs[0].Versions; len(v) != 1 || v[0].Version != "0.2.0" {
		t.Fatalf("versions %+v", v)
	}
}
