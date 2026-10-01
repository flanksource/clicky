package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/route"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

type deleteFlagsRPCItem struct {
	ID string `json:"id"`
}

func (i deleteFlagsRPCItem) GetID() string   { return i.ID }
func (i deleteFlagsRPCItem) GetName() string { return i.ID }

type deleteFlagsRPCOptions struct {
	DryRun      bool   `flag:"dry-run" help:"Preview the delete without applying it"`
	Fingerprint string `flag:"fingerprint" help:"Fingerprint returned by the preview"`
}

func (deleteFlagsRPCOptions) ClickyActionFlags() {}

type deleteFlagsRPCResult struct {
	ID    string            `json:"id"`
	Flags map[string]string `json:"flags"`
}

var _ = Describe("entity delete with flags over HTTP", func() {
	const (
		name        = "delete-flags-rpc-spec"
		id          = "client-1"
		fingerprint = "abc123"
	)

	var (
		server    *SwaggerServer
		mux       *http.ServeMux
		operation RPCOperation
	)

	BeforeEach(func() {
		if _, registered := clicky.GetEntity(name); !registered {
			clicky.NewEntity[deleteFlagsRPCItem, struct{}, deleteFlagsRPCItem](name).
				DeleteWithFlagsAndContext(deleteFlagsRPCOptions{}, func(_ context.Context, id string, flags map[string]string) (any, error) {
					return deleteFlagsRPCResult{ID: id, Flags: flags}, nil
				}).
				Register()
		}

		root := &cobra.Command{Use: "test"}
		clicky.GenerateCLI(root)
		server = NewSwaggerServer(&ServeConfig{
			Host: "localhost", Port: 8080,
			Executor: &ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"},
		}, root, &OpenAPIConfig{})
		mux = http.NewServeMux()
		server.RegisterRoutes(route.NewRouter(mux))

		var matches []RPCOperation
		for _, candidate := range server.executor.service.Operations {
			if meta := candidate.Clicky; meta != nil && meta.Entity == name && meta.Verb == "delete" {
				matches = append(matches, candidate)
			}
		}
		Expect(matches).To(HaveLen(1))
		operation = matches[0]
	})

	It("publishes the flags as query parameters on DELETE /{id}", func() {
		Expect(operation.Method).To(Equal(http.MethodDelete))
		Expect(operation.Path).To(Equal("/api/v1/" + name + "/{id}"))
		Expect(operation.Parameters).To(ConsistOf(
			RPCParameter{Name: "id", Type: "string", Description: "Positional argument from command", Required: true, In: "path"},
			RPCParameter{Name: "dry-run", Type: "boolean", Description: "Preview the delete without applying it", In: "query", Default: false},
			RPCParameter{Name: "fingerprint", Type: "string", Description: "Fingerprint returned by the preview", In: "query"},
		))
	})

	It("documents a dynamic response instead of the success envelope", func() {
		schema, err := ResponseSchema(operation)

		Expect(err).NotTo(HaveOccurred())
		Expect(schema).To(Equal(map[string]any{"type": "object", "description": "Dynamic response value"}))
	})

	It("hands the path id and query flags to the handler and answers with its result", func() {
		request := httptest.NewRequest(http.MethodDelete, "/api/v1/"+name+"/"+id+"?dry-run=true&fingerprint="+fingerprint, nil)
		request.Header.Set("Accept", "application/json")
		response := httptest.NewRecorder()

		mux.ServeHTTP(response, request)

		Expect(response.Code).To(Equal(http.StatusOK), response.Body.String())
		var got deleteFlagsRPCResult
		Expect(json.Unmarshal(response.Body.Bytes(), &got)).To(Succeed())
		Expect(got).To(Equal(deleteFlagsRPCResult{
			ID:    id,
			Flags: map[string]string{"id": id, "dry-run": "true", "fingerprint": fingerprint},
		}))
	})
})
