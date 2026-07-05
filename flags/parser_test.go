package flags

import (
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/flanksource/commons/duration"
)

// Test structs with various embedding patterns

type BaseOptions struct {
	Name  string `flag:"name" help:"Name field" default:"default"`
	Count int    `flag:"count" help:"Count field" default:"5"`
}

type EmbeddedOptions struct {
	BaseOptions
	Active bool `flag:"active" help:"Active flag" default:"true"`
}

type MultiLevelOptions struct {
	EmbeddedOptions
	Extra string `flag:"extra" help:"Extra field"`
}

type MixedOptions struct {
	Direct string `flag:"direct" help:"Direct field"`
	BaseOptions
	Another int `flag:"another" help:"Another field"`
}

type ComplexEmbedding struct {
	CommonFields
	SpecialFields
	Direct string `flag:"direct" help:"Direct field"`
}

type CommonFields struct {
	ID   string `flag:"id" help:"ID field"`
	Name string `flag:"name" help:"Name field"`
}

type SpecialFields struct {
	Tags     []string          `flag:"tags" help:"Tags field"`
	Duration duration.Duration `flag:"duration" help:"Duration field"`
	Since    time.Time         `flag:"since" help:"Time field"`
}

type Args struct {
	Values []string `args:"true" help:"List of values"`
}

type CommaShorthandOptions struct {
	Limit int `flag:"limit,l" help:"Max results"`
}

type NoShorthandOptions struct {
	Limit int `flag:"limit" help:"Max results"`
}

type ExplicitShortWinsOptions struct {
	Limit int `flag:"limit,l" short:"x" help:"Max results"`
}

type MultiCharShorthandOptions struct {
	Limit int `flag:"limit,lim" help:"Max results"`
}

type DisabledFlagOptions struct {
	Limit  int `flag:"-" help:"Disabled"`
	Active int `flag:"active" help:"Active"`
}

type HiddenFlagOptions struct {
	DBURL string `flag:"db-url" hidden:"true" help:"Infra DSN"`
	Name  string `flag:"name" help:"Name"`
}

var _ = Describe("Flags", func() {
	Context("when parsing args struct", func() {
		It("should extract args field correctly", func() {
			fields, err := ParseStructFields(reflect.TypeOf(Args{}))
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveLen(1))

			By("verifying args field")
			Expect(fields[0].IsArgs).To(BeTrue())
		})
	})

	Context("when parsing comma-delimited shorthand", func() {
		It("should split flag tag into long name and shorthand", func() {
			fields, err := ParseStructFields(reflect.TypeOf(CommaShorthandOptions{}))
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveLen(1))
			Expect(fields[0].FlagName).To(Equal("limit"))
			Expect(fields[0].ShortFlag).To(Equal("l"))
		})

		It("should leave shorthand empty when no comma is present", func() {
			fields, err := ParseStructFields(reflect.TypeOf(NoShorthandOptions{}))
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveLen(1))
			Expect(fields[0].FlagName).To(Equal("limit"))
			Expect(fields[0].ShortFlag).To(BeEmpty())
		})

		It("should let an explicit short tag take precedence over the comma alias", func() {
			fields, err := ParseStructFields(reflect.TypeOf(ExplicitShortWinsOptions{}))
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveLen(1))
			Expect(fields[0].FlagName).To(Equal("limit"))
			Expect(fields[0].ShortFlag).To(Equal("x"))
		})

		It("should reject a multi-character shorthand", func() {
			_, err := ParseStructFields(reflect.TypeOf(MultiCharShorthandOptions{}))
			Expect(err).To(MatchError(ContainSubstring("must be a single character")))
		})

		It("should still skip a flag disabled with '-'", func() {
			fields, err := ParseStructFields(reflect.TypeOf(DisabledFlagOptions{}))
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveLen(1))
			Expect(fields[0].FlagName).To(Equal("active"))
		})
	})

	Context("when parsing hidden flags", func() {
		It("should mark a hidden:\"true\" field and leave others visible", func() {
			fields, err := ParseStructFields(reflect.TypeOf(HiddenFlagOptions{}))
			Expect(err).ToNot(HaveOccurred())
			byName := map[string]FieldInfo{}
			for _, f := range fields {
				byName[f.FlagName] = f
			}
			Expect(byName["db-url"].Hidden).To(BeTrue())
			Expect(byName["name"].Hidden).To(BeFalse())
		})

		It("should hide the bound cobra flag so it is dropped from help and schemas", func() {
			cmd := &cobra.Command{Use: "do"}
			fields, err := ParseStructFields(reflect.TypeOf(HiddenFlagOptions{}))
			Expect(err).ToNot(HaveOccurred())
			for _, f := range fields {
				BindFlag(cmd, f)
			}
			dbURL := cmd.Flags().Lookup("db-url")
			Expect(dbURL).ToNot(BeNil())
			Expect(dbURL.Hidden).To(BeTrue())
			Expect(cmd.Flags().Lookup("name").Hidden).To(BeFalse())
		})
	})

	Context("when parsing simple struct", func() {
		It("should extract all direct fields with metadata", func() {
			fields, err := ParseStructFields(reflect.TypeOf(BaseOptions{}))
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveLen(2))

			By("verifying first field")
			Expect(fields[0].FlagName).To(Equal("name"))
			Expect(fields[0].DefaultValue).To(Equal("default"))

			By("verifying second field")
			Expect(fields[1].FlagName).To(Equal("count"))
		})
	})

	Context("when parsing single level embedding", func() {
		It("should extract fields from embedded struct", func() {
			fields, err := ParseStructFields(reflect.TypeOf(EmbeddedOptions{}))
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveLen(3))

			flagNames := make(map[string]bool)
			for _, f := range fields {
				flagNames[f.FlagName] = true
			}

			expectedFlags := []string{"name", "count", "active"}
			for _, expected := range expectedFlags {
				Expect(flagNames).To(HaveKey(expected), "Missing expected flag: %s", expected)
			}
		})

		It("should preserve correct field paths for embedded fields", func() {
			fields, err := ParseStructFields(reflect.TypeOf(EmbeddedOptions{}))
			Expect(err).ToNot(HaveOccurred())

			var nameField *FieldInfo
			for i := range fields {
				if fields[i].FlagName == "name" {
					nameField = &fields[i]
					break
				}
			}

			Expect(nameField).ToNot(BeNil(), "Could not find 'name' field")
			expectedPath := []int{0, 0}
			Expect(nameField.FieldPath).To(Equal(expectedPath))
		})
	})

	Context("when parsing multi-level embedding", func() {
		It("should extract fields from all embedding levels", func() {
			fields, err := ParseStructFields(reflect.TypeOf(MultiLevelOptions{}))
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveLen(4))

			flagNames := make(map[string]bool)
			for _, f := range fields {
				flagNames[f.FlagName] = true
			}

			expectedFlags := []string{"name", "count", "active", "extra"}
			for _, expected := range expectedFlags {
				Expect(flagNames).To(HaveKey(expected), "Missing expected flag: %s", expected)
			}
		})
	})

	Context("when parsing mixed direct and embedded fields", func() {
		It("should extract both direct and embedded fields", func() {
			fields, err := ParseStructFields(reflect.TypeOf(MixedOptions{}))
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveLen(4))

			flagNames := make(map[string]bool)
			for _, f := range fields {
				flagNames[f.FlagName] = true
			}

			expectedFlags := []string{"direct", "name", "count", "another"}
			for _, expected := range expectedFlags {
				Expect(flagNames).To(HaveKey(expected), "Missing expected flag: %s", expected)
			}
		})
	})

	Context("when parsing multiple embedded structs", func() {
		It("should extract fields from all embedded structs", func() {
			fields, err := ParseStructFields(reflect.TypeOf(ComplexEmbedding{}))
			Expect(err).ToNot(HaveOccurred())
			Expect(fields).To(HaveLen(6))

			flagNames := make(map[string]bool)
			for _, f := range fields {
				flagNames[f.FlagName] = true
			}

			expectedFlags := []string{"id", "name", "tags", "duration", "since", "direct"}
			for _, expected := range expectedFlags {
				Expect(flagNames).To(HaveKey(expected), "Missing expected flag: %s", expected)
			}
		})
	})
})

var _ = Describe("GetFieldByPath", func() {
	Context("when accessing direct fields", func() {
		It("should retrieve string field value", func() {
			opts := BaseOptions{Name: "test", Count: 10}
			v := reflect.ValueOf(&opts).Elem()

			field := GetFieldByPath(v, []int{0})
			Expect(field.String()).To(Equal("test"))
		})

		It("should retrieve integer field value", func() {
			opts := BaseOptions{Name: "test", Count: 10}
			v := reflect.ValueOf(&opts).Elem()

			field := GetFieldByPath(v, []int{1})
			Expect(field.Int()).To(Equal(int64(10)))
		})
	})

	Context("when accessing embedded fields", func() {
		var opts EmbeddedOptions
		var v reflect.Value

		BeforeEach(func() {
			opts = EmbeddedOptions{
				BaseOptions: BaseOptions{Name: "embedded", Count: 20},
				Active:      true,
			}
			v = reflect.ValueOf(&opts).Elem()
		})

		It("should retrieve embedded string field", func() {
			field := GetFieldByPath(v, []int{0, 0})
			Expect(field.String()).To(Equal("embedded"))
		})

		It("should retrieve embedded integer field", func() {
			field := GetFieldByPath(v, []int{0, 1})
			Expect(field.Int()).To(Equal(int64(20)))
		})

		It("should retrieve direct boolean field", func() {
			field := GetFieldByPath(v, []int{1})
			Expect(field.Bool()).To(BeTrue())
		})
	})

	Context("when accessing multi-level embedded fields", func() {
		var opts MultiLevelOptions
		var v reflect.Value

		BeforeEach(func() {
			opts = MultiLevelOptions{
				EmbeddedOptions: EmbeddedOptions{
					BaseOptions: BaseOptions{Name: "multilevel", Count: 30},
					Active:      false,
				},
				Extra: "extra-value",
			}
			v = reflect.ValueOf(&opts).Elem()
		})

		It("should retrieve deeply nested string field", func() {
			field := GetFieldByPath(v, []int{0, 0, 0})
			Expect(field.String()).To(Equal("multilevel"))
		})

		It("should retrieve intermediate level boolean field", func() {
			field := GetFieldByPath(v, []int{0, 1})
			Expect(field.Bool()).To(BeFalse())
		})

		It("should retrieve top level string field", func() {
			field := GetFieldByPath(v, []int{1})
			Expect(field.String()).To(Equal("extra-value"))
		})
	})
})
