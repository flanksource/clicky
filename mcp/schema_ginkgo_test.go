package mcp

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/flanksource/clicky/flags"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
)

var _ = Describe("MCP tool input schemas", func() {
	It("publishes a raw JSON flag as an untyped property valid against JSON Schema 2020-12", func() {
		type target struct {
			URI string `json:"uri"`
		}
		type options struct {
			Content  json.RawMessage `flag:"content" help:"Structured content"`
			Target   *target         `flag:"target" help:"Target object"`
			Links    []target        `flag:"links" help:"Target list"`
			Evidence []string        `flag:"evidence" help:"Evidence list"`
		}
		fields, err := flags.ParseStructFields(reflect.TypeFor[options]())
		Expect(err).NotTo(HaveOccurred())
		root := &cobra.Command{Use: "app"}
		leaf := &cobra.Command{Use: "annotate", Short: "annotate a record", Run: func(*cobra.Command, []string) {}}
		for _, field := range fields {
			flags.BindFlag(leaf, field)
		}
		root.AddCommand(leaf)
		config := DefaultConfig()
		config.Tools.AutoExpose = true
		registry := NewToolRegistry(config)
		Expect(registry.RegisterCommandTree(root)).To(Succeed())
		tool, ok := registry.GetTool("annotate")
		Expect(ok).To(BeTrue())

		raw, err := json.Marshal(tool.InputSchema)
		Expect(err).NotTo(HaveOccurred())
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		Expect(err).NotTo(HaveOccurred())
		compiler := jsonschema.NewCompiler()
		Expect(compiler.AddResource("mem:///spec/annotate.json", document)).To(Succeed())
		_, err = compiler.Compile("mem:///spec/annotate.json")
		Expect(err).NotTo(HaveOccurred())

		var published struct {
			Properties map[string]map[string]any `json:"properties"`
		}
		Expect(json.Unmarshal(raw, &published)).To(Succeed())
		Expect(published.Properties["content"]).To(Equal(map[string]any{"description": "Structured content"}))
		Expect(published.Properties["target"]).To(HaveKeyWithValue("type", "object"))
		Expect(published.Properties["links"]).To(HaveKeyWithValue("type", "array"))
		Expect(published.Properties["links"]["items"]).To(Equal(map[string]any{"description": ""}))
		Expect(published.Properties["evidence"]["items"]).To(HaveKeyWithValue("type", "string"))
	})
})
