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

// RunsHandler serves the run listing (RunMeta per group) for the generic
// task-manager view, filtered by the ?kind=, ?status=, and repeated ?label=k=v
// query params.
func RunsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		runs := Runs(RunFilter{
			Kind:   q.Get("kind"),
			Status: q.Get("status"),
			Labels: parseLabelFilter(q["label"]),
		})
		if runs == nil {
			runs = []RunMeta{}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(runs)
	})
}

// RunHandler serves the id-scoped snapshot (group + its tasks) for a single run.
// The id is read from the {id} path value of the registered route.
func RunHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing run id", http.StatusBadRequest)
			return
		}
		snapshots := SnapshotByID(id)
		if len(snapshots) == 0 {
			http.Error(w, "run not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(snapshots)
	})
}

// RegisterHandlers wires the generic task-manager API under prefix:
//
//	GET {prefix}/tasks         run listing (RunMeta[], ?kind=&status=&label=k=v)
//	GET {prefix}/tasks/stream  SSE stream of TaskSnapshots (?tasks=<id>&kind=)
//	GET {prefix}/tasks/{id}    id-scoped snapshot (group + tasks)
//
// The {id} route reuses Go 1.22 net/http path-value routing; the stream route is
// registered before the {id} route so "stream" is not treated as an id.
func RegisterHandlers(mux *http.ServeMux, prefix string) {
	prefix = strings.TrimSuffix(prefix, "/")
	mux.Handle("GET "+prefix+"/tasks", RunsHandler())
	mux.Handle("GET "+prefix+"/tasks/stream", SSEHandler())
	mux.Handle("GET "+prefix+"/tasks/{id}", RunHandler())
}
