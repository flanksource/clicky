package task

import (
	"testing"

	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/stretchr/testify/assert"
)

type waitForPayload struct {
	N     int
	Label string
}

// TestGetResultsReturnsResultAfterInClosureSuccess guards against a result-loss
// race: a task closure that calls t.Success() (or any terminal SetStatus) BEFORE
// `return result` cancels the task's own context, because SetStatus cancels
// t.ctx on terminal statuses. WaitFor watches t.ctx.Done(); without the fix it
// returned the instant the closure self-cancelled — before runFunc stored the
// result — so GetResults handed back the zero value.
//
// Many iterations are run because the window between t.Success() and the result
// store is small; pre-fix this produced zero-valued results intermittently.
func TestGetResultsReturnsResultAfterInClosureSuccess(t *testing.T) {
	const iterations = 1000

	for i := 0; i < iterations; i++ {
		want := waitForPayload{N: i + 1, Label: "item"}
		group := StartGroup[waitForPayload]("self-cancel-result")
		group.Add("t", func(ctx flanksourceContext.Context, tk *Task) (waitForPayload, error) {
			// Mirror a real task body that marks itself done before returning.
			tk.SetName("running")
			tk.Success()
			return want, nil
		})

		results, err := group.GetResults()
		assert.NoError(t, err)
		assert.Len(t, results, 1)
		for _, got := range results {
			assert.Equal(t, want, got, "iteration %d: GetResults must return the populated result, not the zero value", i)
		}
	}
}
