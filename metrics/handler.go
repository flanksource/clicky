package metrics

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/flanksource/clicky/route"
)

// defaultWindow is the look-back applied when a request omits `since`.
const defaultWindow = time.Hour

// queryResponse is the JSON envelope returned by the metrics endpoint.
type queryResponse struct {
	ID     string  `json:"id"`
	Points []Point `json:"points"`
}

// RegisterRoutes mounts the metrics read endpoint on router under prefix:
//
//	GET {prefix}/metrics/{id}?since=&until=
//
// The handler owns all request parsing, querying, and JSON encoding — callers
// supply only a Timeseries. prefix is the leading path segment shared with the
// rest of the API (e.g. "/api/v1"); it is used verbatim, so pass it without a
// trailing slash.
func RegisterRoutes(router *route.Router, ts Timeseries, prefix string) {
	router.RawFunc("GET "+prefix+"/metrics/{id}", func(w http.ResponseWriter, r *http.Request) {
		serveQuery(w, r, ts)
	}, route.Meta{Entity: "metrics", Verb: "query", ReadOnly: true, IDParam: "id"})
}

// Handler returns the metrics endpoint as a standalone http.Handler for callers
// that compose their own mux. It routes through the same declaration as
// RegisterRoutes, so a request is observed identically either way. It expects to
// be mounted such that request paths look like {prefix}/metrics/{id}.
func Handler(ts Timeseries, prefix string) http.Handler {
	router := route.NewRouter(nil)
	RegisterRoutes(router, ts, prefix)
	return router
}

func serveQuery(w http.ResponseWriter, r *http.Request, ts Timeseries) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing metric id", http.StatusBadRequest)
		return
	}

	now := time.Now()
	until, err := resolveBound(r.URL.Query().Get("until"), now, now)
	if err != nil {
		http.Error(w, "invalid until: "+err.Error(), http.StatusBadRequest)
		return
	}
	since, err := resolveBound(r.URL.Query().Get("since"), now, until.Add(-defaultWindow))
	if err != nil {
		http.Error(w, "invalid since: "+err.Error(), http.StatusBadRequest)
		return
	}
	if since.After(until) {
		http.Error(w, "since must be before until", http.StatusBadRequest)
		return
	}

	points, err := ts.Query(QueryRequest{ID: id, Since: since, Until: until})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(queryResponse{ID: id, Points: points})
}

// resolveBound interprets a since/until query value. An empty value yields
// fallback. An RFC3339 timestamp is returned as-is. A Go duration (e.g. "1h",
// "15m") is interpreted relative to now: a look-back for positive durations
// (now-d) so "?since=1h" reads "the last hour".
func resolveBound(raw string, now, fallback time.Time) (time.Time, error) {
	if raw == "" {
		return fallback, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return time.Time{}, err
	}
	return now.Add(-absDuration(d)), nil
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
