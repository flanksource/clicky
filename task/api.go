package task

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/flanksource/clicky/metrics"
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
	return RunsHandlerWithSource(nil)
}

// RunsHandlerWithSource merges in runs owned by another process or store.
func RunsHandlerWithSource(source RunSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		filter := RunFilter{
			Kind:   q.Get("kind"),
			Status: q.Get("status"),
			Labels: parseLabelFilter(q["label"]),
		}
		runs := Runs(filter)
		if source != nil {
			external, err := source.Runs(r.Context(), filter)
			if err != nil {
				http.Error(w, "list external task runs: "+err.Error(), http.StatusInternalServerError)
				return
			}
			runs = mergeRunMetas(runs, external)
		}
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
	return RunHandlerWithSource(nil)
}

// RunHandlerWithSource falls through to source when the run is not in memory.
func RunHandlerWithSource(source RunSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing run id", http.StatusBadRequest)
			return
		}
		snapshots := SnapshotByID(id)
		if len(snapshots) == 0 && source != nil {
			var err error
			snapshots, err = source.Snapshot(r.Context(), id)
			if err != nil {
				http.Error(w, "load external task run: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if len(snapshots) == 0 {
			http.Error(w, "run not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(snapshots)
	})
}

// RunControlHandler performs a lifecycle action advertised by a live run.
func RunControlHandler() http.Handler {
	return RunControlHandlerWithSource(nil)
}

// RunControlHandlerWithSource routes controls to the live owner of a run.
func RunControlHandlerWithSource(source RunSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action, ok := decodeControlAction(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		var err error
		if groupByID(id) != nil {
			err = ControlRun(r.Context(), id, action)
		} else if source != nil {
			err = source.Control(r.Context(), id, action)
		} else {
			err = ControlRun(r.Context(), id, action)
		}
		if err != nil {
			writeControlError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// TaskControlHandler performs a lifecycle action advertised by a child task.
func TaskControlHandler() http.Handler {
	return TaskControlHandlerWithSource(nil)
}

// TaskControlHandlerWithSource routes child controls to the live owner of a
// run, including runs owned by an external source.
func TaskControlHandlerWithSource(source RunSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action, ok := decodeControlAction(w, r)
		if !ok {
			return
		}
		runID := r.PathValue("id")
		taskID := r.PathValue("taskID")
		var err error
		if groupByID(runID) != nil {
			err = ControlTask(r.Context(), runID, taskID, action)
		} else if external, ok := source.(TaskControlSource); ok {
			err = external.ControlTask(r.Context(), runID, taskID, action)
		} else {
			err = ControlTask(r.Context(), runID, taskID, action)
		}
		if err != nil {
			writeControlError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func decodeControlAction(w http.ResponseWriter, r *http.Request) (ControlAction, bool) {
	var request struct {
		Action ControlAction `json:"action"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid control request: "+err.Error(), http.StatusBadRequest)
		return "", false
	}
	return request.Action, true
}

func writeControlError(w http.ResponseWriter, err error) {
	status := http.StatusConflict
	if strings.Contains(err.Error(), "not found") {
		status = http.StatusNotFound
	}
	http.Error(w, err.Error(), status)
}

// TaskStdinHandler writes live stdin to a task. Input is deliberately not
// included in any snapshot or persisted run record.
func TaskStdinHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Data string `json:"data"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, SnapshotStreamLimit))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, "invalid stdin request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := WriteTaskStdin(r.PathValue("id"), r.PathValue("taskID"), []byte(request.Data)); err != nil {
			status := http.StatusConflict
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// RegisterHandlers wires the generic task-manager API under prefix:
//
//	GET {prefix}/tasks         run listing (RunMeta[], ?kind=&status=&label=k=v)
//	GET {prefix}/tasks/stream  SSE stream of TaskSnapshots (?tasks=<id>&kind=)
//	GET {prefix}/tasks/runs/stream persistent SSE stream of RunMeta listings
//	GET {prefix}/tasks/{id}    id-scoped snapshot (group + tasks)
//	POST {prefix}/tasks/{id}/control lifecycle action
//	POST {prefix}/tasks/{id}/tasks/{taskID}/control child lifecycle action
//	POST {prefix}/tasks/{id}/tasks/{taskID}/stdin live stdin (never persisted)
//
// The {id} route reuses Go 1.22 net/http path-value routing; the stream route is
// registered before the {id} route so "stream" is not treated as an id.
func RegisterHandlers(mux *http.ServeMux, prefix string) {
	RegisterHandlersWithSource(mux, prefix, nil)
}

// RegisterHandlersWithSource wires the task API and merges externally-owned runs.
func RegisterHandlersWithSource(mux *http.ServeMux, prefix string, source RunSource) {
	prefix = strings.TrimSuffix(prefix, "/")
	mux.Handle("GET "+prefix+"/tasks", RunsHandlerWithSource(source))
	mux.Handle("GET "+prefix+"/tasks/stream", SSEHandlerWithSource(source))
	mux.Handle("GET "+prefix+"/tasks/runs/stream", RunsSSEHandlerWithSource(source))
	mux.Handle("POST "+prefix+"/tasks/{id}/control", RunControlHandlerWithSource(source))
	mux.Handle("POST "+prefix+"/tasks/{id}/tasks/{taskID}/control", TaskControlHandlerWithSource(source))
	mux.Handle("POST "+prefix+"/tasks/{id}/tasks/{taskID}/stdin", TaskStdinHandler())
	mux.Handle("GET "+prefix+"/tasks/{id}", RunHandlerWithSource(source))
	timeseries := Metrics()
	if external, ok := source.(MetricSource); ok {
		timeseries = sourceMetrics{local: timeseries, external: external}
	}
	metrics.RegisterRoutes(mux, timeseries, prefix+"/tasks")
}
