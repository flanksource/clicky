package task_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/task"
)

// unknownRunSource refuses every id it is asked about, the way a store that
// validates run ids does. A stream must never ask it about runs it holds itself.
type unknownRunSource struct{ failingRunSource }

func (unknownRunSource) Runs(context.Context, task.RunFilter) ([]task.RunMeta, error) {
	return nil, nil
}

var _ = Describe("task stream over an external run source", func() {
	It("does not follow unrelated local or external runs when a kind has no matches", func() {
		run := task.StartManagedRun("unrelated kind run", task.WithKind("other-kind"))
		defer run.Finish(task.StatusSuccess, nil)
		source := unknownRunSource{failingRunSource{err: errors.New("must not request an unrelated snapshot")}}
		request := httptest.NewRequest(http.MethodGet, "/tasks/stream?kind=missing-kind", nil)
		ctx, cancel := context.WithTimeout(request.Context(), 500*time.Millisecond)
		defer cancel()
		response := httptest.NewRecorder()
		task.SSEHandlerWithSource(source).ServeHTTP(response, request.WithContext(ctx))
		Expect(response.Code).To(Equal(http.StatusOK))
		Expect(response.Body.String()).To(BeEmpty())
	})

	It("re-reads a running external run at the external refresh cadence, not every tick", func() {
		const window = 1300 * time.Millisecond
		source := &recordingRunSource{snapshots: map[string][]task.TaskSnapshot{
			"remote-live": {{ID: "remote", GroupID: "remote-live", Type: "group", Status: string(task.StatusRunning)}},
		}}
		request := httptest.NewRequest(http.MethodGet, "/tasks/stream?tasks=remote-live", nil)
		ctx, cancel := context.WithTimeout(request.Context(), window)
		defer cancel()
		task.SSEHandlerWithSource(source).ServeHTTP(newStreamRecorder(), request.WithContext(ctx))

		// Six 200ms ticks fit in the window; a once-per-second refresh reads the
		// run at the first tick and once more after a second has passed.
		Expect(source.snapshotCallCount("remote-live")).To(BeNumerically("<=", 2))
	})

	It("ends the stream with an error event naming the failure when the source cannot list runs", func() {
		response := httptest.NewRecorder()
		source := failingRunSource{err: errors.New("run store offline")}

		task.SSEHandlerWithSource(source).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/tasks/stream", nil))

		Expect(response.Body.String()).To(ContainSubstring(
			"event: error\ndata: {\"error\":\"list external task runs: run store offline\"}\n\n"))
	})

	It("ends the stream with an error event when the source cannot load a run", func() {
		response := httptest.NewRecorder()
		source := failingRunSource{err: errors.New("snapshot store offline")}

		task.SSEHandlerWithSource(source).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/tasks/stream?tasks=remote-1", nil))

		Expect(response.Body.String()).To(ContainSubstring(
			"event: error\ndata: {\"error\":\"load external task run \\\"remote-1\\\": snapshot store offline\"}\n\n"))
	})

	It("never asks the source about a live run named in the request", func() {
		run := task.StartManagedRun("named live run")
		DeferCleanup(func() { run.Finish(task.StatusSuccess, nil) })
		source := unknownRunSource{failingRunSource{err: errors.New("unknown run id")}}
		request := httptest.NewRequest(http.MethodGet, "/tasks/stream?tasks=named+live+run", nil)
		ctx, cancel := context.WithTimeout(request.Context(), 500*time.Millisecond)
		defer cancel()
		stream := newStreamRecorder()

		task.SSEHandlerWithSource(source).ServeHTTP(stream, request.WithContext(ctx))

		Expect(stream.String()).To(ContainSubstring(`"groupId":"` + run.ID() + `"`))
		Expect(stream.String()).NotTo(ContainSubstring("event: error"))
	})
})
