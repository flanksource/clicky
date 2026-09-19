package aichat_test

import (
	"context"
	"fmt"

	"github.com/flanksource/clicky"
	clickyaichat "github.com/flanksource/clicky/aichat"
	"github.com/spf13/cobra"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type toolTodo struct{ ID string }

func (t toolTodo) GetID() string   { return t.ID }
func (t toolTodo) GetName() string { return t.ID }

type toolTodoListOptions struct {
	Search string `flag:"search"`
	Status string `flag:"status"`
}

var _ = Describe("ID-only entity tools", func() {
	It("requires explicit bulk IDs and excludes listing selectors", func(ctx SpecContext) {
		var received []string
		clicky.NewEntity[toolTodo, toolTodoListOptions, toolTodo]("tool-todo-ids-spec").
			List(func(toolTodoListOptions) ([]toolTodo, error) { return nil, nil }).
			WithBulkAction(clicky.BulkActionWithContext("run", func(_ context.Context, ids []string, _ map[string]string) (any, error) {
				received = ids
				return ids, nil
			})).Register()
		root := &cobra.Command{Use: "example"}
		clicky.GenerateCLI(root)
		command, _, err := root.Find([]string{"tool-todo-ids-spec", "run"})
		Expect(err).NotTo(HaveOccurred())
		Expect(command.Flags().Lookup("search")).To(BeNil())
		Expect(command.Flags().Lookup("status")).To(BeNil())
		Expect(command.RunE(command, nil)).To(MatchError(ContainSubstring("requires one or more ids")))
		provider, err := clickyaichat.NewCobraToolProvider(clickyaichat.CobraToolProviderOptions{Root: root})
		Expect(err).NotTo(HaveOccurred())
		set, err := provider.ToolSet(ctx)
		Expect(err).NotTo(HaveOccurred())
		var definitionIndex = -1
		var names []string
		for i, definition := range set.Definitions {
			names = append(names, definition.Name)
			if definition.Name == "tool-todo-ids-spec_run" {
				definitionIndex = i
				break
			}
		}
		Expect(definitionIndex).To(BeNumerically(">=", 0), fmt.Sprint(names))
		definition := set.Definitions[definitionIndex]
		properties := definition.InputSchema["properties"].(map[string]any)
		Expect(properties).To(HaveKey("ids"))
		Expect(properties).NotTo(HaveKey("search"))
		Expect(properties).NotTo(HaveKey("status"))
		Expect(definition.InputSchema["required"]).To(ContainElement("ids"))
		Expect(properties["ids"].(map[string]any)["minItems"]).To(Equal(1))

		_, err = definition.Handler(ctx, map[string]any{"search": "pending"})
		Expect(err).To(HaveOccurred())
		_, err = definition.Handler(ctx, map[string]any{"ids": []any{}})
		Expect(err).To(HaveOccurred())
		_, err = definition.Handler(ctx, map[string]any{"ids": []any{"todo-1", "todo-2"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(received).To(Equal([]string{"todo-1", "todo-2"}))
	})
})
