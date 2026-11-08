package task

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/flanksource/clicky/sse"
	"github.com/flanksource/commons/logger"
)

// streamTick is how often a task stream re-reads the runs it follows. It is
// the stream's only rate limit: every change made between two ticks reaches
// the client as one frame per task.
const streamTick = 200 * time.Millisecond

var (
	// streamPingInterval is how long a task stream may send nothing before it
	// writes a keepalive comment, so an idle proxy does not drop a quiet run's
	// stream. A var so specs can shorten it.
	streamPingInterval = 15 * time.Second
	// externalPollInterval is how often a task stream re-reads runs owned by an
	// external RunSource; in-memory runs are re-read every tick.
	externalPollInterval = time.Second
)

// SSEHandler returns an http.Handler that streams task events via Server-Sent Events.
// If taskIDs are provided, only groups matching those IDs are streamed.
//
// Every 200ms the handler re-reads its runs and sends, per task, only what
// changed: an "event: task" snapshot when its state differs from the last one
// sent, stdout/stderr as "event: output" deltas, and log entries as
// "event: logs" deltas. Snapshots carry timestamps, never elapsed times, so a
// task that is merely running sends nothing. A stream that sent nothing for
// streamPingInterval writes a ": ping" comment.
//
// It sends "event: done" once every tracked group has completed, and
// "event: error" {"error": msg} — ending the stream — when its runs cannot be
// read or encoded.
func SSEHandler(taskIDs ...string) http.Handler {
	return SSEHandlerWithSource(nil, taskIDs...)
}

// SSEHandlerWithSource also streams task runs owned by another process or
// store, re-reading those every externalPollInterval.
func SSEHandlerWithSource(source RunSource, taskIDs ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids, followAll, err := streamRunIDs(r, source, taskIDs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		flusher.Flush()

		stream := newTaskStream(source, ids, followAll)
		ticker := time.NewTicker(streamTick)
		defer ticker.Stop()
		lastWrite := time.Now()
		for {
			select {
			case <-r.Context().Done():
				return
			case now := <-ticker.C:
				var frames bytes.Buffer
				done, err := stream.tick(r.Context(), &frames, now)
				if err != nil {
					if r.Context().Err() != nil {
						return
					}
					logger.Warnf("task stream: %v", err)
					frames.Reset()
					writeStreamError(&frames, err)
					done = true
				}
				if frames.Len() == 0 && now.Sub(lastWrite) >= streamPingInterval {
					frames.WriteString(": ping\n\n")
				}
				if frames.Len() > 0 {
					// One write per tick: a hub relaying this stream flushes a
					// burst once instead of once per frame.
					if _, err := w.Write(frames.Bytes()); err != nil {
						return
					}
					flusher.Flush()
					lastWrite = now
				}
				if done {
					return
				}
			}
		}
	})
}

// streamRunIDs is the runs a stream follows: the handler's own ids, ?tasks=,
// and the runs of ?kind= as of the request. followAll is true only without filters.
func streamRunIDs(r *http.Request, source RunSource, taskIDs []string) (ids []string, followAll bool, err error) {
	ids = append([]string{}, taskIDs...)
	if q := r.URL.Query().Get("tasks"); q != "" {
		ids = append(ids, strings.Split(q, ",")...)
	}
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		return ids, len(ids) == 0, nil
	}
	for _, run := range Runs(RunFilter{Kind: kind}) {
		ids = append(ids, run.ID)
	}
	if source == nil {
		return ids, false, nil
	}
	runs, err := source.Runs(r.Context(), RunFilter{Kind: kind})
	if err != nil {
		return nil, false, fmt.Errorf("list external task runs: %w", err)
	}
	for _, run := range runs {
		ids = append(ids, run.ID)
	}
	return ids, false, nil
}

// taskStream is one connection's position in the runs it follows: what it last
// sent of each snapshot, stream and log, so each tick sends only what changed.
type taskStream struct {
	runs      *runSnapshots
	snapshots map[string]string
	// output holds, per stream, the absolute offset one past the last byte this
	// connection has sent — the client's end position, not its length.
	output map[string]int64
	logs   map[string]logCursor
}

// newTaskStream owns the snapshot and delta cursors for one connection.
func newTaskStream(source RunSource, ids []string, followAll bool) *taskStream {
	return &taskStream{
		runs:      newRunSnapshots(source, ids, followAll),
		snapshots: map[string]string{},
		output:    map[string]int64{},
		logs:      map[string]logCursor{},
	}
}

// tick writes the frames for everything that changed since the last tick, and
// the done frame once every tracked group has finished, which it reports.
func (s *taskStream) tick(ctx context.Context, frames *bytes.Buffer, now time.Time) (bool, error) {
	snapshots, err := s.runs.read(ctx, now)
	if err != nil {
		return false, err
	}
	snapshots = visibleSnapshots(ctx, s.runs.source, snapshots)
	allDone := true
	for _, snapshot := range snapshots {
		if snapshot.Type == "group" && (snapshot.Status == string(StatusRunning) || snapshot.Status == string(StatusPending)) {
			allDone = false
		}
		if err := s.write(frames, snapshot); err != nil {
			return false, err
		}
	}
	if len(snapshots) == 0 || !allDone {
		// With no groups registered yet, keep waiting for them.
		return false, nil
	}
	frames.WriteString("event: done\ndata: {\"status\":\"completed\"}\n\n")
	return true, nil
}

// write sends one snapshot's changes. A task frame never carries its output or
// logs: those travel as deltas, which a client accumulates beside the snapshot.
func (s *taskStream) write(frames io.Writer, snapshot TaskSnapshot) error {
	full := snapshot
	if snapshot.Type == "task" {
		snapshot.Stdout, snapshot.Stderr, snapshot.Logs = "", "", nil
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode %s %q snapshot: %w", snapshot.Type, snapshot.ID, err)
	}
	key := snapshot.Type + ":" + snapshot.ID + ":" + snapshot.GroupID
	if s.snapshots[key] != string(data) {
		s.snapshots[key] = string(data)
		if _, err := fmt.Fprintf(frames, "event: task\ndata: %s\n\n", data); err != nil {
			return err
		}
	}
	if snapshot.Type != "task" {
		return nil
	}
	streams := []struct {
		name string
		view StreamView
	}{
		{"stdout", StreamView{Data: full.Stdout, Offset: full.StdoutOffset, Truncated: full.StdoutTruncated}},
		{"stderr", StreamView{Data: full.Stderr, Offset: full.StderrOffset, Truncated: full.StderrTruncated}},
	}
	for _, stream := range streams {
		if _, err := emitOutputDelta(frames, full, stream.name, stream.view, s.output); err != nil {
			return err
		}
	}
	_, err = emitLogsDelta(frames, full, s.logs)
	return err
}

// writeEvent writes one named event carrying payload as JSON.
func writeEvent(w io.Writer, name string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s frame: %w", name, err)
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
	return err
}

// writeStreamError is the stream's last frame when it cannot go on, the same
// "event: error" {"error": msg} contract as sse.Snapshot.
func writeStreamError(frames *bytes.Buffer, cause error) {
	if err := writeEvent(frames, "error", map[string]string{"error": cause.Error()}); err != nil {
		panic(fmt.Sprintf("task stream: %v", err))
	}
}

// StreamView is one snapshot's view of a task stream: the retained tail, the
// absolute offset of its first byte, and whether anything was dropped ahead of
// it.
type StreamView struct {
	Data      string
	Offset    int64
	Truncated bool
}

func (v StreamView) end() int64 { return v.Offset + int64(len(v.Data)) }

// outputDelta is one frame of a task's stream. Offset is an ABSOLUTE position
// in that stream, not a length: on an append it is where Data belongs, and on a
// reset it is where the retained tail now starts. A client that has fallen
// behind the retained window gets Reset so it discards what it can no longer
// continue from.
type outputDelta struct {
	ID        string `json:"id"`
	GroupID   string `json:"groupId,omitempty"`
	Stream    string `json:"stream"`
	Data      string `json:"data"`
	Offset    int64  `json:"offset"`
	Reset     bool   `json:"reset,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// emitOutputDelta sends what the client has not seen of one stream, deciding
// append-vs-reset from stream offsets. Comparing text instead — asking whether
// the new tail extends the old one — degenerates the moment a bounded buffer
// rolls over: the tail becomes a sliding window that is never again a prefix
// extension, so every poll would resend the whole retained buffer.
func emitOutputDelta(
	w io.Writer,
	snapshot TaskSnapshot,
	stream string,
	view StreamView,
	sent map[string]int64,
) (bool, error) {
	key := snapshot.GroupID + ":" + snapshot.ID + ":" + stream
	end := view.end()
	have, seen := sent[key]
	delta := outputDelta{
		ID: snapshot.ID, GroupID: snapshot.GroupID, Stream: stream,
		Truncated: view.Truncated,
	}
	switch {
	case seen && have == end:
		return false, nil
	case !seen && view.Data == "":
		// A task that has produced nothing yet has nothing to reset the client to.
		sent[key] = end
		return false, nil
	case seen && have >= view.Offset && have <= end:
		delta.Data = view.Data[have-view.Offset:]
		delta.Offset = have
	default:
		delta.Data = view.Data
		delta.Offset = view.Offset
		delta.Reset = true
	}
	sent[key] = end
	return true, writeEvent(w, "output", delta)
}

// logsDelta is one frame of a task's log. Entries are in Seq order; Reset tells
// the client to replace the entries it holds instead of appending to them.
type logsDelta struct {
	ID      string     `json:"id"`
	GroupID string     `json:"groupId,omitempty"`
	Entries []LogEntry `json:"entries"`
	Reset   bool       `json:"reset,omitempty"`
}

// logCursor is what a connection has sent of one task's log: the highest Seq,
// and the clear (LogsClearedThrough) it was sent after.
type logCursor struct{ seq, cleared uint64 }

// emitLogsDelta sends the log entries the client lacks. Entries are identified
// by Seq, so retention dropping an old entry is not a change; only a clear
// (LogsClearedThrough moving) resets the client. A first frame is a reset to
// every retained entry — the only frame that carries entries with no Seq, as
// in snapshots persisted before entries were sequenced.
func emitLogsDelta(w io.Writer, snapshot TaskSnapshot, sent map[string]logCursor) (bool, error) {
	key := snapshot.GroupID + ":" + snapshot.ID
	have, seen := sent[key]
	next := logCursor{seq: have.seq, cleared: snapshot.LogsClearedThrough}
	if n := len(snapshot.Logs); n > 0 {
		next.seq = max(have.seq, snapshot.Logs[n-1].Seq)
	}
	delta := logsDelta{ID: snapshot.ID, GroupID: snapshot.GroupID}
	switch {
	case !seen && len(snapshot.Logs) == 0:
		// A task that has logged nothing yet has nothing to reset the client to.
		sent[key] = next
		return false, nil
	case !seen || snapshot.LogsClearedThrough > have.cleared:
		delta.Entries, delta.Reset = append([]LogEntry{}, snapshot.Logs...), true
	default:
		from := sort.Search(len(snapshot.Logs), func(i int) bool { return snapshot.Logs[i].Seq > have.seq })
		if from == len(snapshot.Logs) {
			return false, nil
		}
		delta.Entries = snapshot.Logs[from:]
	}
	sent[key] = next
	return true, writeEvent(w, "logs", delta)
}

// RunsSSEHandler streams the run listing (RunMeta) as SSE. Unlike SSEHandler it
// never sends a terminal event: a manager view stays subscribed to observe new
// and changing runs. supplement (may be nil) merges extra runs — e.g. archived
// or persisted runs the in-memory registry no longer holds; live runs win on id.
// It emits a single "event: runs" frame carrying the full listing, only
// re-emits when the listing changes, and pings in between (sse.Snapshot). A
// failure to list runs ends the stream with an "event: error" frame.
func RunsSSEHandler(supplement func(RunFilter) []RunMeta) http.Handler {
	return runsSSEHandler(nil, func(_ context.Context, filter RunFilter) ([]RunMeta, error) {
		if supplement == nil {
			return nil, nil
		}
		return supplement(filter), nil
	})
}

// RunsSSEHandlerWithSource streams both in-memory and externally-owned runs.
func RunsSSEHandlerWithSource(source RunSource) http.Handler {
	return runsSSEHandler(source, func(ctx context.Context, filter RunFilter) ([]RunMeta, error) {
		if source == nil {
			return nil, nil
		}
		return source.Runs(ctx, filter)
	})
}

func runsSSEHandler(source RunSource, loadExternal func(context.Context, RunFilter) ([]RunMeta, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := runFilterFromQuery(r)
		err := sse.ServeSnapshot(w, r, sse.SnapshotOptions{
			Load: func(ctx context.Context) (any, error) {
				GCRuns()
				external, err := loadExternal(ctx, filter)
				if err != nil {
					return nil, fmt.Errorf("list external task runs: %w", err)
				}
				return visibleRuns(ctx, source, mergeRunMetas(RunsRaw(filter), external)), nil
			},
			// Listing updates are cheaper and less urgent than per-task progress,
			// so poll on a slower tick than SSEHandler.
			Interval: 500 * time.Millisecond,
			Event:    "runs",
		})
		if err != nil {
			// A load failure has already reached the client as an error event;
			// this is the server's copy.
			logger.Warnf("task runs stream: %v", err)
		}
	})
}

// runFilterFromQuery builds a RunFilter from the standard task listing query
// params: kind, status, and repeated label=k=v pairs.
func runFilterFromQuery(r *http.Request) RunFilter {
	q := r.URL.Query()
	filter := RunFilter{
		Kind:   q.Get("kind"),
		Status: q.Get("status"),
	}
	for _, v := range q["label"] {
		k, val, ok := strings.Cut(v, "=")
		if !ok {
			continue
		}
		if filter.Labels == nil {
			filter.Labels = map[string]string{}
		}
		filter.Labels[k] = val
	}
	return filter
}
