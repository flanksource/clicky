package rpc

import (
	"net/http"
	"net/http/httptest"

	rpchttp "github.com/flanksource/clicky/rpc/http"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RPC server timing", func() {
	It("reports command execution and response formatting phases", func() {
		op := RPCOperation{
			Name: "timed list", Path: "/api/v1/timed", Method: http.MethodGet,
			DataFunc: func(_ map[string]string, _ []string) (any, error) {
				return map[string]string{"status": "ok"}, nil
			},
		}
		server := &SwaggerServer{
			config: &ServeConfig{Executor: &ExecutorConfig{Enabled: true}},
			executor: NewCommandExecutor(
				&RPCService{Name: "api", Operations: []RPCOperation{op}},
				&ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"},
			),
		}
		handler := rpchttp.TimingMiddleware(http.HandlerFunc(server.handleExecuteCommand))
		request := httptest.NewRequest(http.MethodGet, "/api/v1/timed", nil)
		request.Header.Set("Accept", "application/json")
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		Expect(response.Code).To(Equal(http.StatusOK))
		Expect(response.Header().Get("Server-Timing")).To(And(
			ContainSubstring("total;dur="),
			ContainSubstring("command;dur="),
			ContainSubstring("format;dur="),
		))
	})
})
