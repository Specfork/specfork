package server

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/specfork/specfork/internal/stats"
)

func osReadFile(p string) ([]byte, error) { return os.ReadFile(p) }

func pemOf(t *testing.T) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key(t))}))
}

// pgOrSkip opens a clean database from TEST_DATABASE_URL.
func pgOrSkip(t *testing.T) *PG {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pg, err := OpenPG(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pg.Close)
	for _, tb := range []string{"scores", "users", "counters"} {
		if _, err := pg.pool.Exec(ctx, "DELETE FROM "+tb); err != nil {
			t.Fatal(err)
		}
	}
	return pg
}

// API-4
func TestMigrateIdempotent(t *testing.T) {
	pg := pgOrSkip(t)
	ctx := context.Background()
	if err := pg.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	pg.pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n)
	entries, _ := migrations.ReadDir("migrations")
	if n != len(entries) {
		t.Fatalf("%d migrations recorded for %d files", n, len(entries))
	}
}

// API-14: no column can hold email, password, token or address data.
func TestSchemaColumns(t *testing.T) {
	sql, _ := migrations.ReadFile("migrations/001_init.sql")
	if banned := regexp.MustCompile(`(?i)\b(email|mail|password|passwd|token|secret|ip|ip_address|user_agent|body|text_content)\b`).FindString(string(sql)); banned != "" {
		t.Errorf("migration mentions %q", banned)
	}
	for _, want := range []string{"github_id", "login", "signed_in_at", "spec", "score", "updated_at", "day", "tool", "count"} {
		if !strings.Contains(string(sql), want) {
			t.Errorf("migration lacks %s", want)
		}
	}
	if os.Getenv("TEST_DATABASE_URL") == "" {
		return
	}
	pg := pgOrSkip(t)
	rows, err := pg.pool.Query(context.Background(), `SELECT table_name, column_name FROM information_schema.columns WHERE table_schema = 'public' ORDER BY 1, 2`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string][]string{}
	for rows.Next() {
		var tb, col string
		rows.Scan(&tb, &col)
		got[tb] = append(got[tb], col)
	}
	want := map[string]string{"users": "github_id login signed_in_at", "scores": "github_id score spec updated_at", "counters": "count day spec tool", "schema_migrations": "version"}
	if len(got) != len(want) {
		t.Fatalf("tables %v", got)
	}
	for tb, cols := range want {
		if strings.Join(got[tb], " ") != cols {
			t.Errorf("%s columns %v, want %s", tb, got[tb], cols)
		}
	}
}

// MCP-10
func TestCounterPostgres(t *testing.T) {
	pg := pgOrSkip(t)
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	k := stats.NewKey(now, "resolve_spec", "@specfork/scheduling")
	for i := 0; i < 3; i++ {
		if err := pg.Add(k); err != nil {
			t.Fatal(err)
		}
	}
	pg.Add(stats.NewKey(now.AddDate(0, 0, -95), "resolve_spec", "@old/spec"))
	rows, err := pg.RowsErr(now)
	if err != nil || len(rows) != 1 || rows[0] != (stats.Row{Day: "2026-10-27", Tool: "resolve_spec", Spec: "@specfork/scheduling", Count: 3}) {
		t.Fatalf("%v %+v", err, rows)
	}
}

// API-13, API-15: the Postgres store behaves like the in-memory one the handler tests use.
func TestPostgresStore(t *testing.T) {
	pg := pgOrSkip(t)
	ctx := context.Background()
	now := time.Now()
	pg.UpsertUser(ctx, 1, "ann", now)
	pg.UpsertUser(ctx, 1, "ann2", now)
	if l, ok, _ := pg.User(ctx, 1); !ok || l != "ann2" {
		t.Fatalf("%q %v", l, ok)
	}
	pg.PutScore(ctx, 1, "@a/b", 4, now)
	pg.PutScore(ctx, 1, "@a/b", 2, now)
	if n, ok, _ := pg.Score(ctx, 1, "@a/b"); !ok || n != 2 {
		t.Fatalf("%d %v", n, ok)
	}
	if sums, _ := pg.ScoreSums(ctx); len(sums) != 1 || sums[0] != (ScoreSum{"@a/b", 2, 1}) {
		t.Fatalf("%+v", sums)
	}
	pg.DeleteUser(ctx, 1)
	if sums, _ := pg.ScoreSums(ctx); len(sums) != 0 {
		t.Fatalf("scores survived the account: %+v", sums)
	}
	if err := pg.PutScore(ctx, 1, "@a/b", 9, now); err == nil {
		t.Error("score 9 accepted")
	}
}
