package rpc

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OpenAPI response headers", func() {
	It("documents the total count relation for paged operations", func() {
		spec := NewOpenAPIGenerator(nil).GenerateFromService(&RPCService{Operations: []RPCOperation{{
			Name: "items", Method: http.MethodGet, Path: "/items", ResponsePaged: true,
		}}})
		headers := spec.Paths["/items"]["get"].Responses["200"].Headers

		relation, ok := headers["X-Total-Relation"]
		Expect(ok).To(BeTrue())
		Expect(relation.Schema).ToNot(BeNil())
		Expect(relation.Schema.Type).To(Equal("string"))
		Expect(relation.Description).To(ContainSubstring("unknown means unavailable"))
	})
})
