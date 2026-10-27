package stats

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var day = time.Date(2026, 10, 27, 23, 59, 0, 0, time.FixedZone("x", -5*3600))

func TestFileStoreSurvivesRestart(t *testing.T) {
	p := filepath.Join(t.TempDir(), "stats.json")
	f, err := OpenFile(p)
	if err != nil {
		t.Fatal(err)
	}
	f.Add(NewKey(day, "resolve_spec", "@a/b"))
	f.Add(NewKey(day, "resolve_spec", "@a/b"))
	g, err := OpenFile(p)
	if err != nil {
		t.Fatal(err)
	}
	rows := g.Rows(day)
	if len(rows) != 1 || rows[0].Count != 2 || rows[0].Day != "2026-10-28" {
		t.Fatalf("rows %+v", rows)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}

func TestRowsWindow(t *testing.T) {
	m := NewMemory()
	m.Add(NewKey(day.AddDate(0, 0, -90), "t", "s"))
	m.Add(NewKey(day.AddDate(0, 0, -89), "t", "s"))
	if r := m.Rows(day); len(r) != 1 {
		t.Fatalf("rows %+v", r)
	}
}

func TestResolvesBySpec(t *testing.T) {
	rows := []Row{
		{"2026-10-28", "resolve_spec", "@a/b", 3},
		{"2026-08-01", "resolve_spec", "@a/b", 5},
		{"2026-10-28", "get_spec", "@a/b", 9},
		{"2026-10-28", "resolve_spec", Unknown, 9},
	}
	got := ResolvesBySpec(rows, day)
	if len(got) != 1 || got["@a/b"] != (Resolves{3, 8}) {
		t.Fatalf("got %+v", got)
	}
}
