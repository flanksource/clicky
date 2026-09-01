package rpc

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("RPC escaped paths", func() {
	It("passes an encoded slash inside a path parameter to the command", func() {
		want := "ar://b1fa58c0-e86e-497a-b166-3ce197b87f96"
		var got string
		command := &cobra.Command{
			Use:  "get <id>",
			Args: cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				got = args[0]
				return nil
			},
		}
		operation := RPCOperation{
			Name: "accounts get", Path: "/api/v1/accounts/{id}", Method: http.MethodGet,
			Command:    NewCobraExecutableCommand(command),
			Parameters: []RPCParameter{{Name: "id", Type: "string", Required: true, In: "path"}},
		}
		executor := NewCommandExecutor(
			&RPCService{Operations: []RPCOperation{operation}},
			&ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"},
		)
		server := &SwaggerServer{executor: executor}
		request := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/ar%3A%2F%2Fb1fa58c0-e86e-497a-b166-3ce197b87f96", nil)

		_, _, status, err := server.executeCommandCore(request)

		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal(http.StatusOK))
		Expect(got).To(Equal(want))
	})
})
