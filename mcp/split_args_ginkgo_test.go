package mcp

import (
	"github.com/flanksource/clicky/rpc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// MCP tool calls route a named positional ("pretty <file>") to args, as HTTP
// does; as a flag the command has none of, it would be silently skipped.
var _ = Describe("splitMCPArgs", func() {
	const dataFile = "orders.json"

	operation := &rpc.RPCOperation{Parameters: []rpc.RPCParameter{
		{Name: "file", Type: "string", In: "query", Positional: true},
		{Name: "schema", Type: "string", In: "query"},
	}}

	DescribeTable("delivers the positional as args and the rest as flags",
		func(args map[string]interface{}) {
			flags, posArgs := splitMCPArgs(operation, args)

			Expect(posArgs).To(Equal([]string{dataFile}))
			Expect(flags).To(Equal(map[string]string{"schema": "s.yaml"}))
		},
		Entry("named positional", map[string]interface{}{"file": dataFile, "schema": "s.yaml"}),
		Entry("generic args array", map[string]interface{}{"args": []interface{}{dataFile}, "schema": "s.yaml"}),
	)

	It("keeps every non-args key a flag when the tool has no operation", func() {
		flags, posArgs := splitMCPArgs(nil, map[string]interface{}{"file": dataFile})

		Expect(posArgs).To(BeEmpty())
		Expect(flags).To(Equal(map[string]string{"file": dataFile}))
	})
})
