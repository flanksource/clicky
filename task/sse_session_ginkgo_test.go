package task_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/task"
)

var fakeSessionTypes atomic.Int64

// fakeSessionSource is a SessionSource whose follows a spec drives by hand:
// it counts the follows opened per session id and hands back a channel the
// spec sends to and closes.
type fakeSessionSource struct {
	err error

	mu       sync.Mutex
	follows  map[string]int
	channels map[string]chan task.SessionEvent
}

// registerFakeSessionSource registers a fresh source under a type no other
// spec uses, since the registry refuses a type twice.
func registerFakeSessionSource(err error) (string, *fakeSessionSource) {
	sessionType := fmt.Sprintf("fake-session-%d", fakeSessionTypes.Add(1))
	source := &fakeSessionSource{err: err, follows: map[string]int{}, channels: map[string]chan task.SessionEvent{}}
	task.RegisterSessionSource(sessionType, source)
	return sessionType, source
}

func (s *fakeSessionSource) Follow(_ context.Context, ref task.SessionRef) (<-chan task.SessionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.follows[ref.ID]++
	if s.err != nil {
		return nil, s.err
	}
	events := make(chan task.SessionEvent, 16)
	s.channels[ref.ID] = events
	return events, nil
}

func (s *fakeSessionSource) followCount(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.follows[id]
}

func (s *fakeSessionSource) channel(id string) chan task.SessionEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.channels[id]
}

// openTaskStream serves /tasks/stream for one run until the spec ends or the
// stream finishes, whichever is first; the returned channel closes when the
// handler returns.
func openTaskStream(runID string) (*streamRecorder, <-chan struct{}) {
	request := httptest.NewRequest(http.MethodGet, "/tasks/stream?tasks="+runID, nil)
	ctx, cancel := context.WithCancel(request.Context())
	stream := newStreamRecorder()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		task.SSEHandler().ServeHTTP(stream, request.WithContext(ctx))
	}()
	DeferCleanup(func() {
		cancel()
		Eventually(finished).Should(BeClosed())
	})
	return stream, finished
}

func sessionFrame(event, sessionType, id, field, payload string) string {
	return fmt.Sprintf("event: %s\ndata: {\"session\":{\"type\":%q,\"id\":%q},%q:%s}\n\n", event, sessionType, id, field, payload)
}

var _ = Describe("task stream session follows", func() {
	It("multiplexes each followed session's entries and state into the stream, keyed by its ref", func() {
		sessionType, source := registerFakeSessionSource(nil)
		run := task.StartManagedRun("session multiplex")
		DeferCleanup(func() { run.Finish(task.StatusSuccess, nil) })
		run.Group().SetSession(task.SessionRef{Type: sessionType, ID: "group-session"})
		run.Task().SetSession(task.SessionRef{Type: sessionType, ID: "task-session"})

		stream, _ := openTaskStream(run.ID())
		Eventually(func() int { return source.followCount("group-session") + source.followCount("task-session") }).Should(Equal(2))
		source.channel("group-session") <- task.SessionEvent{Entry: json.RawMessage(`{"id":"g1"}`)}
		source.channel("task-session") <- task.SessionEvent{Entry: json.RawMessage(`{"id":"t1"}`), State: json.RawMessage(`{"status":"running"}`)}

		Eventually(stream.String).Should(SatisfyAll(
			ContainSubstring(sessionFrame("session", sessionType, "group-session", "entry", `{"id":"g1"}`)),
			ContainSubstring(sessionFrame("session", sessionType, "task-session", "entry", `{"id":"t1"}`)),
			ContainSubstring(sessionFrame("session-state", sessionType, "task-session", "state", `{"status":"running"}`)),
		))
	})

	It("follows a session shared by a run's group and its task once", func() {
		sessionType, source := registerFakeSessionSource(nil)
		run := task.StartManagedRun("session shared")
		DeferCleanup(func() { run.Finish(task.StatusSuccess, nil) })
		ref := task.SessionRef{Type: sessionType, ID: "shared"}
		run.Group().SetSession(ref)
		run.Task().SetSession(ref)

		openTaskStream(run.ID())
		Eventually(func() int { return source.followCount("shared") }).Should(Equal(1))
		Consistently(func() int { return source.followCount("shared") }, 500*time.Millisecond).Should(Equal(1))
	})

	It("opens no follow for a run that is already terminal", func() {
		sessionType, source := registerFakeSessionSource(nil)
		run := task.StartManagedRun("session terminal")
		run.Task().SetSession(task.SessionRef{Type: sessionType, ID: "finished"})
		run.Group().SetSession(task.SessionRef{Type: sessionType, ID: "finished"})
		run.Finish(task.StatusSuccess, nil)

		stream, finished := openTaskStream(run.ID())
		Eventually(finished).Should(BeClosed())
		Expect(stream.String()).To(ContainSubstring("event: done"))
		Expect(source.followCount("finished")).To(BeZero())
	})

	It("reports a failed follow as a session-error and keeps streaming the run", func() {
		sessionType, source := registerFakeSessionSource(nil)
		run := task.StartManagedRun("session event error")
		DeferCleanup(func() { run.Finish(task.StatusSuccess, nil) })
		run.Task().SetSession(task.SessionRef{Type: sessionType, ID: "broken"})

		stream, finished := openTaskStream(run.ID())
		Eventually(func() int { return source.followCount("broken") }).Should(Equal(1))
		source.channel("broken") <- task.SessionEvent{Err: errors.New("session store offline")}
		Eventually(stream.String).Should(ContainSubstring(sessionFrame("session-error", sessionType, "broken", "error", `"session store offline"`)))

		run.Task().Infof("still streaming")
		Eventually(stream.String).Should(ContainSubstring("still streaming"))
		Expect(finished).NotTo(BeClosed())
		Expect(source.followCount("broken")).To(Equal(1), "a dropped follow is never reopened")
	})

	It("reports a session whose follow cannot open as a session-error and keeps streaming the run", func() {
		sessionType, _ := registerFakeSessionSource(errors.New("unknown session"))
		run := task.StartManagedRun("session open error")
		DeferCleanup(func() { run.Finish(task.StatusSuccess, nil) })
		run.Task().SetSession(task.SessionRef{Type: sessionType, ID: "missing"})

		stream, finished := openTaskStream(run.ID())
		Eventually(stream.String).Should(ContainSubstring(sessionFrame("session-error", sessionType, "missing", "error", `"follow session: unknown session"`)))
		run.Task().Infof("after the open failed")
		Eventually(stream.String).Should(ContainSubstring("after the open failed"))
		Expect(finished).NotTo(BeClosed())
	})

	It("sends done only after a finished run's follows have drained", func() {
		sessionType, source := registerFakeSessionSource(nil)
		run := task.StartManagedRun("session drain")
		run.Task().SetSession(task.SessionRef{Type: sessionType, ID: "draining"})

		stream, finished := openTaskStream(run.ID())
		Eventually(func() int { return source.followCount("draining") }).Should(Equal(1))
		run.Finish(task.StatusSuccess, nil)
		Consistently(finished, 500*time.Millisecond).ShouldNot(BeClosed())

		source.channel("draining") <- task.SessionEvent{Entry: json.RawMessage(`{"id":"last"}`)}
		close(source.channel("draining"))
		Eventually(finished).Should(BeClosed())
		body := stream.String()
		last := strings.Index(body, sessionFrame("session", sessionType, "draining", "entry", `{"id":"last"}`))
		Expect(last).To(BeNumerically(">=", 0))
		Expect(strings.Index(body, "event: done")).To(BeNumerically(">", last))
	})

	It("re-emits the task frame and the runs listing when a session's summary changes", func() {
		run := task.StartManagedRun("session summary")
		DeferCleanup(func() { run.Finish(task.StatusSuccess, nil) })
		summarized := func(actions int) task.SessionRef {
			return task.SessionRef{Type: "unregistered", ID: "summary", Summary: &task.SessionSummary{
				Model:   "model-a",
				Context: &task.SessionContext{UsedTokens: 1200, WindowTokens: 200000},
				Actions: actions,
				Latest:  &task.SessionLatest{Kind: "tool", Tool: "Read", Text: "main.go"},
			}}
		}
		run.Task().SetSession(summarized(1))
		run.Group().SetSession(summarized(1))
		taskStream, _ := openTaskStream(run.ID())
		runsRequest := httptest.NewRequest(http.MethodGet, "/tasks/runs/stream", nil)
		ctx, cancel := context.WithCancel(runsRequest.Context())
		runsStream := newStreamRecorder()
		runsFinished := make(chan struct{})
		go func() {
			defer close(runsFinished)
			task.RunsSSEHandler(nil).ServeHTTP(runsStream, runsRequest.WithContext(ctx))
		}()
		DeferCleanup(func() { cancel(); Eventually(runsFinished).Should(BeClosed()) })
		first := `"summary":{"model":"model-a","context":{"usedTokens":1200,"windowTokens":200000},"actions":1,"latest":{"kind":"tool","tool":"Read","text":"main.go"}}`
		Eventually(taskStream.String).Should(ContainSubstring(first))
		Eventually(runsStream.String).Should(ContainSubstring(first))

		run.Task().SetSession(summarized(2))
		run.Group().SetSession(summarized(2))
		second := strings.Replace(first, `"actions":1`, `"actions":2`, 1)
		Eventually(taskStream.String).Should(ContainSubstring(second))
		Eventually(runsStream.String).Should(ContainSubstring(second))
	})

	DescribeTable("refuses a session source registration it cannot honour",
		func(register func()) { Expect(register).To(Panic()) },
		Entry("empty type", func() { task.RegisterSessionSource("", &fakeSessionSource{}) }),
		Entry("nil source", func() { task.RegisterSessionSource("fake-session-nil", nil) }),
		Entry("duplicate type", func() {
			sessionType, source := registerFakeSessionSource(nil)
			task.RegisterSessionSource(sessionType, source)
		}),
	)
})
