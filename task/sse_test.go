package task

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// runsFrameReader reads SSE "event: runs" frames off a stream, decoding each
// data payload into []RunMeta. readFrame returns the next listing or fails the
// test if none arrives within the timeout.
type runsFrameReader struct {
	t   *testing.T
	br  *bufio.Reader
	out chan []RunMeta
}

func newRunsFrameReader(t *testing.T, body *bufio.Reader) *runsFrameReader {
	t.Helper()
	r := &runsFrameReader{t: t, br: body, out: make(chan []RunMeta, 8)}
	go r.pump()
	return r
}

// pump scans the stream line-by-line, emitting the JSON that follows each
// "event: runs" line.
func (r *runsFrameReader) pump() {
	var pendingRuns bool
	for {
		line, err := r.br.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\n")
		switch {
		case line == "event: runs":
			pendingRuns = true
		case strings.HasPrefix(line, "data: ") && pendingRuns:
			pendingRuns = false
			var runs []RunMeta
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &runs); err == nil {
				r.out <- runs
			}
		}
	}
}

func (r *runsFrameReader) readFrame(timeout time.Duration) []RunMeta {
	r.t.Helper()
	select {
	case runs := <-r.out:
		return runs
	case <-time.After(timeout):
		r.t.Fatalf("timed out waiting for an event: runs frame")
		return nil
	}
}

// expectNoFrame asserts no further listing arrives within the window — proves
// the handler dedups identical listings instead of re-emitting every tick.
func (r *runsFrameReader) expectNoFrame(window time.Duration) {
	r.t.Helper()
	select {
	case runs := <-r.out:
		r.t.Fatalf("expected no frame for an unchanged listing, got %+v", runs)
	case <-time.After(window):
	}
}

func startRunsStream(t *testing.T, h http.Handler) (*runsFrameReader, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	reader := newRunsFrameReader(t, bufio.NewReader(resp.Body))
	stop := func() {
		cancel()
		_ = resp.Body.Close()
		srv.Close()
	}
	return reader, stop
}

func runNames(runs []RunMeta) map[string]bool {
	set := make(map[string]bool, len(runs))
	for _, r := range runs {
		set[r.Name] = true
	}
	return set
}

func TestRunsSSEHandlerEmitsListingAndReEmitsOnChange(t *testing.T) {
	withTestGlobal(t)

	g1 := StartGroup[any]("first-run", WithConcurrency(1))
	runGroupToCompletion(t, g1, "step")

	reader, stop := startRunsStream(t, RunsSSEHandler(nil))
	defer stop()

	// Initial frame carries the one live run.
	first := reader.readFrame(2 * time.Second)
	if names := runNames(first); !names["first-run"] || len(first) != 1 {
		t.Fatalf("initial frame expected [first-run], got %+v", first)
	}

	// An unchanged listing must NOT be re-emitted every tick.
	reader.expectNoFrame(1200 * time.Millisecond)

	// A new run changes the listing → the handler re-emits.
	g2 := StartGroup[any]("second-run", WithConcurrency(1))
	runGroupToCompletion(t, g2, "step")

	second := reader.readFrame(2 * time.Second)
	if names := runNames(second); !names["first-run"] || !names["second-run"] || len(second) != 2 {
		t.Fatalf("change frame expected [first-run, second-run], got %+v", second)
	}
}

func TestRunsSSEHandlerMergesSupplementDedupedByID(t *testing.T) {
	withTestGlobal(t)

	live := StartGroup[any]("live-run", WithKind("test"), WithConcurrency(1))
	runGroupToCompletion(t, live, "step")
	liveID := live.ID()

	// Supplement returns an archived run plus a stale duplicate of the live id;
	// the live run must win on id and the archived run must be appended.
	supplement := func(RunFilter) []RunMeta {
		return []RunMeta{
			{ID: liveID, Name: "STALE-DUP", Status: "success"},
			{ID: "archived-1", Name: "archived-run", Status: "success"},
		}
	}

	reader, stop := startRunsStream(t, RunsSSEHandler(supplement))
	defer stop()

	frame := reader.readFrame(2 * time.Second)
	if len(frame) != 2 {
		t.Fatalf("expected live + archived (deduped), got %d: %+v", len(frame), frame)
	}
	names := runNames(frame)
	if !names["live-run"] || !names["archived-run"] {
		t.Fatalf("expected [live-run, archived-run], got %+v", frame)
	}
	if names["STALE-DUP"] {
		t.Fatalf("stale supplement duplicate of live id should be dropped, got %+v", frame)
	}
}
