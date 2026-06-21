package entity

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/api"
)

var _ = Describe("Named filter registry", func() {
	BeforeEach(resetFilterRegistry)

	It("registers and retrieves a filter by name", func() {
		f := NamedFilter{Name: "owners", Source: StaticOptions(nil)}
		RegisterFilter(f)

		got, ok := GetFilter("owners")
		Expect(ok).To(BeTrue())
		Expect(got.Name).To(Equal("owners"))
	})

	It("panics on a duplicate name", func() {
		RegisterFilter(NamedFilter{Name: "dup", Source: StaticOptions(nil)})
		Expect(func() {
			RegisterFilter(NamedFilter{Name: "dup", Source: StaticOptions(nil)})
		}).To(Panic())
	})

	It("panics on a missing source", func() {
		Expect(func() { RegisterFilter(NamedFilter{Name: "nosrc"}) }).To(Panic())
	})

	It("panics from MustGetFilter when unregistered", func() {
		Expect(func() { MustGetFilter("ghost") }).To(Panic())
	})
})

var _ = Describe("StaticOptions source", func() {
	options := map[string]api.Textable{
		"team/platform": api.Text{Content: "Platform"},
		"team/core":     api.Text{Content: "Core"},
	}

	It("returns all options with the true total for an empty query", func() {
		opts, total, err := StaticOptions(options).Options(FilterContext{}, "", 0)
		Expect(err).ToNot(HaveOccurred())
		Expect(total).To(Equal(2))
		Expect(opts).To(HaveLen(2))
	})

	It("narrows by case-insensitive substring over key and label", func() {
		opts, _, err := StaticOptions(options).Options(FilterContext{}, "plat", 0)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts).To(HaveKey("team/platform"))
		Expect(opts).ToNot(HaveKey("team/core"))
	})

	It("caps the head set at the limit", func() {
		opts, total, err := StaticOptions(options).Options(FilterContext{}, "", 1)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts).To(HaveLen(1))
		Expect(total).To(Equal(2), "total reflects the full set, not the capped page")
	})

	It("labels selected values and echoes unknown ones", func() {
		sel, err := StaticOptions(options).Resolve(FilterContext{}, []string{"team/core", "team/ghost"})
		Expect(err).ToNot(HaveOccurred())
		Expect(sel["team/core"].(api.Text).Content).To(Equal("Core"))
		Expect(sel["team/ghost"].(api.Text).Content).To(Equal("team/ghost"))
	})
})

var _ = Describe("FuncOptions source", func() {
	source := FuncOptions(func(_ FilterContext, query string, _ int) (map[string]api.Textable, int, error) {
		all := map[string]api.Textable{"a": api.Text{Content: "Apple"}, "b": api.Text{Content: "Banana"}}
		if query == "ban" {
			return map[string]api.Textable{"b": all["b"]}, 1, nil
		}
		return all, len(all), nil
	})

	It("delegates option resolution to the function", func() {
		opts, total, err := source.Options(FilterContext{}, "ban", 5)
		Expect(err).ToNot(HaveOccurred())
		Expect(total).To(Equal(1))
		Expect(opts).To(HaveKey("b"))
	})

	It("resolves selected labels via the head set", func() {
		sel, err := source.Resolve(FilterContext{}, []string{"a"})
		Expect(err).ToNot(HaveOccurred())
		Expect(sel["a"].(api.Text).Content).To(Equal("Apple"))
	})
})

var _ = Describe("Declarative filter specs", func() {
	BeforeEach(resetFilterRegistry)

	It("builds and registers a static filter from a spec", func() {
		RegisterFilterSpec(FilterSpec{
			Name:  "fruit",
			Label: "Fruit",
			Source: FilterSourceSpec{
				Kind:    SourceStatic,
				Options: map[string]string{"a": "Apple"},
			},
		})

		f := MustGetFilter("fruit")
		opts, _, err := f.Source.Options(FilterContext{}, "", 0)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts).To(HaveKey("a"))
	})

	It("round-trips a Go filter to its declarative spec", func() {
		f := NamedFilter{
			Name:   "fruit",
			Label:  "Fruit",
			Source: StaticOptions(map[string]api.Textable{"a": api.Text{Content: "Apple"}}),
		}
		spec := f.Spec()
		Expect(spec.Name).To(Equal("fruit"))
		Expect(spec.Type).To(Equal("select"), "single-select is the default control type")
		Expect(spec.Source.Kind).To(Equal(SourceStatic))
		Expect(spec.Source.Options).To(HaveKeyWithValue("a", "Apple"))
	})

	It("rejects a spec with an unconstructible source kind", func() {
		_, err := FilterFromSpec(FilterSpec{Name: "x", Source: FilterSourceSpec{Kind: SourceFunc}})
		Expect(err).To(HaveOccurred())
	})

	It("rejects an entity source spec without an entity name", func() {
		_, err := FilterFromSpec(FilterSpec{Name: "x", Source: FilterSourceSpec{Kind: SourceEntity}})
		Expect(err).To(HaveOccurred())
	})
})
