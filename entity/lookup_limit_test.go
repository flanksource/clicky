package entity

import (
	"context"
	"testing"

	"github.com/flanksource/clicky/api"
)

// countingFilter is a DynamicFilter over a fixed pool that records what it was
// asked for. The counters are the point: a targeted search must leave every
// other filter's Options and Selected untouched, and only a probe that can say
// "never called" proves it.
type countingFilter struct {
	key           string
	pool          []string
	limit         int
	optionsCalls  int
	selectedCalls int
	lastLimit     int
	lastQuery     string
}

func (c *countingFilter) filter() DynamicFilter {
	return DynamicFilter{
		Key:        c.key,
		Label:      c.key,
		Type:       "multi-filter",
		Multi:      true,
		Searchable: true,
		Limit:      c.limit,
		Options: func(_ context.Context, _ map[string]string, query string, limit int) (map[string]api.Textable, int, error) {
			c.optionsCalls++
			c.lastLimit = limit
			c.lastQuery = query
			matched := make([]string, 0, len(c.pool))
			for _, value := range c.pool {
				if query == "" || contains(value, query) {
					matched = append(matched, value)
				}
			}
			out := make(map[string]api.Textable)
			for _, value := range matched {
				if limit > 0 && len(out) >= limit {
					break
				}
				out[value] = api.Text{}.Append(value)
			}
			return out, len(matched), nil
		},
		Selected: func(context.Context, map[string]string) (map[string]api.Textable, error) {
			c.selectedCalls++
			return nil, nil
		},
	}
}

func resolveFilters(t *testing.T, flags map[string]string, filters ...DynamicFilter) entityLookupResponse {
	t.Helper()
	response, err := resolveDynamicLookup(context.Background(), filters, flags)
	if err != nil {
		t.Fatalf("resolveDynamicLookup: %v", err)
	}
	return response
}

func TestLookupHeadTakesTheFiltersOwnLimit(t *testing.T) {
	probe := &countingFilter{key: "service", pool: makePool(400), limit: 50}

	entry := resolveFilters(t, map[string]string{}, probe.filter()).Filters["service"]

	if probe.lastLimit != 50 {
		t.Fatalf("expected the filter to be asked for 50 options, got %d", probe.lastLimit)
	}
	if len(entry.Options) != 50 {
		t.Fatalf("expected a head of 50, got %d", len(entry.Options))
	}
	if entry.Total != 400 || !entry.Truncated {
		t.Fatalf("expected total=400 truncated=true, got total=%d truncated=%v", entry.Total, entry.Truncated)
	}
}

// The declared limit is a request, not an override: a filter cannot ask for a
// larger head than the response is willing to carry.
func TestLookupHeadClampsADeclaredLimitToTheCeiling(t *testing.T) {
	for name, declared := range map[string]int{
		"above the ceiling": MaxLookupOptions + 300,
		"undeclared":        0,
	} {
		t.Run(name, func(t *testing.T) {
			probe := &countingFilter{key: "service", pool: makePool(1000), limit: declared}

			entry := resolveFilters(t, map[string]string{}, probe.filter()).Filters["service"]

			if probe.lastLimit != MaxLookupOptions {
				t.Fatalf("expected the ceiling %d, got %d", MaxLookupOptions, probe.lastLimit)
			}
			if len(entry.Options) != MaxLookupOptions {
				t.Fatalf("expected a head of %d, got %d", MaxLookupOptions, len(entry.Options))
			}
		})
	}
}

// A search overflows its cap as readily as a head set does. Reporting the total
// is what stops a clipped result being rendered as the whole answer.
func TestLookupSearchReportsItsOwnTruncation(t *testing.T) {
	probe := &countingFilter{key: "service", pool: makePool(400), limit: 50}
	flags := map[string]string{lookupFilterKeyParam: "service", lookupQueryParam: "plan-0"}

	entry := resolveFilters(t, flags, probe.filter()).Filters["service"]

	// "plan-0" matches plan-0000..plan-0399 — every value in the pool.
	if entry.Total != 400 {
		t.Fatalf("expected the match count 400 behind the cap, got %d", entry.Total)
	}
	if !entry.Truncated {
		t.Fatalf("expected truncated=true when 400 matches are served 50 at a time")
	}
	if len(entry.Options) != 50 {
		t.Fatalf("expected 50 matches, got %d", len(entry.Options))
	}
}

func TestLookupSearchThatFitsIsNotTruncated(t *testing.T) {
	probe := &countingFilter{key: "service", pool: makePool(400), limit: 50}
	flags := map[string]string{lookupFilterKeyParam: "service", lookupQueryParam: "plan-0042"}

	entry := resolveFilters(t, flags, probe.filter()).Filters["service"]

	if entry.Truncated || entry.Total != 1 {
		t.Fatalf("expected one exact match reported untruncated, got total=%d truncated=%v", entry.Total, entry.Truncated)
	}
}

// The cost this guards: a keystroke used to enumerate every filter on the
// entity, so a profile with six filters paid six backend round trips per
// debounce interval for five answers the client discards.
func TestTargetedSearchLeavesTheOtherFiltersAlone(t *testing.T) {
	searched := &countingFilter{key: "service", pool: makePool(400), limit: 50}
	bystander := &countingFilter{key: "region", pool: makePool(10), limit: 50}
	flags := map[string]string{lookupFilterKeyParam: "service", lookupQueryParam: "plan-01"}

	response := resolveFilters(t, flags, searched.filter(), bystander.filter())

	if bystander.optionsCalls != 0 {
		t.Fatalf("expected the unsearched filter's options to go unread, got %d calls", bystander.optionsCalls)
	}
	if bystander.selectedCalls != 0 {
		t.Fatalf("expected the unsearched filter's selection to go unresolved, got %d calls", bystander.selectedCalls)
	}
	if searched.optionsCalls != 1 {
		t.Fatalf("expected the searched filter to be read once, got %d", searched.optionsCalls)
	}
	if _, present := response.Filters["region"]; present {
		t.Fatalf("expected only the searched filter in the response, got %v", filterKeys(response))
	}
}

func TestHeadRequestStillResolvesEveryFilter(t *testing.T) {
	first := &countingFilter{key: "service", pool: makePool(5), limit: 50}
	second := &countingFilter{key: "region", pool: makePool(5), limit: 50}

	for name, flags := range map[string]map[string]string{
		"no search at all":     {},
		"a key with no query":  {lookupFilterKeyParam: "service"},
		"a query with no key":  {lookupQueryParam: "plan"},
		"a key nothing serves": {lookupFilterKeyParam: "nonexistent", lookupQueryParam: "plan"},
	} {
		t.Run(name, func(t *testing.T) {
			first.optionsCalls, second.optionsCalls = 0, 0
			response := resolveFilters(t, flags, first.filter(), second.filter())

			if len(response.Filters) != 2 {
				t.Fatalf("expected every filter in the response, got %v", filterKeys(response))
			}
			if first.optionsCalls != 1 || second.optionsCalls != 1 {
				t.Fatalf("expected both filters read once, got %d and %d", first.optionsCalls, second.optionsCalls)
			}
		})
	}
}

// A filter that is not searchable enumerates in full and says nothing about a
// total, which is the pre-existing contract for an option set that is complete
// by construction.
func TestUnsearchableFilterIsNeitherCappedNorTruncated(t *testing.T) {
	probe := &countingFilter{key: "service", pool: makePool(400), limit: 50}
	unsearchable := probe.filter()
	unsearchable.Searchable = false

	entry := resolveFilters(t, map[string]string{}, unsearchable).Filters["service"]

	if probe.lastLimit != 0 {
		t.Fatalf("expected an uncapped enumeration, got limit %d", probe.lastLimit)
	}
	if len(entry.Options) != 400 || entry.Truncated || entry.Total != 0 {
		t.Fatalf("expected all 400 options with no total, got %d options total=%d truncated=%v",
			len(entry.Options), entry.Total, entry.Truncated)
	}
}

// limitedTestFilter is the typed-entity equivalent: a SearchableFilter that also
// implements LimitedFilter.
type limitedTestFilter struct {
	*searchableTestFilter
	limit int
}

func (f *limitedTestFilter) LookupLimit() int { return f.limit }

func TestTypedFilterCarriesItsLimitThrough(t *testing.T) {
	inner := &searchableTestFilter{pool: makePool(250)}
	lookup := buildLookupFunc[searchableTestOpts]([]Filter[searchableTestOpts]{
		&limitedTestFilter{searchableTestFilter: inner, limit: 25},
	})

	data, err := lookup(map[string]string{}, nil)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	entry := data.(entityLookupResponse).Filters["plan"]

	if inner.lastLimit != 25 {
		t.Fatalf("expected the declared limit 25 to reach the filter, got %d", inner.lastLimit)
	}
	if len(entry.Options) != 25 || entry.Total != 250 || !entry.Truncated {
		t.Fatalf("expected 25 of 250 reported truncated, got %d options total=%d truncated=%v",
			len(entry.Options), entry.Total, entry.Truncated)
	}
}

func filterKeys(response entityLookupResponse) []string {
	out := make([]string, 0, len(response.Filters))
	for key := range response.Filters {
		out = append(out, key)
	}
	return out
}
