package task

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SSEHandler returns an http.Handler that streams task events via Server-Sent Events.
// If taskIDs are provided, only groups matching those IDs are streamed.
// The handler polls every 200ms, emits changed JSON snapshots, and sends stdout
// and stderr as append-only output deltas (or a reset when the bounded tail
// rolls over).
// It sends an "event: done" when all tracked groups have completed.
func SSEHandler(taskIDs ...string) http.Handler {
	return SSEHandlerWithSource(nil, taskIDs...)
}

// SSEHandlerWithSource also streams task runs owned by another process or store.
func SSEHandlerWithSource(source RunSource, taskIDs ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Merge query param IDs with function-level IDs. A ?kind= filter is
		// resolved to the matching runs' ids so the stream can follow a whole
		// kind, not just an explicit id/name list.
		ids := append([]string{}, taskIDs...)
		if q := r.URL.Query().Get("tasks"); q != "" {
			ids = append(ids, strings.Split(q, ",")...)
		}
		if kind := r.URL.Query().Get("kind"); kind != "" {
			for _, run := range Runs(RunFilter{Kind: kind}) {
				ids = append(ids, run.ID)
			}
			if source != nil {
				runs, err := source.Runs(r.Context(), RunFilter{Kind: kind})
				if err != nil {
					http.Error(w, "list external task runs: "+err.Error(), http.StatusInternalServerError)
					return
				}
				for _, run := range runs {
					ids = append(ids, run.ID)
				}
			}
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

		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		lastSnapshots := map[string]string{}
		lastOutput := map[string]string{}
		finished := finishedSnapshots{}

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				snapshots := snapshotsWithSource(r.Context(), source, finished, ids...)
				allDone := true
				anyEmitted := false

				for _, snap := range snapshots {
					if snap.Type == "group" {
						if snap.Status == string(StatusRunning) || snap.Status == string(StatusPending) {
							allDone = false
						}
					}
					stdout, stderr := snap.Stdout, snap.Stderr
					if snap.Type == "task" {
						snap.Stdout = ""
						snap.Stderr = ""
					}
					data, err := json.Marshal(snap)
					if err != nil {
						continue
					}
					key := snap.Type + ":" + snap.ID + ":" + snap.GroupID
					if lastSnapshots[key] != string(data) {
						lastSnapshots[key] = string(data)
						_, _ = fmt.Fprintf(w, "event: task\ndata: %s\n\n", data)
						anyEmitted = true
					}
					if snap.Type == "task" {
						if emitOutputDelta(w, snap, "stdout", stdout, snap.StdoutTruncated, lastOutput) {
							anyEmitted = true
						}
						if emitOutputDelta(w, snap, "stderr", stderr, snap.StderrTruncated, lastOutput) {
							anyEmitted = true
						}
					}
				}

				if anyEmitted {
					flusher.Flush()
				}

				if len(snapshots) > 0 && allDone {
					_, _ = fmt.Fprintf(w, "event: done\ndata: {\"status\":\"completed\"}\n\n")
					flusher.Flush()
					return
				}

				// No groups registered yet — keep waiting
				if len(snapshots) == 0 {
					continue
				}
			}
		}
	})
}

type outputDelta struct {
	ID        string `json:"id"`
	GroupID   string `json:"groupId,omitempty"`
	Stream    string `json:"stream"`
	Data      string `json:"data"`
	Offset    int    `json:"offset"`
	Reset     bool   `json:"reset,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

func emitOutputDelta(
	w http.ResponseWriter,
	snapshot TaskSnapshot,
	stream string,
	current string,
	truncated bool,
	last map[string]string,
) bool {
	key := snapshot.GroupID + ":" + snapshot.ID + ":" + stream
	previous := last[key]
	if current == previous {
		return false
	}
	delta := outputDelta{
		ID: snapshot.ID, GroupID: snapshot.GroupID, Stream: stream,
		Data: current, Reset: true, Truncated: truncated,
	}
	if strings.HasPrefix(current, previous) {
		delta.Data = current[len(previous):]
		delta.Offset = len(previous)
		delta.Reset = false
	}
	last[key] = current
	data, err := json.Marshal(delta)
	if err != nil {
		return false
	}
	_, _ = fmt.Fprintf(w, "event: output\ndata: %s\n\n", data)
	return true
}

// RunsSSEHandler streams the run listing (RunMeta) as SSE. Unlike SSEHandler it
// never sends a terminal event: a manager view stays subscribed to observe new
// and changing runs. supplement (may be nil) merges extra runs — e.g. archived
// or persisted runs the in-memory registry no longer holds; live runs win on id.
// It emits a single "event: runs" frame carrying the full listing, and only
// re-emits when the listing changes.
func RunsSSEHandler(supplement func(RunFilter) []RunMeta) http.Handler {
	return runsSSEHandler(func(_ context.Context, filter RunFilter) ([]RunMeta, error) {
		if supplement == nil {
			return nil, nil
		}
		return supplement(filter), nil
	})
}

// RunsSSEHandlerWithSource streams both in-memory and externally-owned runs.
func RunsSSEHandlerWithSource(source RunSource) http.Handler {
	return runsSSEHandler(func(ctx context.Context, filter RunFilter) ([]RunMeta, error) {
		if source == nil {
			return nil, nil
		}
		return source.Runs(ctx, filter)
	})
}

func runsSSEHandler(loadExternal func(context.Context, RunFilter) ([]RunMeta, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := runFilterFromQuery(r)

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

		// Listing updates are cheaper and less urgent than per-task progress, so
		// poll on a slower tick than SSEHandler.
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		var lastSent string
		emit := func() {
			GCRuns()
			external, err := loadExternal(r.Context(), filter)
			if err != nil {
				return
			}
			runs := mergeRunMetas(RunsRaw(filter), external)
			data, err := json.Marshal(runs)
			if err != nil {
				return
			}
			if string(data) == lastSent {
				return
			}
			lastSent = string(data)
			_, _ = fmt.Fprintf(w, "event: runs\ndata: %s\n\n", data)
			flusher.Flush()
		}

		emit()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				emit()
			}
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
