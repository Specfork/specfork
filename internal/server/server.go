// Package server is `specfork serve`: the hosted MCP server and the API behind specfork.dev
// (sign-in, scores, one-click claims). It stores accounts, scores and counters, never spec text (IDX-13).
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/specfork/specfork/internal/ghapi"
	"github.com/specfork/specfork/internal/hosted"
	"github.com/specfork/specfork/internal/index"
	"github.com/specfork/specfork/internal/mcpserver"
	"github.com/specfork/specfork/internal/source"
	"github.com/specfork/specfork/internal/stats"
)

const (
	indexTTL     = 10 * time.Minute
	writeLimit   = 60
	writeWindow  = time.Hour
	csrfHeader   = "X-Specfork"
	githubFailed = "github_unavailable"
)

// Deps are the parts of the server that tests swap out.
type Deps struct {
	Store     Store
	Counters  stats.Store
	LoadIndex func() (*index.Index, error)
	Fetcher   source.Fetcher
	Version   string
	Now       func() time.Time
	HTTP      *http.Client // for GitHub
	Log       *slog.Logger
}

// Server answers on api.specfork.dev and mcp.specfork.dev alike (API-1).
type Server struct {
	cfg *Config
	Deps
	sg  signer
	lim *limiter

	imu   sync.Mutex
	ix    *index.Index
	ixAt  time.Time
	tmu   sync.Mutex
	token string
	tExp  time.Time
}

func New(cfg *Config, d Deps) *Server {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.HTTP == nil {
		d.HTTP = &http.Client{Timeout: 15 * time.Second}
	}
	if d.Log == nil {
		d.Log = slog.New(slog.NewJSONHandler(discard{}, nil))
	}
	return &Server{cfg: cfg, Deps: d, sg: signer{cfg.SessionSecret}, lim: &limiter{hits: map[int64][]time.Time{}}}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func (s *Server) now() time.Time { return s.Now() }

var errNoIndex = errors.New("no index")

// index is the site's index.json kept for 10 minutes; a failed refresh keeps the last copy (API-21).
func (s *Server) loadIndex() (*index.Index, error) {
	s.imu.Lock()
	defer s.imu.Unlock()
	if s.ix != nil && s.now().Sub(s.ixAt) < indexTTL {
		return s.ix, nil
	}
	ix, err := s.LoadIndex()
	if err != nil {
		if s.ix != nil {
			return s.ix, nil
		}
		return nil, errNoIndex
	}
	s.ix, s.ixAt = ix, s.now()
	return ix, nil
}

// gh is a GitHub client for one token against the configured API.
func (s *Server) gh(token string) *ghapi.Client {
	c := ghapi.New(token)
	c.BaseURL, c.RawURL, c.HTTP = s.cfg.GitHubAPI, s.cfg.GitHubRaw, s.HTTP
	return c
}

// app is a client acting as the Specfork GitHub App, with its installation token cached (API-22, API-24).
func (s *Server) app() (*ghapi.Client, error) {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if s.token == "" || s.now().Add(time.Minute).After(s.tExp) {
		jwt, err := ghapi.AppJWT(s.cfg.AppID, s.cfg.AppKey, s.now())
		if err != nil {
			return nil, err
		}
		tok, exp, err := s.gh(jwt).InstallationToken(s.cfg.InstallationID)
		if err != nil {
			return nil, err
		}
		s.token, s.tExp = tok, exp
	}
	return s.gh(s.token), nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mcp := hosted.Handler(mcpserver.Deps{LoadIndex: s.loadIndex, Fetcher: s.Fetcher, Version: s.Version}, s.Counters, s.now)
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.Handle("/mcp", mcp)
	mux.Handle("GET /stats.json", mcp)
	mux.HandleFunc("GET /scores.json", s.scoresJSON)
	mux.HandleFunc("GET /auth/github/login", s.login)
	mux.HandleFunc("GET /auth/github/callback", s.callback)
	mux.HandleFunc("GET /me", s.me)
	mux.HandleFunc("POST /logout", s.write(s.logout))
	mux.HandleFunc("DELETE /me", s.write(s.deleteMe))
	mux.HandleFunc("PUT /scores/{scope}/{name}", s.write(s.putScore))
	mux.HandleFunc("DELETE /scores/{scope}/{name}", s.write(s.deleteScore))
	mux.HandleFunc("GET /scores/{scope}/{name}/mine", s.myScore)
	mux.HandleFunc("GET /claim/repos", s.claimRepos)
	mux.HandleFunc("POST /claim", s.write(s.claim))
	return s.logged(s.cors(mux))
}

// healthz pings Postgres and tells nothing else (API-3).
func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if s.Store.Ping(ctx) != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// write guards state-changing routes: the custom header cannot be sent cross-site without a preflight (API-7).
func (s *Server) write(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(csrfHeader) != "1" {
			writeErr(w, http.StatusBadRequest, "missing_header")
			return
		}
		h(w, r)
	}
}

// cors lets the static site call session routes with credentials, and anyone read the public documents (API-7).
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		switch r.URL.Path {
		case "/scores.json", "/stats.json":
			h.Set("Access-Control-Allow-Origin", "*")
		case "/healthz", "/mcp":
		default:
			h.Add("Vary", "Origin")
			if r.Header.Get("Origin") == s.cfg.SiteURL {
				h.Set("Access-Control-Allow-Origin", s.cfg.SiteURL)
				h.Set("Access-Control-Allow-Credentials", "true")
			}
		}
		if r.Method == http.MethodOptions && h.Get("Access-Control-Allow-Origin") != "" {
			h.Set("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE")
			h.Set("Access-Control-Allow-Headers", csrfHeader+", Content-Type")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(c int) {
	w.status = c
	w.ResponseWriter.WriteHeader(c)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// logged writes one JSON line per request: route pattern, status, duration. Nothing that names a person (API-5).
func (s *Server) logged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := s.now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		level := slog.LevelInfo
		if sw.status >= 500 {
			level = slog.LevelError
		}
		s.Log.LogAttrs(r.Context(), level, "request", slog.String("route", route), slog.Int("status", sw.status), slog.Duration("duration", s.now().Sub(start)))
	})
}

// limiter allows each user 60 writes an hour (API-19).
type limiter struct {
	mu   sync.Mutex
	hits map[int64][]time.Time
}

// allow records a write, or says how long to wait.
func (l *limiter) allow(id int64, now time.Time) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	recent := l.hits[id][:0]
	for _, t := range l.hits[id] {
		if now.Sub(t) < writeWindow {
			recent = append(recent, t)
		}
	}
	if len(recent) >= writeLimit {
		l.hits[id] = recent
		return writeWindow - now.Sub(recent[0]), false
	}
	l.hits[id] = append(recent, now)
	return 0, true
}

// user is the signed-in account; the user row must still exist, so a deleted account is signed out.
func (s *Server) user(w http.ResponseWriter, r *http.Request) (int64, string, bool) {
	id, ok := s.sessionID(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "signed_out")
		return 0, "", false
	}
	login, found, err := s.Store.User(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return 0, "", false
	}
	if !found {
		writeErr(w, http.StatusUnauthorized, "signed_out")
		return 0, "", false
	}
	return id, login, true
}

// throttle answers 429 when the user has used up their writes.
func (s *Server) throttle(w http.ResponseWriter, id int64) bool {
	wait, ok := s.lim.allow(id, s.now())
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		writeErr(w, http.StatusTooManyRequests, "rate_limited")
	}
	return ok
}

func githubDown(w http.ResponseWriter) { writeErr(w, http.StatusBadGateway, githubFailed) }
