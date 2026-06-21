package clicky

import (
	"fmt"
	"testing"

	"github.com/flanksource/clicky/api"
)

type searchableTestOpts struct {
	Plan MultiFilter `flag:"plan"`
}

// searchableTestFilter is a SearchableFilter stub over a fixed pool of "plan"
// values. The head returns the first `limit` sorted values plus the true total;
// a query returns the matching subset (substring), including values that sort
// past the head — the behaviour the real DistinctColumnFilter provides via SQL.
type searchableTestFilter struct {
	pool         []string
	lastQuery    string
	lastLimit    int
	headRequests int
}

func (f *searchableTestFilter) Key() string   { return "plan" }
func (f *searchableTestFilter) Label() string { return "Plan" }

func (f *searchableTestFilter) Lookup(opts *searchableTestOpts) (map[string]api.Textable, error) {
	return nil, nil
}

func (f *searchableTestFilter) Options(opts searchableTestOpts) map[string]api.Textable {
	head, _ := f.OptionsWithQuery(opts, "", lookupOptionsLimit)
	return head
}

func (f *searchableTestFilter) OptionsWithQuery(_ searchableTestOpts, query string, limit int) (map[string]api.Textable, int) {
	f.lastQuery = query
	f.lastLimit = limit
	out := make(map[string]api.Textable)
	count := 0
	for _, v := range f.pool {
		if query != "" && !contains(v, query) {
			continue
		}
		if count >= limit {
			if query == "" {
				continue // keep counting the head total below
			}
			break
		}
		out[v] = api.Text{Content: v}
		count++
	}
	if query == "" {
		f.headRequests++
		return out, len(f.pool)
	}
	// match total isn't meaningful for a query call
	return out, 0
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func makePool(n int) []string {
	pool := make([]string, n)
	for i := 0; i < n; i++ {
		// zero-padded so lexical sort == numeric order
		pool[i] = fmt.Sprintf("plan-%04d", i)
	}
	return pool
}

func TestBuildLookupFunc_SearchableHeadReportsTruncatedAndTotal(t *testing.T) {
	filter := &searchableTestFilter{pool: makePool(250)}
	lookup := buildLookupFunc[searchableTestOpts]([]Filter[searchableTestOpts]{filter})

	data, err := lookup(map[string]string{}, nil)
	if err != nil {
		t.Fatalf("lookup returned error: %v", err)
	}
	resp := data.(entityLookupResponse)
	plan := resp.Filters["plan"]

	if plan.Total != 250 {
		t.Fatalf("expected total 250, got %d", plan.Total)
	}
	if !plan.Truncated {
		t.Fatalf("expected truncated=true when total (250) > head (%d)", len(plan.Options))
	}
	if len(plan.Options) != lookupOptionsLimit {
		t.Fatalf("expected head of %d options, got %d", lookupOptionsLimit, len(plan.Options))
	}
	if filter.lastQuery != "" {
		t.Fatalf("head request must pass an empty query, got %q", filter.lastQuery)
	}
}

func TestBuildLookupFunc_SearchableQueryFindsBeyondHead(t *testing.T) {
	filter := &searchableTestFilter{pool: makePool(250)}
	lookup := buildLookupFunc[searchableTestOpts]([]Filter[searchableTestOpts]{filter})

	// plan-0225 sorts past the 200-item head, so only a server-side search finds it.
	data, err := lookup(map[string]string{
		"__lookup_filter": "plan",
		"__lookup_q":      "plan-0225",
	}, nil)
	if err != nil {
		t.Fatalf("lookup returned error: %v", err)
	}
	resp := data.(entityLookupResponse)
	plan := resp.Filters["plan"]

	if _, ok := plan.Options["plan-0225"]; !ok {
		t.Fatalf("expected beyond-head value plan-0225 in search results, got %v", keysOf(plan.Options))
	}
	if filter.lastQuery != "plan-0225" {
		t.Fatalf("expected the filter to receive query plan-0225, got %q", filter.lastQuery)
	}
}

func TestBuildLookupFunc_StripsReservedParamsFromOpts(t *testing.T) {
	filter := &searchableTestFilter{pool: makePool(3)}
	lookup := buildLookupFunc[searchableTestOpts]([]Filter[searchableTestOpts]{filter})

	flags := map[string]string{"__lookup_filter": "plan", "__lookup_q": "plan-0001"}
	if _, err := lookup(flags, nil); err != nil {
		t.Fatalf("lookup returned error: %v", err)
	}
	// The reserved params must be removed so they never leak into buildOpts.
	if _, ok := flags["__lookup_filter"]; ok {
		t.Fatalf("__lookup_filter must be stripped from the flag map")
	}
	if _, ok := flags["__lookup_q"]; ok {
		t.Fatalf("__lookup_q must be stripped from the flag map")
	}
}

func keysOf(m map[string]clickyNode) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
