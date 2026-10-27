// Package stats holds the daily usage counters of the hosted MCP server (MCP-7, MCP-8).
// A counter is keyed by UTC day, tool and spec name; nothing else is kept.
package stats

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Window is how many days /stats.json covers (MCP-8).
const Window = 90

// Unknown is the spec name of failed calls and of names not in the index (MCP-7).
const Unknown = "unknown"

const dayFormat = "2006-01-02"

// Key identifies one counter.
type Key struct{ Day, Tool, Spec string }

// NewKey keys a call made at now.
func NewKey(now time.Time, tool, spec string) Key {
	return Key{Day: now.UTC().Format(dayFormat), Tool: tool, Spec: spec}
}

// Row is one counter as served by /stats.json.
type Row struct {
	Day   string `json:"day"`
	Tool  string `json:"tool"`
	Spec  string `json:"spec"`
	Count int    `json:"count"`
}

// Store keeps counters.
type Store interface {
	Add(Key) error
	// Rows returns the counters of the last Window days up to now, sorted by day, tool, spec.
	Rows(now time.Time) []Row
}

// Memory is a Store in memory.
type Memory struct {
	mu sync.Mutex
	m  map[Key]int
}

func NewMemory() *Memory { return &Memory{m: map[Key]int{}} }

func (s *Memory) Add(k Key) error {
	s.mu.Lock()
	s.m[k]++
	s.mu.Unlock()
	return nil
}

func (s *Memory) Rows(now time.Time) []Row {
	since := now.UTC().AddDate(0, 0, -(Window - 1)).Format(dayFormat)
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []Row{}
	for k, n := range s.m {
		if k.Day >= since {
			rows = append(rows, Row{Day: k.Day, Tool: k.Tool, Spec: k.Spec, Count: n})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Day != b.Day {
			return a.Day < b.Day
		}
		if a.Tool != b.Tool {
			return a.Tool < b.Tool
		}
		return a.Spec < b.Spec
	})
	return rows
}

// File is a Memory that survives restarts: every change rewrites one JSON file atomically.
type File struct {
	*Memory
	path string
	wmu  sync.Mutex
}

// OpenFile loads path if it exists.
func OpenFile(path string) (*File, error) {
	f := &File{Memory: NewMemory(), path: path}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return f, nil
	}
	if err != nil {
		return nil, err
	}
	var rows []Row
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for _, r := range rows {
		f.m[Key{r.Day, r.Tool, r.Spec}] = r.Count
	}
	return f, nil
}

func (f *File) Add(k Key) error {
	f.Memory.Add(k)
	f.wmu.Lock()
	defer f.wmu.Unlock()
	f.Memory.mu.Lock()
	rows := make([]Row, 0, len(f.m))
	for k, n := range f.m {
		rows = append(rows, Row{Day: k.Day, Tool: k.Tool, Spec: k.Spec, Count: n})
	}
	f.Memory.mu.Unlock()
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		return a.Day+"\x00"+a.Tool+"\x00"+a.Spec < b.Day+"\x00"+b.Tool+"\x00"+b.Spec
	})
	b, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), ".stats-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.path)
}

// Fetch reads a /stats.json document.
func Fetch(url string) ([]Row, error) {
	c := &http.Client{Timeout: 15 * time.Second}
	res, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, res.Status)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	var rows []Row
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return rows, nil
}

// Resolves is how often a spec was resolved (MCP-9). Total covers the Window served by /stats.json.
type Resolves struct{ Last30, Total int }

// ResolvesBySpec sums resolve_spec counters per spec name. Specs without any appear nowhere.
func ResolvesBySpec(rows []Row, now time.Time) map[string]Resolves {
	since := now.UTC().AddDate(0, 0, -29).Format(dayFormat)
	out := map[string]Resolves{}
	for _, r := range rows {
		if r.Tool != "resolve_spec" || r.Spec == "" || r.Spec == Unknown || r.Count <= 0 {
			continue
		}
		v := out[r.Spec]
		v.Total += r.Count
		if r.Day >= since {
			v.Last30 += r.Count
		}
		out[r.Spec] = v
	}
	return out
}
