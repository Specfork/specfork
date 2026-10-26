// Package index defines index.json (IDX-6) and loads it for the CLI (CLI-5).
package index

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/specfork/specfork/internal/semver"
	"github.com/specfork/specfork/internal/spec"
)

// DefaultURL is where the index workflow publishes (IDX-9).
const DefaultURL = "https://specfork.dev/index.json"

// CacheTTL bounds how long a fetched index is reused (CLI-5).
const CacheTTL = 10 * time.Minute

// Snapshot is what a spec declares at one commit.
type Snapshot struct {
	Requires  map[string]string `json:"requires,omitempty"`
	Overrides map[string]string `json:"overrides,omitempty"`
	Files     []string          `json:"files"`
}

// Version is a tagged release.
type Version struct {
	Version string `json:"version"`
	SHA     string `json:"sha"`
	Snapshot
	BuildsWith *BuildsWith `json:"builds_with"`
	Builds     []Build     `json:"builds"`
}

// Impl is an implementation repo (FMT-8).
type Impl struct {
	Repo    string   `json:"repo"`
	Stack   []string `json:"stack,omitempty"`
	URL     string   `json:"url,omitempty"`
	Version string   `json:"version,omitempty"`
}

// Spec is one indexed spec repo.
type Spec struct {
	Name            string            `json:"name"`
	Repo            string            `json:"repo"`
	RepoID          int64             `json:"repo_id"`
	OwnerID         int64             `json:"owner_id"`
	State           string            `json:"state"`
	Summary         string            `json:"summary"`
	License         string            `json:"license"`
	Tags            []string          `json:"tags"`
	Stars           int               `json:"stars"`
	PushedAt        string            `json:"pushed_at"`
	DerivedFrom     *spec.DerivedFrom `json:"derived_from,omitempty"`
	Base            *spec.Base        `json:"base,omitempty"`
	HeadSHA         string            `json:"head_sha"`
	Head            Snapshot          `json:"head"`
	Versions        []Version         `json:"versions"`
	Implementations []Impl            `json:"implementations"`
}

// Index is index.json.
type Index struct {
	GeneratedAt string   `json:"generated_at"`
	Specs       []Spec   `json:"specs"`
	Unclaimed   []string `json:"unclaimed"`
}

// Find returns the spec with that name.
func (ix *Index) Find(name string) (*Spec, bool) {
	for i := range ix.Specs {
		if ix.Specs[i].Name == name {
			return &ix.Specs[i], true
		}
	}
	return nil, false
}

// Resolvable reports whether the spec may be resolved (IDX-4).
func (s Spec) Resolvable() bool { return s.State == "active" }

// Latest returns the newest tagged version, or "head" when untagged.
func (s Spec) Latest() string {
	if len(s.Versions) > 0 {
		return s.Versions[0].Version
	}
	return "head"
}

// At returns the snapshot and SHA for a version ("head" included).
func (s *Spec) At(v semver.Version) (Snapshot, string, bool) {
	if v.Head {
		return s.Head, s.HeadSHA, s.HeadSHA != ""
	}
	for _, ver := range s.Versions {
		if pv, err := semver.Parse(ver.Version); err == nil && semver.Compare(pv, v) == 0 {
			return ver.Snapshot, ver.SHA, true
		}
	}
	return Snapshot{}, "", false
}

// Available lists every resolvable version, head included.
func (s *Spec) Available() []semver.Version {
	var out []semver.Version
	for _, ver := range s.Versions {
		if pv, err := semver.Parse(ver.Version); err == nil {
			out = append(out, pv)
		}
	}
	if s.HeadSHA != "" {
		out = append(out, semver.Version{Head: true})
	}
	return out
}

// RequiredBy lists specs whose head or latest version requires name.
func (ix *Index) RequiredBy(name string) []string {
	var out []string
	for _, s := range ix.Specs {
		snap := s.Head
		if len(s.Versions) > 0 {
			snap = s.Versions[0].Snapshot
		}
		if _, ok := snap.Requires[name]; ok {
			out = append(out, s.Name)
		}
	}
	sort.Strings(out)
	return out
}

// Search matches every word of q against name and summary, case-insensitively (CLI-1).
func (ix *Index) Search(q string) []Spec {
	words := strings.Fields(strings.ToLower(q))
	var out []Spec
	for _, s := range ix.Specs {
		hay := strings.ToLower(s.Name + " " + s.Summary)
		ok := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Stars != out[j].Stars {
			return out[i].Stars > out[j].Stars
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Source returns SPECFORK_INDEX or the default URL.
func Source() string {
	if s := os.Getenv("SPECFORK_INDEX"); s != "" {
		return s
	}
	return DefaultURL
}

// Load reads the index from a URL (cached for CacheTTL) or a file path.
func Load(src string) (*Index, error) {
	var data []byte
	var err error
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		data, err = fetchCached(src)
	} else {
		data, err = os.ReadFile(src)
	}
	if err != nil {
		return nil, err
	}
	var ix Index
	if err := json.Unmarshal(data, &ix); err != nil {
		return nil, fmt.Errorf("index %s: %w", src, err)
	}
	return &ix, nil
}

func cachePath(url string) string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	name := strings.NewReplacer("://", "_", "/", "_", ":", "_").Replace(url)
	return filepath.Join(dir, "specfork", name)
}

func fetchCached(url string) ([]byte, error) {
	p := cachePath(url)
	if p != "" {
		if st, err := os.Stat(p); err == nil && time.Since(st.ModTime()) < CacheTTL {
			if b, err := os.ReadFile(p); err == nil {
				return b, nil
			}
		}
	}
	b, err := Get(url)
	if err != nil {
		return nil, err
	}
	if p != "" {
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, b, 0o644)
	}
	return b, nil
}

// Client is shared by all plain HTTP fetches.
var Client = &http.Client{Timeout: 30 * time.Second}

// Get fetches a URL and fails on non-2xx.
func Get(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "specfork-cli")
	resp, err := Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return b, nil
}
