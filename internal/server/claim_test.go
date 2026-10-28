package server

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/specfork/specfork/internal/registry"
)

// API-22
func TestClaimRepos(t *testing.T) {
	e := newEnv(t)
	if w := e.do("GET", "/claim/repos", "", 0); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := e.do("GET", "/claim/repos", "", 42)
	var list []claimable
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || w.Code != 200 {
		t.Fatalf("%d %v %s", w.Code, err, w.Body)
	}
	by := map[string]claimable{}
	for _, c := range list {
		by[c.Repo] = c
	}
	if len(by) != 2 || by["ann/claimed"].Repo != "" {
		t.Fatalf("an already claimed repo is listed: %+v", list)
	}
	if c := by["ann/booking"]; !c.Claimable || c.Name != "@ann/booking" || c.RepoID != 10 || len(c.Failures) != 0 {
		t.Errorf("booking: %+v", c)
	}
	if c := by["ann/draft"]; c.Claimable || !strings.Contains(strings.Join(c.Failures, ";"), "scope must be @ann") {
		t.Errorf("draft: %+v", c)
	}
	if !strings.Contains(w.Body.String(), `"failures":[]`) {
		t.Error("failures must be a list, not null")
	}
}

// API-23, REG-12
func TestClaimChecksServer(t *testing.T) {
	e := newEnv(t)
	for _, c := range []struct {
		id   int64
		want string
	}{{20, "only bob can claim"}, {11, "scope must be @ann"}, {12, "already claimed"}} {
		w := e.do("POST", "/claim", `{"repo_id":`+itoa(c.id)+`}`, 42)
		if w.Code != 422 || !strings.Contains(w.Body.String(), c.want) {
			t.Errorf("repo %d: %d %s", c.id, w.Code, w.Body)
		}
	}
	if w := e.do("POST", "/claim", `{"repo_id":999}`, 42); w.Code != 422 {
		t.Errorf("unknown repo: %d", w.Code)
	}
	if w := e.do("POST", "/claim", `{}`, 42); w.Code != 400 {
		t.Errorf("no repo_id: %d", w.Code)
	}
	if w := e.do("POST", "/claim", `{"repo_id":10}`, 0); w.Code != 401 {
		t.Errorf("signed out: %d", w.Code)
	}
	if len(e.gh.puts)+len(e.gh.prs) != 0 {
		t.Error("a refused claim opened something")
	}
}

// API-24, REG-8, REG-11
func TestClaimOpensPR(t *testing.T) {
	e := newEnv(t)
	w := e.do("POST", "/claim", `{"repo_id":10}`, 42)
	if w.Code != 201 || !strings.Contains(w.Body.String(), `"pr_url":"https://github.com/specfork/specfork/pull/9"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if len(e.gh.puts) != 1 || len(e.gh.prs) != 1 {
		t.Fatalf("puts %d, prs %d", len(e.gh.puts), len(e.gh.prs))
	}
	put := e.gh.puts[0]
	if put["branch"] != "claim/@ann/booking" || put["sha"] != "blob1" {
		t.Errorf("put %v", put)
	}
	raw, _ := base64.StdEncoding.DecodeString(put["content"])
	after, err := registry.Parse(raw)
	if err != nil || len(after.Names) != 2 || after.Names["@ann/claimed"].RepoID != 12 {
		t.Fatalf("names.json after: %v %s", err, raw)
	}
	if got := after.Names["@ann/booking"]; got.RepoID != 10 || got.OwnerID != 42 || got.Repo != "ann/booking" || got.State != "active" || got.ClaimedAt != "2026-10-27" {
		t.Errorf("entry %+v", got)
	}
	pr := e.gh.prs[0]
	if pr["head"] != "claim/@ann/booking" || pr["base"] != "main" || !strings.Contains(pr["body"], "\nClaimant: 42\n") {
		t.Errorf("pr %v", pr)
	}
	if id, ok := registry.ClaimantID(pr["body"]); !ok || id != 42 {
		t.Error("the workflow cannot read the claimant line")
	}
}

// API-25
func TestClaimIdempotent(t *testing.T) {
	e := newEnv(t)
	e.gh.openPR = "https://github.com/specfork/specfork/pull/5"
	w := e.do("POST", "/claim", `{"repo_id":10}`, 42)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "/pull/5") || len(e.gh.puts)+len(e.gh.prs) != 0 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}
