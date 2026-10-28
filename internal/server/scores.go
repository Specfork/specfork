package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/specfork/specfork/internal/scores"
)

// specName turns the {scope}/{name} of a route into "@scope/name".
func specName(r *http.Request) string { return "@" + r.PathValue("scope") + "/" + r.PathValue("name") }

// active reports whether the index has this spec as an active name (API-15).
func (s *Server) active(w http.ResponseWriter, name string) bool {
	ix, err := s.loadIndex()
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "index_unavailable")
		return false
	}
	if sp, ok := ix.Find(name); !ok || sp.State != "active" {
		writeErr(w, http.StatusNotFound, "unknown_spec")
		return false
	}
	return true
}

// putScore sets the user's one score for a spec (API-15, API-18).
func (s *Server) putScore(w http.ResponseWriter, r *http.Request) {
	id, _, ok := s.user(w, r)
	if !ok || !s.throttle(w, id) {
		return
	}
	name := specName(r)
	if !s.active(w, name) {
		return
	}
	var body struct {
		Score json.RawMessage `json:"score"`
	}
	b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<10))
	n := 0
	err := json.Unmarshal(b, &body)
	if err == nil { // Atoi refuses 4.5 and "4"
		n, err = strconv.Atoi(strings.TrimSpace(string(body.Score)))
	}
	if err != nil || n < 1 || n > 5 {
		writeErr(w, http.StatusBadRequest, "score_must_be_1_to_5")
		return
	}
	if s.cfg.MinAccountAgeDays > 0 {
		app, err := s.app()
		if err != nil {
			githubDown(w)
			return
		}
		u, err := app.UserByID(id)
		if err != nil {
			githubDown(w)
			return
		}
		if s.now().Sub(u.CreatedAt) < time.Duration(s.cfg.MinAccountAgeDays)*24*time.Hour {
			writeErr(w, http.StatusForbidden, "account_too_new")
			return
		}
	}
	if err := s.Store.PutScore(r.Context(), id, name, n, s.now()); err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"score": n})
}

func (s *Server) deleteScore(w http.ResponseWriter, r *http.Request) {
	id, _, ok := s.user(w, r)
	if !ok || !s.throttle(w, id) {
		return
	}
	if err := s.Store.DeleteScore(r.Context(), id, specName(r)); err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// myScore is never cached: it belongs to one person (API-17).
func (s *Server) myScore(w http.ResponseWriter, r *http.Request) {
	id, _, ok := s.user(w, r)
	if !ok {
		return
	}
	n, found, err := s.Store.Score(r.Context(), id, specName(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if !found {
		writeJSON(w, http.StatusOK, map[string]any{"score": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"score": n})
}

// scoresJSON is the public average and count per spec, with no user data (API-20).
func (s *Server) scoresJSON(w http.ResponseWriter, r *http.Request) {
	sums, err := s.Store.ScoreSums(r.Context())
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "store")
		return
	}
	doc := scores.Doc{GeneratedAt: s.now().UTC().Format(time.RFC3339), Scores: []scores.Score{}}
	for _, x := range sums {
		if x.Count > 0 {
			doc.Scores = append(doc.Scores, scores.Score{Spec: x.Spec, Average: scores.Average(x.Sum, x.Count), Count: int(x.Count)})
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, doc)
}
