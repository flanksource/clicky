package entity

import (
	"context"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

type purgeFlags struct {
	DryRun      bool   `flag:"dry-run" help:"Preview the delete without applying it"`
	Fingerprint string `flag:"fingerprint" help:"Fingerprint returned by the preview"`
}

func (purgeFlags) ClickyActionFlags() {}

type purgeResult struct {
	ID    string
	Flags map[string]string
	Actor any
}

type purgeActorKey struct{}

var _ = Describe("entity delete with flags", func() {
	const (
		id          = "client-1"
		fingerprint = "abc123"
		actor       = "request-scoped"
	)

	var rendered []any

	BeforeEach(func() {
		rendered = nil
		originalRender := RenderResult
		RenderResult = func(value any) error {
			rendered = append(rendered, value)
			return nil
		}
		DeferCleanup(func() { RenderResult = originalRender })
	})

	generate := func(name string) (*cobra.Command, *cobra.Command) {
		root := &cobra.Command{Use: "test"}
		GenerateCLI(root)
		deleteCmd, _, err := root.Find([]string{name, "delete"})
		Expect(err).NotTo(HaveOccurred())
		Expect(deleteCmd.Name()).To(Equal("delete"))
		return root, deleteCmd
	}

	purge := func(ctx context.Context, id string, flags map[string]string) (any, error) {
		return purgeResult{ID: id, Flags: flags, Actor: ctx.Value(purgeActorKey{})}, nil
	}

	It("sets the flag schema and the handler together on the builder", func() {
		builder := NewEntity[samplePlainEntity, struct{}, samplePlainEntity]("delete-flags-builder-spec").
			DeleteWithFlagsAndContext(purgeFlags{}, purge)

		Expect(builder.entity.DeleteFlags).To(Equal(purgeFlags{}))
		Expect(builder.entity.DeleteWithFlagsAndContext).NotTo(BeNil())
		Expect(builder.entity.DeleteWithContext).To(BeNil())
		Expect(builder.entity.Delete).To(BeNil())
	})

	It("registers DeleteFlags on the generated delete subcommand", func() {
		name := "delete-flags-registered-spec"
		RegisterEntity(Entity[samplePlainEntity, struct{}, samplePlainEntity]{
			Name:                      name,
			DeleteFlags:               purgeFlags{},
			DeleteWithFlagsAndContext: purge,
		})

		_, deleteCmd := generate(name)

		Expect(deleteCmd.Use).To(Equal("delete <id> [flags]"))
		Expect(deleteCmd.Flags().Lookup("dry-run").Value.Type()).To(Equal("bool"))
		Expect(deleteCmd.Flags().Lookup("fingerprint").Value.Type()).To(Equal("string"))
	})

	It("passes the id, flags and context to the handler and renders what it returns", func() {
		name := "delete-flags-handler-spec"
		RegisterEntity(Entity[samplePlainEntity, struct{}, samplePlainEntity]{
			Name:                      name,
			DeleteFlags:               purgeFlags{},
			DeleteWithFlagsAndContext: purge,
		})

		root, deleteCmd := generate(name)
		root.SetArgs([]string{name, "delete", id, "--dry-run", "--fingerprint", fingerprint})

		Expect(root.ExecuteContext(context.WithValue(context.Background(), purgeActorKey{}, actor))).To(Succeed())

		Expect(rendered).To(Equal([]any{purgeResult{
			ID:    id,
			Flags: map[string]string{"dry-run": "true", "fingerprint": fingerprint},
			Actor: actor,
		}}))
		Expect(GetCommandResponseMeta(deleteCmd)).To(Equal(&ResponseOpenAPIMeta{Type: reflect.TypeFor[any]()}))
	})

	It("prefers DeleteWithFlagsAndContext over DeleteWithContext and Delete", func() {
		name := "delete-flags-precedence-spec"
		var legacyCalls []string
		RegisterEntity(Entity[samplePlainEntity, struct{}, samplePlainEntity]{
			Name:                      name,
			DeleteFlags:               purgeFlags{},
			DeleteWithFlagsAndContext: purge,
			DeleteWithContext: func(_ context.Context, id string) error {
				legacyCalls = append(legacyCalls, "context:"+id)
				return nil
			},
			Delete: func(id string) error {
				legacyCalls = append(legacyCalls, "plain:"+id)
				return nil
			},
		})

		root, _ := generate(name)
		root.SetArgs([]string{name, "delete", id})

		Expect(root.Execute()).To(Succeed())

		Expect(legacyCalls).To(BeEmpty())
		Expect(rendered).To(Equal([]any{purgeResult{ID: id, Flags: map[string]string{}}}))
	})

	It("keeps DeleteWithContext flagless and silent", func() {
		name := "delete-flags-legacy-spec"
		var deleted []string
		RegisterEntity(Entity[samplePlainEntity, struct{}, samplePlainEntity]{
			Name: name,
			DeleteWithContext: func(ctx context.Context, id string) error {
				deleted = append(deleted, id+"@"+ctx.Value(purgeActorKey{}).(string))
				return nil
			},
		})

		root, deleteCmd := generate(name)
		root.SetArgs([]string{name, "delete", id})

		Expect(root.ExecuteContext(context.WithValue(context.Background(), purgeActorKey{}, actor))).To(Succeed())

		Expect(deleted).To(Equal([]string{id + "@" + actor}))
		Expect(rendered).To(BeEmpty())
		Expect(deleteCmd.Use).To(Equal("delete <id>"))
		Expect(deleteCmd.Flags().Lookup("dry-run")).To(BeNil())
		Expect(GetCommandResponseMeta(deleteCmd)).To(BeNil())
	})
})
