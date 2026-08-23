package aichat

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/captain/pkg/api"
	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/rpc"
)

func readOnly(v bool) *bool { return &v }

var _ = Describe("defaultToolPermission", func() {
	resolve := func(method string, hints entity.MCPToolHints) (api.ToolPolicy, error) {
		return defaultToolPermission(&rpc.RPCOperation{Name: "sample", Method: method}, hints)
	}

	DescribeTable("resolves the policy for each derivation branch",
		func(method string, hints entity.MCPToolHints, want api.ToolPolicy) {
			policy, err := resolve(method, hints)
			Expect(err).NotTo(HaveOccurred())
			Expect(policy).To(Equal(want))
		},
		Entry("explicit allow wins over the method", "POST", entity.MCPToolHints{DefaultPermission: "allow"}, api.ToolPolicyAllow),
		Entry("explicit ask wins over the method", "GET", entity.MCPToolHints{DefaultPermission: "ask"}, api.ToolPolicyAsk),
		Entry("explicit auto defers to the runtime policy", "GET", entity.MCPToolHints{DefaultPermission: "auto"}, api.ToolPolicyAuto),
		Entry("explicit deny omits the tool", "GET", entity.MCPToolHints{DefaultPermission: "deny"}, api.ToolPolicyDeny),
		Entry("explicit policy is trimmed and case-folded", "POST", entity.MCPToolHints{DefaultPermission: " Allow "}, api.ToolPolicyAllow),
		Entry("legacy on spelling runs without asking", "POST", entity.MCPToolHints{DefaultPermission: entity.ToolPermissionOn}, api.ToolPolicyAllow),
		Entry("legacy off spelling omits the tool", "GET", entity.MCPToolHints{DefaultPermission: entity.ToolPermissionOff}, api.ToolPolicyDeny),
		Entry("read-only hint auto-runs even a mutating method", "DELETE", entity.MCPToolHints{ReadOnlyHint: readOnly(true)}, api.ToolPolicyAllow),
		Entry("explicitly non-read-only hint defers to the method", "POST", entity.MCPToolHints{ReadOnlyHint: readOnly(false)}, api.ToolPolicyAsk),
		Entry("GET auto-runs", "GET", entity.MCPToolHints{}, api.ToolPolicyAllow),
		Entry("HEAD auto-runs", "HEAD", entity.MCPToolHints{}, api.ToolPolicyAllow),
		Entry("OPTIONS auto-runs", "OPTIONS", entity.MCPToolHints{}, api.ToolPolicyAllow),
		Entry("method matching is case-insensitive", "get", entity.MCPToolHints{}, api.ToolPolicyAllow),
		Entry("POST asks", "POST", entity.MCPToolHints{}, api.ToolPolicyAsk),
		Entry("PUT asks", "PUT", entity.MCPToolHints{}, api.ToolPolicyAsk),
		Entry("PATCH asks", "PATCH", entity.MCPToolHints{}, api.ToolPolicyAsk),
		Entry("DELETE asks", "DELETE", entity.MCPToolHints{}, api.ToolPolicyAsk),
		Entry("an unknown method defers to the runtime policy", "SUBSCRIBE", entity.MCPToolHints{}, api.ToolPolicyAuto),
		Entry("a missing method defers to the runtime policy", "", entity.MCPToolHints{}, api.ToolPolicyAuto),
	)

	It("rejects an explicit permission outside captain's vocabulary", func() {
		_, err := resolve("GET", entity.MCPToolHints{DefaultPermission: "sometimes"})
		Expect(err).To(MatchError(ContainSubstring(`operation "sample" has invalid default tool permission "sometimes"`)))
	})
})
