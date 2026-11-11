package rpc

import (
	"encoding/json"
	"github.com/flanksource/clicky/flags"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("schema driven flag forms", func() {
	It("keeps declared connection choices available when the current store is empty", func() {
		cmd := &cobra.Command{Use: "capture", Run: func(*cobra.Command, []string) {}}
		cmd.Flags().String("connection", "", "Saved connection")
		Expect(cmd.RegisterFlagCompletionFunc("connection", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return []string{}, cobra.ShellCompDirectiveNoFileComp
		})).To(Succeed())
		op, err := NewConverter(nil).ConvertCommand(cmd)
		Expect(err).NotTo(HaveOccurred())
		completions, err := commandCompletions(cmd, *op, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(completions).To(HaveKey("connection"))
	})
	It("retains labels constraints and remote choices in terminal and web schemas", func() {
		cmd := &cobra.Command{Use: "capture [flags] [-- command [args...]]", Args: cobra.ArbitraryArgs, Run: func(*cobra.Command, []string) {}}
		cmd.Flags().String("connection", "", "Saved connection")
		metadata := map[string]any{"type": "string", "title": "Connection", "description": "Saved connection", "minLength": float64(1), "x-clicky-lookup": map[string]any{"url": "/api/v1/connection?types=postgres", "filter": "connection"}}
		Expect(flags.SetSchema(cmd.Flags().Lookup("connection"), metadata)).To(Succeed())
		op, err := NewConverter(nil).ConvertCommand(cmd)
		Expect(err).NotTo(HaveOccurred())
		Expect(op.Schema.Properties).NotTo(HaveKey("flags"))
		Expect(op.Schema.Properties).To(HaveKey("args"))
		raw, err := OperationInputSchema(*op, OperationSchemaOptions{Completions: []string{"connection"}})
		Expect(err).NotTo(HaveOccurred())
		var form struct {
			Properties map[string]map[string]any `json:"properties"`
		}
		Expect(json.Unmarshal(raw, &form)).To(Succeed())
		Expect(form.Properties["connection"]).To(Equal(metadata))
		generator := NewOpenAPIGenerator(nil)
		body := generator.convertOperationToOpenAPI(*op).RequestBody.Content["application/json"].Schema
		data, err := json.Marshal(body.Properties["connection"])
		Expect(err).NotTo(HaveOccurred())
		var served map[string]any
		Expect(json.Unmarshal(data, &served)).To(Succeed())
		Expect(served).To(Equal(metadata))
	})
	It("rejects metadata whose type disagrees with the real flag", func() {
		cmd := &cobra.Command{Use: "capture", Run: func(*cobra.Command, []string) {}}
		cmd.Flags().Int("limit", 1, "Row limit")
		Expect(flags.SetSchema(cmd.Flags().Lookup("limit"), map[string]any{"type": "string"})).To(Succeed())
		_, err := NewConverter(nil).ConvertCommand(cmd)
		Expect(err).To(MatchError(ContainSubstring("schema type string disagrees with flag type integer")))
	})
})
