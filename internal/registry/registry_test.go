package registry

import (
	"os"
	"strings"
	"testing"

	"github.com/specfork/specfork/internal/ghapi"
)

type fakeGH struct {
	repos  map[int64]ghapi.Repo
	readme map[string]string
	perms  map[string]string
}

func (f fakeGH) RepoByID(id int64) (ghapi.Repo, error) {
	r, ok := f.repos[id]
	if !ok {
		return r, ghapi.ErrNotFound
	}
	return r, nil
}
func (f fakeGH) BranchSHA(full, branch string) (string, error) { return "head", nil }
func (f fakeGH) Raw(full, sha, path string) ([]byte, error) {
	s, ok := f.readme[full]
	if !ok {
		return nil, ghapi.ErrNotFound
	}
	return []byte(s), nil
}
func (f fakeGH) Permission(full, user string) (string, error) { return f.perms[full+"/"+user], nil }

func gh() fakeGH {
	return fakeGH{
		repos: map[int64]ghapi.Repo{
			10: {ID: 10, FullName: "ann/booking", Owner: ghapi.Owner{ID: 1, Login: "Ann", Type: "User"}, Topics: []string{"specfork"}, DefaultBranch: "main"},
			20: {ID: 20, FullName: "acme/crm", Owner: ghapi.Owner{ID: 2, Login: "acme", Type: "Organization"}, Topics: []string{"specfork"}, DefaultBranch: "main"},
			30: {ID: 30, FullName: "ann/notopic", Owner: ghapi.Owner{ID: 1, Login: "Ann", Type: "User"}, DefaultBranch: "main"},
		},
		readme: map[string]string{
			"ann/booking": "---\nspecfork: 1\nname: \"@ann/booking\"\n---\n",
			"acme/crm":    "---\nspecfork: 1\nname: \"@acme/crm\"\n---\n",
			"ann/notopic": "---\nspecfork: 1\nname: \"@ann/other\"\n---\n",
		},
		perms: map[string]string{"acme/crm/bob": "admin", "acme/crm/eve": "write"},
	}
}

func names(entries map[string]Entry) *Names { return &Names{Names: entries} }

func entry(id, owner int64, repo string) Entry {
	return Entry{RepoID: id, OwnerID: owner, Repo: repo, ClaimedAt: "2026-10-22", State: "active"}
}

// REG-1
func TestRegistrySchema(t *testing.T) {
	n, err := Parse([]byte(`{"names": {"@a/b": {"repo_id": 1, "owner_id": 2, "repo": "a/b", "claimed_at": "2026-10-22", "state": "active"}, "@Bad": {"state": "gone"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	p := strings.Join(n.Validate(), "\n")
	if !strings.Contains(p, "@Bad: bad name") || !strings.Contains(p, "@Bad: state must be") || strings.Contains(p, "@a/b") {
		t.Fatalf("problems:\n%s", p)
	}
}

// REG-2
func TestClaimChecks(t *testing.T) {
	base := names(map[string]Entry{})
	ok := CheckClaim(base, names(map[string]Entry{"@ann/booking": entry(10, 1, "ann/booking")}), "ann", gh())
	if !ok.OK() {
		t.Fatalf("valid user claim refused: %v", ok.Problems)
	}
	org := CheckClaim(base, names(map[string]Entry{"@acme/crm": entry(20, 2, "acme/crm")}), "bob", gh())
	if !org.OK() {
		t.Fatalf("org admin refused: %v", org.Problems)
	}
	for _, c := range []struct {
		head   map[string]Entry
		author string
		want   string
	}{
		{map[string]Entry{"@acme/crm": entry(20, 2, "acme/crm")}, "eve", "eve needs admin"},
		{map[string]Entry{"@ann/booking": entry(10, 1, "ann/booking")}, "mallory", "only Ann can claim"},
		{map[string]Entry{"@ann/other": entry(30, 1, "ann/notopic")}, "ann", "lacks the `specfork` topic"},
		{map[string]Entry{"@ann/wrong": entry(10, 1, "ann/booking")}, "ann", "README front matter name is \"@ann/booking\""},
		{map[string]Entry{"@ann/booking": entry(99, 1, "ann/booking")}, "ann", "repo_id 99: not found"},
	} {
		v := CheckClaim(base, names(c.head), c.author, gh())
		if v.OK() || !strings.Contains(strings.Join(v.Problems, "\n"), c.want) {
			t.Errorf("want %q, got %v", c.want, v.Problems)
		}
	}
	// Existing entries can't be changed or removed in a claim PR (REG-7).
	b := names(map[string]Entry{"@ann/booking": entry(10, 1, "ann/booking")})
	v := CheckClaim(b, names(map[string]Entry{"@ann/booking": entry(30, 1, "ann/notopic")}), "ann", gh())
	if v.OK() || !v.Manual {
		t.Fatalf("change accepted: %+v", v)
	}
	if v := CheckClaim(b, names(map[string]Entry{}), "ann", gh()); v.OK() || !v.Manual {
		t.Fatalf("removal accepted: %+v", v)
	}
}

// REG-3
func TestClaimScope(t *testing.T) {
	base := names(map[string]Entry{})
	v := CheckClaim(base, names(map[string]Entry{"@bob/booking": entry(10, 1, "ann/booking")}), "ann", gh())
	if v.OK() || !strings.Contains(strings.Join(v.Problems, "\n"), "scope must be @ann") {
		t.Fatalf("got %v", v.Problems)
	}
	g := gh()
	g.readme["ann/booking"] = "---\nspecfork: 1\nname: \"@specfork/booking\"\n---\n"
	v = CheckClaim(base, names(map[string]Entry{"@specfork/booking": entry(10, 1, "ann/booking")}), "ann", g)
	if v.OK() || !v.Manual {
		t.Fatalf("reserved scope auto-merged: %+v", v)
	}
}

// REG-4
func TestBindIDs(t *testing.T) {
	v := CheckClaim(names(map[string]Entry{}), names(map[string]Entry{"@ann/booking": entry(10, 7, "ann/booking")}), "ann", gh())
	if v.OK() || !strings.Contains(strings.Join(v.Problems, "\n"), "owner_id is 1, not 7") {
		t.Fatalf("got %v", v.Problems)
	}
	v = CheckClaim(names(map[string]Entry{}), names(map[string]Entry{"@ann/booking": entry(10, 1, "ann/renamed")}), "ann", gh())
	if v.OK() {
		t.Fatal("repo name must match the repo_id at claim time")
	}
}

// The committed registry is valid.
func TestCommittedRegistry(t *testing.T) {
	n, err := Load("../../registry/names.json")
	if err != nil {
		t.Fatal(err)
	}
	if p := n.Validate(); len(p) > 0 {
		t.Fatal(p)
	}
}

type fakeUsers map[int64]string

func (f fakeUsers) LoginByID(id int64) (string, error) {
	if l, ok := f[id]; ok {
		return l, nil
	}
	return "", ghapi.ErrNotFound
}

// REG-9
func TestClaimBotClaimant(t *testing.T) {
	users := fakeUsers{1: "ann"}
	author, probs := Author(DefaultBot, "Claim @ann/booking.\n\nClaimant: 1\n", DefaultBot, users)
	if author != "ann" || len(probs) != 0 {
		t.Fatalf("author %q, %v", author, probs)
	}
	// The claimant's rights decide, not the bot's.
	v := CheckClaim(names(map[string]Entry{}), names(map[string]Entry{"@ann/booking": entry(10, 1, "ann/booking")}), author, gh())
	if !v.OK() {
		t.Fatalf("claimant refused: %v", v.Problems)
	}
	if v := CheckClaim(names(map[string]Entry{}), names(map[string]Entry{"@ann/booking": entry(10, 1, "ann/booking")}), DefaultBot, gh()); v.OK() {
		t.Fatal("the bot is not the repo owner")
	}
	// REG-12: a claimant who is not the owner or an admin is refused.
	users[2] = "eve"
	eve, _ := Author(DefaultBot, "Claimant: 2", DefaultBot, users)
	if v := CheckClaim(names(map[string]Entry{}), names(map[string]Entry{"@acme/crm": entry(20, 2, "acme/crm")}), eve, gh()); v.OK() {
		t.Fatal("a non-admin claimant was accepted")
	}
	for _, body := range []string{"", "Claimant: abc", "Claimant: 1\nClaimant: 2", "claimant: 1"} {
		if _, probs := Author(DefaultBot, body, DefaultBot, users); len(probs) == 0 {
			t.Errorf("body %q accepted", body)
		}
	}
	if _, probs := Author(DefaultBot, "Claimant: 99", DefaultBot, users); len(probs) == 0 {
		t.Error("unknown claimant accepted")
	}
}

// REG-9
func TestClaimBotLineIgnored(t *testing.T) {
	author, probs := Author("mallory", "Claimant: 1", DefaultBot, fakeUsers{1: "ann"})
	if author != "mallory" || len(probs) != 0 {
		t.Fatalf("a Claimant line from a normal user changed the author: %q %v", author, probs)
	}
}

// REG-10
func TestClaimBotOnlyNames(t *testing.T) {
	if p := FileProblems([]string{"registry/names.json"}); len(p) != 0 {
		t.Fatal(p)
	}
	p := FileProblems([]string{"registry/names.json", ".github/workflows/claim.yml"})
	if len(p) != 1 || !strings.Contains(p[0], ".github/workflows/claim.yml") {
		t.Fatalf("got %v", p)
	}
}

// REG-11: a new entry changes nothing else in names.json.
func TestAddEntryKeepsOthers(t *testing.T) {
	raw, err := os.ReadFile("../../registry/names.json")
	if err != nil {
		t.Fatal(err)
	}
	out, err := AddEntry(raw, "@zed/new", entry(5, 6, "zed/new"))
	if err != nil {
		t.Fatal(err)
	}
	before, _ := Parse(raw)
	after, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Names) != len(before.Names)+1 || after.Names["@zed/new"].RepoID != 5 {
		t.Fatal("entry not added")
	}
	for k, v := range before.Names {
		if after.Names[k] != v {
			t.Errorf("%s changed", k)
		}
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if l := strings.TrimSuffix(strings.TrimSpace(line), ","); l != "" && !strings.Contains(string(out), l) {
			t.Errorf("line lost: %s", line)
		}
	}
	if _, err := AddEntry(raw, "@specfork/auth", entry(5, 6, "x/y")); err == nil {
		t.Error("an existing name was overwritten")
	}
}
