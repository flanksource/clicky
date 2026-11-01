package metrics

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Series is one plottable line: the points of a single label set, ready for a
// clicky-ui metrics panel without any client-side reshaping.
type Series struct {
	// ID is unique within a response: the requested id itself, or the
	// requested id plus "#" and the canonical label set when one requested id
	// expands to several series (a query profile grouped by pod, say).
	ID string `json:"id"`
	// Source is the id the panel asked for, so a panel can find every series a
	// request of its expanded into.
	Source string            `json:"source"`
	Label  string            `json:"label,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	// Unit is a clicky-ui formatUnit key (percent, bytes, ms, …).
	Unit   string  `json:"unit,omitempty"`
	Points []Point `json:"points"`
}

// SeriesRequest reads every series of IDs in [Since, Until]. Step is the
// resolution a computing source (Prometheus) evaluates at; a stored source
// returns its raw points and ignores it.
type SeriesRequest struct {
	IDs   []string
	Since time.Time
	Until time.Time
	Step  time.Duration
}

// SeriesSource answers many ids in one call, so a page of panels costs one
// query per tick rather than one per series.
type SeriesSource interface {
	QuerySeries(ctx context.Context, req SeriesRequest) ([]Series, error)
}

// UnknownSeriesError reports an id no registered source owns.
type UnknownSeriesError struct{ ID string }

func (e *UnknownSeriesError) Error() string {
	return fmt.Sprintf("no series source for metric id %q", e.ID)
}

// Sources routes each id to the source registered for its prefix — the text
// before the first ":" (e.g. "profile:pod-cpu?namespace=x"). Ids whose prefix
// is not registered go to the "" source; store ids such as task metrics
// ("<runID>:cpu") contain colons, so an unregistered prefix cannot be told
// apart from a plain id. With no "" source such an id is an UnknownSeriesError.
type Sources map[string]SeriesSource

func (s Sources) QuerySeries(ctx context.Context, req SeriesRequest) ([]Series, error) {
	groups := map[string][]string{}
	var order []string
	for _, id := range req.IDs {
		key := s.route(id)
		if _, ok := s[key]; !ok {
			return nil, &UnknownSeriesError{ID: id}
		}
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], id)
	}
	out := []Series{}
	for _, key := range order {
		sub := req
		sub.IDs = groups[key]
		series, err := s[key].QuerySeries(ctx, sub)
		if err != nil {
			return nil, fmt.Errorf("query %q series %v: %w", key, sub.IDs, err)
		}
		out = append(out, series...)
	}
	return out, nil
}

func (s Sources) route(id string) string {
	if prefix, _, ok := strings.Cut(id, ":"); ok && prefix != "" {
		if _, registered := s[prefix]; registered {
			return prefix
		}
	}
	return ""
}

// StoreSource serves a Timeseries store as a SeriesSource: one series per id,
// labelled by nothing but its id.
func StoreSource(ts Timeseries) SeriesSource { return storeSource{ts: ts} }

type storeSource struct{ ts Timeseries }

func (s storeSource) QuerySeries(_ context.Context, req SeriesRequest) ([]Series, error) {
	out := make([]Series, 0, len(req.IDs))
	for _, id := range req.IDs {
		points, err := s.ts.Query(QueryRequest{ID: id, Since: req.Since, Until: req.Until})
		if err != nil {
			return nil, fmt.Errorf("metric %s: %w", id, err)
		}
		if points == nil {
			points = []Point{}
		}
		out = append(out, Series{ID: id, Source: id, Points: points})
	}
	return out, nil
}
