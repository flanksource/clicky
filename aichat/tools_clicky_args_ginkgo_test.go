package aichat

import (
	"github.com/flanksource/clicky/rpc"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("toExecutionRequest", func() {
	// A command whose positional arguments have no name in its Use string gets
	// the converter's generic `args` array (rpc/converter.go), and that is the
	// only argument slot the model ever sees for such a tool. It has to reach
	// ExecutionRequest.Args: routed into Flags instead, the command runs with no
	// positional arguments at all and reports its own argument as missing.
	ginkgo.It("routes the generic args array to positional arguments", func() {
		request := toExecutionRequest(map[string]any{
			"args": []any{"SELECT TOP 1 * FROM AsCode"},
		}, nil)

		gomega.Expect(request.Args).To(gomega.Equal([]string{"SELECT TOP 1 * FROM AsCode"}))
		gomega.Expect(request.Flags).NotTo(gomega.HaveKey("args"))
	})

	ginkgo.It("accepts a bare string for the args array", func() {
		// Models routinely send the scalar when the array holds one element;
		// the HTTP body path already tolerates it, so the tool path must too.
		request := toExecutionRequest(map[string]any{"args": "group-life"}, nil)

		gomega.Expect(request.Args).To(gomega.Equal([]string{"group-life"}))
	})

	ginkgo.It("keeps named path parameters ahead of the generic args", func() {
		request := toExecutionRequest(map[string]any{
			"id":     "abc-123",
			"args":   []any{"extra"},
			"format": "json",
		}, []string{"id"})

		gomega.Expect(request.Args).To(gomega.Equal([]string{"abc-123", "extra"}))
		gomega.Expect(request.Flags).To(gomega.Equal(map[string]string{"format": "json"}))
	})

	ginkgo.It("leaves every other key a flag", func() {
		request := toExecutionRequest(map[string]any{"limit": 10, "trace": true}, nil)

		gomega.Expect(request.Args).To(gomega.BeEmpty())
		gomega.Expect(request.Flags).To(gomega.Equal(map[string]string{"limit": "10", "trace": "true"}))
	})
})

var _ = ginkgo.Describe("positionalParams", func() {
	ginkgo.It("names only the path parameters, in operation order", func() {
		op := &rpc.RPCOperation{Parameters: []rpc.RPCParameter{
			{Name: "format", In: "query"},
			{Name: "id", In: "path"},
			{Name: "child", In: "path"},
		}}

		gomega.Expect(positionalParams(op)).To(gomega.Equal([]string{"id", "child"}))
	})
})
