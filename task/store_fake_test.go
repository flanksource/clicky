package task

import (
	"context"
	"sync"
	"testing"
	"time"
)

// storeWriteTimeout bounds how long a test waits for the background writer.
// Generous on purpose: the writer is a goroutine competing with the manager's
// own, and a flaky wait here would be indistinguishable from a real regression.
const storeWriteTimeout = 5 * time.Second

// testStore is an in-memory Store that reports every save on a channel, so a
// test can wait for the background writer instead of sleeping.
type testStore struct {
	saves chan RunRecord

	mu        sync.Mutex
	runs      map[string]RunRecord
	schedules map[string]Schedule
	fires     map[string][]Fire
	controls  []string
}

func newTestStore() *testStore {
	return &testStore{
		saves:     make(chan RunRecord, 64),
		runs:      map[string]RunRecord{},
		schedules: map[string]Schedule{},
		fires:     map[string][]Fire{},
	}
}

// installTestStore installs a fresh store for the duration of the test and
// detaches it afterwards, so the package-level writer never leaks between tests.
func installTestStore(t *testing.T) *testStore {
	t.Helper()
	store := newTestStore()
	ctx, cancel := context.WithCancel(context.Background())
	SetStore(ctx, store)
	t.Cleanup(func() {
		SetStore(context.Background(), nil)
		cancel()
	})
	return store
}

func (s *testStore) SaveRun(_ context.Context, groupID string, snapshots []TaskSnapshot) error {
	record, ok := RunFromSnapshots(snapshots)
	if !ok {
		record = RunRecord{RunMeta: RunMeta{ID: groupID}, Snapshots: snapshots}
	}
	s.mu.Lock()
	s.runs[record.ID] = record
	s.mu.Unlock()

	select {
	case s.saves <- record:
	default:
	}
	return nil
}

func (s *testStore) Runs(_ context.Context, filter RunFilter) ([]RunMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RunMeta, 0, len(s.runs))
	for _, record := range s.runs {
		if filter.Matches(record.RunMeta) {
			out = append(out, record.RunMeta)
		}
	}
	return out, nil
}

func (s *testStore) Snapshot(_ context.Context, id string) ([]TaskSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runs[id].Snapshots, nil
}

func (s *testStore) Control(_ context.Context, id string, action ControlAction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.controls = append(s.controls, id+":"+string(action))
	return nil
}

func (s *testStore) ListSchedules(context.Context) ([]Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Schedule, 0, len(s.schedules))
	for _, schedule := range s.schedules {
		out = append(out, schedule)
	}
	return out, nil
}

func (s *testStore) SaveSchedule(_ context.Context, schedule Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schedules[schedule.Name] = schedule
	return nil
}

func (s *testStore) DeleteSchedule(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.schedules, name)
	return nil
}

func (s *testStore) RecordFire(_ context.Context, name string, fire Fire) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fires[name] = append(s.fires[name], fire)
	return nil
}

// await blocks until the writer saves a run, failing the test if none arrives.
func (s *testStore) await(t *testing.T) RunRecord {
	t.Helper()
	select {
	case record := <-s.saves:
		return record
	case <-time.After(storeWriteTimeout):
		t.Fatal("timed out waiting for the store to save a run")
		return RunRecord{}
	}
}

// tryAwait reports whether a save arrives shortly, for asserting that one does
// not. The wait is short because the expected answer is "nothing happened".
func (s *testStore) tryAwait() (RunRecord, bool) {
	select {
	case record := <-s.saves:
		return record, true
	case <-time.After(250 * time.Millisecond):
		return RunRecord{}, false
	}
}

// drain discards saves already queued, so a test can assert on what happens next
// rather than on the setup that got it there.
func (s *testStore) drain() {
	for {
		select {
		case <-s.saves:
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
}

// schedule returns the stored definition for one name, zero if there is none.
func (s *testStore) schedule(name string) Schedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.schedules[name]
}

// firesFor returns the fires recorded for one schedule.
func (s *testStore) firesFor(name string) []Fire {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Fire(nil), s.fires[name]...)
}
