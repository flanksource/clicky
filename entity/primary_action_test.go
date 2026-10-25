package entity

import (
	"context"
	"fmt"

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

type actionPayload struct {
	ConnectionID string `json:"connectionId"`
	Provider     string `json:"provider"`
}

// A failing typed handler must contribute no data. Boxing its zero value into
// the `any` return makes it non-nil, and the RPC executor then serializes that
// zero struct as the response body instead of the error.
var _ = Describe("typed handler results", func() {
	failure := fmt.Errorf("refresh xero token: tenant name %q matches 2 entities", "Acme")

	It("drops the zero value when a context action fails", func() {
		info := ActionWithContext("refresh", func(context.Context, string, map[string]string) (actionPayload, error) {
			return actionPayload{}, failure
		}).actionInfo()

		data, err := info.ContextDataFunc(context.Background(), map[string]string{"id": "conn-1"}, nil)

		Expect(err).To(MatchError(failure))
		Expect(data).To(BeNil())
	})

	It("returns the value when a context action succeeds", func() {
		want := actionPayload{ConnectionID: "conn-1", Provider: "xero"}
		info := ActionWithContext("refresh", func(context.Context, string, map[string]string) (actionPayload, error) {
			return want, nil
		}).actionInfo()

		data, err := info.ContextDataFunc(context.Background(), map[string]string{"id": "conn-1"}, nil)

		Expect(err).ToNot(HaveOccurred())
		Expect(data).To(Equal(want))
	})

	It("drops a typed nil pointer when a context action fails", func() {
		info := ActionWithContext("refresh", func(context.Context, string, map[string]string) (*actionPayload, error) {
			return nil, failure
		}).actionInfo()

		data, err := info.ContextDataFunc(context.Background(), map[string]string{"id": "conn-1"}, nil)

		Expect(err).To(MatchError(failure))
		Expect(data).To(BeNil())
	})

	// A handler that fails after producing a partial result (a run that failed
	// with a trace of where it failed) returns both; the result is the
	// diagnostic, so it must reach the caller alongside the error.
	It("keeps a non-zero partial result when a context action fails", func() {
		partial := &actionPayload{ConnectionID: "conn-1", Provider: "xero"}
		info := ActionWithContext("refresh", func(context.Context, string, map[string]string) (any, error) {
			return partial, failure
		}).actionInfo()

		data, err := info.ContextDataFunc(context.Background(), map[string]string{"id": "conn-1"}, nil)

		Expect(err).To(MatchError(failure))
		Expect(data).To(Equal(partial))
	})

	It("drops the zero value when a flag action fails", func() {
		info := Action("refresh", func(string, map[string]string) (actionPayload, error) {
			return actionPayload{}, failure
		}).actionInfo()

		data, err := info.DataFunc(map[string]string{"id": "conn-1"}, nil)

		Expect(err).To(MatchError(failure))
		Expect(data).To(BeNil())
	})
})
