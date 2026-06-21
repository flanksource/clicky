package entity

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/flanksource/clicky/api"
)

type attachTaskOpts struct {
	Owner  string   `flag:"owner"`
	Status string   `flag:"status"`
	Tags   []string `flag:"tags"`
}

// filterTestUser is a source entity item for the EntityOptions integration test.
type filterTestUser struct {
	ID   string
	Name string
}

func (u filterTestUser) GetID() string   { return u.ID }
func (u filterTestUser) GetName() string { return u.Name }

type filterTestUserOpts struct{}

var _ = Describe("structToFlagMap", func() {
	It("renders non-zero flag-tagged fields, joining slices", func() {
		m := structToFlagMap(attachTaskOpts{Owner: "alice", Tags: []string{"a", "b"}})
		Expect(m).To(HaveKeyWithValue("owner", "alice"))
		Expect(m).To(HaveKeyWithValue("tags", "a,b"))
		Expect(m).ToNot(HaveKey("status"), "zero values are omitted")
	})
})

var _ = Describe("fieldValue", func() {
	It("returns a scalar field's value", func() {
		Expect(fieldValue(attachTaskOpts{Owner: "alice"}, "owner")).To(Equal([]string{"alice"}))
	})

	It("returns slice field elements", func() {
		Expect(fieldValue(attachTaskOpts{Tags: []string{"x", "y"}}, "tags")).To(Equal([]string{"x", "y"}))
	})

	It("returns nil for an unset field", func() {
		Expect(fieldValue(attachTaskOpts{}, "owner")).To(BeNil())
	})
})

var _ = Describe("Use/As typed adapter", func() {
	BeforeEach(func() {
		resetFilterRegistry()
		RegisterFilter(NamedFilter{
			Name:   "users",
			Source: StaticOptions(map[string]api.Textable{"u1": api.Text{Content: "Alice"}, "u2": api.Text{Content: "Bob"}}),
		})
	})

	It("binds to the filter name by default and to As() when given", func() {
		Expect(Use[attachTaskOpts]("users").Key()).To(Equal("users"))
		Expect(Use[attachTaskOpts]("users").As("owner").Key()).To(Equal("owner"))
	})

	It("defaults the label to the filter name", func() {
		Expect(Use[attachTaskOpts]("users").Label()).To(Equal("users"))
	})

	It("resolves options and supports server-side search", func() {
		af := Use[attachTaskOpts]("users").As("owner")
		Expect(af.Options(attachTaskOpts{})).To(HaveLen(2))

		opts, total := af.OptionsWithQuery(attachTaskOpts{}, "ali", 0)
		Expect(opts).To(HaveKey("u1"))
		Expect(opts).ToNot(HaveKey("u2"))
		Expect(total).To(Equal(1))
	})

	It("labels the selected value from the bound field", func() {
		sel, err := Use[attachTaskOpts]("users").As("owner").Lookup(&attachTaskOpts{Owner: "u1"})
		Expect(err).ToNot(HaveOccurred())
		Expect(sel).To(HaveKey("u1"))
		Expect(sel["u1"].(api.Text).Content).To(Equal("Alice"))
	})

	It("leaves LookupType empty so field inference stands for a plain select", func() {
		Expect(Use[attachTaskOpts]("users").As("owner").LookupType()).To(BeEmpty())
	})

	It("panics when the referenced filter is not registered", func() {
		Expect(func() { Use[attachTaskOpts]("missing") }).To(Panic())
	})
})

var _ = Describe("EntityOptions sourcing from another entity", func() {
	BeforeEach(resetFilterRegistry)

	It("resolves a task entity's owner options from the users entity lookup", func() {
		RegisterEntity(Entity[filterTestUser, filterTestUserOpts, filterTestUser]{
			Name: "attach-users",
			List: func(filterTestUserOpts) ([]filterTestUser, error) {
				return []filterTestUser{{ID: "u1", Name: "Alice"}, {ID: "u2", Name: "Bob"}}, nil
			},
		})

		RegisterFilter(NamedFilter{Name: "attach-users-src", Source: EntityOptions("attach-users")})

		RegisterEntity(Entity[filterTestUser, attachTaskOpts, filterTestUser]{
			Name:    "attach-tasks",
			Filters: []Filter[attachTaskOpts]{Use[attachTaskOpts]("attach-users-src").As("owner")},
			List:    func(attachTaskOpts) ([]filterTestUser, error) { return nil, nil },
		})

		root := &cobra.Command{Use: "root"}
		GenerateCLI(root)

		listCmd, _, err := root.Find([]string{"attach-tasks", "list"})
		Expect(err).ToNot(HaveOccurred())
		Expect(listCmd).ToNot(BeNil())

		lookupFunc := GetLookupFunc(listCmd)
		Expect(lookupFunc).ToNot(BeNil())

		resp, err := lookupFunc(map[string]string{}, nil)
		Expect(err).ToNot(HaveOccurred())

		data, err := json.Marshal(resp)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("Alice"))
		Expect(string(data)).To(ContainSubstring("u1"))
		Expect(string(data)).To(ContainSubstring("Bob"))
	})
})
