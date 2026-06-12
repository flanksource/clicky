package task

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SSEHandler returns an http.Handler that streams task events via Server-Sent Events.
// If taskIDs are provided, only groups matching those IDs are streamed.
// The handler polls for dirty tasks every 200ms and emits JSON snapshots.
// It sends an "event: done" when all tracked groups have completed.
func SSEHandler(taskIDs ...string) http.Handler {
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

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				snapshots := SnapshotAll(ids...)
				allDone := true
				anyEmitted := false

				for _, snap := range snapshots {
					if snap.Type == "group" {
						if snap.Status == string(StatusRunning) || snap.Status == string(StatusPending) {
							allDone = false
						}
					}
					data, err := json.Marshal(snap)
					if err != nil {
						continue
					}
					_, _ = fmt.Fprintf(w, "event: task\ndata: %s\n\n", data)
					anyEmitted = true
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

// RunsSSEHandler streams the run listing (RunMeta) as SSE. Unlike SSEHandler it
// never sends a terminal event: a manager view stays subscribed to observe new
// and changing runs. supplement (may be nil) merges extra runs — e.g. archived
// or persisted runs the in-memory registry no longer holds; live runs win on id.
// It emits a single "event: runs" frame carrying the full listing, and only
// re-emits when the listing changes.
func RunsSSEHandler(supplement func(RunFilter) []RunMeta) http.Handler {
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
			runs := mergeRuns(RunsRaw(filter), supplement, filter)
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

// mergeRuns unions live runs with supplement(filter), deduped by id. Live runs
// win on collision (a supplement source may hold a stale terminal snapshot of a
// run that is still live in memory).
func mergeRuns(live []RunMeta, supplement func(RunFilter) []RunMeta, filter RunFilter) []RunMeta {
	if supplement == nil {
		if live == nil {
			return []RunMeta{}
		}
		return live
	}
	seen := make(map[string]bool, len(live))
	for _, m := range live {
		seen[m.ID] = true
	}
	out := append([]RunMeta{}, live...)
	for _, m := range supplement(filter) {
		if !seen[m.ID] {
			out = append(out, m)
		}
	}
	return out
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
