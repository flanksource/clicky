package entity

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/flanksource/clicky/api"
)

const ticketSchema = `{
  "type": "object",
  "x-clicky-aliases": ["tkt"],
  "properties": {
    "id":     {"type": "string", "x-clicky-id": true},
    "title":  {"type": "string", "x-clicky-name": true},
    "status": {"type": "string", "x-clicky-filter": "dyn-status", "x-clicky-filter-key": "filter.status", "x-clicky-label": "Status"}
  }
}`

var _ = Describe("parseSchema", func() {
	It("extracts id/name keys and orders fields deterministically", func() {
		ps, err := parseSchema([]byte(ticketSchema))
		Expect(err).ToNot(HaveOccurred())
		Expect(ps.IDKey).To(Equal("id"))
		Expect(ps.NameKey).To(Equal("title"))
		Expect(ps.Aliases).To(Equal([]string{"tkt"}))
		Expect([]string{ps.Fields[0].Name, ps.Fields[1].Name, ps.Fields[2].Name}).
			To(Equal([]string{"id", "status", "title"}), "fields are sorted by name")
	})

	It("defaults the name key to the id key when no x-clicky-name is set", func() {
		ps, err := parseSchema([]byte(`{"properties":{"uid":{"type":"string","x-clicky-id":true}}}`))
		Expect(err).ToNot(HaveOccurred())
		Expect(ps.NameKey).To(Equal("uid"))
	})

	It("errors when no property is marked x-clicky-id", func() {
		_, err := parseSchema([]byte(`{"properties":{"name":{"type":"string"}}}`))
		Expect(err).To(HaveOccurred())
	})

	It("errors on a null property schema instead of panicking", func() {
		_, err := parseSchema([]byte(`{"properties":{"id":{"type":"string","x-clicky-id":true},"bad":null}}`))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("bad"))
	})

	It("rejects multiple x-clicky-id properties", func() {
		_, err := parseSchema([]byte(`{"properties":{"a":{"type":"string","x-clicky-id":true},"b":{"type":"string","x-clicky-id":true}}}`))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("multiple x-clicky-id"))
	})

	It("rejects multiple x-clicky-name properties", func() {
		_, err := parseSchema([]byte(`{"properties":{"id":{"type":"string","x-clicky-id":true},"a":{"type":"string","x-clicky-name":true},"b":{"type":"string","x-clicky-name":true}}}`))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("multiple x-clicky-name"))
	})

	It("builds a ListOpts type with only the filterable fields", func() {
		ps, err := parseSchema([]byte(ticketSchema))
		Expect(err).ToNot(HaveOccurred())
		lt := ps.listType()
		Expect(lt.NumField()).To(Equal(1))
		Expect(lt.Field(0).Tag.Get("flag")).To(Equal("filter.status"))
	})
})

var _ = Describe("Dynamic entity registration", func() {
	BeforeEach(func() {
		resetFilterRegistry()
		RegisterFilter(NamedFilter{
			Name:  "dyn-status",
			Label: "Status",
			Source: StaticOptions(map[string]api.Textable{
				"open":   api.Text{Content: "Open"},
				"closed": api.Text{Content: "Closed"},
			}),
		})
	})

	It("registers an entity from a schema with a working list, flag, and lookup", func() {
		NewDynamicEntity("dyn-tickets", []byte(ticketSchema)).
			List(func(context.Context, map[string]string) ([]map[string]any, error) {
				return []map[string]any{{"id": "t1", "title": "Boot loop", "status": "open"}}, nil
			}).
			Get(func(_ context.Context, id string) (map[string]any, error) {
				return map[string]any{"id": id, "title": "Boot loop", "status": "open"}, nil
			}).
			Register()

		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)

		listCmd, _, err := root.Find([]string{"dyn-tickets", "list"})
		Expect(err).ToNot(HaveOccurred())
		Expect(listCmd).ToNot(BeNil())
		Expect(listCmd.Flags().Lookup("filter.status")).ToNot(BeNil(), "the bound filter key becomes a CLI flag")

		// List returns rows wrapped with an injected _id.
		dataFunc := GetContextDataFunc(listCmd)
		Expect(dataFunc).ToNot(BeNil())
		rows, err := dataFunc(context.Background(), map[string]string{}, nil)
		Expect(err).ToNot(HaveOccurred())
		rowsJSON, err := json.Marshal(rows)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(rowsJSON)).To(ContainSubstring(`"_id":"t1"`))

		// The lookup resolves the named filter's options.
		lookupFunc := GetLookupFunc(listCmd)
		Expect(lookupFunc).ToNot(BeNil())
		lookup, err := lookupFunc(map[string]string{}, nil)
		Expect(err).ToNot(HaveOccurred())
		lookupJSON, err := json.Marshal(lookup)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(lookupJSON)).To(ContainSubstring("Open"))
		Expect(string(lookupJSON)).To(ContainSubstring("Closed"))
		Expect(string(lookupJSON)).To(ContainSubstring(`"filter.status"`))
	})

	It("propagates named-filter lookup failures", func() {
		RegisterFilter(NamedFilter{Name: "dyn-error", Source: errorFilterSource{}})
		schema := []byte(`{"properties":{"id":{"type":"string","x-clicky-id":true},"value":{"type":"string","x-clicky-filter":"dyn-error"}}}`)
		NewDynamicEntity("dyn-errors", schema).
			List(func(context.Context, map[string]string) ([]map[string]any, error) { return nil, nil }).
			Register()

		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)
		listCmd, _, err := root.Find([]string{"dyn-errors", "list"})
		Expect(err).ToNot(HaveOccurred())
		_, err = GetLookupFunc(listCmd)(map[string]string{}, nil)
		Expect(err).To(MatchError("lookup unavailable"))
	})

	It("shares one named filter between a dynamic and a static entity", func() {
		NewDynamicEntity("dyn-issues", []byte(ticketSchema)).
			List(func(context.Context, map[string]string) ([]map[string]any, error) { return nil, nil }).
			Register()

		RegisterEntity(Entity[staticIssue, staticIssueOpts, staticIssue]{
			Name:    "static-issues",
			Filters: []Filter[staticIssueOpts]{Use[staticIssueOpts]("dyn-status").As("status")},
			List:    func(staticIssueOpts) ([]staticIssue, error) { return nil, nil },
		})

		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)

		dynOptions := lookupOptionsJSON(root, "dyn-issues")
		staticOptions := lookupOptionsJSON(root, "static-issues")
		Expect(dynOptions).To(ContainSubstring("Open"))
		Expect(staticOptions).To(ContainSubstring("Open"))
		Expect(dynOptions).To(ContainSubstring(`"filter.status"`))
		Expect(staticOptions).To(ContainSubstring(`"status"`))
	})

	// A caller may filter on an input the rows never carry — a query parameter,
	// say — which has no schema property to hang x-clicky-filter on.
	Describe("filters that are not schema properties", func() {
		It("serves a filter bound to a key with no matching property", func() {
			NewDynamicEntity("dyn-params", []byte(ticketSchema)).
				List(func(context.Context, map[string]string) ([]map[string]any, error) { return nil, nil }).
				Filter("region", "dyn-status").
				Register()

			root := &cobra.Command{Use: "root"}
			GenerateCLI(root)

			options := lookupOptionsJSON(root, "dyn-params")
			Expect(options).To(ContainSubstring(`"region"`), "the explicit filter is offered")
			Expect(options).To(ContainSubstring(`"filter.status"`), "the schema filter still is")
			Expect(options).To(ContainSubstring("Open"))
		})

		It("carries the label and multi flag of the named filter it binds", func() {
			RegisterFilter(NamedFilter{
				Name: "dyn-regions", Label: "Regions", Type: "multi-filter", Multi: true,
				Source: StaticOptions(map[string]api.Textable{"eu": api.Text{Content: "eu"}}),
			})
			NewDynamicEntity("dyn-labelled", []byte(ticketSchema)).
				List(func(context.Context, map[string]string) ([]map[string]any, error) { return nil, nil }).
				Filter("regions", "dyn-regions").
				Register()

			root := &cobra.Command{Use: "root"}
			GenerateCLI(root)

			Expect(lookupOptionsJSON(root, "dyn-labelled")).To(SatisfyAll(
				ContainSubstring(`"Regions"`),
				ContainSubstring(`"multi-filter"`),
			))
		})

		It("resolves the selected values of an explicit filter", func() {
			NewDynamicEntity("dyn-selected", []byte(ticketSchema)).
				List(func(context.Context, map[string]string) ([]map[string]any, error) { return nil, nil }).
				Filter("region", "dyn-status").
				Register()

			root := &cobra.Command{Use: "root"}
			GenerateCLI(root)
			listCmd, _, err := root.Find([]string{"dyn-selected", "list"})
			Expect(err).ToNot(HaveOccurred())

			// "!closed" is an exclusion, so the mode marker is stripped before the
			// source is asked to label the value.
			resp, err := GetLookupFunc(listCmd)(map[string]string{"region": "open,!closed"}, nil)
			Expect(err).ToNot(HaveOccurred())
			data, err := json.Marshal(resp)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(data)).To(SatisfyAll(ContainSubstring("Open"), ContainSubstring("Closed")))
		})

		It("panics when an explicit filter shadows a schema filter key", func() {
			Expect(func() {
				NewDynamicEntity("dyn-clash", []byte(ticketSchema)).
					List(func(context.Context, map[string]string) ([]map[string]any, error) { return nil, nil }).
					Filter("filter.status", "dyn-status").
					Register()
			}).To(PanicWith(ContainSubstring("filter.status")))
		})

		It("panics when an explicit filter names no registered filter", func() {
			Expect(func() {
				NewDynamicEntity("dyn-unknown", []byte(ticketSchema)).
					List(func(context.Context, map[string]string) ([]map[string]any, error) { return nil, nil }).
					Filter("region", "never-registered").
					Register()
			}).To(Panic())
		})

		It("panics when an explicit filter has no key", func() {
			Expect(func() {
				NewDynamicEntity("dyn-keyless", []byte(ticketSchema)).
					List(func(context.Context, map[string]string) ([]map[string]any, error) { return nil, nil }).
					Filter("", "dyn-status").
					Register()
			}).To(Panic())
		})
	})
})

type staticIssue struct {
	ID   string
	Name string
}

type errorFilterSource struct{}

func (errorFilterSource) Options(FilterContext, string, int) (map[string]api.Textable, int, error) {
	return nil, 0, fmt.Errorf("lookup unavailable")
}

func (errorFilterSource) Resolve(FilterContext, []string) (map[string]api.Textable, error) {
	return nil, nil
}

func (s staticIssue) GetID() string   { return s.ID }
func (s staticIssue) GetName() string { return s.Name }

type staticIssueOpts struct {
	Status string `flag:"status"`
}

func lookupOptionsJSON(root *cobra.Command, entityName string) string {
	listCmd, _, err := root.Find([]string{entityName, "list"})
	Expect(err).ToNot(HaveOccurred())
	lookupFunc := GetLookupFunc(listCmd)
	Expect(lookupFunc).ToNot(BeNil())
	resp, err := lookupFunc(map[string]string{}, nil)
	Expect(err).ToNot(HaveOccurred())
	data, err := json.Marshal(resp)
	Expect(err).ToNot(HaveOccurred())
	return string(data)
}
