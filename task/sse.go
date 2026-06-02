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
