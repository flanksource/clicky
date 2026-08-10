package entity

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

type primaryActionItem struct {
	ID string
}

func (i primaryActionItem) GetID() string   { return i.ID }
func (i primaryActionItem) GetName() string { return i.ID }

type primaryActionOptions struct {
	Profile string   `flag:"profile" default:"safe"`
	Hosts   []string `flag:"host"`
}

func (primaryActionOptions) ClickyActionFlags() {}

var _ = Describe("Entity primary actions", func() {
	It("runs the typed action at the entity root and retains list as a child", func() {
		name := "primary-action-cli-spec"
		var received primaryActionOptions
		NewEntity[primaryActionItem, struct{}, primaryActionItem](name).
			List(func(struct{}) ([]primaryActionItem, error) {
				return []primaryActionItem{{ID: "history"}}, nil
			}).
			WithPrimaryAction(PrimaryActionWithContext(primaryActionOptions{}, func(_ context.Context, opts primaryActionOptions) (primaryActionItem, error) {
				received = opts
				return primaryActionItem{ID: "started"}, nil
			}).WithShort("Start a run")).
			Register()

		root := &cobra.Command{Use: "test"}
		GenerateCLI(root)
		root.SetArgs([]string{name, "--profile", "full", "--host", "one.example.test", "--host", "two.example.test"})

		Expect(root.Execute()).To(Succeed())
		Expect(received).To(Equal(primaryActionOptions{
			Profile: "full",
			Hosts:   []string{"one.example.test", "two.example.test"},
		}))

		command, _, err := root.Find([]string{name})
		Expect(err).NotTo(HaveOccurred())
		Expect(command.Short).To(Equal("Start a run"))
		Expect(command.CommandPath()).To(Equal("test " + name))
		Expect(command.Commands()).To(ContainElement(WithTransform(func(cmd *cobra.Command) string {
			return cmd.Name()
		}, Equal("list"))))
	})
})
