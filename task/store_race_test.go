package task

import (
	"context"
	"sync"
	"testing"
)

// TestSetStoreRacesWithEnqueue swaps and detaches the store while runs are being
// handed to it. SetStore closes the write queue; enqueueRunWrite sends on it. If
// the close is not serialised against the send, this is a send on a closed
// channel — a panic that takes the process down, not merely a reported race — so
// under -race (and, when unlucky, without it) this fails if the two are allowed
// to overlap.
func TestSetStoreRacesWithEnqueue(t *testing.T) {
	withTestGlobal(t)
	t.Cleanup(func() { SetStore(context.Background(), nil) })

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					persistTerminalRun("race-run")
				}
			}
		}()
	}

	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		SetStore(ctx, newTestStore())
		SetStore(context.Background(), nil)
		cancel()
	}

	close(stop)
	wg.Wait()
}
