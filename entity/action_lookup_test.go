package entity

import (
	"context"
	"sync"

	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

type actionLookupOptions struct {
	Database     string `flag:"database"`
	ConnectionID string
}

func (actionLookupOptions) ClickyActionFlags() {}

func (actionLookupOptions) Filters() []Filter[actionLookupOptions] {
	return []Filter[actionLookupOptions]{actionDatabaseFilter{}}
}

func (opts *actionLookupOptions) SetClickyActionID(id string) {
	opts.ConnectionID = id
}

type actionDatabaseFilter struct{}

func (actionDatabaseFilter) Key() string   { return "database" }
func (actionDatabaseFilter) Label() string { return "Database" }
func (actionDatabaseFilter) Lookup(opts *actionLookupOptions) (map[string]api.Textable, error) {
	if opts.Database == "" {
		return nil, nil
	}
	opts.Database = "database/" + opts.Database
	return map[string]api.Textable{opts.Database: api.Text{Content: opts.Database}}, nil
}
func (actionDatabaseFilter) Options(opts actionLookupOptions) map[string]api.Textable {
	value := opts.ConnectionID + "/analytics"
	return map[string]api.Textable{value: api.Text{Content: value}}
}

var _ = Describe("typed entity action lookups", func() {
	BeforeEach(func() {
		entityRegistryMu.Lock()
		entityRegistry = nil
		entityRegistryMu.Unlock()
		dataFuncRegistry = sync.Map{}
		contextDataFuncRegistry = sync.Map{}
		lookupFuncRegistry = sync.Map{}
		contextLookupFuncRegistry = sync.Map{}
		responseMetaRegistry = sync.Map{}
	})

	It("registers Filterable action flags as lookups with the entity ID in their options", func(ctx SpecContext) {
		var received actionLookupOptions
		RegisterEntity(Entity[entityFilterTestEntity, struct{}, any]{
			Name: "lookup-action",
			Actions: []EntityAction{
				TypedActionWithContext("inspect", actionLookupOptions{}, func(_ context.Context, _ string, opts actionLookupOptions) (any, error) {
					received = opts
					return opts, nil
				}),
			},
		})

		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)
		command, _, err := root.Find([]string{"lookup-action", "inspect"})
		Expect(err).ToNot(HaveOccurred())
		Expect(command).ToNot(BeNil())
		Expect(GetCommandOpenAPIMeta(command).SupportsLookup).To(BeTrue())
		completion, exists := command.GetFlagCompletionFunc("database")
		Expect(exists).To(BeTrue())
		completions, directive := completion(command, []string{"connection-main"}, "")
		Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
		Expect(completions).To(ConsistOf("connection-main/analytics"))

		lookup := GetContextLookupFunc(command)
		Expect(lookup).ToNot(BeNil())
		response, err := lookup(ctx, map[string]string{"database": "analytics"}, []string{"connection-main"})
		Expect(err).ToNot(HaveOccurred())
		resolved := response.(entityLookupResponse)
		Expect(resolved.Filters["database"].Options).To(HaveKey("connection-main/analytics"))
		Expect(resolved.Filters["database"].Selected).To(HaveKey("database/analytics"))

		_, err = GetContextDataFunc(command)(ctx, map[string]string{"database": "analytics"}, []string{"connection-main"})
		Expect(err).ToNot(HaveOccurred())
		Expect(received).To(Equal(actionLookupOptions{Database: "database/analytics", ConnectionID: "connection-main"}))
	})
})
