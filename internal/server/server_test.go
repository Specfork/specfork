package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

// API-1, MCP-11
func TestServeBothHosts(t *testing.T) {
	e := newEnv(t)
	for _, host := range []string{"api.specfork.dev", "mcp.specfork.dev"} {
		for _, p := range []string{"/healthz", "/stats.json", "/scores.json"} {
			if w := e.do("GET", p, "", 0, "Host", host); w.Code != 200 {
				t.Errorf("%s %s: %d", host, p, w.Code)
			}
		}
		r := e.do("POST", "/mcp", `{}`, 0, "Host", host, "Content-Type", "application/json")
		if r.Code == 404 || r.Code == 503 {
			t.Errorf("%s /mcp: %d", host, r.Code)
		}
	}
}

func goodEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://u:pw@postgres:5432/specfork", "GITHUB_CLIENT_ID": "cid", "GITHUB_CLIENT_SECRET": "csecret-value",
		"SESSION_SECRET": "0123456789abcdef0123456789abcdef", "GITHUB_APP_ID": "5", "GITHUB_APP_INSTALLATION_ID": "7",
	}
}

func withKey(t *testing.T, m map[string]string) func(string) string {
	t.Helper()
	m["GITHUB_APP_PRIVATE_KEY"] = pemOf(t)
	return func(k string) string { return m[k] }
}

// API-2
func TestConfigRequired(t *testing.T) {
	cfg, err := LoadConfig(withKey(t, goodEnv()))
	if err != nil || cfg.Port != "8080" || cfg.MinAccountAgeDays != 30 || cfg.AppKey == nil {
		t.Fatalf("%+v %v", cfg, err)
	}
	for _, name := range []string{"DATABASE_URL", "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET", "SESSION_SECRET", "GITHUB_APP_ID", "GITHUB_APP_INSTALLATION_ID", "GITHUB_APP_PRIVATE_KEY"} {
		m := goodEnv()
		get := withKey(t, m)
		delete(m, name)
		if _, err := LoadConfig(get); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("without %s: %v", name, err)
		}
	}
	m := goodEnv()
	m["SCORE_MIN_ACCOUNT_AGE_DAYS"] = "7"
	m["PORT"] = "9000"
	if cfg, err := LoadConfig(withKey(t, m)); err != nil || cfg.MinAccountAgeDays != 7 || cfg.Port != "9000" {
		t.Fatalf("%+v %v", cfg, err)
	}
}

// API-2
func TestConfigNoSecretInError(t *testing.T) {
	for name, bad := range map[string]string{"SESSION_SECRET": "short-secret-value", "GITHUB_APP_PRIVATE_KEY": "not-a-key-SECRETVALUE", "GITHUB_APP_ID": "abc-SECRETVALUE", "SCORE_MIN_ACCOUNT_AGE_DAYS": "x-SECRETVALUE"} {
		m := goodEnv()
		get := withKey(t, m)
		m[name] = bad
		_, err := LoadConfig(get)
		if err == nil || !strings.Contains(err.Error(), name) || strings.Contains(err.Error(), bad) || strings.Contains(err.Error(), "SECRETVALUE") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// API-3
func TestHealthz(t *testing.T) {
	e := newEnv(t)
	if w := e.do("GET", "/healthz", "", 0); w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	e.st.down = true
	if w := e.do("GET", "/healthz", "", 0); w.Code != 503 || strings.TrimSpace(w.Body.String()) != `{"status":"unavailable"}` {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

// API-5
func TestLogNoPersonalData(t *testing.T) {
	e := newEnv(t)
	e.st.users[4242424] = "secret-login"
	w := e.do("GET", "/me?token=needle", "", 4242424, "X-Forwarded-For", "203.0.113.9", "User-Agent", "secret-agent")
	e.do("GET", "/scores/specfork/scheduling/mine", "", 4242424)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	logs := e.logs.String()
	for _, s := range []string{"needle", "secret-login", "4242424", "203.0.113.9", "secret-agent", "192.0.2.1", sessionCookie} {
		if strings.Contains(logs, s) {
			t.Errorf("log holds %q", s)
		}
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.Split(logs, "\n")[0]), &line); err != nil {
		t.Fatalf("not JSON lines: %v: %s", err, logs)
	}
	for _, k := range []string{"time", "level", "route", "status", "duration"} {
		if _, ok := line[k]; !ok {
			t.Errorf("log line lacks %s: %v", k, line)
		}
	}
	if line["route"] != "GET /me" {
		t.Errorf("route %v", line["route"])
	}
}

// API-6
func TestImageFiles(t *testing.T) {
	// The checked part of API-6: the Dockerfile builds a static binary and runs it as a non-root user.
	b, err := osReadFile("../../Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	must := []string{"CGO_ENABLED=0", "USER 10001", "ENTRYPOINT", `"serve"`}
	for _, s := range must {
		if !strings.Contains(string(b), s) {
			t.Errorf("Dockerfile lacks %q", s)
		}
	}
}

// API-7
func TestCORS(t *testing.T) {
	e := newEnv(t)
	w := e.do("GET", "/scores.json", "", 0, "Origin", "https://evil.example")
	if w.Header().Get("Access-Control-Allow-Origin") != "*" || w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Errorf("scores.json headers %v", w.Header())
	}
	w = e.do("GET", "/stats.json", "", 0)
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("stats.json headers %v", w.Header())
	}
	w = e.do("GET", "/me", "", 42, "Origin", "https://specfork.dev")
	if w.Header().Get("Access-Control-Allow-Origin") != "https://specfork.dev" || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("session route headers %v", w.Header())
	}
	if w = e.do("GET", "/me", "", 42, "Origin", "https://evil.example"); w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("other origin allowed")
	}
	w = e.do("OPTIONS", "/scores/specfork/scheduling", "", 0, "Origin", "https://specfork.dev")
	if w.Code != 204 || !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "X-Specfork") {
		t.Errorf("preflight %d %v", w.Code, w.Header())
	}
}

// API-7
func TestCSRFHeader(t *testing.T) {
	e := newEnv(t)
	e.nocsrf = true
	for _, c := range [][2]string{{"PUT", "/scores/specfork/scheduling"}, {"DELETE", "/scores/specfork/scheduling"}, {"POST", "/claim"}, {"POST", "/logout"}, {"DELETE", "/me"}} {
		if w := e.do(c[0], c[1], `{"score":4}`, 42); w.Code != 400 {
			t.Errorf("%s %s without header: %d", c[0], c[1], w.Code)
		}
	}
	if len(e.st.scores) != 0 || e.st.users[42] == "" {
		t.Error("a request without the header changed state")
	}
}

// API-8
func TestLoginRedirect(t *testing.T) {
	e := newEnv(t)
	w := e.do("GET", "/auth/github/login?return=/specfork/scheduling/", "", 0)
	if w.Code != 302 {
		t.Fatal(w.Code)
	}
	loc, _ := url.Parse(w.Header().Get("Location"))
	q := loc.Query()
	if !strings.HasSuffix(loc.Path, "/login/oauth/authorize") || q.Get("scope") != "read:user" || q.Get("client_id") != "cid" || len(q.Get("state")) < 16 {
		t.Fatalf("location %s", loc)
	}
	c := cookie(w, oauthCookie)
	if c == nil || !c.HttpOnly || !c.Secure || c.MaxAge != 600 {
		t.Fatalf("state cookie %+v", c)
	}
}

// API-8
func TestLoginReturnPath(t *testing.T) {
	e := newEnv(t)
	for in, want := range map[string]string{
		"/specfork/scheduling/":      "/specfork/scheduling/",
		"/docs/claim/?x=1":           "/docs/claim/?x=1",
		"https://specfork.dev/docs/": "/docs/",
		"https://evil.example/x":     "/",
		"//evil.example/x":           "/",
		"/\\evil.example":            "/",
		"javascript:alert(1)":        "/",
		"":                           "/",
		"http://specfork.dev/docs/":  "/",

		"https://specfork.dev.evil.example/docs/": "/",
	} {
		if got := e.s.returnPath(in); got != want {
			t.Errorf("returnPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// signIn runs login and callback and returns the callback response.
func (e *env) signIn(ret string) *httptest.ResponseRecorder {
	e.t.Helper()
	w := e.do("GET", "/auth/github/login?return="+url.QueryEscape(ret), "", 0)
	loc, _ := url.Parse(w.Header().Get("Location"))
	r := e.do("GET", "/auth/github/callback?code=thecode&state="+loc.Query().Get("state"), "", 0, "Cookie", oauthCookie+"="+cookie(w, oauthCookie).Value)
	return r
}

// API-9
func TestCallbackUpsert(t *testing.T) {
	e := newEnv(t)
	delete(e.st.users, 42)
	w := e.signIn("/specfork/scheduling/")
	if w.Code != 302 || w.Header().Get("Location") != "https://specfork.dev/specfork/scheduling/" {
		t.Fatalf("%d %s", w.Code, w.Header().Get("Location"))
	}
	if e.st.users[42] != "ann" {
		t.Fatal("user not stored")
	}
	s := cookie(w, sessionCookie)
	if s == nil {
		t.Fatal("no session")
	}
	if me := e.do("GET", "/me", "", 0, "Cookie", sessionCookie+"="+s.Value); me.Code != 200 {
		t.Fatalf("/me with the new session: %d", me.Code)
	}
}

// API-9
func TestCallbackBadState(t *testing.T) {
	e := newEnv(t)
	delete(e.st.users, 42)
	login := e.do("GET", "/auth/github/login", "", 0)
	for name, req := range map[string]*httptest.ResponseRecorder{
		"wrong state": e.do("GET", "/auth/github/callback?code=c&state=nope", "", 0, "Cookie", oauthCookie+"="+cookie(login, oauthCookie).Value),
		"no cookie":   e.do("GET", "/auth/github/callback?code=c&state=nope", "", 0),
	} {
		if req.Code != 302 || !strings.Contains(req.Header().Get("Location"), "auth=failed") || cookie(req, sessionCookie) != nil {
			t.Errorf("%s: %d %s", name, req.Code, req.Header().Get("Location"))
		}
	}
	if len(e.st.users) != 0 {
		t.Error("a failed sign-in created a user")
	}
}

// API-9
func TestTokenNotStored(t *testing.T) {
	e := newEnv(t)
	w := e.signIn("/")
	all := w.Body.String()
	for k, v := range w.Header() {
		all += k + strings.Join(v, ",")
	}
	for _, l := range e.st.users {
		all += l
	}
	if strings.Contains(all+e.logs.String(), oauthToken) {
		t.Error("the GitHub token leaked into a response, the store or the log")
	}
	if v, _ := base64.RawURLEncoding.DecodeString(strings.Split(cookie(w, sessionCookie).Value, ".")[0]); strings.Contains(string(v), "gho_") {
		t.Error("token in session cookie")
	}
}

// API-10
func TestSessionCookie(t *testing.T) {
	e := newEnv(t)
	c := cookie(e.signIn("/"), sessionCookie)
	if c == nil || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.MaxAge != 30*24*3600 || c.Path != "/" || c.Domain != "" {
		t.Fatalf("%+v", c)
	}
}

// API-10
func TestSessionTamper(t *testing.T) {
	e := newEnv(t)
	good := e.s.sg.seal(sessionData{ID: 42, Exp: e.clock.Add(time.Hour).Unix()})
	other := signer{[]byte("another-secret-another-secret-xx")}.seal(sessionData{ID: 42, Exp: e.clock.Add(time.Hour).Unix()})
	expired := e.s.sg.seal(sessionData{ID: 42, Exp: e.clock.Add(-time.Hour).Unix()})
	forged := regexp.MustCompile(`^[^.]*`).ReplaceAllString(good, base64.RawURLEncoding.EncodeToString([]byte(`{"id":77,"exp":9999999999}`)))
	for name, v := range map[string]string{"good": good, "other key": other, "expired": expired, "forged payload": forged, "garbage": "x.y", "empty": ""} {
		w := e.do("GET", "/me", "", 0, "Cookie", sessionCookie+"="+v)
		if want := map[bool]int{true: 200, false: 401}[name == "good"]; w.Code != want {
			t.Errorf("%s: %d, want %d", name, w.Code, want)
		}
	}
}

// API-11
func TestMe(t *testing.T) {
	e := newEnv(t)
	w := e.do("GET", "/me", "", 42)
	var m map[string]any
	json.Unmarshal(w.Body.Bytes(), &m)
	if w.Code != 200 || m["login"] != "ann" || m["id"] != float64(42) || len(m) != 2 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := e.do("GET", "/me", "", 0); w.Code != 401 {
		t.Fatalf("signed out: %d", w.Code)
	}
	if w := e.do("GET", "/me", "", 999); w.Code != 401 {
		t.Fatalf("unknown user: %d", w.Code)
	}
}

// API-12
func TestLogout(t *testing.T) {
	e := newEnv(t)
	w := e.do("POST", "/logout", "", 42)
	if c := cookie(w, sessionCookie); w.Code != 204 || c == nil || c.MaxAge >= 0 {
		t.Fatalf("%d %+v", w.Code, c)
	}
}

// API-13
func TestDeleteAccount(t *testing.T) {
	e := newEnv(t)
	e.do("PUT", "/scores/specfork/scheduling", `{"score":5}`, 42)
	if len(e.st.scores[42]) != 1 {
		t.Fatal("no score to delete")
	}
	w := e.do("DELETE", "/me", "", 42)
	if c := cookie(w, sessionCookie); w.Code != 204 || c == nil || c.MaxAge >= 0 {
		t.Fatalf("%d %+v", w.Code, c)
	}
	if _, ok := e.st.users[42]; ok || len(e.st.scores[42]) != 0 {
		t.Error("account or scores survived")
	}
	if w := e.do("GET", "/scores.json", "", 0); strings.Contains(w.Body.String(), "scheduling") {
		t.Error("deleted scores still counted")
	}
}
