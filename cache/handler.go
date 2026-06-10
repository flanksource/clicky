package cache

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// RegisterRoutes mounts the cache-browser endpoints on mux under prefix:
//
//	GET    {prefix}/cache/tree?prefix=&max=
//	GET    {prefix}/cache/key?key=
//	GET    {prefix}/cache/search?q=&limit=
//	GET    {prefix}/cache/stats
//	DELETE {prefix}/cache/key?key=
//	DELETE {prefix}/cache/prefix?prefix=
//
// Keys travel in query parameters, not path segments, because cache keys
// routinely contain ":" and "/". prefix is the leading path segment shared
// with the rest of the API (e.g. "/api/v1"); pass it without a trailing
// slash.
func RegisterRoutes(mux *http.ServeMux, b Browser, prefix string) {
	mux.Handle(prefix+"/cache/", Handler(b, prefix))
}

// Handler returns the cache-browser endpoints as a standalone http.Handler
// for callers that compose their own mux.
func Handler(b Browser, prefix string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+prefix+"/cache/tree", func(w http.ResponseWriter, r *http.Request) {
		max, err := intParam(r, "max")
		if err != nil {
			http.Error(w, "invalid max: "+err.Error(), http.StatusBadRequest)
			return
		}
		v, err := b.Tree(r.Context(), TreeRequest{
			Prefix:      r.URL.Query().Get("prefix"),
			MaxChildren: max,
		})
		respond(w, v, err)
	})
	mux.HandleFunc("GET "+prefix+"/cache/key", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "missing key", http.StatusBadRequest)
			return
		}
		v, err := b.Key(r.Context(), key)
		respond(w, v, err)
	})
	mux.HandleFunc("GET "+prefix+"/cache/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			http.Error(w, "missing q", http.StatusBadRequest)
			return
		}
		limit, err := intParam(r, "limit")
		if err != nil {
			http.Error(w, "invalid limit: "+err.Error(), http.StatusBadRequest)
			return
		}
		v, err := b.Search(r.Context(), SearchRequest{Query: q, Limit: limit})
		respond(w, v, err)
	})
	mux.HandleFunc("GET "+prefix+"/cache/stats", func(w http.ResponseWriter, r *http.Request) {
		v, err := b.Stats(r.Context())
		respond(w, v, err)
	})
	mux.HandleFunc("DELETE "+prefix+"/cache/key", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "missing key", http.StatusBadRequest)
			return
		}
		v, err := b.DeleteKey(r.Context(), key)
		respond(w, v, err)
	})
	mux.HandleFunc("DELETE "+prefix+"/cache/prefix", func(w http.ResponseWriter, r *http.Request) {
		// An empty prefix would wipe the whole keyspace; that must never be
		// reachable from a missing parameter.
		p := r.URL.Query().Get("prefix")
		if p == "" {
			http.Error(w, "missing prefix", http.StatusBadRequest)
			return
		}
		v, err := b.DeletePrefix(r.Context(), p)
		respond(w, v, err)
	})
	return mux
}

// respond writes v as JSON, mapping ErrKeyNotFound to 404 and any other
// error to 500.
func respond[T any](w http.ResponseWriter, v T, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrKeyNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func intParam(r *http.Request, name string) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}
