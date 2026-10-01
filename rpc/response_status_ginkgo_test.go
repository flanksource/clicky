package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/flanksource/clicky/entity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type startedRun struct {
	RunID  string `json:"run_id"`
	status int
}

func (run startedRun) ResponseStatus() int { return run.status }

func serverReturning(result any) *SwaggerServer {
	op := RPCOperation{
		Name: "modules add", Path: "/api/v1/modules/add", Method: http.MethodPost,
		DataFunc: func(map[string]string, []string) (any, error) { return result, nil },
	}
	service := &RPCService{Name: "api", Operations: []RPCOperation{op}}
	return &SwaggerServer{
		config:      &ServeConfig{Executor: &ExecutorConfig{Enabled: true}, StructuredErrorResponses: true},
		executor:    NewCommandExecutor(service, &ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"}),
		errorWriter: entity.NewErrorWriter(entity.ErrorOptions{}),
	}
}

func post(server *SwaggerServer) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/modules/add", nil)
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	server.handleExecuteCommand(response, request)
	return response
}

var _ = Describe("operation result status", func() {
	It("answers with the success status an operation result declares", func() {
		response := post(serverReturning(startedRun{RunID: "run-1", status: http.StatusAccepted}))

		Expect(response.Code).To(Equal(http.StatusAccepted), response.Body.String())
		var body map[string]string
		Expect(json.Unmarshal(response.Body.Bytes(), &body)).To(Succeed())
		Expect(body).To(Equal(map[string]string{"run_id": "run-1"}))
	})

	It("rejects a declared status outside 2xx as a server error", func() {
		response := post(serverReturning(startedRun{RunID: "run-1", status: http.StatusNotFound}))

		Expect(response.Code).To(Equal(http.StatusInternalServerError), response.Body.String())
		Expect(response.Body.String()).To(ContainSubstring("declared response status 404"))
	})
})
