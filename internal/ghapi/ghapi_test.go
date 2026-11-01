package ghapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// IDX-2
func TestSearchPartition(t *testing.T) {
	var mu sync.Mutex
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		mu.Lock()
		queries = append(queries, q)
		mu.Unlock()
		w.Header().Set("X-RateLimit-Remaining", "4000")
		if !strings.Contains(q, "created:") {
			fmt.Fprint(w, `{"total_count": 1500, "items": []}`)
			return
		}
		fmt.Fprintf(w, `{"total_count": 1, "items": [{"id": %d, "full_name": "a/%d"}]}`, len(queries), len(queries))
	}))
	defer srv.Close()
	c := New("")
	c.BaseURL = srv.URL
	repos, err := c.SearchRepos("topic:specfork")
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 || len(queries) != 3 || !strings.Contains(queries[1], "created:2008-01-01T00:00:00Z..") {
		t.Fatalf("repos %v queries %v", repos, queries)
	}
}

// IDX-8
func TestRateLimitStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "99")
		fmt.Fprint(w, `{"id": 1}`)
	}))
	defer srv.Close()
	c := New("t")
	c.BaseURL = srv.URL
	if _, err := c.RepoByID(1); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RepoByID(1); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("got %v", err)
	}
	if c.Calls != 1 {
		t.Fatalf("calls %d", c.Calls)
	}
}

// IDX-8: the small search quota doesn't stop core calls, and vice versa.
func TestRateLimitPerResource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/search/") {
			w.Header().Set("X-RateLimit-Resource", "search")
			w.Header().Set("X-RateLimit-Remaining", "1")
			fmt.Fprint(w, `{"total_count": 0, "items": []}`)
			return
		}
		w.Header().Set("X-RateLimit-Resource", "core")
		w.Header().Set("X-RateLimit-Remaining", "4999")
		fmt.Fprint(w, `{"id": 1}`)
	}))
	defer srv.Close()
	c := New("t")
	c.BaseURL = srv.URL
	if _, err := c.SearchRepos("topic:specfork"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RepoByID(1); err != nil {
		t.Fatalf("core call blocked by the search quota: %v", err)
	}
	if _, err := c.SearchRepos("topic:specfork"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("search quota not honoured: %v", err)
	}
}

func TestNotFound(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	c := New("")
	c.BaseURL, c.RawURL = srv.URL, srv.URL
	if _, err := c.RepoByID(1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("api: %v", err)
	}
	if _, err := c.Raw("a/b", "sha", "README.md"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("raw: %v", err)
	}
}

// REG-2
func TestAutoMerge(t *testing.T) {
	for _, tc := range []struct {
		name, graphql string
		calls         []string
		fails         bool
	}{
		{"waits for checks", `{"data":{"enablePullRequestAutoMerge":{"clientMutationId":null}}}`, []string{"POST /graphql"}, false},
		{"already mergeable", `{"errors":[{"message":"Pull request Pull request is in clean status"}]}`, []string{"POST /graphql", "PUT /repos/a/b/pulls/7/merge"}, false},
		{"other error", `{"errors":[{"message":"Auto merge is not allowed for this repository"}]}`, []string{"POST /graphql"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.Method+" "+r.URL.Path)
				if r.URL.Path == "/graphql" {
					var body struct {
						Variables map[string]string `json:"variables"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Variables["id"] != "PR_x" || body.Variables["title"] != "chore(registry): claim @ann/booking" {
						t.Errorf("graphql variables: %v %v", body.Variables, err)
					}
					fmt.Fprint(w, tc.graphql)
					return
				}
				fmt.Fprint(w, `{"merged":true}`)
			}))
			defer srv.Close()
			c := New("")
			c.BaseURL = srv.URL
			err := c.AutoMerge("a/b", PR{Number: 7, NodeID: "PR_x"}, "chore(registry): claim @ann/booking")
			if (err != nil) != tc.fails {
				t.Fatalf("err = %v", err)
			}
			if strings.Join(calls, ", ") != strings.Join(tc.calls, ", ") {
				t.Fatalf("calls = %v, want %v", calls, tc.calls)
			}
		})
	}
}
