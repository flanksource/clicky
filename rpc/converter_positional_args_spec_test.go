package rpc

import (
	"github.com/flanksource/clicky/entity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

type positionalArgsNoneOpts struct {
	Force bool `flag:"force" help:"Rebuild even when fragmentation is low"`
}

type positionalArgsAcceptedOpts struct {
	Tables []string `args:"true" help:"Tables to rebuild"`
}

var _ = Describe("positional args conversion", func() {
	convert := func(cmd *cobra.Command) *RPCOperation {
		operation, err := NewConverter(DefaultConfig()).ConvertCommand(cmd)
		Expect(err).NotTo(HaveOccurred())
		return operation
	}
	parameterNames := func(operation *RPCOperation) []string {
		names := make([]string, 0, len(operation.Parameters))
		for _, parameter := range operation.Parameters {
			names = append(names, parameter.Name)
		}
		return names
	}

	It("omits args for a generated command whose options declare no positional field", func() {
		root := &cobra.Command{Use: "app"}
		cmd := entity.AddNamedCommand("rebuild-all-indexes", root, positionalArgsNoneOpts{},
			func(positionalArgsNoneOpts) (string, error) { return "", nil })

		operation := convert(cmd)
		Expect(operation.Schema.Properties).NotTo(HaveKey("args"))
		Expect(parameterNames(operation)).To(Equal([]string{"force"}))
	})

	It("keeps args for a generated command whose options declare a positional field", func() {
		root := &cobra.Command{Use: "app"}
		cmd := entity.AddNamedCommand("rebuild", root, positionalArgsAcceptedOpts{},
			func(positionalArgsAcceptedOpts) (string, error) { return "", nil })

		operation := convert(cmd)
		Expect(operation.Schema.Properties).To(HaveKey("args"))
		Expect(parameterNames(operation)).To(Equal([]string{"args"}))
	})

	It("keeps args for a plain command with a custom Args validator", func() {
		cmd := &cobra.Command{
			Use:  "copy",
			Args: cobra.RangeArgs(1, 2),
			RunE: func(*cobra.Command, []string) error { return nil },
		}

		operation := convert(cmd)
		Expect(operation.Schema.Properties).To(HaveKey("args"))
		Expect(parameterNames(operation)).To(Equal([]string{"args"}))
	})
})
