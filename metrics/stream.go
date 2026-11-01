package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/route"
	"github.com/flanksource/clicky/sse"
)

const (
	defaultStreamInterval = 5 * time.Second
	minStreamInterval     = time.Second
	// stepPoints is how many points a computed series is aimed at when the
	// request names no step: enough for a panel's width, not one per second.
	stepPoints = 300
)

// StreamFrame is the payload of the stream's "snapshot" and "append" events.
// A snapshot carries every series in full; an append carries only the series
// with points newer than the last frame (just those points) plus the ids of
// series that no longer exist. Clients trim points older than their window.
type StreamFrame struct {
	Series  []Series `json:"series"`
	Removed []string `json:"removed,omitempty"`
}

// RegisterStreamRoutes mounts the batched metrics stream on router:
//
//	GET {prefix}/metrics/stream?id=a&id=b&since=15m&until=&step=&interval=5s
//
// One subscription serves every id a page of panels shows: each interval it
// queries src once for all of them and sends an "append" frame (a "snapshot"
// first), or a ping when nothing changed. A query failure is sent as an
// "error" event {"error": msg} and ends the stream. Subscribe through the sse
// Hub (clicky-ui createEventHub) so a tab holds one connection for all streams.
// since/until take what GET {prefix}/metrics/{id} takes and are re-resolved
// every tick, so a duration window slides.
func RegisterStreamRoutes(router *route.Router, src SeriesSource, prefix string) {
	router.RawFunc("GET "+prefix+"/metrics/stream", func(w http.ResponseWriter, r *http.Request) {
		serveStream(w, r, src)
	}, route.Meta{Entity: "metrics", Verb: "stream", ReadOnly: true})
}

type streamQuery struct {
	ids                []string
	since, until, step string
	interval           time.Duration
}

func parseStreamQuery(r *http.Request) (streamQuery, error) {
	values := r.URL.Query()
	q := streamQuery{since: values.Get("since"), until: values.Get("until"), step: values.Get("step"), interval: defaultStreamInterval}
	seen := map[string]bool{}
	for _, id := range values["id"] {
		if id == "" {
			return q, errors.New("empty metric id")
		}
		if !seen[id] {
			seen[id] = true
			q.ids = append(q.ids, id)
		}
	}
	if len(q.ids) == 0 {
		return q, errors.New("at least one id is required")
	}
	if raw := values.Get("interval"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return q, fmt.Errorf("invalid interval: %w", err)
		}
		if d < minStreamInterval {
			return q, fmt.Errorf("interval must be at least %s, got %s", minStreamInterval, d)
		}
		q.interval = d
	}
	_, err := q.request(time.Now())
	return q, err
}

func (q streamQuery) request(now time.Time) (SeriesRequest, error) {
	until, err := resolveBound(q.until, now, now)
	if err != nil {
		return SeriesRequest{}, fmt.Errorf("invalid until: %w", err)
	}
	since, err := resolveBound(q.since, now, until.Add(-defaultWindow))
	if err != nil {
		return SeriesRequest{}, fmt.Errorf("invalid since: %w", err)
	}
	if since.After(until) {
		return SeriesRequest{}, errors.New("since must be before until")
	}
	step := max(until.Sub(since)/stepPoints, time.Second)
	if q.step != "" {
		if step, err = time.ParseDuration(q.step); err != nil || step <= 0 {
			return SeriesRequest{}, fmt.Errorf("invalid step %q: must be a positive duration", q.step)
		}
	}
	return SeriesRequest{IDs: q.ids, Since: since, Until: until, Step: step}, nil
}

func serveStream(w http.ResponseWriter, r *http.Request, src SeriesSource) {
	q, err := parseStreamQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	writer := sse.NewWriter(w)
	ticker := time.NewTicker(q.interval)
	defer ticker.Stop()
	sent := map[string]time.Time{}
	for first := true; ; first = false {
		series, err := querySeries(r, src, q)
		switch {
		case err != nil && ctx.Err() != nil:
			return
		case err != nil && !writer.Started():
			status := http.StatusInternalServerError
			if unknown := (*UnknownSeriesError)(nil); errors.As(err, &unknown) {
				status = http.StatusBadRequest
			}
			http.Error(w, err.Error(), status)
			return
		case err != nil:
			_ = writer.Send(entity.StreamEvent{Name: "error", Data: map[string]string{"error": err.Error()}})
			return
		}
		frame, changed := nextFrame(series, sent, first)
		event := "append"
		if first {
			event = "snapshot"
		}
		if first || changed {
			err = writer.Send(entity.StreamEvent{Name: event, Data: frame})
		} else {
			err = writer.Ping()
		}
		if err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func querySeries(r *http.Request, src SeriesSource, q streamQuery) ([]Series, error) {
	req, err := q.request(time.Now())
	if err != nil {
		return nil, err
	}
	return src.QuerySeries(r.Context(), req)
}

// nextFrame builds the frame for series given the last point already sent per
// series id, and advances sent. full sends every point (the snapshot).
func nextFrame(series []Series, sent map[string]time.Time, full bool) (StreamFrame, bool) {
	frame := StreamFrame{Series: []Series{}}
	current := map[string]bool{}
	for _, s := range series {
		current[s.ID] = true
		last, known := sent[s.ID]
		fresh := s.Points
		if known && !full {
			fresh = []Point{}
			for _, p := range s.Points {
				if p.At.After(last) {
					fresh = append(fresh, p)
				}
			}
		}
		if len(fresh) > 0 {
			last = fresh[len(fresh)-1].At
		}
		sent[s.ID] = last
		if full || !known || len(fresh) > 0 {
			s.Points = fresh
			frame.Series = append(frame.Series, s)
		}
	}
	for id := range sent {
		if !current[id] {
			frame.Removed = append(frame.Removed, id)
			delete(sent, id)
		}
	}
	slices.Sort(frame.Removed)
	return frame, len(frame.Series) > 0 || len(frame.Removed) > 0
}
