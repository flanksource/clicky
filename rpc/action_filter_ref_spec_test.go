package rpc

import (
	"context"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/entity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

type actionRefOptions struct {
	Owner string `flag:"owner"`
}

func (actionRefOptions) ClickyActionFlags() {}

var _ = Describe("action named filter references", func() {
	It("executes native repeatable arguments with the same encoding as JSON arrays", func() {
		executor := NewCommandExecutor(&RPCService{Operations: []RPCOperation{{
			Name: "records", Parameters: []RPCParameter{{Name: "activity", Type: "array"}},
			ContextDataFunc: func(_ context.Context, flags map[string]string, _ []string) (any, error) {
				return flags["activity"], nil
			},
		}}}, &ExecutorConfig{Enabled: true})
		for _, activities := range []any{[]string{"first", "second"}, []any{"first", "second"}} {
			value, _, err := executor.ExecuteOperation(context.Background(), "records", map[string]any{"activity": activities})
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(Equal("first,second"))
		}
	})
	It("uses the action's filter and complete surface identity in OpenAPI", func() {
		for _, parent := range []string{"ref-primary", "ref-secondary"} {
			filter := parent + "-owners"
			entity.RegisterFilter(entity.NamedFilter{Name: filter, Source: entity.StaticOptions(map[string]api.Textable{"owner": api.Text{Content: parent}})})
			clicky.RegisterEntity(clicky.Entity[refUser, struct{}, any]{
				Name: "ref-subject", Parent: parent,
				Actions: []clicky.EntityAction{entity.WithActionFilters(
					entity.ActionWithFlagsAndContext("inspect", actionRefOptions{}, func(context.Context, string, map[string]string) (any, error) { return nil, nil }).WithMethod("GET"),
					[]entity.Filter[actionRefOptions]{entity.Use[actionRefOptions](filter).As("owner")},
				)},
			})
		}
		root := &cobra.Command{Use: "testapp"}
		clicky.GenerateCLI(root)
		spec, err := NewOpenAPIGenerator(nil).GenerateFromCobra(root)
		Expect(err).NotTo(HaveOccurred())
		found := map[string]string{}
		for path, methods := range spec.Paths {
			for _, operation := range methods {
				if operation.Clicky == nil || operation.Clicky.ActionName != "inspect" {
					continue
				}
				for _, parameter := range operation.Parameters {
					if parameter.Name != "owner" {
						continue
					}
					Expect(parameter.Clicky).NotTo(BeNil())
					Expect(parameter.Clicky.Role).To(Equal("filter"))
					Expect(parameter.Lookup).NotTo(BeNil())
					Expect(parameter.Lookup.URL).To(Equal(path))
					found[operation.Clicky.Surface] = parameter.Lookup.Ref
				}
			}
		}
		Expect(found).To(HaveLen(2))
		Expect(found).To(ContainElement("#/components/x-clicky-filters/ref-primary-owners"))
		Expect(found).To(ContainElement("#/components/x-clicky-filters/ref-secondary-owners"))
	})
})
