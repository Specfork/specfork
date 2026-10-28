package server

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/specfork/specfork/internal/ghapi"
	"github.com/specfork/specfork/internal/index"
	"github.com/specfork/specfork/internal/stats"
	"github.com/specfork/specfork/internal/testfx"
)

var (
	keyOnce sync.Once
	testKey *rsa.PrivateKey
)

func key(t testing.TB) *rsa.PrivateKey {
	keyOnce.Do(func() {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		testKey = k
	})
	return testKey
}

const oauthToken = "gho_SECRETOAUTHTOKEN"

// fakeGH plays github.com, api.github.com and raw.githubusercontent.com.
type fakeGH struct {
	*httptest.Server
	mu      sync.Mutex
	down    bool
	clock   func() time.Time
	created time.Time
	repos   []ghapi.Repo
	readme  map[string]string
	names   string
	openPR  string
	puts    []map[string]string
	prs     []map[string]string
}

func repo(id int64, full string, owner ghapi.Owner, topics ...string) ghapi.Repo {
	return ghapi.Repo{ID: id, FullName: full, Owner: owner, Topics: topics, DefaultBranch: "main"}
}

func newFakeGH(clock func() time.Time) *fakeGH {
	ann := ghapi.Owner{ID: 42, Login: "ann", Type: "User"}
	bob := ghapi.Owner{ID: 77, Login: "bob", Type: "User"}
	f := &fakeGH{
		clock: clock, created: clock().AddDate(-1, 0, 0),
		repos: []ghapi.Repo{
			repo(10, "ann/booking", ann, "specfork"),
			repo(11, "ann/draft", ann, "specfork"),
			repo(12, "ann/claimed", ann, "specfork"),
			repo(20, "bob/thing", bob, "specfork"),
		},
		readme: map[string]string{
			"ann/booking": "---\nspecfork: 1\nname: \"@ann/booking\"\n---\n",
			"ann/draft":   "---\nspecfork: 1\nname: \"@wrong/draft\"\n---\n",
			"ann/claimed": "---\nspecfork: 1\nname: \"@ann/claimed\"\n---\n",
			"bob/thing":   "---\nspecfork: 1\nname: \"@bob/thing\"\n---\n",
		},
		names: "{\n  \"names\": {\n    \"@ann/claimed\": {\n      \"claimed_at\": \"2026-10-22\",\n      \"owner_id\": 42,\n      \"repo\": \"ann/claimed\",\n      \"repo_id\": 12,\n      \"state\": \"active\"\n    }\n  }\n}\n",
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	return f
}

func (f *fakeGH) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		http.Error(w, "boom", http.StatusInternalServerError)
		return
	}
	out := func(code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(v)
	}
	p := r.URL.Path
	body := map[string]string{}
	if r.Method == "PUT" || r.Method == "POST" {
		json.NewDecoder(r.Body).Decode(&body)
	}
	switch {
	case p == "/login/oauth/access_token":
		out(200, map[string]string{"access_token": oauthToken})
	case p == "/user":
		out(200, map[string]any{"id": 42, "login": "ann"})
	case p == "/user/42":
		out(200, map[string]any{"id": 42, "login": "ann", "created_at": f.created})
	case strings.HasPrefix(p, "/app/installations/"):
		out(201, map[string]any{"token": "inst-token", "expires_at": f.clock().Add(time.Hour)})
	case p == "/search/repositories":
		out(200, map[string]any{"total_count": len(f.repos), "items": f.repos})
	case strings.HasPrefix(p, "/repositories/"):
		for _, rp := range f.repos {
			if p == "/repositories/"+itoa(rp.ID) {
				out(200, rp)
				return
			}
		}
		http.NotFound(w, r)
	case strings.HasSuffix(p, "/branches/main"):
		out(200, map[string]any{"commit": map[string]string{"sha": "abc"}})
	case p == "/specfork/specfork/main/registry/names.json":
		w.Write([]byte(f.names))
	case strings.HasSuffix(p, "/abc/README.md"):
		full := strings.TrimSuffix(strings.TrimPrefix(p, "/"), "/abc/README.md")
		if s, ok := f.readme[full]; ok {
			w.Write([]byte(s))
			return
		}
		http.NotFound(w, r)
	case r.Method == "GET" && p == "/repos/specfork/specfork/pulls":
		if f.openPR != "" {
			out(200, []map[string]string{{"html_url": f.openPR}})
			return
		}
		out(200, []any{})
	case r.Method == "GET" && p == "/repos/specfork/specfork/git/ref/heads/main":
		out(200, map[string]any{"object": map[string]string{"sha": "mainsha"}})
	case r.Method == "GET" && strings.HasPrefix(p, "/repos/specfork/specfork/git/ref/heads/"):
		http.NotFound(w, r)
	case r.Method == "POST" && p == "/repos/specfork/specfork/git/refs":
		out(201, map[string]string{})
	case r.Method == "GET" && p == "/repos/specfork/specfork/contents/registry/names.json":
		out(200, map[string]string{"sha": "blob1"})
	case r.Method == "PUT" && p == "/repos/specfork/specfork/contents/registry/names.json":
		f.puts = append(f.puts, body)
		out(200, map[string]string{})
	case r.Method == "POST" && p == "/repos/specfork/specfork/pulls":
		f.prs = append(f.prs, body)
		out(201, map[string]string{"html_url": "https://github.com/specfork/specfork/pull/9"})
	default:
		http.NotFound(w, r)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

// env is a server wired to fakes.
type env struct {
	t       *testing.T
	s       *Server
	h       http.Handler
	gh      *fakeGH
	st      *memStore
	cnt     *stats.Memory
	logs    *bytes.Buffer
	clock   time.Time
	indexN  int
	indexEr error
	nocsrf  bool
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{t: t, st: newMem(), cnt: stats.NewMemory(), logs: &bytes.Buffer{}, clock: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
	now := func() time.Time { return e.clock }
	e.gh = newFakeGH(now)
	t.Cleanup(e.gh.Close)
	ix, fetch := testfx.Seeds(t)
	cfg := &Config{
		Port: "8080", ClientID: "cid", ClientSecret: "csecret", SessionSecret: []byte("0123456789abcdef0123456789abcdef"),
		AppID: 5, AppKey: key(t), InstallationID: 7, MinAccountAgeDays: 30,
		SiteURL: "https://specfork.dev", GitHubWeb: e.gh.URL, GitHubAPI: e.gh.URL, GitHubRaw: e.gh.URL,
		RegistryRepo: "specfork/specfork", RegistryRef: "main",
	}
	e.s = New(cfg, Deps{
		Store: e.st, Counters: e.cnt, Fetcher: fetch, Version: "test", Now: now,
		LoadIndex: func() (*index.Index, error) {
			e.indexN++
			if e.indexEr != nil {
				return nil, e.indexEr
			}
			return ix, nil
		},
		Log: slog.New(slog.NewJSONHandler(e.logs, nil)),
	})
	e.h = e.s.Handler()
	e.st.users[42] = "ann"
	return e
}

// do sends a request; user 0 is signed out. Extra headers come as name, value pairs.
func (e *env) do(method, target, body string, user int64, hdr ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if method != "GET" && !e.nocsrf {
		r.Header.Set(csrfHeader, "1")
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	if user != 0 {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: e.s.sg.seal(sessionData{ID: user, Exp: e.clock.Add(time.Hour).Unix()})})
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

func cookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}
