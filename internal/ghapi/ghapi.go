// Package ghapi is the small GitHub REST client the crawler and claim check use (IDX-8).
package ghapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrRateLimited = errors.New("rate limit nearly spent")
)

// MinRemaining stops the crawl before a quota runs dry, per rate-limit resource (IDX-8).
var MinRemaining = map[string]int{"core": 100, "search": 2}

// Client calls the GitHub API with one token.
type Client struct {
	Token   string
	BaseURL string // default https://api.github.com
	RawURL  string // default https://raw.githubusercontent.com
	HTTP    *http.Client

	Remaining map[string]int // by X-RateLimit-Resource; missing until the first response
	Calls     int
}

func New(token string) *Client {
	return &Client{Token: token, Remaining: map[string]int{}, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// resource names the rate-limit bucket a path draws from.
func resource(path string) string {
	if strings.HasPrefix(path, "/search/") {
		return "search"
	}
	return "core"
}

func (c *Client) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://api.github.com"
}

func (c *Client) raw() string {
	if c.RawURL != "" {
		return strings.TrimRight(c.RawURL, "/")
	}
	return "https://raw.githubusercontent.com"
}

// Do sends one API request and decodes JSON into out (if non-nil).
func (c *Client) Do(method, path string, body, out any) error {
	res := resource(path)
	if left, ok := c.Remaining[res]; ok && left < MinRemaining[res] {
		return ErrRateLimited
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base()+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "specfork-crawler")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	c.Calls++
	if r := resp.Header.Get("X-RateLimit-Remaining"); r != "" {
		if n, err := strconv.Atoi(r); err == nil {
			bucket := resp.Header.Get("X-RateLimit-Resource")
			if bucket == "" {
				bucket = res
			}
			if c.Remaining == nil {
				c.Remaining = map[string]int{}
			}
			c.Remaining[bucket] = n
		}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, truncate(string(data), 300))
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// Owner is a repo owner.
type Owner struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Type  string `json:"type"` // User or Organization
}

// Repo is the subset of repository fields Specfork reads (IDX-3).
type Repo struct {
	ID            int64    `json:"id"`
	FullName      string   `json:"full_name"`
	Owner         Owner    `json:"owner"`
	Stars         int      `json:"stargazers_count"`
	PushedAt      string   `json:"pushed_at"`
	CreatedAt     string   `json:"created_at"`
	Topics        []string `json:"topics"`
	DefaultBranch string   `json:"default_branch"`
	Fork          bool     `json:"fork"`
}

// HasTopic reports whether the repo carries topic t.
func (r Repo) HasTopic(t string) bool {
	for _, x := range r.Topics {
		if x == t {
			return true
		}
	}
	return false
}

// RepoByID looks a repo up by its numeric ID, following renames and transfers.
func (c *Client) RepoByID(id int64) (Repo, error) {
	var r Repo
	err := c.Do("GET", fmt.Sprintf("/repositories/%d", id), nil, &r)
	return r, err
}

// Repo looks a repo up by owner/name.
func (c *Client) Repo(full string) (Repo, error) {
	var r Repo
	err := c.Do("GET", "/repos/"+full, nil, &r)
	return r, err
}

// SearchCap is GitHub's limit on results per search query (IDX-2).
const SearchCap = 1000

// SearchRepos returns every repo matching q, splitting by created date when a
// query has more than SearchCap results (IDX-2).
func (c *Client) SearchRepos(q string) ([]Repo, error) {
	return c.searchRange(q, time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC), time.Now().UTC().Add(24*time.Hour), 0)
}

func (c *Client) searchRange(q string, from, to time.Time, depth int) ([]Repo, error) {
	query := q
	if depth > 0 {
		query = fmt.Sprintf("%s created:%s..%s", q, from.Format(time.RFC3339), to.Format(time.RFC3339))
	}
	var all []Repo
	for page := 1; ; page++ {
		var res struct {
			Total int    `json:"total_count"`
			Items []Repo `json:"items"`
		}
		path := fmt.Sprintf("/search/repositories?q=%s&per_page=100&page=%d&sort=updated", url.QueryEscape(query), page)
		if err := c.Do("GET", path, nil, &res); err != nil {
			return nil, err
		}
		if page == 1 && res.Total > SearchCap {
			if depth > 40 || !to.After(from.Add(time.Second)) {
				return nil, fmt.Errorf("search %q: %d results in one second", q, res.Total)
			}
			mid := from.Add(to.Sub(from) / 2)
			left, err := c.searchRange(q, from, mid, depth+1)
			if err != nil {
				return nil, err
			}
			right, err := c.searchRange(q, mid.Add(time.Second), to, depth+1)
			if err != nil {
				return nil, err
			}
			return append(left, right...), nil
		}
		all = append(all, res.Items...)
		if len(res.Items) < 100 || len(all) >= res.Total {
			return all, nil
		}
	}
}

// BranchSHA returns the head commit of a branch.
func (c *Client) BranchSHA(full, branch string) (string, error) {
	var b struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	err := c.Do("GET", "/repos/"+full+"/branches/"+url.PathEscape(branch), nil, &b)
	return b.Commit.SHA, err
}

// Tag is a git tag and the commit it points at.
type Tag struct {
	Name string
	SHA  string
}

// Tags lists every tag (up to 1,000).
func (c *Client) Tags(full string) ([]Tag, error) {
	var out []Tag
	for page := 1; page <= 10; page++ {
		var res []struct {
			Name   string `json:"name"`
			Commit struct {
				SHA string `json:"sha"`
			} `json:"commit"`
		}
		if err := c.Do("GET", fmt.Sprintf("/repos/%s/tags?per_page=100&page=%d", full, page), nil, &res); err != nil {
			return nil, err
		}
		for _, t := range res {
			out = append(out, Tag{Name: t.Name, SHA: t.Commit.SHA})
		}
		if len(res) < 100 {
			break
		}
	}
	return out, nil
}

// TreeFiles lists blob paths of a commit's tree.
func (c *Client) TreeFiles(full, sha string) ([]string, error) {
	var t struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := c.Do("GET", "/repos/"+full+"/git/trees/"+sha+"?recursive=1", nil, &t); err != nil {
		return nil, err
	}
	var out []string
	for _, e := range t.Tree {
		if e.Type == "blob" {
			out = append(out, e.Path)
		}
	}
	return out, nil
}

// Raw fetches a file at a commit from raw.githubusercontent.com (no API quota).
func (c *Client) Raw(full, sha, path string) ([]byte, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s/%s/%s", c.raw(), full, sha, path), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "specfork-crawler")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("raw %s@%s/%s: %s", full, sha, path, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

// Permission returns a user's permission on a repo: admin, maintain, write, triage, read or none.
func (c *Client) Permission(full, user string) (string, error) {
	var p struct {
		Permission string `json:"permission"`
		RoleName   string `json:"role_name"`
	}
	if err := c.Do("GET", "/repos/"+full+"/collaborators/"+url.PathEscape(user)+"/permission", nil, &p); err != nil {
		return "", err
	}
	if p.RoleName != "" {
		return p.RoleName, nil
	}
	return p.Permission, nil
}

// PR is the subset of pull request fields the claim check reads.
type PR struct {
	Number int    `json:"number"`
	Body   string `json:"body"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		SHA  string `json:"sha"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func (c *Client) PullRequest(full string, n int) (PR, error) {
	var pr PR
	err := c.Do("GET", fmt.Sprintf("/repos/%s/pulls/%d", full, n), nil, &pr)
	return pr, err
}

// PRFiles lists the paths a PR changes.
func (c *Client) PRFiles(full string, n int) ([]string, error) {
	var res []struct {
		Filename string `json:"filename"`
	}
	if err := c.Do("GET", fmt.Sprintf("/repos/%s/pulls/%d/files?per_page=100", full, n), nil, &res); err != nil {
		return nil, err
	}
	var out []string
	for _, f := range res {
		out = append(out, f.Filename)
	}
	return out, nil
}

func (c *Client) Comment(full string, n int, body string) error {
	return c.Do("POST", fmt.Sprintf("/repos/%s/issues/%d/comments", full, n), map[string]string{"body": body}, nil)
}

func (c *Client) Merge(full string, n int, title string) error {
	return c.Do("PUT", fmt.Sprintf("/repos/%s/pulls/%d/merge", full, n), map[string]string{"merge_method": "squash", "commit_title": title}, nil)
}
