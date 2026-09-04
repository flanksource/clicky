package aichat

import (
	"context"
	"fmt"

	"github.com/flanksource/clicky/rpc"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

// findingsError is the shape a command uses when it fails but its findings are
// the point of running it — a validator, a linter, a check. clicky already
// supports it end to end for CLI and HTTP output: the executor deliberately
// returns the payload alongside the error so both survive.
type findingsError struct{ count int }

func (e findingsError) Error() string { return fmt.Sprintf("validate: %d error(s)", e.count) }

// dataFuncHandler builds the tool handler for a one-operation service whose
// command returns the given result and error.
func dataFuncHandler(data any, err error) func(context.Context, map[string]any) (any, error) {
	op := rpc.RPCOperation{
		Name: "validate", Method: "POST", Path: "/api/v1/validate",
		ContextDataFunc: func(context.Context, map[string]string, []string) (any, error) {
			return data, err
		},
	}
	service := &rpc.RPCService{Operations: []rpc.RPCOperation{op}}
	provider := &CobraToolProvider{
		service: service,
		executor: rpc.NewCommandExecutor(service, &rpc.ExecutorConfig{
			Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1",
		}),
	}
	return provider.handlerFor(&service.Operations[0], context.Background())
}

var _ = ginkgo.Describe("a tool whose command fails", func() {
	// The model gets one string back for a failed tool call — captain renders a
	// handler error as the MCP error result — so anything the command found has
	// to be in it. Dropping the payload turned `oipa-cli test validate` into
	// "validate: 1 error(s)" with no field, file or line, and an observed agent
	// read the command's own source to work out what it had been told.
	ginkgo.It("carries the command's own findings in the error", func() {
		handler := dataFuncHandler(map[string]any{
			"ok": false,
			"diagnostics": []any{map[string]any{
				"severity": "error", "source": "schema",
				"message": "steps.2.policy: unknown field CoverAmount",
			}},
		}, findingsError{count: 1})

		_, err := handler(context.Background(), map[string]any{})

		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("validate: 1 error(s)"),
			"the failure itself must still be stated")
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("unknown field CoverAmount"),
			"the findings returned alongside the error must reach the model")
	})

	ginkgo.It("reports a failure that produced no findings with just its message", func() {
		// The executor echoes its own ExecutionResponse in the data slot when the
		// command returned none. That is bookkeeping, not a finding, and pasting
		// it into the error would bury the message in a JSON envelope.
		handler := dataFuncHandler(nil, fmt.Errorf("no such file"))

		_, err := handler(context.Background(), map[string]any{})

		gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("no such file")))
		gomega.Expect(err.Error()).NotTo(gomega.ContainSubstring("exitCode"))
		gomega.Expect(err.Error()).NotTo(gomega.ContainSubstring("\"success\""))
	})

	ginkgo.It("returns the result unchanged when the command succeeds", func() {
		handler := dataFuncHandler(map[string]any{"ok": true}, nil)

		data, err := handler(context.Background(), map[string]any{})

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(data).To(gomega.Equal(map[string]any{"ok": true}))
	})
})
