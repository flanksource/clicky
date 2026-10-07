package sse

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/flanksource/clicky/entity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const snapshotTick = 5 * time.Millisecond

type sentFrame struct {
	Name string
	Data string
}

// snapshotSink records what Snapshot sends and how often it pings.
type snapshotSink struct {
	mu      sync.Mutex
	frames  []sentFrame
	pings   int
	sendErr error
}

func (s *snapshotSink) send(event entity.StreamEvent) error {
	data, err := entity.StreamText(event.Data)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sendErr != nil {
		return s.sendErr
	}
	s.frames = append(s.frames, sentFrame{Name: event.Name, Data: data})
	return nil
}

func (s *snapshotSink) ping() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pings++
	return nil
}

func (s *snapshotSink) Frames() []sentFrame {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]sentFrame(nil), s.frames...)
}

func (s *snapshotSink) Pings() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pings
}

// awaitPings waits until n more pings than now arrived: n more loads that
// compared equal to what was last sent.
func (s *snapshotSink) awaitPings(n int) {
	start := s.Pings()
	Eventually(s.Pings, 2*time.Second).Should(BeNumerically(">=", start+n))
}

// payloadSource is a Load whose next result a spec can swap.
type payloadSource struct {
	mu    sync.Mutex
	value any
	err   error
}

func (p *payloadSource) set(value any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.value = value
}

func (p *payloadSource) load(context.Context) (any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.value, p.err
}

// runSnapshot runs Snapshot in the background until the spec ends, returning
// the sink and a channel that receives Snapshot's result.
func runSnapshot(sink *snapshotSink, opts SnapshotOptions) <-chan error {
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		result <- Snapshot(ctx, sink.send, sink.ping, opts)
	}()
	DeferCleanup(func() {
		cancel()
		Eventually(stopped, 2*time.Second).Should(BeClosed())
	})
	return result
}

var _ = Describe("Snapshot", func() {
	It("sends an unchanged payload once under its event name, then only pings", func() {
		source := &payloadSource{value: map[string]any{"items": []int{1, 2}}}
		sink := &snapshotSink{}

		runSnapshot(sink, SnapshotOptions{Load: source.load, Interval: snapshotTick, Event: "runs"})

		sink.awaitPings(3)
		Expect(sink.Frames()).To(Equal([]sentFrame{{Name: "runs", Data: `{"items":[1,2]}`}}))
	})

	DescribeTable("sends no frame when only an excluded field changed",
		func(exclude []string, first, second any, want string) {
			source := &payloadSource{value: first}
			sink := &snapshotSink{}
			runSnapshot(sink, SnapshotOptions{Load: source.load, Interval: snapshotTick, Exclude: exclude})
			Eventually(sink.Frames).Should(HaveLen(1))

			source.set(second)
			sink.awaitPings(3)

			frames := sink.Frames()
			Expect(frames).To(HaveLen(1))
			Expect(frames[0].Data).To(MatchJSON(want))
		},
		Entry("a top-level field",
			[]string{"rateLimit"},
			map[string]any{"rateLimit": 10, "prs": []int{1}},
			map[string]any{"rateLimit": 9, "prs": []int{1}},
			`{"rateLimit":10,"prs":[1]}`),
		Entry("a field under wildcard map keys",
			[]string{"*.processes.*.openFiles"},
			map[string]any{"web": map[string]any{"processes": map[string]any{"101": map[string]any{"openFiles": 209, "pid": 101}}}},
			map[string]any{"web": map[string]any{"processes": map[string]any{"101": map[string]any{"openFiles": 212, "pid": 101}}}},
			`{"web":{"processes":{"101":{"openFiles":209,"pid":101}}}}`),
		Entry("a field of every array element",
			[]string{"items.*.seenAt"},
			map[string]any{"items": []map[string]any{{"id": 1, "seenAt": "t1"}, {"id": 2, "seenAt": "t1"}}},
			map[string]any{"items": []map[string]any{{"id": 1, "seenAt": "t2"}, {"id": 2, "seenAt": "t3"}}},
			`{"items":[{"id":1,"seenAt":"t1"},{"id":2,"seenAt":"t1"}]}`),
	)

	It("sends one frame carrying the current excluded value when a real field changes alongside it", func() {
		source := &payloadSource{value: map[string]any{"rateLimit": 10, "count": 1}}
		sink := &snapshotSink{}
		runSnapshot(sink, SnapshotOptions{Load: source.load, Interval: snapshotTick, Exclude: []string{"rateLimit"}})
		Eventually(sink.Frames).Should(HaveLen(1))
		source.set(map[string]any{"rateLimit": 9, "count": 1})
		sink.awaitPings(3)

		source.set(map[string]any{"rateLimit": 8, "count": 2})

		Eventually(sink.Frames).Should(HaveLen(2))
		sink.awaitPings(3)
		frames := sink.Frames()
		Expect(frames).To(HaveLen(2))
		Expect(frames[0].Data).To(MatchJSON(`{"rateLimit":10,"count":1}`))
		Expect(frames[1].Data).To(MatchJSON(`{"rateLimit":8,"count":2}`))
	})

	It("loads immediately when woken instead of waiting for the interval", func() {
		source := &payloadSource{value: map[string]any{"version": 1}}
		sink := &snapshotSink{}
		wake := make(chan struct{}, 1)
		runSnapshot(sink, SnapshotOptions{Load: source.load, Interval: time.Hour, Wake: wake})
		Eventually(sink.Frames).Should(HaveLen(1))

		source.set(map[string]any{"version": 2})
		wake <- struct{}{}

		Eventually(sink.Frames, time.Second).Should(Equal([]sentFrame{
			{Data: `{"version":1}`},
			{Data: `{"version":2}`},
		}))
	})

	DescribeTable("reports a payload it cannot produce as an error event and returns the error",
		func(source *payloadSource, wantMessage string) {
			sink := &snapshotSink{}

			result := runSnapshot(sink, SnapshotOptions{Load: source.load, Interval: snapshotTick})

			var err error
			Eventually(result, 2*time.Second).Should(Receive(&err))
			Expect(err).To(MatchError(ContainSubstring(wantMessage)))
			if source.err != nil {
				Expect(errors.Is(err, source.err)).To(BeTrue(), "the load error must stay in the chain")
			}
			frames := sink.Frames()
			Expect(frames).To(HaveLen(1))
			Expect(frames[0].Name).To(Equal("error"))
			Expect(frames[0].Data).To(MatchJSON(`{"error":` + jsonString(err.Error()) + `}`))
		},
		Entry("a load error", &payloadSource{err: errors.New("backend down")}, "backend down"),
		Entry("a payload that cannot be encoded", &payloadSource{value: make(chan int)}, "encode snapshot"),
	)

	It("returns the send error when the client is gone", func() {
		gone := errors.New("client gone")
		source := &payloadSource{value: map[string]any{"version": 1}}
		sink := &snapshotSink{sendErr: gone}

		result := runSnapshot(sink, SnapshotOptions{Load: source.load, Interval: snapshotTick})

		Eventually(result, 2*time.Second).Should(Receive(MatchError(gone)))
	})

	It("returns the context error once cancelled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		source := &payloadSource{value: map[string]any{"version": 1}}
		sink := &snapshotSink{}
		result := make(chan error, 1)
		go func() {
			result <- Snapshot(ctx, sink.send, sink.ping, SnapshotOptions{Load: source.load, Interval: snapshotTick})
		}()
		Eventually(sink.Frames).Should(HaveLen(1))

		cancel()

		Eventually(result, 2*time.Second).Should(Receive(MatchError(context.Canceled)))
	})

	It("fails when its wake channel is closed rather than spinning", func() {
		wake := make(chan struct{})
		close(wake)
		source := &payloadSource{value: map[string]any{"version": 1}}

		result := runSnapshot(&snapshotSink{}, SnapshotOptions{Load: source.load, Interval: time.Hour, Wake: wake})

		Eventually(result, 2*time.Second).Should(Receive(MatchError(ContainSubstring("wake channel closed"))))
	})

	DescribeTable("panics on options it cannot run",
		func(mutate func(*SnapshotOptions), ping func() error, want string) {
			opts := SnapshotOptions{Load: (&payloadSource{}).load, Interval: snapshotTick}
			mutate(&opts)
			sink := &snapshotSink{}

			Expect(func() { _ = Snapshot(context.Background(), sink.send, ping, opts) }).To(PanicWith(ContainSubstring(want)))
		},
		Entry("no Load", func(o *SnapshotOptions) { o.Load = nil }, func() error { return nil }, "Load"),
		Entry("a zero Interval", func(o *SnapshotOptions) { o.Interval = 0 }, func() error { return nil }, "Interval"),
		Entry("no ping", func(*SnapshotOptions) {}, nil, "ping"),
		Entry("an empty exclude segment", func(o *SnapshotOptions) { o.Exclude = []string{"a..b"} }, func() error { return nil }, "Exclude"),
	)
})

func jsonString(s string) string {
	quoted, err := json.Marshal(s)
	Expect(err).NotTo(HaveOccurred())
	return string(quoted)
}
