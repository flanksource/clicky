package aichat

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// registerThreadRoutes wires the conversation-history endpoints onto mux. They
// all fail loud with 501 when no ThreadStore is configured rather than
// silently dropping data.
func (s *Server) registerThreadRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/chat/threads", s.handleCreateThread)
	mux.HandleFunc("GET /api/chat/threads", s.handleListThreads)
	mux.HandleFunc("GET /api/chat/threads/{id}", s.handleGetThread)
	mux.HandleFunc("DELETE /api/chat/threads/{id}", s.handleDeleteThread)
}

// threadsOrError returns the configured store, or writes a 501 and returns nil.
func (s *Server) threadsOrError(w http.ResponseWriter) ThreadStore {
	if s.opts.Threads == nil {
		http.Error(w, "thread persistence is not configured on this server", http.StatusNotImplemented)
		return nil
	}
	return s.opts.Threads
}

func (s *Server) handleCreateThread(w http.ResponseWriter, r *http.Request) {
	store := s.threadsOrError(w)
	if store == nil {
		return
	}
	var body struct {
		Title string `json:"title"`
	}
	// An empty body is fine — title defaults below.
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if body.Title == "" {
		body.Title = "New conversation"
	}
	t, err := store.Create(r.Context(), body.Title)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleListThreads(w http.ResponseWriter, r *http.Request) {
	store := s.threadsOrError(w)
	if store == nil {
		return
	}
	threads, err := store.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, threads)
}

func (s *Server) handleGetThread(w http.ResponseWriter, r *http.Request) {
	store := s.threadsOrError(w)
	if store == nil {
		return
	}
	t, err := store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteThread(w http.ResponseWriter, r *http.Request) {
	store := s.threadsOrError(w)
	if store == nil {
		return
	}
	if err := store.Delete(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers already sent; nothing actionable left but to log via the error.
		_, _ = fmt.Fprintf(w, "\n%v\n", err)
	}
}
