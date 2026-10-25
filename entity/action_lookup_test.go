package entity

import (
	"context"
	"errors"
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

// fallibleDatabaseFilter answers from LookupOptions, failing with *err when
// it is set; its plain Options must never be consulted.
type fallibleDatabaseFilter struct{ err *error }

func (fallibleDatabaseFilter) Key() string   { return "database" }
func (fallibleDatabaseFilter) Label() string { return "Database" }
func (fallibleDatabaseFilter) Lookup(*actionLookupOptions) (map[string]api.Textable, error) {
	return nil, nil
}
func (fallibleDatabaseFilter) Options(actionLookupOptions) map[string]api.Textable {
	panic("a FallibleFilter is asked through LookupOptions")
}
func (f fallibleDatabaseFilter) LookupOptions(context.Context, actionLookupOptions) (map[string]api.Textable, error) {
	if *f.err != nil {
		return nil, *f.err
	}
	return map[string]api.Textable{"analytics": api.Text{}, "billing": api.Text{}}, nil
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
		resolved := response.(LookupResponse)
		Expect(resolved.Filters["database"].Options).To(HaveKey("connection-main/analytics"))
		Expect(resolved.Filters["database"].Selected).To(HaveKey("database/analytics"))

		_, err = GetContextDataFunc(command)(ctx, map[string]string{"database": "analytics"}, []string{"connection-main"})
		Expect(err).ToNot(HaveOccurred())
		Expect(received).To(Equal(actionLookupOptions{Database: "database/analytics", ConnectionID: "connection-main"}))
	})

	It("binds a flags struct's filters to a map-based action with WithActionFilters", func() {
		RegisterEntity(Entity[entityFilterTestEntity, struct{}, any]{
			Name: "map-action",
			Actions: []EntityAction{
				WithActionFilters(ActionWithFlagsAndContext("inspect", actionLookupOptions{}, func(context.Context, string, map[string]string) (any, error) {
					return nil, nil
				}), actionLookupOptions{}.Filters()),
			},
		})
		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)
		command, _, err := root.Find([]string{"map-action", "inspect"})
		Expect(err).ToNot(HaveOccurred())
		completion, exists := command.GetFlagCompletionFunc("database")
		Expect(exists).To(BeTrue())
		completions, _ := completion(command, []string{"connection-main"}, "")
		Expect(completions).To(ConsistOf("connection-main/analytics"))
		Expect(GetCommandOpenAPIMeta(command).SupportsLookup).To(BeTrue())
	})

	It("completes a FallibleFilter's options, and fails completion with its error", func() {
		var failure error
		RegisterEntity(Entity[entityFilterTestEntity, struct{}, any]{
			Name: "fallible",
			Actions: []EntityAction{
				WithActionFilters(ActionWithFlagsAndContext("inspect", actionLookupOptions{}, func(context.Context, string, map[string]string) (any, error) {
					return nil, nil
				}), []Filter[actionLookupOptions]{fallibleDatabaseFilter{err: &failure}}),
			},
		})
		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)
		command, _, err := root.Find([]string{"fallible", "inspect"})
		Expect(err).ToNot(HaveOccurred())
		completion, exists := command.GetFlagCompletionFunc("database")
		Expect(exists).To(BeTrue())

		completions, directive := completion(command, nil, "an")
		Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
		Expect(completions).To(Equal([]string{"analytics"}), "options come from LookupOptions, prefix-matched")

		failure = errors.New("catalog unreachable")
		_, directive = completion(command, nil, "")
		Expect(directive).To(Equal(cobra.ShellCompDirectiveError))
	})

	It("completes an action's id from the entity's own list when it declares no ValidArgs", func() {
		RegisterEntity(Entity[entityFilterTestEntity, struct{}, any]{
			Name: "listed",
			List: func(struct{}) ([]entityFilterTestEntity, error) {
				return []entityFilterTestEntity{{ID: "p-review", Name: "review"}, {ID: "p-summarize", Name: "summarize"}}, nil
			},
			Actions: []EntityAction{
				ActionWithContext("run", func(context.Context, string, map[string]string) (any, error) { return nil, nil }).WithOptionalID(),
			},
		})
		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)
		command, _, err := root.Find([]string{"listed", "run"})
		Expect(err).ToNot(HaveOccurred())
		Expect(command.ValidArgsFunction).ToNot(BeNil())

		all, directive := command.ValidArgsFunction(command, nil, "")
		Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
		Expect(all).To(Equal([]string{"p-review\treview", "p-summarize\tsummarize"}))
		byName, _ := command.ValidArgsFunction(command, nil, "sum")
		Expect(byName).To(Equal([]string{"p-summarize\tsummarize"}), "a name prefix matches too")
	})
})
