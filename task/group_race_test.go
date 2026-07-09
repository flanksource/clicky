package task

import (
	"fmt"
	"sync"
	"testing"
)

// TestStartGroupSnapshotAllRace runs StartGroup concurrently with SnapshotAll;
// under -race it fails if global.groups is mutated without holding global.mu.
func TestStartGroupSnapshotAllRace(t *testing.T) {
	withTestGlobal(t)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			StartGroup[int](fmt.Sprintf("race-group-%d", n), WithKind("race-test"))
		}(i)
		go func() {
			defer wg.Done()
			SnapshotAll()
		}()
	}
	wg.Wait()
}
