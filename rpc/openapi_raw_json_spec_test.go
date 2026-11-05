package rpc

import (
	"encoding/json"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("opaque JSON response schemas", func() {
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
