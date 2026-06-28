package prompt

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// filterFromQuery builds a Filter from ?owner=, ?kind=, ?state= and repeated
// ?label=k=v query params.
func filterFromQuery(r *http.Request) Filter {
	q := r.URL.Query()
	f := Filter{Owner: q.Get("owner"), Kind: q.Get("kind"), State: q.Get("state")}
	for _, v := range q["label"] {
		k, val, ok := strings.Cut(v, "=")
		if !ok {
			continue
		}
		if f.Labels == nil {
			f.Labels = map[string]string{}
		}
		f.Labels[k] = val
	}
	return f
}

// JSONHandler serves the current prompt snapshots (filtered) as JSON — the poll
// fallback and reconnect path for the UI.
func (m *Manager) JSONHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snaps := m.List(filterFromQuery(r))
		if snaps == nil {
			snaps = []PromptSnapshot{}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(snaps)
	})
}

// SSEHandler streams prompt snapshots (filtered) as Server-Sent Events, re-emitting
// only when the snapshot set changes. Unlike the task stream it never sends a
// terminal event: a dashboard stays subscribed to observe new prompts for the life
// of the page.
func (m *Manager) SSEHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := filterFromQuery(r)
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

		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()

		var lastSent string
		emit := func() {
			data, err := json.Marshal(m.List(filter))
			if err != nil {
				return
			}
			if string(data) == lastSent {
				return
			}
			lastSent = string(data)
			_, _ = fmt.Fprintf(w, "event: prompts\ndata: %s\n\n", data)
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

// ResolveHandler resolves a prompt by id from a POSTed answer body. The id is read
// from the {id} path value. A schema-invalid answer yields 400; an unknown or
// already-resolved prompt yields 409.
func (m *Manager) ResolveHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing prompt id", http.StatusBadRequest)
			return
		}
		var body struct {
			Values    map[string]any `json:"values"`
			Cancelled bool           `json:"cancelled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		err := m.Resolve(id, Answer{Values: body.Values, Cancelled: body.Cancelled})
		if err != nil {
			status := http.StatusConflict
			if strings.Contains(err.Error(), "answer for prompt") {
				status = http.StatusBadRequest
			}
			http.Error(w, err.Error(), status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"resolved": true})
	})
}

// RegisterHandlers wires the prompt API under prefix:
//
//	GET  {prefix}/prompts             snapshot listing (?owner=&kind=&state=&label=k=v)
//	GET  {prefix}/prompts/stream      SSE stream of snapshots (same filters)
//	GET  {prefix}/prompts/{id}        single snapshot
//	POST {prefix}/prompts/{id}/answer resolve a prompt
//
// The static "stream" route is registered before "{id}" so it is not parsed as an
// id (Go 1.22 path-value routing).
func (m *Manager) RegisterHandlers(mux *http.ServeMux, prefix string) {
	prefix = strings.TrimSuffix(prefix, "/")
	mux.Handle("GET "+prefix+"/prompts", m.JSONHandler())
	mux.Handle("GET "+prefix+"/prompts/stream", m.SSEHandler())
	mux.Handle("GET "+prefix+"/prompts/{id}", m.singleHandler())
	mux.Handle("POST "+prefix+"/prompts/{id}/answer", m.ResolveHandler())
}

func (m *Manager) singleHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snap, ok := m.Snapshot(r.PathValue("id"))
		if !ok {
			http.Error(w, "prompt not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(snap)
	})
}
