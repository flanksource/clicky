package aichat

import (
	"testing"
	"time"

	capapi "github.com/flanksource/captain/pkg/api"
)

func (f *fakeStreamProvider) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// countingFactory returns a factory that builds a fresh fake per call, plus a
// pointer to the call count and the list of providers it built.
func countingFactory() (AgentProviderFactory, *int, *[]*fakeStreamProvider) {
	var calls int
	var made []*fakeStreamProvider
	factory := func(cfg capapi.Config) (capapi.StreamingProvider, error) {
		calls++
		p := &fakeStreamProvider{model: cfg.Model.Name, backend: cfg.Model.Backend}
		made = append(made, p)
		return p, nil
	}
	return factory, &calls, &made
}

var testAgentConfig = capapi.Config{Model: capapi.Model{Name: "claude-agent-sonnet", Backend: capapi.BackendClaudeAgent}}

func TestProviderPoolReusesEntry(t *testing.T) {
	factory, calls, _ := countingFactory()
	pool := newProviderPool(factory, time.Minute)

	e1, err := pool.acquire("k", testAgentConfig)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	e2, err := pool.acquire("k", testAgentConfig)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if e1 != e2 {
		t.Errorf("acquire returned different entries for the same key")
	}
	if *calls != 1 {
		t.Errorf("factory calls = %d, want 1 (entry reused)", *calls)
	}
}

func TestProviderPoolRekey(t *testing.T) {
	factory, calls, _ := countingFactory()
	pool := newProviderPool(factory, time.Minute)

	e1, _ := pool.acquire("pending:1", testAgentConfig)
	pool.rekey("pending:1", "session:s")

	e2, _ := pool.acquire("session:s", testAgentConfig)
	if e1 != e2 {
		t.Errorf("rekey lost the entry: acquire built a new provider")
	}
	if *calls != 1 {
		t.Errorf("factory calls = %d, want 1 after rekey", *calls)
	}
}

func TestProviderPoolEvictsIdle(t *testing.T) {
	factory, calls, made := countingFactory()
	pool := newProviderPool(factory, time.Minute)
	now := time.Unix(1000, 0)
	pool.now = func() time.Time { return now }

	if _, err := pool.acquire("k", testAgentConfig); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	first := (*made)[0]

	now = now.Add(2 * time.Minute) // exceed the 1m TTL
	pool.evictIdle()

	if !first.isClosed() {
		t.Errorf("idle provider was not closed on eviction")
	}
	if _, err := pool.acquire("k", testAgentConfig); err != nil {
		t.Fatalf("re-acquire: %v", err)
	}
	if *calls != 2 {
		t.Errorf("factory calls = %d, want 2 (evicted entry rebuilt)", *calls)
	}
}

func TestProviderPoolEvictSkipsInUse(t *testing.T) {
	factory, calls, made := countingFactory()
	pool := newProviderPool(factory, time.Minute)
	now := time.Unix(1000, 0)
	pool.now = func() time.Time { return now }

	e1, _ := pool.acquire("k", testAgentConfig)
	e1.turn.Lock() // simulate a turn in progress

	now = now.Add(2 * time.Minute)
	pool.evictIdle() // must skip the locked entry

	pool.mu.Lock()
	_, present := pool.entries["k"]
	pool.mu.Unlock()
	e1.turn.Unlock()

	if !present {
		t.Errorf("evicted an entry whose turn was in progress")
	}
	if (*made)[0].isClosed() {
		t.Errorf("closed a provider whose turn was in progress")
	}
	if *calls != 1 {
		t.Errorf("factory calls = %d, want 1 (no rebuild)", *calls)
	}
}

func TestProviderPoolCloseAll(t *testing.T) {
	factory, _, made := countingFactory()
	pool := newProviderPool(factory, time.Minute)

	if _, err := pool.acquire("a", testAgentConfig); err != nil {
		t.Fatalf("acquire a: %v", err)
	}
	if _, err := pool.acquire("b", testAgentConfig); err != nil {
		t.Fatalf("acquire b: %v", err)
	}

	pool.closeAll()

	for i, p := range *made {
		if !p.isClosed() {
			t.Errorf("provider %d not closed by closeAll", i)
		}
	}
	// After draining, a new acquire rebuilds.
	factoryCallsBefore := len(*made)
	if _, err := pool.acquire("a", testAgentConfig); err != nil {
		t.Fatalf("acquire after close: %v", err)
	}
	if len(*made) != factoryCallsBefore+1 {
		t.Errorf("closeAll did not clear entries; got %d providers", len(*made))
	}
}
