package server

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/specfork/specfork/internal/stats"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Store is what the handlers keep in the database (API-14).
type Store interface {
	Ping(ctx context.Context) error
	UpsertUser(ctx context.Context, id int64, login string, at time.Time) error
	User(ctx context.Context, id int64) (login string, ok bool, err error)
	DeleteUser(ctx context.Context, id int64) error
	PutScore(ctx context.Context, id int64, spec string, score int, at time.Time) error
	DeleteScore(ctx context.Context, id int64, spec string) error
	Score(ctx context.Context, id int64, spec string) (score int, ok bool, err error)
	ScoreSums(ctx context.Context) ([]ScoreSum, error)
}

// ScoreSum is the total and number of the scores of one spec.
type ScoreSum struct {
	Spec       string
	Sum, Count int64
}

// PG is the Postgres Store; it also keeps the MCP counters (MCP-10).
type PG struct{ pool *pgxpool.Pool }

// OpenPG connects and applies the migrations (API-4).
func OpenPG(ctx context.Context, url string) (*PG, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL: cannot connect")
	}
	p := &PG{pool}
	if err := p.Migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return p, nil
}

func (p *PG) Close() { p.pool.Close() }

// Migrate runs each numbered file once, in its own transaction.
func (p *PG) Migrate(ctx context.Context) error {
	if _, err := p.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		var done bool
		if err := p.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&done); err != nil {
			return fmt.Errorf("migrate %s: %w", name, err)
		}
		if done {
			continue
		}
		sql, _ := migrations.ReadFile("migrations/" + name)
		tx, err := p.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migrate %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migrate %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *PG) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

func (p *PG) UpsertUser(ctx context.Context, id int64, login string, at time.Time) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO users (github_id, login, signed_in_at) VALUES ($1, $2, $3)
		ON CONFLICT (github_id) DO UPDATE SET login = EXCLUDED.login, signed_in_at = EXCLUDED.signed_in_at`, id, login, at)
	return err
}

func (p *PG) User(ctx context.Context, id int64) (string, bool, error) {
	var login string
	err := p.pool.QueryRow(ctx, `SELECT login FROM users WHERE github_id = $1`, id).Scan(&login)
	if err == pgx.ErrNoRows {
		return "", false, nil
	}
	return login, err == nil, err
}

// DeleteUser removes the account; its scores go with it through the foreign key (API-13).
func (p *PG) DeleteUser(ctx context.Context, id int64) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM users WHERE github_id = $1`, id)
	return err
}

func (p *PG) PutScore(ctx context.Context, id int64, spec string, score int, at time.Time) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO scores (github_id, spec, score, updated_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (github_id, spec) DO UPDATE SET score = EXCLUDED.score, updated_at = EXCLUDED.updated_at`, id, spec, score, at)
	return err
}

func (p *PG) DeleteScore(ctx context.Context, id int64, spec string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM scores WHERE github_id = $1 AND spec = $2`, id, spec)
	return err
}

func (p *PG) Score(ctx context.Context, id int64, spec string) (int, bool, error) {
	var s int
	err := p.pool.QueryRow(ctx, `SELECT score FROM scores WHERE github_id = $1 AND spec = $2`, id, spec).Scan(&s)
	if err == pgx.ErrNoRows {
		return 0, false, nil
	}
	return s, err == nil, err
}

func (p *PG) ScoreSums(ctx context.Context) ([]ScoreSum, error) {
	rows, err := p.pool.Query(ctx, `SELECT spec, SUM(score)::bigint, COUNT(*)::bigint FROM scores GROUP BY spec ORDER BY spec`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScoreSum
	for rows.Next() {
		var s ScoreSum
		if err := rows.Scan(&s.Spec, &s.Sum, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Add counts one tool call with one atomic upsert (MCP-10).
func (p *PG) Add(k stats.Key) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := p.pool.Exec(ctx, `INSERT INTO counters (day, tool, spec, count) VALUES ($1, $2, $3, 1)
		ON CONFLICT (day, tool, spec) DO UPDATE SET count = counters.count + 1`, k.Day, k.Tool, k.Spec)
	return err
}

// RowsErr is the counters of the last stats.Window days; /stats.json answers 503 on its error (MCP-12).
func (p *PG) RowsErr(now time.Time) ([]stats.Row, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	since := now.UTC().AddDate(0, 0, -(stats.Window - 1)).Format("2006-01-02")
	rows, err := p.pool.Query(ctx, `SELECT day, tool, spec, count FROM counters WHERE day >= $1 ORDER BY day, tool, spec`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []stats.Row{}
	for rows.Next() {
		var r stats.Row
		var n int64
		if err := rows.Scan(&r.Day, &r.Tool, &r.Spec, &n); err != nil {
			return nil, err
		}
		r.Count = int(n)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *PG) Rows(now time.Time) []stats.Row {
	rows, _ := p.RowsErr(now)
	return rows
}
