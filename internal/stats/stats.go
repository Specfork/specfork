// Package stats holds the daily usage counters of the hosted MCP server (MCP-7, MCP-8).
// A row is a UTC day, a tool and a spec name with a count; nothing else is stored.
package stats

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

// Unknown is the spec name for failed calls and names not in the index (MCP-7).
const Unknown = "unknown"

// Window is how many days /stats.json covers (MCP-8).
const Window = 90

// Row is one counter.
type Row struct {
	Day   string `json:"day"`
	Tool  string `json:"tool"`
	Spec  string `json:"spec"`
	Count int    `json:"count"`
}

// Store keeps counters. Day is "2006-01-02" in UTC.
type Store interface {
	Add(day, tool, spec string) error
	// Rows returns every counter with day >= since, sorted by day, tool, spec.
	Rows(since string) ([]Row, error)
}

type key struct{ day, tool, spec string }

// Mem is an in-memory Store; File embeds it and saves after each Add.
type Mem struct {
	mu     sync.Mutex
	counts map[key]int
}

func (m *Mem) Add(day, tool, spec string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counts == nil {
		m.counts = map[key]int{}
	}
	m.counts[key{day, tool, spec}]++
	return nil
}

func (m *Mem) Rows(since string) ([]Row, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rows := []Row{}
	for k, n := range m.counts {
		if k.day >= since {
			rows = append(rows, Row{k.day, k.tool, k.spec, n})
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
	return rows, nil
}

// File is a Store persisted as a JSON array of rows.
type File struct {
	Mem
	path string
}

// OpenFile loads path if it exists.
func OpenFile(path string) (*File, error) {
	f := &File{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return nil, err
	}
	var rows []Row
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	f.counts = map[key]int{}
	for _, r := range rows {
		f.counts[key{r.Day, r.Tool, r.Spec}] = r.Count
	}
	return f, nil
}

func (f *File) Add(day, tool, spec string) error {
	if err := f.Mem.Add(day, tool, spec); err != nil {
		return err
	}
	rows, _ := f.Rows("")
	data, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

// Day formats t as a UTC day.
func Day(t time.Time) string { return t.UTC().Format("2006-01-02") }

// Fetch reads a /stats.json document.
func Fetch(url string) ([]Row, error) {
	c := http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	var rows []Row
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return rows, nil
}

// Resolves is a spec's resolve_spec calls in the last 30 days up to now, and in all rows given (MCP-9).
func Resolves(rows []Row, spec string, now time.Time) (last30, total int) {
	since := Day(now.AddDate(0, 0, -29))
	for _, r := range rows {
		if r.Tool != "resolve_spec" || r.Spec != spec {
			continue
		}
		total += r.Count
		if r.Day >= since {
			last30 += r.Count
		}
	}
	return
}
