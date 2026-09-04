package entity

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

type actionErrorEntity struct {
	ID string
}

func (e actionErrorEntity) GetID() string   { return e.ID }
func (e actionErrorEntity) GetName() string { return e.ID }

var _ = Describe("entity action errors", func() {
	It("renders a structured error before returning it", func() {
		name := "entity-action-rich-error-spec"
		RegisterEntity(Entity[actionErrorEntity, struct{}, any]{
			Name: name,
			List: func(struct{}) ([]actionErrorEntity, error) {
				return nil, nil
			},
			Actions: []EntityAction{ActionWithContext("lint", func(context.Context, string, map[string]string) (any, error) {
				return nil, &prettyErr{msg: "lint failed"}
			})},
		})

		var rendered any
		originalRender := RenderResult
		RenderResult = func(value any) error {
			rendered = value
			return nil
		}
		DeferCleanup(func() { RenderResult = originalRender })

		root := &cobra.Command{Use: "test"}
		GenerateCLI(root)
		root.SetArgs([]string{name, "lint", "entity-1"})

		err := root.Execute()

		Expect(err).To(MatchError("lint failed"))
		Expect(rendered).To(BeAssignableToTypeOf(&prettyErr{}))
	})
})
