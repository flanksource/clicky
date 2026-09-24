package flags

import (
	"reflect"

	"github.com/flanksource/commons/duration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

type budgetOptions struct {
	Budget float64 `flag:"budget" help:"Maximum budget"`
}

var _ = Describe("float64 option binding", func() {
	It("carries the CLI flag and RPC request into the same typed field", func() {
		fields, err := ParseStructFields(reflect.TypeFor[budgetOptions]())
		Expect(err).NotTo(HaveOccurred())
		Expect(fields).To(HaveLen(1))

		command := &cobra.Command{Use: "budget"}
		bound := BindFlag(command, fields[0])
		Expect(command.Flags().Set("budget", "2.75")).To(Succeed())
		var fromCLI budgetOptions
		Expect(AssignFieldValue(reflect.ValueOf(&fromCLI).Elem(), bound, nil, false)).To(Succeed())
		Expect(fromCLI.Budget).To(Equal(2.75))

		var fromRequest budgetOptions
		Expect(PopulateFromRequest(reflect.ValueOf(&fromRequest).Elem(), fields, map[string]string{"budget": "2.75"}, nil)).To(Succeed())
		Expect(fromRequest).To(Equal(fromCLI))
		Expect(PopulateFromRequest(reflect.ValueOf(&fromRequest).Elem(), fields, map[string]string{"budget": "bad"}, nil)).To(MatchError(ContainSubstring("parsing float")))
	})

	DescribeTable("rejects a default tag its field type cannot parse",
		func(optionsType reflect.Type, field string) {
			_, err := ParseStructFields(optionsType)
			Expect(err).To(MatchError(And(ContainSubstring(field), ContainSubstring("invalid default"))))
		},
		Entry("float64", reflect.TypeFor[struct {
			Budget float64 `flag:"budget" default:"cheap"`
		}](), "Budget"),
		Entry("int", reflect.TypeFor[struct {
			Limit int `flag:"limit" default:"10x"`
		}](), "Limit"),
		Entry("bool", reflect.TypeFor[struct {
			Verbose bool `flag:"verbose" default:"yes please"`
		}](), "Verbose"),
		Entry("duration", reflect.TypeFor[struct {
			Timeout duration.Duration `flag:"timeout" default:"soon"`
		}](), "Timeout"),
	)

	It("accepts default tags that parse as their field type", func() {
		fields, err := ParseStructFields(reflect.TypeFor[struct {
			Budget  float64           `flag:"budget" default:"2.5"`
			Limit   int               `flag:"limit" default:"10"`
			Verbose bool              `flag:"verbose" default:"true"`
			Timeout duration.Duration `flag:"timeout" default:"5m"`
		}]())
		Expect(err).NotTo(HaveOccurred())
		Expect(fields).To(HaveLen(4))
	})
})
