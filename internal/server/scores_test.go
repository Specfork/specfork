package server

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/specfork/specfork/internal/scores"
)

const sched = "/scores/specfork/scheduling"

// API-15
func TestPutScore(t *testing.T) {
	e := newEnv(t)
	if w := e.do("PUT", sched, `{"score":4}`, 42); w.Code != 200 || e.st.scores[42]["@specfork/scheduling"] != 4 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	e.do("PUT", sched, `{"score":2}`, 42)
	if len(e.st.scores[42]) != 1 || e.st.scores[42]["@specfork/scheduling"] != 2 {
		t.Fatalf("a second score did not replace the first: %v", e.st.scores)
	}
	if w := e.do("PUT", sched, `{"score":4}`, 0); w.Code != 401 {
		t.Fatalf("signed out: %d", w.Code)
	}
}

// API-15
func TestPutScoreUnknownSpec(t *testing.T) {
	e := newEnv(t)
	if w := e.do("PUT", "/scores/nobody/nothing", `{"score":4}`, 42); w.Code != 404 {
		t.Fatalf("%d", w.Code)
	}
	ix, _ := e.s.loadIndex()
	for i := range ix.Specs {
		if ix.Specs[i].Name == "@specfork/scheduling" {
			ix.Specs[i].State = "frozen"
		}
	}
	if w := e.do("PUT", sched, `{"score":4}`, 42); w.Code != 404 {
		t.Fatalf("frozen spec: %d", w.Code)
	}
	if len(e.st.scores) != 0 {
		t.Error("score stored for an unknown spec")
	}
}

// API-15
func TestPutScoreRange(t *testing.T) {
	e := newEnv(t)
	for _, b := range []string{`{"score":0}`, `{"score":6}`, `{"score":4.5}`, `{"score":"4"}`, `{}`, `nope`, `{"score":-1}`, `{"score":null}`} {
		if w := e.do("PUT", sched, b, 42); w.Code != 400 {
			t.Errorf("%s: %d", b, w.Code)
		}
	}
	for n := 1; n <= 5; n++ {
		if w := e.do("PUT", sched, `{"score":`+strconv.Itoa(n)+`}`, 42); w.Code != 200 {
			t.Errorf("score %d: %d", n, w.Code)
		}
	}
}

// API-16
func TestDeleteScore(t *testing.T) {
	e := newEnv(t)
	e.do("PUT", sched, `{"score":3}`, 42)
	for i := 0; i < 2; i++ { // also when there was none
		if w := e.do("DELETE", sched, "", 42); w.Code != 204 {
			t.Fatalf("%d", w.Code)
		}
	}
	if len(e.st.scores[42]) != 0 {
		t.Error("score not removed")
	}
}

// API-17
func TestMyScore(t *testing.T) {
	e := newEnv(t)
	w := e.do("GET", sched+"/mine", "", 42)
	if strings.TrimSpace(w.Body.String()) != `{"score":null}` || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%s %v", w.Body, w.Header())
	}
	e.do("PUT", sched, `{"score":5}`, 42)
	if w := e.do("GET", sched+"/mine", "", 42); strings.TrimSpace(w.Body.String()) != `{"score":5}` {
		t.Fatal(w.Body)
	}
	if w := e.do("GET", sched+"/mine", "", 0); w.Code != 401 {
		t.Fatal(w.Code)
	}
}

// API-18
func TestScoreAccountAge(t *testing.T) {
	e := newEnv(t)
	e.gh.created = e.clock.Add(-3 * 24 * time.Hour)
	w := e.do("PUT", sched, `{"score":5}`, 42)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "account_too_new") || len(e.st.scores) != 0 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	e.gh.created = e.clock.Add(-31 * 24 * time.Hour)
	if w := e.do("PUT", sched, `{"score":5}`, 42); w.Code != 200 {
		t.Fatalf("old enough: %d", w.Code)
	}
}

// API-19
func TestWriteRateLimit(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 60; i++ {
		if w := e.do("DELETE", sched, "", 42); w.Code != 204 {
			t.Fatalf("write %d: %d", i+1, w.Code)
		}
	}
	w := e.do("DELETE", sched, "", 42)
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	e.st.users[43] = "bob"
	if w := e.do("DELETE", sched, "", 43); w.Code != 204 {
		t.Fatal("the limit is per user")
	}
	e.clock = e.clock.Add(61 * time.Minute)
	if w := e.do("DELETE", sched, "", 42); w.Code != 204 {
		t.Fatalf("after the hour: %d", w.Code)
	}
}

// API-20
func TestScoresJSON(t *testing.T) {
	e := newEnv(t)
	for id, n := range map[int64]int{1: 5, 2: 4, 3: 4} {
		e.st.PutScore(nil, id, "@specfork/scheduling", n, e.clock)
	}
	e.st.PutScore(nil, 1, "@specfork/auth", 1, e.clock)
	w := e.do("GET", "/scores.json", "", 0)
	if w.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Errorf("Cache-Control %q", w.Header().Get("Cache-Control"))
	}
	var d scores.Doc
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil || d.GeneratedAt == "" {
		t.Fatalf("%v %s", err, w.Body)
	}
	if len(d.Scores) != 2 || d.Scores[0] != (scores.Score{Spec: "@specfork/auth", Average: 1, Count: 1}) || d.Scores[1] != (scores.Score{Spec: "@specfork/scheduling", Average: 4.33, Count: 3}) {
		t.Fatalf("%+v", d.Scores)
	}
	if empty := newEnv(t).do("GET", "/scores.json", "", 0); !strings.Contains(empty.Body.String(), `"scores":[]`) {
		t.Errorf("no data must be an empty list: %s", empty.Body)
	}
}

// API-20
func TestScoresJSONNoUsers(t *testing.T) {
	e := newEnv(t)
	e.do("PUT", sched, `{"score":5}`, 42)
	var raw map[string]any
	w := e.do("GET", "/scores.json", "", 0)
	json.Unmarshal(w.Body.Bytes(), &raw)
	row := raw["scores"].([]any)[0].(map[string]any)
	if len(raw) != 2 || len(row) != 3 || strings.Contains(w.Body.String(), "ann") || strings.Contains(w.Body.String(), "42") {
		t.Fatalf("user data in %s", w.Body)
	}
}

// API-21
func TestIndexCache(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 3; i++ {
		e.do("PUT", sched, `{"score":4}`, 42)
	}
	if e.indexN != 1 {
		t.Fatalf("index fetched %d times in 10 minutes", e.indexN)
	}
	e.clock = e.clock.Add(11 * time.Minute)
	e.indexEr = errors.New("pages down")
	if w := e.do("PUT", sched, `{"score":4}`, 42); w.Code != 200 || e.indexN != 2 {
		t.Fatalf("stale copy not served: %d, fetches %d", w.Code, e.indexN)
	}
	fresh := newEnv(t)
	fresh.indexEr = errors.New("pages down")
	if w := fresh.do("PUT", sched, `{"score":4}`, 42); w.Code != 503 {
		t.Fatalf("no copy at all: %d", w.Code)
	}
}

// API-26
func TestGitHubDown(t *testing.T) {
	e := newEnv(t)
	e.gh.down = true
	delete(e.st.users, 42)
	if w := e.signIn("/"); w.Code != 502 || !strings.Contains(w.Body.String(), "github_unavailable") || len(e.st.users) != 0 {
		t.Errorf("sign-in: %d %s", w.Code, w.Body)
	}
	e.st.users[42] = "ann"
	if w := e.do("PUT", sched, `{"score":4}`, 42); w.Code != 502 || len(e.st.scores) != 0 {
		t.Errorf("score: %d", w.Code)
	}
	if w := e.do("POST", "/claim", `{"repo_id":10}`, 42); w.Code != 502 || len(e.gh.puts)+len(e.gh.prs) != 0 {
		t.Errorf("claim: %d", w.Code)
	}
	if w := e.do("GET", "/claim/repos", "", 42); w.Code != 502 {
		t.Errorf("claim repos: %d", w.Code)
	}
}
