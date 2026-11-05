package task

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The stream a task's output travels on is append-only: the server sends what a
// client has not seen yet and never repeats itself. These tests pin the two
// halves of that contract — the offsets that decide append-vs-reset, and the
// fact that a task frame carries no output of its own, so a client must
// accumulate output somewhere a task frame cannot overwrite.

func decodeOutputFrame(t *testing.T, body string) outputDelta {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var delta outputDelta
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &delta); err != nil {
			t.Fatalf("decode output frame %q: %v", line, err)
		}
		return delta
	}
	t.Fatalf("no data line in frame %q", body)
	return outputDelta{}
}

func TestEmitOutputDeltaSendsOnlyWhatTheClientHasNotSeen(t *testing.T) {
	// One stream, read at five moments. The retained window slides; the absolute
	// positions do not.
	const stream = "hello\nworld\nmore\n"
	snapshot := TaskSnapshot{ID: "task-1", GroupID: "run-1", Type: "task"}
	sent := map[string]int64{}

	emit := func(view StreamView) (outputDelta, bool) {
		t.Helper()
		var frame bytes.Buffer
		emitted, err := emitOutputDelta(&frame, snapshot, "stdout", view, sent)
		if err != nil {
			t.Fatalf("emit output delta: %v", err)
		}
		if !emitted {
			return outputDelta{}, false
		}
		return decodeOutputFrame(t, frame.String()), true
	}

	for _, tc := range []struct {
		name    string
		view    StreamView
		emitted bool
		want    outputDelta
	}{{
		name:    "a fresh subscriber is reset to the whole retained tail",
		view:    StreamView{Data: stream[:6], Offset: 0},
		emitted: true,
		want:    outputDelta{Data: "hello\n", Offset: 0, Reset: true},
	}, {
		name:    "an unchanged stream sends nothing",
		view:    StreamView{Data: stream[:6], Offset: 0},
		emitted: false,
	}, {
		name:    "new bytes are appended at the position they belong",
		view:    StreamView{Data: stream[:12], Offset: 0},
		emitted: true,
		want:    outputDelta{Data: "world\n", Offset: 6},
	}, {
		// The window has rolled past what the client holds, but the client's
		// position is still inside it — so this is an append, not a redraw.
		name:    "a rolled window the client is still inside stays an append",
		view:    StreamView{Data: stream[7:], Offset: 7, Truncated: true},
		emitted: true,
		want:    outputDelta{Data: "more\n", Offset: 12, Truncated: true},
	}, {
		name:    "a client left behind by the window is reset to the new tail",
		view:    StreamView{Data: "far-ahead\n", Offset: 100, Truncated: true},
		emitted: true,
		want:    outputDelta{Data: "far-ahead\n", Offset: 100, Reset: true, Truncated: true},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			delta, emitted := emit(tc.view)
			if emitted != tc.emitted {
				t.Fatalf("emitted = %v, want %v (delta %+v)", emitted, tc.emitted, delta)
			}
			if !tc.emitted {
				return
			}
			want := tc.want
			want.ID, want.GroupID, want.Stream = snapshot.ID, snapshot.GroupID, "stdout"
			if delta != want {
				t.Fatalf("delta = %+v, want %+v", delta, want)
			}
		})
	}
}

func TestEmitOutputDeltaStaysSilentForATaskThatHasWrittenNothing(t *testing.T) {
	var frame bytes.Buffer
	sent := map[string]int64{}

	emitted, err := emitOutputDelta(&frame, TaskSnapshot{ID: "task-1", Type: "task"}, "stdout", StreamView{}, sent)

	if err != nil || emitted || frame.Len() != 0 {
		t.Fatalf("expected no frame for an empty stream, got %q (err %v)", frame.String(), err)
	}
}

// Logs travel like output: a client is sent the entries it lacks, keyed by
// their sequence number, and is reset only when it holds entries that are gone.
func TestEmitLogsDeltaSendsOnlyTheEntriesTheClientLacks(t *testing.T) {
	entry := func(seq uint64) LogEntry {
		return LogEntry{Seq: seq, Level: "info", Message: fmt.Sprintf("line %d", seq), Time: "2026-11-05T10:00:00Z"}
	}
	snapshot := TaskSnapshot{ID: "task-1", GroupID: "run-1", Type: "task"}
	sent := map[string]logCursor{}

	for _, tc := range []struct {
		name    string
		logs    []LogEntry
		cleared uint64
		emitted bool
		want    logsDelta
	}{{
		name:    "a fresh subscriber is reset to every retained entry",
		logs:    []LogEntry{entry(1), entry(2)},
		emitted: true,
		want:    logsDelta{Entries: []LogEntry{entry(1), entry(2)}, Reset: true},
	}, {
		name:    "an unchanged log sends nothing",
		logs:    []LogEntry{entry(1), entry(2)},
		emitted: false,
	}, {
		name:    "a new entry is appended alone",
		logs:    []LogEntry{entry(1), entry(2), entry(3)},
		emitted: true,
		want:    logsDelta{Entries: []LogEntry{entry(3)}},
	}, {
		// Retention dropped entry 1; the client keeps what it has.
		name:    "eviction of an entry the client holds is not a reset",
		logs:    []LogEntry{entry(2), entry(3), entry(4)},
		emitted: true,
		want:    logsDelta{Entries: []LogEntry{entry(4)}},
	}, {
		name:    "a clear resets the client to what was logged since",
		logs:    []LogEntry{entry(5)},
		cleared: 4,
		emitted: true,
		want:    logsDelta{Entries: []LogEntry{entry(5)}, Reset: true},
	}, {
		name:    "a clear with nothing logged since empties the client",
		cleared: 5,
		emitted: true,
		want:    logsDelta{Entries: []LogEntry{}, Reset: true},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot.Logs, snapshot.LogsClearedThrough = tc.logs, tc.cleared
			var frame bytes.Buffer
			emitted, err := emitLogsDelta(&frame, snapshot, sent)
			if err != nil {
				t.Fatalf("emit logs delta: %v", err)
			}
			if emitted != tc.emitted {
				t.Fatalf("emitted = %v, want %v (frame %q)", emitted, tc.emitted, frame.String())
			}
			if !tc.emitted {
				return
			}
			want := tc.want
			want.ID, want.GroupID = snapshot.ID, snapshot.GroupID
			wantJSON, err := json.Marshal(want)
			if err != nil {
				t.Fatalf("encode want: %v", err)
			}
			if got := frame.String(); got != "event: logs\ndata: "+string(wantJSON)+"\n\n" {
				t.Fatalf("frame = %q, want logs %s", got, wantJSON)
			}
		})
	}
}

func TestEmitLogsDeltaStaysSilentForATaskThatHasLoggedNothing(t *testing.T) {
	var frame bytes.Buffer

	emitted, err := emitLogsDelta(&frame, TaskSnapshot{ID: "task-1", Type: "task"}, map[string]logCursor{})

	if err != nil || emitted || frame.Len() != 0 {
		t.Fatalf("expected no frame for an empty log, got %q (err %v)", frame.String(), err)
	}
}

// sseFrames collects "event: <name>" frames off a live stream.
type sseFrames struct {
	t   *testing.T
	out chan [2]string // {event, data}
}

func startTaskStream(t *testing.T, h http.Handler) (*sseFrames, func()) {
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
	frames := &sseFrames{t: t, out: make(chan [2]string, 64)}
	go func() {
		reader := bufio.NewReader(resp.Body)
		var event string
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\n")
			switch {
			case strings.HasPrefix(line, ":"):
				frames.out <- [2]string{":", line}
			case strings.HasPrefix(line, "event: "):
				event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: ") && event != "":
				frames.out <- [2]string{event, strings.TrimPrefix(line, "data: ")}
				event = ""
			}
		}
	}()
	return frames, func() {
		cancel()
		_ = resp.Body.Close()
		srv.Close()
	}
}

// await returns the next frame of the named event, ignoring the others.
func (f *sseFrames) await(name string, timeout time.Duration) string {
	f.t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case frame := <-f.out:
			if frame[0] == name {
				return frame[1]
			}
		case <-deadline:
			f.t.Fatalf("timed out waiting for an event: %s frame", name)
			return ""
		}
	}
}

func TestSSEHandlerCarriesLogsOnlyInDeltaFrames(t *testing.T) {
	withTestGlobal(t)

	run := StartManagedRun("logger", WithGroupID("run-logs"))
	defer run.Finish(StatusSuccess, nil)
	run.Task().Infof("first line")

	frames, stop := startTaskStream(t, SSEHandler("run-logs"))
	defer stop()

	var delta logsDelta
	if err := json.Unmarshal([]byte(frames.await("logs", 3*time.Second)), &delta); err != nil {
		t.Fatalf("decode logs frame: %v", err)
	}
	if !delta.Reset || len(delta.Entries) != 1 || delta.Entries[0].Message != "first line" {
		t.Fatalf("first logs frame = %+v, want a reset to [first line]", delta)
	}

	run.Task().Infof("second line")
	for deadline := time.After(3 * time.Second); ; {
		select {
		case frame := <-frames.out:
			switch frame[0] {
			case "task":
				var snapshot TaskSnapshot
				if err := json.Unmarshal([]byte(frame[1]), &snapshot); err != nil {
					t.Fatalf("decode task frame: %v", err)
				}
				if snapshot.Type == "task" && len(snapshot.Logs) > 0 {
					t.Fatalf("task frame carried logs: %+v", snapshot.Logs)
				}
			case "logs":
				var appended logsDelta
				if err := json.Unmarshal([]byte(frame[1]), &appended); err != nil {
					t.Fatalf("decode logs frame: %v", err)
				}
				if appended.Reset || len(appended.Entries) != 1 || appended.Entries[0].Message != "second line" {
					t.Fatalf("second logs frame = %+v, want an append of [second line] alone", appended)
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for the appended logs frame")
		}
	}
}

func TestSSEHandlerPingsAQuietStream(t *testing.T) {
	withTestGlobal(t)
	previous := streamPingInterval
	streamPingInterval = 300 * time.Millisecond
	t.Cleanup(func() { streamPingInterval = previous })

	run := StartManagedRun("quiet", WithGroupID("run-quiet"))
	defer run.Finish(StatusSuccess, nil)

	frames, stop := startTaskStream(t, SSEHandler("run-quiet"))
	defer stop()

	if ping := frames.await(":", 3*time.Second); ping != ": ping" {
		t.Fatalf("keepalive = %q, want %q", ping, ": ping")
	}
}

// A running task whose state does not change must not be re-sent: the frame
// carries timestamps, not an elapsed value that moves on every tick.
func TestSSEHandlerSendsAnIdleRunningTaskOnce(t *testing.T) {
	withTestGlobal(t)
	const window = 5 * 200 * time.Millisecond

	run := StartManagedRun("idle", WithGroupID("run-idle"))
	defer run.Finish(StatusSuccess, nil)

	frames, stop := startTaskStream(t, SSEHandler("run-idle"))
	defer stop()

	taskFrames := 0
	for deadline := time.After(window); ; {
		select {
		case frame := <-frames.out:
			if frame[0] != "task" {
				continue
			}
			var snapshot TaskSnapshot
			if err := json.Unmarshal([]byte(frame[1]), &snapshot); err != nil {
				t.Fatalf("decode task frame: %v", err)
			}
			if snapshot.Type == "task" {
				taskFrames++
			}
		case <-deadline:
			if taskFrames != 1 {
				t.Fatalf("idle running task sent %d task frames over %s, want 1", taskFrames, window)
			}
			return
		}
	}
}

func TestSSEHandlerCarriesOutputOnlyInDeltaFrames(t *testing.T) {
	withTestGlobal(t)

	run := StartManagedRun("streamer", WithGroupID("run-out"))
	run.SetOutputProvider(func() OutputSnapshot { return OutputSnapshot{Stdout: "first\n"} })
	defer run.Finish(StatusSuccess, nil)

	frames, stop := startTaskStream(t, SSEHandler("run-out"))
	defer stop()

	// The output arrives on its own frame...
	var delta outputDelta
	if err := json.Unmarshal([]byte(frames.await("output", 3*time.Second)), &delta); err != nil {
		t.Fatalf("decode output frame: %v", err)
	}
	if delta.Data != "first\n" || delta.Stream != "stdout" {
		t.Fatalf("output frame = %+v, want stdout %q", delta, "first\n")
	}

	// ...and never on a task frame, however often the snapshot changes. A client
	// that stored the accumulated output on the snapshot object would lose it
	// here, which is why it has to live somewhere a task frame cannot replace.
	run.Task().SetDescription("second phase")
	for deadline := time.After(3 * time.Second); ; {
		select {
		case frame := <-frames.out:
			if frame[0] != "task" {
				continue
			}
			var snapshot TaskSnapshot
			if err := json.Unmarshal([]byte(frame[1]), &snapshot); err != nil {
				t.Fatalf("decode task frame: %v", err)
			}
			if snapshot.Type != "task" {
				continue
			}
			if snapshot.Stdout != "" || snapshot.Stderr != "" {
				t.Fatalf("task frame carried output: %+v", snapshot)
			}
			if snapshot.Description == "second phase" {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for the changed task frame")
		}
	}
}
