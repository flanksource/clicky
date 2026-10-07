package task_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/task"
)

// failingRunSource is a RunSource whose run listing is unavailable.
type failingRunSource struct{ err error }

func (s failingRunSource) Runs(context.Context, task.RunFilter) ([]task.RunMeta, error) {
	return nil, s.err
}

func (s failingRunSource) Snapshot(context.Context, string) ([]task.TaskSnapshot, error) {
	return nil, s.err
}

func (s failingRunSource) Control(context.Context, string, task.ControlAction) error { return s.err }

var _ = Describe("run listing stream", func() {
	It("ends with an error event naming the failure when the run source cannot list runs", func() {
		recorder := httptest.NewRecorder()
		source := failingRunSource{err: errors.New("run store offline")}

		task.RunsSSEHandlerWithSource(source).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/tasks/runs/stream", nil))

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Header().Get("Content-Type")).To(Equal("text/event-stream"))
		Expect(recorder.Body.String()).To(Equal(
			"event: error\ndata: {\"error\":\"load snapshot: list external task runs: run store offline\"}\n\n"))
	})
})
