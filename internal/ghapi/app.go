package ghapi

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// AppJWT signs the short-lived token a GitHub App uses to ask for installation tokens (API-24).
func AppJWT(appID int64, key *rsa.PrivateKey, now time.Time) (string, error) {
	enc := base64.RawURLEncoding
	head := enc.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]any{"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(9 * time.Minute).Unix(), "iss": appID})
	msg := head + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(msg))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return msg + "." + enc.EncodeToString(sig), nil
}

// InstallationToken trades an App JWT (the client's token) for an installation token.
func (c *Client) InstallationToken(installation int64) (string, time.Time, error) {
	var res struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := c.Do("POST", fmt.Sprintf("/app/installations/%d/access_tokens", installation), nil, &res); err != nil {
		return "", time.Time{}, err
	}
	return res.Token, res.ExpiresAt, nil
}

// User is the public part of a GitHub account.
type User struct {
	ID        int64     `json:"id"`
	Login     string    `json:"login"`
	CreatedAt time.Time `json:"created_at"`
}

// UserByID looks an account up by its numeric ID.
func (c *Client) UserByID(id int64) (User, error) {
	var u User
	err := c.Do("GET", fmt.Sprintf("/user/%d", id), nil, &u)
	return u, err
}

// Ref returns the commit a branch points at, or ErrNotFound.
func (c *Client) Ref(full, branch string) (string, error) {
	var r struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	err := c.Do("GET", "/repos/"+full+"/git/ref/heads/"+branch, nil, &r)
	return r.Object.SHA, err
}

// SetBranch points a branch at sha, creating it when missing.
func (c *Client) SetBranch(full, branch, sha string) error {
	if _, err := c.Ref(full, branch); err == nil {
		return c.Do("PATCH", "/repos/"+full+"/git/refs/heads/"+branch, map[string]any{"sha": sha, "force": true}, nil)
	} else if err != ErrNotFound {
		return err
	}
	return c.Do("POST", "/repos/"+full+"/git/refs", map[string]string{"ref": "refs/heads/" + branch, "sha": sha}, nil)
}

// FileSHA is the blob SHA of a file on a branch, which an update must quote.
func (c *Client) FileSHA(full, path, ref string) (string, error) {
	var f struct {
		SHA string `json:"sha"`
	}
	err := c.Do("GET", "/repos/"+full+"/contents/"+path+"?ref="+url.QueryEscape(ref), nil, &f)
	return f.SHA, err
}

// PutFile commits a new version of a file to a branch.
func (c *Client) PutFile(full, path, branch, message string, content []byte, blobSHA string) error {
	return c.Do("PUT", "/repos/"+full+"/contents/"+path, map[string]string{
		"message": message, "content": base64.StdEncoding.EncodeToString(content), "sha": blobSHA, "branch": branch,
	}, nil)
}

// OpenPR opens a pull request from a branch of the same repo and returns its URL.
func (c *Client) OpenPR(full, branch, base, title, body string) (string, error) {
	var pr struct {
		URL string `json:"html_url"`
	}
	err := c.Do("POST", "/repos/"+full+"/pulls", map[string]string{"title": title, "head": branch, "base": base, "body": body}, &pr)
	return pr.URL, err
}

// FindOpenPR returns the URL of an open pull request from a branch of the same repo.
func (c *Client) FindOpenPR(owner, repo, branch string) (string, bool, error) {
	var prs []struct {
		URL string `json:"html_url"`
	}
	q := "?state=open&head=" + url.QueryEscape(owner+":"+branch)
	if err := c.Do("GET", "/repos/"+owner+"/"+repo+"/pulls"+q, nil, &prs); err != nil {
		return "", false, err
	}
	if len(prs) == 0 {
		return "", false, nil
	}
	return prs[0].URL, true, nil
}
