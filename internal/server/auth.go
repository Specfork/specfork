package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// login sends the browser to GitHub asking only for read:user (API-8).
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	state := randHex(16)
	ret := s.returnPath(r.URL.Query().Get("return"))
	s.setCookie(w, oauthCookie, s.sg.seal(oauthData{State: state, Return: ret, Exp: s.now().Add(oauthTTL).Unix()}), oauthTTL)
	q := url.Values{"client_id": {s.cfg.ClientID}, "scope": {"read:user"}, "state": {state}}
	http.Redirect(w, r, s.cfg.GitHubWeb+"/login/oauth/authorize?"+q.Encode(), http.StatusFound)
}

// callback finishes sign-in: it keeps the user's ID and login and throws the GitHub token away (API-9).
func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	var od oauthData
	c, err := r.Cookie(oauthCookie)
	valid := err == nil && s.sg.open(c.Value, &od) && s.now().Unix() < od.Exp &&
		od.State != "" && od.State == r.URL.Query().Get("state")
	s.setCookie(w, oauthCookie, "", -1)
	if !valid {
		s.authFailed(w, r, "/")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" || r.URL.Query().Get("error") != "" {
		s.authFailed(w, r, od.Return)
		return
	}
	token, status, err := s.exchange(code)
	if err != nil || status >= 500 {
		githubDown(w)
		return
	}
	if token == "" {
		s.authFailed(w, r, od.Return)
		return
	}
	var u struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	status, err = s.getJSON(s.cfg.GitHubAPI+"/user", token, &u)
	if err != nil || status >= 500 {
		githubDown(w)
		return
	}
	if status != http.StatusOK || u.ID <= 0 || u.Login == "" {
		s.authFailed(w, r, od.Return)
		return
	}
	if err := s.Store.UpsertUser(r.Context(), u.ID, u.Login, s.now()); err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	s.startSession(w, u.ID)
	http.Redirect(w, r, s.cfg.SiteURL+od.Return, http.StatusFound)
}

func (s *Server) authFailed(w http.ResponseWriter, r *http.Request, ret string) {
	sep := "?"
	if strings.Contains(ret, "?") {
		sep = "&"
	}
	http.Redirect(w, r, s.cfg.SiteURL+ret+sep+"auth=failed", http.StatusFound)
}

// exchange trades the OAuth code for a token; the token lives only in this call chain.
func (s *Server) exchange(code string) (token string, status int, err error) {
	form := url.Values{"client_id": {s.cfg.ClientID}, "client_secret": {s.cfg.ClientSecret}, "code": {code}}
	req, err := http.NewRequest("POST", s.cfg.GitHubWeb+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := s.HTTP.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out)
	return out.AccessToken, res.StatusCode, nil
}

func (s *Server) getJSON(u, token string, out any) (int, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "specfork-api")
	res, err := s.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(out)
	}
	return res.StatusCode, nil
}

// me is the signed-in account (API-11).
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	id, login, ok := s.user(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "login": login})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.setCookie(w, sessionCookie, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

// deleteMe removes the account and, with it, its scores (API-13).
func (s *Server) deleteMe(w http.ResponseWriter, r *http.Request) {
	id, _, ok := s.user(w, r)
	if !ok {
		return
	}
	if err := s.Store.DeleteUser(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	s.setCookie(w, sessionCookie, "", -1)
	w.WriteHeader(http.StatusNoContent)
}
