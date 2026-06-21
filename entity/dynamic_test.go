package entity

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
)

const ticketSchema = `{
  "type": "object",
  "x-clicky-aliases": ["tkt"],
  "properties": {
    "id":     {"type": "string", "x-clicky-id": true},
    "title":  {"type": "string", "x-clicky-name": true},
    "status": {"type": "string", "x-clicky-filter": "dyn-status", "x-clicky-label": "Status"}
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

	It("builds a ListOpts type with only the filterable fields", func() {
		ps, err := parseSchema([]byte(ticketSchema))
		Expect(err).ToNot(HaveOccurred())
		lt := ps.listType()
		Expect(lt.NumField()).To(Equal(1))
		Expect(lt.Field(0).Tag.Get("flag")).To(Equal("status"))
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
		clicky.GenerateCLI(root)

		listCmd, _, err := root.Find([]string{"dyn-tickets", "list"})
		Expect(err).ToNot(HaveOccurred())
		Expect(listCmd).ToNot(BeNil())
		Expect(listCmd.Flags().Lookup("status")).ToNot(BeNil(), "the filter property becomes a CLI flag")

		// List returns rows wrapped with an injected _id.
		dataFunc := clicky.GetContextDataFunc(listCmd)
		Expect(dataFunc).ToNot(BeNil())
		rows, err := dataFunc(context.Background(), map[string]string{}, nil)
		Expect(err).ToNot(HaveOccurred())
		rowsJSON, err := json.Marshal(rows)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(rowsJSON)).To(ContainSubstring(`"_id":"t1"`))

		// The lookup resolves the named filter's options.
		lookupFunc := clicky.GetLookupFunc(listCmd)
		Expect(lookupFunc).ToNot(BeNil())
		lookup, err := lookupFunc(map[string]string{}, nil)
		Expect(err).ToNot(HaveOccurred())
		lookupJSON, err := json.Marshal(lookup)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(lookupJSON)).To(ContainSubstring("Open"))
		Expect(string(lookupJSON)).To(ContainSubstring("Closed"))
		Expect(string(lookupJSON)).To(ContainSubstring(`"status"`))
	})

	It("shares one named filter between a dynamic and a static entity", func() {
		NewDynamicEntity("dyn-issues", []byte(ticketSchema)).
			List(func(context.Context, map[string]string) ([]map[string]any, error) { return nil, nil }).
			Register()

		clicky.RegisterEntity(clicky.Entity[staticIssue, staticIssueOpts, staticIssue]{
			Name:    "static-issues",
			Filters: []clicky.Filter[staticIssueOpts]{Use[staticIssueOpts]("dyn-status").As("status")},
			List:    func(staticIssueOpts) ([]staticIssue, error) { return nil, nil },
		})

		root := &cobra.Command{Use: "root"}
		clicky.GenerateCLI(root)

		dynOptions := lookupOptionsJSON(root, "dyn-issues")
		staticOptions := lookupOptionsJSON(root, "static-issues")
		Expect(dynOptions).To(ContainSubstring("Open"))
		Expect(staticOptions).To(ContainSubstring("Open"))
		Expect(dynOptions).To(Equal(staticOptions), "both entities resolve the same shared filter options")
	})
})

type staticIssue struct {
	ID   string
	Name string
}

func (s staticIssue) GetID() string   { return s.ID }
func (s staticIssue) GetName() string { return s.Name }

type staticIssueOpts struct {
	Status string `flag:"status"`
}

func lookupOptionsJSON(root *cobra.Command, entityName string) string {
	listCmd, _, err := root.Find([]string{entityName, "list"})
	Expect(err).ToNot(HaveOccurred())
	lookupFunc := clicky.GetLookupFunc(listCmd)
	Expect(lookupFunc).ToNot(BeNil())
	resp, err := lookupFunc(map[string]string{}, nil)
	Expect(err).ToNot(HaveOccurred())
	data, err := json.Marshal(resp)
	Expect(err).ToNot(HaveOccurred())
	return string(data)
}
