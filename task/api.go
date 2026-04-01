package task

import (
	"encoding/json"
	"net/http"
	"strings"
)

// JSONHandler returns an http.Handler that serves the full task state as JSON.
// If taskIDs are provided, only groups matching those IDs are included.
// This is used for initial page load, reconnection after SSE drop, or polling fallback.
func JSONHandler(taskIDs ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids := append([]string{}, taskIDs...)
		if q := r.URL.Query().Get("tasks"); q != "" {
			ids = append(ids, strings.Split(q, ",")...)
		}

		snapshots := SnapshotAll(ids...)
		if snapshots == nil {
			snapshots = []TaskSnapshot{}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(snapshots)
	})
}
