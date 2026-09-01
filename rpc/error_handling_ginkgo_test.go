package rpc

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/flanksource/clicky/entity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RPC error handling", func() {
	It("keeps error hiding disabled unless the serve flag opts in", func() {
		Expect(DefaultServeConfig().HideErrorDetails).To(BeFalse())
		Expect(newOpenAPIServeCommand(nil).Flags().Lookup("hide-error-details").DefValue).To(Equal("false"))
	})

	It("continues an incoming trace and returns sanitized details by default", func() {
		server := NewSwaggerServer(&ServeConfig{}, nil, nil)
		handler := server.traceHandler("GET /failure", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.writeError(w, r, errors.New("database failed: password=correct-horse-battery-staple"))
		}))
		request := httptest.NewRequest(http.MethodGet, "/failure", nil)
		request.Header.Set("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		var body entity.ErrorResponse
		Expect(json.Unmarshal(response.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Trace).To(Equal("0123456789abcdef0123456789abcdef"))
		Expect(response.Header().Get(entity.ErrorTraceHeader)).To(Equal(body.Trace))
		Expect(body.Message).To(ContainSubstring("database failed"))
		Expect(response.Body.String()).ToNot(ContainSubstring("correct-horse-battery-staple"))
	})

	It("hides unclassified details only when configured", func() {
		server := NewSwaggerServer(&ServeConfig{HideErrorDetails: true}, nil, nil)
		handler := server.traceHandler("GET /failure", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.writeError(w, r, errors.New("private backend detail"))
		}))
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/failure", nil))

		var body entity.ErrorResponse
		Expect(json.Unmarshal(response.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Message).To(Equal(entity.InternalErrorMessage))
		Expect(body.Trace).To(MatchRegexp(`^[0-9a-f]{32}$`))
		Expect(response.Body.String()).ToNot(ContainSubstring("private backend detail"))
	})

	// The body caps are sized for a full stack trace. Header and trailer values
	// share the same rendering path but not the same ceiling: proxies refuse
	// oversized header blocks outright, so a body-sized diagnostic in
	// X-Stream-Error would fail the response instead of truncating it.
	It("bounds header and trailer error text far below the body cap", func() {
		server := NewSwaggerServer(&ServeConfig{}, nil, nil)

		value := server.clientErrorMessage(errors.New(strings.Repeat("stack frame ", 8*1024)))

		Expect(len(value)).To(BeNumerically("<=", entity.DefaultMaxErrorHeaderBytes))
		Expect(entity.DefaultMaxErrorHeaderBytes).To(BeNumerically("<", entity.DefaultMaxErrorDetailBytes))
		Expect(value).ToNot(ContainSubstring("\n"), "a header value cannot carry newlines")
	})

	It("documents the shared error envelope and trace header", func() {
		spec := NewOpenAPIGenerator(nil).GenerateFromService(&RPCService{Operations: []RPCOperation{{
			Name: "failure", Method: http.MethodGet, Path: "/failure",
		}}})

		for _, status := range []string{"400", "404", "405", "406", "500"} {
			response := spec.Paths["/failure"]["get"].Responses[status]
			Expect(response.Headers).To(HaveKey(entity.ErrorTraceHeader))
			schema := response.Content["application/json"].Schema
			Expect(schema.Properties).To(HaveKey("code"))
			Expect(schema.Properties).To(HaveKey("message"))
			Expect(schema.Properties).To(HaveKey("trace"))
			Expect(schema.Required).To(ConsistOf("code", "message", "trace"))
		}
	})
})
