package flags

import (
	"bytes"
	"encoding/csv"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("Literal string array flags", func() {
	It("preserves quotes and commas in repeated literal values", func() {
		type options struct {
			Selection []string `flag:"selection" split:"false"`
		}
		fields, err := ParseStructFields(reflect.TypeFor[options]())
		Expect(err).NotTo(HaveOccurred())
		cmd := &cobra.Command{Use: "preview"}
		binding := BindFlag(cmd, fields[0])
		values := []string{`column:"dbo"."orders"/"label"`, `index:"tenant,a"."orders"/"IX_label"`}
		Expect(cmd.ParseFlags([]string{"--selection", values[0], "--selection", values[1]})).To(Succeed())
		var result options
		Expect(AssignFieldValue(reflect.ValueOf(&result).Elem(), binding, nil, false)).To(Succeed())
		Expect(result.Selection).To(Equal(values))
		var encoded bytes.Buffer
		writer := csv.NewWriter(&encoded)
		Expect(writer.Write(values)).To(Succeed())
		writer.Flush()
		Expect(writer.Error()).NotTo(HaveOccurred())
		var fromRequest options
		Expect(PopulateFromRequest(reflect.ValueOf(&fromRequest).Elem(), fields, map[string]string{"selection": encoded.String()}, nil)).To(Succeed())
		Expect(fromRequest.Selection).To(Equal(values))
	})

	It("retains CSV splitting for ordinary string slices", func() {
		type options struct {
			Values []string `flag:"value"`
		}
		fields, err := ParseStructFields(reflect.TypeFor[options]())
		Expect(err).NotTo(HaveOccurred())
		cmd := &cobra.Command{Use: "list"}
		binding := BindFlag(cmd, fields[0])
		Expect(cmd.ParseFlags([]string{"--value", "alpha,beta"})).To(Succeed())
		Expect(*binding.StringSlicePtr).To(Equal([]string{"alpha", "beta"}))
	})

	DescribeTable("rejects invalid split metadata", func(typ reflect.Type) {
		_, err := ParseStructFields(typ)
		Expect(err).To(HaveOccurred())
	},
		Entry("invalid boolean", reflect.TypeFor[struct {
			Values []string `flag:"value" split:"literal"`
		}]()),
		Entry("scalar", reflect.TypeFor[struct {
			Value string `flag:"value" split:"false"`
		}]()),
	)
})
