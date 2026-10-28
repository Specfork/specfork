package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	sessionCookie = "__Host-session"
	oauthCookie   = "__Host-oauth"
	sessionTTL    = 30 * 24 * time.Hour
	oauthTTL      = 10 * time.Minute
)

// signer makes cookie values the server can verify without keeping state (API-10).
type signer struct{ key []byte }

func (s signer) mac(msg string) []byte {
	h := hmac.New(sha256.New, s.key)
	h.Write([]byte(msg))
	return h.Sum(nil)
}

// seal signs a JSON payload.
func (s signer) seal(v any) string {
	b, _ := json.Marshal(v)
	p := base64.RawURLEncoding.EncodeToString(b)
	return p + "." + base64.RawURLEncoding.EncodeToString(s.mac(p))
}

// open verifies the signature and decodes the payload into v.
func (s signer) open(value string, v any) bool {
	p, sig, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, s.mac(p)) {
		return false
	}
	b, err := base64.RawURLEncoding.DecodeString(p)
	return err == nil && json.Unmarshal(b, v) == nil
}

type sessionData struct {
	ID  int64 `json:"id"`
	Exp int64 `json:"exp"`
}

type oauthData struct {
	State  string `json:"s"`
	Return string `json:"r"`
	Exp    int64  `json:"e"`
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) setCookie(w http.ResponseWriter, name, value string, ttl time.Duration) {
	c := &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: int(ttl / time.Second)}
	if ttl < 0 {
		c.MaxAge = -1
	}
	http.SetCookie(w, c)
}

func (s *Server) startSession(w http.ResponseWriter, id int64) {
	s.setCookie(w, sessionCookie, s.sg.seal(sessionData{ID: id, Exp: s.now().Add(sessionTTL).Unix()}), sessionTTL)
}

// sessionID is the signed-in GitHub user ID; a tampered or expired cookie counts as signed out.
func (s *Server) sessionID(r *http.Request) (int64, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return 0, false
	}
	var d sessionData
	if !s.sg.open(c.Value, &d) || d.ID <= 0 || s.now().Unix() >= d.Exp {
		return 0, false
	}
	return d.ID, true
}

// returnPath keeps a sign-in redirect on the site: a path, or a full URL on the site's own origin.
func (s *Server) returnPath(v string) string {
	u, err := url.Parse(v)
	if err != nil || v == "" || strings.Contains(v, `\`) {
		return "/"
	}
	if u.Scheme != "" || u.Host != "" {
		site, _ := url.Parse(s.cfg.SiteURL)
		if u.Scheme != site.Scheme || u.Host != site.Host {
			return "/"
		}
		u = &url.URL{Path: u.Path, RawQuery: u.RawQuery}
	}
	if !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") {
		return "/"
	}
	return u.String()
}
