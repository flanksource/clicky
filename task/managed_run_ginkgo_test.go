package task_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/metrics"
	"github.com/flanksource/clicky/task"
)

type recordingController struct {
	actions []task.ControlAction
	called  task.ControlAction
}

type recordingRunSource struct {
	runs       []task.RunMeta
	snapshots  map[string][]task.TaskSnapshot
	controlled string
	points     []metrics.Point
}

func (s *recordingRunSource) Runs(_ context.Context, filter task.RunFilter) ([]task.RunMeta, error) {
	var runs []task.RunMeta
	for _, run := range s.runs {
		if filter.Matches(run) {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (s *recordingRunSource) Snapshot(_ context.Context, id string) ([]task.TaskSnapshot, error) {
	return s.snapshots[id], nil
}

func (s *recordingRunSource) Control(_ context.Context, id string, action task.ControlAction) error {
	s.controlled = id + ":" + string(action)
	return nil
}

func (s *recordingRunSource) QueryMetric(_ context.Context, _ metrics.QueryRequest) ([]metrics.Point, error) {
	return s.points, nil
}

func (c *recordingController) Actions() []task.ControlAction {
	return append([]task.ControlAction(nil), c.actions...)
}

func (c *recordingController) Control(_ context.Context, action task.ControlAction) error {
	c.called = action
	return nil
}

var _ = Describe("Managed task runs", func() {
	It("exposes href, output, details, and controls without occupying a worker", func() {
		controller := &recordingController{actions: []task.ControlAction{task.ControlStop, task.ControlRestart}}
		run := task.StartManagedRun(
			"api",
			task.WithKind("supervised-process"),
			task.WithHref("/tasks/api-run"),
			task.WithController(controller),
		)
		run.SetOutputProvider(func() task.OutputSnapshot {
			return task.OutputSnapshot{Stdout: "ready\n", Stderr: "warning\n"}
		})
		run.SetDetailsProvider(func() any {
			return map[string]any{"pid": 1234.0, "status": "running"}
		})

		runs := task.Runs(task.RunFilter{Kind: "supervised-process"})
		var meta *task.RunMeta
		for i := range runs {
			if runs[i].ID == run.ID() {
				meta = &runs[i]
				break
			}
		}
		Expect(meta).ToNot(BeNil())
		Expect(meta.Href).To(Equal("/tasks/api-run"))
		Expect(meta.Controls).To(Equal([]task.ControlAction{task.ControlStop, task.ControlRestart}))

		snapshots := task.SnapshotByID(run.ID())
		Expect(snapshots).To(HaveLen(2))
		Expect(snapshots[0].Href).To(Equal("/tasks/api-run"))
		Expect(snapshots[0].Details).To(Equal(map[string]any{"pid": 1234.0, "status": "running"}))
		Expect(snapshots[1].Stdout).To(Equal("ready\n"))
		Expect(snapshots[1].Stderr).To(Equal("warning\n"))

		Expect(task.ControlRun(context.Background(), run.ID(), task.ControlRestart)).To(Succeed())
		Expect(controller.called).To(Equal(task.ControlRestart))
	})

	It("freezes terminal output and details and aggregates cancellation correctly", func() {
		stdout := "first"
		pid := 10
		run := task.StartManagedRun("worker")
		run.SetOutputProvider(func() task.OutputSnapshot { return task.OutputSnapshot{Stdout: stdout} })
		run.SetDetailsProvider(func() any { return map[string]any{"pid": pid} })

		run.Finish(task.StatusCancelled, errors.New("stopped"))
		stdout = "second"
		pid = 20

		snapshots := task.SnapshotByID(run.ID())
		Expect(snapshots[0].Status).To(Equal(string(task.StatusCancelled)))
		Expect(snapshots[0].Details).To(Equal(map[string]any{"pid": 10}))
		Expect(snapshots[1].Stdout).To(Equal("first"))
		Expect(snapshots[1].Error).To(Equal("stopped"))
	})

	It("retains only the configured tail of each stream in snapshots", func() {
		prefix := strings.Repeat("x", task.SnapshotStreamLimit)
		run := task.StartManagedRun("bounded")
		run.SetOutputProvider(func() task.OutputSnapshot {
			return task.OutputSnapshot{Stdout: prefix + "stdout-tail", Stderr: prefix + "stderr-tail"}
		})

		snapshot := task.SnapshotByID(run.ID())[1]
		Expect(snapshot.Stdout).To(HaveLen(task.SnapshotStreamLimit))
		Expect(snapshot.Stdout).To(HaveSuffix("stdout-tail"))
		Expect(snapshot.StdoutTruncated).To(BeTrue())
		Expect(snapshot.Stderr).To(HaveLen(task.SnapshotStreamLimit))
		Expect(snapshot.Stderr).To(HaveSuffix("stderr-tail"))
		Expect(snapshot.StderrTruncated).To(BeTrue())
	})

	It("registers run listing SSE and lifecycle control routes", func() {
		controller := &recordingController{actions: []task.ControlAction{task.ControlStop}}
		run := task.StartManagedRun("controlled", task.WithController(controller))
		mux := http.NewServeMux()
		task.RegisterHandlers(mux, "/api")

		request := httptest.NewRequest(
			http.MethodPost,
			"/api/tasks/"+run.ID()+"/control",
			bytes.NewBufferString(`{"action":"stop"}`),
		)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		Expect(response.Code).To(Equal(http.StatusNoContent), response.Body.String())
		Expect(controller.called).To(Equal(task.ControlStop))

		request = httptest.NewRequest(http.MethodGet, "/api/tasks/runs/stream", nil)
		response = httptest.NewRecorder()
		ctx, cancel := context.WithCancel(request.Context())
		request = request.WithContext(ctx)
		done := make(chan struct{})
		go func() {
			mux.ServeHTTP(response, request)
			close(done)
		}()
		Eventually(response.Body.String).Should(ContainSubstring("event: runs"))
		cancel()
		Eventually(done).Should(BeClosed())
	})

	It("merges externally-owned runs into list, detail, stream, and control routes", func() {
		source := &recordingRunSource{
			runs: []task.RunMeta{{ID: "remote-1", Name: "api", Kind: "supervised-process", Status: "running"}},
			snapshots: map[string][]task.TaskSnapshot{
				"remote-1": {{ID: "api", GroupID: "remote-1", Name: "api", Type: "group", Status: "running"}},
			},
		}
		source.points = []metrics.Point{{At: time.Now().UTC(), Value: 42}}
		mux := http.NewServeMux()
		task.RegisterHandlersWithSource(mux, "/api", source)

		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/tasks?kind=supervised-process", nil))
		Expect(response.Code).To(Equal(http.StatusOK), response.Body.String())
		Expect(response.Body.String()).To(ContainSubstring("remote-1"))

		response = httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/tasks/remote-1", nil))
		Expect(response.Code).To(Equal(http.StatusOK), response.Body.String())
		Expect(response.Body.String()).To(ContainSubstring("remote-1"))

		request := httptest.NewRequest(http.MethodPost, "/api/tasks/remote-1/control", bytes.NewBufferString(`{"action":"restart"}`))
		response = httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		Expect(response.Code).To(Equal(http.StatusNoContent), response.Body.String())
		Expect(source.controlled).To(Equal("remote-1:restart"))

		response = httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/tasks/metrics/remote-1%3Acpu?since=1h", nil))
		Expect(response.Code).To(Equal(http.StatusOK), response.Body.String())
		Expect(response.Body.String()).To(ContainSubstring(`"value":42`))

		request = httptest.NewRequest(http.MethodGet, "/api/tasks/stream?tasks=remote-1", nil)
		response = httptest.NewRecorder()
		ctx, cancel := context.WithCancel(request.Context())
		request = request.WithContext(ctx)
		done := make(chan struct{})
		go func() {
			mux.ServeHTTP(response, request)
			close(done)
		}()
		Eventually(response.Body.String).Should(ContainSubstring("remote-1"))
		cancel()
		Eventually(done).Should(BeClosed())
	})
})
