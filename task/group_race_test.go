package task

import (
	"fmt"
	"sync"
	"testing"

	flanksourceContext "github.com/flanksource/commons/context"
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

// TestGroupReadersRaceWithAdd runs Group.Status/GetTasks/Duration concurrently
// with TypedGroup.Add; under -race it fails if g.Items is read without g.mu.
func TestGroupReadersRaceWithAdd(t *testing.T) {
	withTestGlobal(t)

	group := StartGroup[int]("reader-race-group")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			group.Add(fmt.Sprintf("reader-race-task-%d", n), func(ctx flanksourceContext.Context, tk *Task) (int, error) {
				return n, nil
			})
		}(i)
		go func() {
			defer wg.Done()
			group.Status()
			group.GetTasks()
			group.Duration()
		}()
	}
	wg.Wait()
	group.WaitFor()
}
