package entity

import (
	"fmt"

	"github.com/flanksource/clicky/api"
)

// boundFilter is a type-agnostic, already-resolved filter descriptor that
// resolveLookupCore renders into the lookup response. Both the typed entity path
// (resolveLookup) and the dynamic entity path (buildDynamicLookup) produce a
// slice of these, so the response shape is built in exactly one place.
type boundFilter struct {
	Key        string
	Label      string
	Type       string
	Multi      bool
	Searchable bool
	// TimeEnabled offers a clock on a range control; nil leaves the choice to the
	// control type.
	TimeEnabled *bool
	Selected    map[string]api.Textable
	// Limit caps this filter's option set. Zero takes lookupOptionsLimit, which
	// is also the ceiling — a filter cannot ask for a larger head than the
	// response is willing to carry.
	Limit int
	// Options returns the head set (query == "") or search matches, the true
	// total behind the head, and any per-value counts. A non-positive limit means
	// "no cap".
	Options func(query string, limit int) (FilterOptions, error)
}

// limit is the option cap for one filter: its own when it declares a smaller
// one, the package ceiling otherwise.
func (f boundFilter) limit() int {
	if f.Limit > 0 && f.Limit < lookupOptionsLimit {
		return f.Limit
	}
	return lookupOptionsLimit
}

// optionsRequest is the query and cap one filter is asked for.
//
// A targeted search asks for this filter's matches only; a searchable filter's
// head is its first N options, reported against the true total so the UI can
// show "… and N more" and decide whether to search; an unsearchable filter is
// complete by construction and enumerates uncapped.
func (f boundFilter) optionsRequest(searchKey, searchQuery string) (string, int) {
	switch {
	case f.Searchable && searchKey == f.Key && searchQuery != "":
		return searchQuery, f.limit()
	case f.Searchable:
		return "", f.limit()
	default:
		return "", 0
	}
}

// searchTarget reports which filter a targeted search names, or -1 when the
// request is a plain head request over all of them.
//
// A search asks about one filter. Answering it by re-enumerating every other
// filter's head set costs one backend round trip per filter per keystroke, and
// the client reads only the filter it asked about — so the rest is work nobody
// receives.
func searchTarget[T any](filters []T, describe func(T) (key string, searchable bool), searchKey, searchQuery string) int {
	if searchKey == "" || searchQuery == "" {
		return -1
	}
	for i, filter := range filters {
		if key, searchable := describe(filter); searchable && key == searchKey {
			return i
		}
	}
	return -1
}

func describeBoundFilter(f boundFilter) (string, bool) { return f.Key, f.Searchable }

// resolveLookupCore renders bound filters into the lookup response, applying the
// searchable head/search/total logic uniformly. It is the single place the
// lookup wire shape is built, shared by the typed and dynamic entity paths.
func resolveLookupCore(filters []boundFilter, searchKey, searchQuery string) (entityLookupResponse, error) {
	if target := searchTarget(filters, describeBoundFilter, searchKey, searchQuery); target >= 0 {
		filters = filters[target : target+1]
	}
	response := entityLookupResponse{
		Filters: make(map[string]entityLookupFilter, len(filters)),
	}
	for _, f := range filters {
		entry, err := lookupEntry(f, searchKey, searchQuery)
		if err != nil {
			return entityLookupResponse{}, err
		}
		response.Filters[f.Key] = entry
	}
	return response, nil
}

// lookupEntry resolves one filter's options into its wire entry. Only a
// searchable filter reports a total: an unsearchable one enumerates in full, so
// its option set is the answer rather than a head of one.
func lookupEntry(f boundFilter, searchKey, searchQuery string) (entityLookupFilter, error) {
	query, limit := f.optionsRequest(searchKey, searchQuery)
	result, err := f.Options(query, limit)
	if err != nil {
		return entityLookupFilter{}, err
	}
	// The counts are read by option value, so one keyed by a value the filter
	// does not offer has nowhere to render: only a source that mixed up its keys
	// produces one. A negative tally is not a row count either — it can only come
	// from a sentinel or an underflowed subtraction — and forwarding it would
	// render the bad value to the reader as if it were real.
	for value, count := range result.Counts {
		if _, offered := result.Options[value]; !offered {
			return entityLookupFilter{}, fmt.Errorf("filter %q counted %q, which is not among the options it offered", f.Key, value)
		}
		if count < 0 {
			return entityLookupFilter{}, fmt.Errorf("filter %q counted %d rows for %q, which is not a row count", f.Key, count, value)
		}
	}
	entry := entityLookupFilter{
		Label:       f.Label,
		Options:     toClickyNodeMap(result.Options),
		Counts:      result.Counts,
		Selected:    toClickyNodeMap(f.Selected),
		Multi:       f.Multi,
		Type:        f.Type,
		TimeEnabled: f.TimeEnabled,
	}
	if f.Searchable {
		// A search can overflow its cap just as a head set can — reporting the
		// total is what stops a clipped result being rendered as the whole answer.
		entry.Total = result.Total
		entry.Truncated = result.Total > len(result.Options)
	}
	return entry, nil
}
