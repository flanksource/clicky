package prompt

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// waitPending polls until the manager reports id as pending, so a test resolving a
// prompt from another goroutine does not race the Ask registration.
func waitPending(t *testing.T, m *Manager, id string) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if _, ok := m.Pending(id); ok {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("prompt %q never became pending", id)
}

func TestAskResolveRoundTrip(t *testing.T) {
	m := NewManager(NewMemory(MemoryConfig{}))
	type result struct {
		ans Answer
		err error
	}
	done := make(chan result, 1)
	go func() {
		ans, err := m.Ask(context.Background(), Prompt{
			ID:     "p1",
			Schema: SelectSchema("Pick", []string{"alpha", "beta", "gamma"}),
		})
		done <- result{ans, err}
	}()

	waitPending(t, m, "p1")
	if err := m.Resolve("p1", Answer{Values: map[string]any{"choice": "2"}}); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	got := <-done
	if got.err != nil {
		t.Fatalf("ask returned error: %v", got.err)
	}
	if idx := SelectedIndex(got.ans); idx != 2 {
		t.Fatalf("expected selected index 2, got %d", idx)
	}
	snap, ok := m.Snapshot("p1")
	if !ok || snap.State != string(StateAnswered) {
		t.Fatalf("expected answered snapshot, got %+v (ok=%v)", snap, ok)
	}
}

func TestResolveRejectsInvalidAnswer(t *testing.T) {
	m := NewManager(NewMemory(MemoryConfig{}))
	go func() {
		_, _ = m.Ask(context.Background(), Prompt{ID: "p2", Schema: SelectSchema("Pick", []string{"a", "b"})})
	}()
	waitPending(t, m, "p2")

	// "9" is not a valid enum index for a 2-option select.
	if err := m.Resolve("p2", Answer{Values: map[string]any{"choice": "9"}}); err == nil {
		t.Fatal("expected validation error for out-of-range choice")
	}
	// The prompt must stay pending so the producer can still be answered.
	if _, ok := m.Pending("p2"); !ok {
		t.Fatal("prompt should remain pending after a rejected answer")
	}
	if err := m.Resolve("p2", Answer{Values: map[string]any{"choice": "1"}}); err != nil {
		t.Fatalf("valid resolve after rejection failed: %v", err)
	}
}

func TestAskCancelledByContext(t *testing.T) {
	m := NewManager(NewMemory(MemoryConfig{}))
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		_, err := m.Ask(ctx, Prompt{ID: "p3", Schema: TextSchema("Name", false)})
		errc <- err
	}()
	waitPending(t, m, "p3")
	cancel()
	if err := <-errc; err == nil {
		t.Fatal("expected context cancellation error")
	}
	if _, ok := m.Pending("p3"); ok {
		t.Fatal("cancelled prompt should no longer be pending")
	}
}

func TestListFilterByOwnerAndLabels(t *testing.T) {
	m := NewManager(NewMemory(MemoryConfig{}))
	for _, p := range []Prompt{
		{ID: "a", Owner: "todo-1", Labels: map[string]string{"session": "s1"}, Schema: TextSchema("x", false)},
		{ID: "b", Owner: "todo-2", Labels: map[string]string{"session": "s2"}, Schema: TextSchema("x", false)},
	} {
		go func(p Prompt) { _, _ = m.Ask(context.Background(), p) }(p)
		waitPending(t, m, p.ID)
	}
	owned := m.List(Filter{Owner: "todo-1"})
	if len(owned) != 1 || owned[0].ID != "a" {
		t.Fatalf("owner filter returned %+v", owned)
	}
	bySession := m.List(Filter{Labels: map[string]string{"session": "s2"}})
	if len(bySession) != 1 || bySession[0].ID != "b" {
		t.Fatalf("label filter returned %+v", bySession)
	}
}

func TestResolveHandler(t *testing.T) {
	m := NewManager(NewMemory(MemoryConfig{}))
	mux := http.NewServeMux()
	m.RegisterHandlers(mux, "/api")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	answered := make(chan Answer, 1)
	go func() {
		ans, _ := m.Ask(context.Background(), Prompt{ID: "h1", Schema: SelectSchema("Pick", []string{"x", "y"})})
		answered <- ans
	}()
	waitPending(t, m, "h1")

	body, _ := json.Marshal(map[string]any{"values": map[string]any{"choice": "1"}})
	resp, err := http.Post(srv.URL+"/api/prompts/h1/answer", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post answer: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if idx := SelectedIndex(<-answered); idx != 1 {
		t.Fatalf("expected index 1 from HTTP resolve, got %d", idx)
	}
}

func TestSelectViaManagerWithScope(t *testing.T) {
	m := NewManager(NewMemory(MemoryConfig{}))
	ctx := WithScope(context.Background(), Scope{Owner: "todo-9", Labels: map[string]string{"session": "s9"}})
	got := make(chan []int, 1)
	go func() {
		idx, ok := m.Select(ctx, "Pick", []string{"a", "b", "c"}, SelectOptions{})
		if !ok {
			got <- nil
			return
		}
		got <- idx
	}()

	var id string
	for i := 0; i < 200; i++ {
		if ps := m.List(Filter{Owner: "todo-9"}); len(ps) == 1 {
			id = ps[0].ID
			break
		}
		time.Sleep(time.Millisecond)
	}
	if id == "" {
		t.Fatal("scoped prompt never appeared under its owner filter")
	}
	snap, _ := m.Snapshot(id)
	if snap.Labels["session"] != "s9" {
		t.Fatalf("ctx scope label not applied to prompt: %+v", snap.Labels)
	}
	if err := m.Resolve(id, Answer{Values: map[string]any{"choice": "2"}}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if idx := <-got; len(idx) != 1 || idx[0] != 2 {
		t.Fatalf("expected selected index [2], got %v", idx)
	}
}

func TestResolveTwiceRejectsSecondAndKeepsFirstAnswer(t *testing.T) {
	m := NewManager(NewMemory(MemoryConfig{}))
	done := make(chan struct{})
	go func() {
		_, _ = m.Ask(context.Background(), Prompt{ID: "r1", Schema: SelectSchema("Pick", []string{"a", "b"})})
		close(done)
	}()
	waitPending(t, m, "r1")

	if err := m.Resolve("r1", Answer{Values: map[string]any{"choice": "1"}}); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	<-done

	// The prompt was claimed by the first resolve, so a second (valid) answer must
	// be rejected rather than overwrite the stored snapshot.
	if err := m.Resolve("r1", Answer{Values: map[string]any{"choice": "0"}}); err == nil {
		t.Fatal("expected the second resolve to be rejected")
	}
	snap, _ := m.Snapshot("r1")
	if snap.Value["choice"] != "1" {
		t.Fatalf("stored answer changed after a rejected second resolve: %+v", snap.Value)
	}
}

func TestFilterMatchesRequiresLabelKeyPresence(t *testing.T) {
	f := Filter{Labels: map[string]string{"session": ""}}
	if f.matches(PromptSnapshot{}) {
		t.Fatal("a snapshot missing the label key must not match an empty-value label filter")
	}
	if !f.matches(PromptSnapshot{Labels: map[string]string{"session": ""}}) {
		t.Fatal("a snapshot carrying the exact empty-value label must match")
	}
}

func TestMemoryStoreExpiresResolvedSnapshotOnGet(t *testing.T) {
	store := NewMemory(MemoryConfig{Retention: time.Minute})
	stale := time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)
	if err := store.Set(PromptSnapshot{ID: "old", State: string(StateAnswered), ResolvedAt: stale}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, ok := store.Get("old"); ok {
		t.Fatal("expected a resolved snapshot older than the retention window to expire on Get")
	}
}

func TestSelectSchemaShape(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(SelectSchema("Choose", []string{"first", "second"}), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	props := doc["properties"].(map[string]any)
	choice := props["choice"].(map[string]any)
	labels := choice["x-enum-labels"].(map[string]any)
	if labels["0"] != "first" || labels["1"] != "second" {
		t.Fatalf("unexpected enum labels: %v", labels)
	}
	if choice["x-enum-display"] != "radio" {
		t.Fatalf("expected radio display, got %v", choice["x-enum-display"])
	}
}
