package flags

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type categorizedModelOptions struct {
	Model string `flag:"model" category:"Model selection"`
}

type categorizedRuntimeOptions struct {
	*categorizedModelOptions
	Access string `flag:"access" category:"Model permissions"`
}

var _ = Describe("flag categories", func() {
	It("preserves other flag annotations when assigning a category", func() {
		command := &cobra.Command{Use: "demo"}
		command.Flags().String("access", "", "Access contract")
		flag := command.Flags().Lookup("access")
		SetAllowedValues(flag, "read-only", "full")
		SetCategory(flag, CategoryModelPermissions)
		Expect(Category(flag)).To(Equal(CategoryModelPermissions))
		Expect(AllowedValues(flag)).To(Equal([]string{"read-only", "full"}))
	})

	DescribeTable("rejects invalid category assignments", func(flag *pflag.Flag, category string) {
		Expect(func() { SetCategory(flag, category) }).To(Panic())
	},
		Entry("unregistered flag", (*pflag.Flag)(nil), CategoryModelSelection),
		Entry("empty category", &pflag.Flag{Name: "model"}, ""),
	)

	It("preserves categories through nested embedded options and binding", func() {
		fields, err := ParseStructFields(reflect.TypeFor[struct {
			categorizedRuntimeOptions
			File string `flag:"file"`
		}]())
		Expect(err).NotTo(HaveOccurred())
		command := &cobra.Command{Use: "demo"}
		for _, field := range fields {
			BindFlag(command, field)
		}
		Expect(command.Flags().Lookup("model").Annotations["clicky_category"]).To(Equal([]string{"Model selection"}))
		Expect(command.Flags().Lookup("access").Annotations["clicky_category"]).To(Equal([]string{"Model permissions"}))
		Expect(command.Flags().Lookup("file").Annotations["clicky_category"]).To(BeEmpty())
	})
})
