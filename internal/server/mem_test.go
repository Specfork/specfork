package server

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// memStore is a Store in memory for handler tests.
type memStore struct {
	mu     sync.Mutex
	users  map[int64]string
	scores map[int64]map[string]int
	down   bool
}

func newMem() *memStore {
	return &memStore{users: map[int64]string{}, scores: map[int64]map[string]int{}}
}

var errDown = errors.New("db down")

func (m *memStore) Ping(context.Context) error {
	if m.down {
		return errDown
	}
	return nil
}

func (m *memStore) UpsertUser(_ context.Context, id int64, login string, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[id] = login
	return nil
}

func (m *memStore) User(_ context.Context, id int64) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.users[id]
	return l, ok, nil
}

func (m *memStore) DeleteUser(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.users, id)
	delete(m.scores, id)
	return nil
}

func (m *memStore) PutScore(_ context.Context, id int64, spec string, n int, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.scores[id] == nil {
		m.scores[id] = map[string]int{}
	}
	m.scores[id][spec] = n
	return nil
}

func (m *memStore) DeleteScore(_ context.Context, id int64, spec string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.scores[id], spec)
	return nil
}

func (m *memStore) Score(_ context.Context, id int64, spec string) (int, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.scores[id][spec]
	return n, ok, nil
}

func (m *memStore) ScoreSums(context.Context) ([]ScoreSum, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	by := map[string]*ScoreSum{}
	for _, per := range m.scores {
		for spec, n := range per {
			if by[spec] == nil {
				by[spec] = &ScoreSum{Spec: spec}
			}
			by[spec].Sum += int64(n)
			by[spec].Count++
		}
	}
	var out []ScoreSum
	for _, s := range by {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Spec < out[j].Spec })
	return out, nil
}
