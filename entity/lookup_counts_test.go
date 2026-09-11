package entity

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/flanksource/clicky/api"
)

// statusRows is how many rows hold each status in the source the counted
// filter stands for.
var statusRows = map[string]int{"open": 7, "closed": 2}

// countedStatuses answers like a backend that counts: every status it offers,
// each with the rows behind it.
func countedStatuses(FilterContext, string, int) (FilterOptions, error) {
	options := make(map[string]api.Textable, len(statusRows))
	counts := make(map[string]int, len(statusRows))
	for value, rows := range statusRows {
		options[value] = api.Text{Content: value}
		counts[value] = rows
	}
	return FilterOptions{Options: options, Counts: counts, Total: len(options)}, nil
}

const countedSchema = `{
  "type": "object",
  "properties": {
    "id":     {"type": "string", "x-clicky-id": true},
    "status": {"type": "string", "x-clicky-filter": "counted-status", "x-clicky-filter-key": "filter.status"}
  }
}`

// lookupWire is the lookup response as the browser reads it: one object per
// filter key, decoded from the JSON the transport writes.
func lookupWire(root *cobra.Command, entityName string) map[string]map[string]any {
	var body struct {
		Filters map[string]map[string]any `json:"filters"`
	}
	Expect(json.Unmarshal([]byte(lookupOptionsJSON(root, entityName)), &body)).To(Succeed())
	return body.Filters
}

func registerCountedDynamicEntity(name string) {
	NewDynamicEntity(name, []byte(countedSchema)).
		List(func(context.Context, map[string]string) ([]map[string]any, error) { return nil, nil }).
		Register()
}

type countedOuterOpts struct {
	staticIssueOpts
}

func liftedCountedFilters(filterName string) []Filter[countedOuterOpts] {
	return LiftFilters[countedOuterOpts, staticIssueOpts](
		[]Filter[staticIssueOpts]{Use[staticIssueOpts](filterName).As("status")},
		func(opts *countedOuterOpts) *staticIssueOpts { return &opts.staticIssueOpts },
	)
}

var _ = Describe("Lookup facet counts", func() {
	BeforeEach(func() {
		resetFilterRegistry()
		RegisterFilter(NamedFilter{Name: "counted-status", Source: CountedOptions(countedStatuses)})
		RegisterFilter(NamedFilter{Name: "uncounted-status", Source: StaticOptions(map[string]api.Textable{
			"open": api.Text{Content: "Open"},
		})})
	})

	It("carries a dynamic entity's per-value counts, keyed like its options", func() {
		registerCountedDynamicEntity("counted-tickets")
		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)

		status := lookupWire(root, "counted-tickets")["filter.status"]
		Expect(status["counts"]).To(Equal(map[string]any{"open": float64(7), "closed": float64(2)}))
		Expect(status["options"]).To(And(HaveKey("open"), HaveKey("closed")))
	})

	It("carries the counts of a named filter attached to a typed entity", func() {
		RegisterEntity(Entity[staticIssue, staticIssueOpts, staticIssue]{
			Name:    "counted-issues",
			Filters: []Filter[staticIssueOpts]{Use[staticIssueOpts]("counted-status").As("status")},
			List:    func(staticIssueOpts) ([]staticIssue, error) { return nil, nil },
		})
		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)

		Expect(lookupWire(root, "counted-issues")["status"]["counts"]).
			To(Equal(map[string]any{"open": float64(7), "closed": float64(2)}))
	})

	It("carries named-filter counts through LiftFilters", func() {
		RegisterEntity(Entity[staticIssue, countedOuterOpts, staticIssue]{
			Name: "counted-lifted-issues", Filters: liftedCountedFilters("counted-status"),
			List: func(countedOuterOpts) ([]staticIssue, error) { return nil, nil },
		})
		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)

		Expect(lookupWire(root, "counted-lifted-issues")["status"]["counts"]).
			To(Equal(map[string]any{"open": float64(7), "closed": float64(2)}))
	})

	It("propagates named-filter errors through LiftFilters", func() {
		RegisterFilter(NamedFilter{Name: "failing-status", Source: errorFilterSource{}})
		RegisterEntity(Entity[staticIssue, countedOuterOpts, staticIssue]{
			Name: "failed-lifted-issues", Filters: liftedCountedFilters("failing-status"),
			List: func(countedOuterOpts) ([]staticIssue, error) { return nil, nil },
		})
		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)

		listCmd, _, err := root.Find([]string{"failed-lifted-issues", "list"})
		Expect(err).ToNot(HaveOccurred())
		_, err = GetLookupFunc(listCmd)(map[string]string{}, nil)
		Expect(err).To(MatchError("lookup unavailable"))
	})

	It("omits counts for a source that does not count", func() {
		NewDynamicEntity("uncounted-tickets", []byte(countedSchema)).
			List(func(context.Context, map[string]string) ([]map[string]any, error) { return nil, nil }).
			Filter("region", "uncounted-status").
			Register()
		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)

		region := lookupWire(root, "uncounted-tickets")["region"]
		Expect(region).To(HaveKey("options"))
		Expect(region).ToNot(HaveKey("counts"))
	})

	// A count for a value the control does not list has nowhere to render, and
	// only a source that mixed up its keys could produce one.
	It("refuses a count for a value the source did not offer", func() {
		response, err := resolveDynamicLookup(context.Background(), []DynamicFilter{{
			Key: "filter.status", Searchable: true,
			CountedOptions: func(context.Context, map[string]string, string, int) (FilterOptions, error) {
				return FilterOptions{
					Options: map[string]api.Textable{"open": api.Text{Content: "open"}},
					Counts:  map[string]int{"open": 7, "archived": 4},
					Total:   1,
				}, nil
			},
		}}, map[string]string{})
		Expect(err).To(MatchError(And(ContainSubstring("filter.status"), ContainSubstring("archived"))))
		Expect(response.Filters).To(BeEmpty())
	})
})
