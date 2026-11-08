package rpc

import (
	"encoding/json"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("opaque JSON response schemas", func() {
	DescribeTable("preserves array element choices", func(property Property, expected []interface{}) {
		schema := NewOpenAPIGenerator(nil).convertPropertyToOpenAPI(property)
		Expect(schema.Enum).To(BeEmpty())
		Expect(schema.Items.Enum).To(Equal(expected))
	},
		Entry("explicit item enum", Property{Type: "array", Items: &Property{Type: "string", Enum: []string{"ready", "waiting"}}}, []interface{}{"ready", "waiting"}),
		Entry("repeatable flag enum", Property{Type: "array", Enum: []string{"ready", "waiting"}, Items: &Property{Type: "string"}}, []interface{}{"ready", "waiting"}),
	)

	It("keeps raw JSON unconstrained while describing ordinary binary bytes as base64", func() {
		type payload struct {
			Content json.RawMessage   `json:"content"`
			Files   []json.RawMessage `json:"files"`
			Bytes   []byte            `json:"bytes"`
		}
		schema, err := ResponseSchema(RPCOperation{ResponseType: reflect.TypeFor[payload]()})
		Expect(err).NotTo(HaveOccurred())
		Expect(schema["properties"]).To(Equal(map[string]any{
			"content": map[string]any{},
			"files":   map[string]any{"type": "array", "items": map[string]any{}},
			"bytes":   map[string]any{"type": "string", "format": "byte"},
		}))
	})
})
