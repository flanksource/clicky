package aichat_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	captools "github.com/flanksource/captain/pkg/ai/tools"
	"github.com/flanksource/captain/pkg/api"
	"github.com/flanksource/clicky"
	clickyaichat "github.com/flanksource/clicky/aichat"
	"github.com/spf13/cobra"
)

type auditOptions struct {
	Reason string `flag:"reason" help:"why"`
}

type auditResult struct {
	OK bool `json:"ok"`
}

// What clicky hands captain is the operation and what its author declared —
// never an answer about authority. Deciding here is what left a consumer's rules
// arguing with a value clicky had already written into the slot they resolve.
var _ = Describe("Handing a command tree to captain", func() {
	toolSet := func(ctx SpecContext, root *cobra.Command, options clickyaichat.CobraToolProviderOptions) []api.ToolDefinition {
		GinkgoHelper()
		options.Root = root
		provider, err := clickyaichat.NewCobraToolProvider(options)
		Expect(err).NotTo(HaveOccurred())
		set, err := provider.ToolSet(ctx)
		Expect(err).NotTo(HaveOccurred())
		return set.Definitions
	}

	auditRoot := func() *cobra.Command {
		root := &cobra.Command{Use: "example"}
		clicky.AddCommand(root, auditOptions{}, func(auditOptions) (auditResult, error) {
			return auditResult{OK: true}, nil
		})
		return root
	}

	It("leaves the permission unset when no author registered one", func(ctx SpecContext) {
		definitions := toolSet(ctx, auditRoot(), clickyaichat.CobraToolProviderOptions{})

		Expect(definitions).To(HaveLen(1))
		Expect(definitions[0].DefaultPermission).To(BeEmpty(),
			"a derived answer here would pre-empt the strategies and rules captain resolves")
	})

	It("carries an explicitly registered permission through", func(ctx SpecContext) {
		root := &cobra.Command{Use: "example"}
		command := clicky.AddCommand(root, auditOptions{}, func(auditOptions) (auditResult, error) {
			return auditResult{OK: true}, nil
		})
		clicky.AnnotateTool(command, clicky.MCPToolHints{DefaultPermission: clicky.ToolPermissionOn})

		definitions := toolSet(ctx, root, clickyaichat.CobraToolProviderOptions{})

		Expect(definitions[0].DefaultPermission).To(Equal(api.ToolPolicyAllow))
	})

	It("carries the operation so a rule can select on its method", func(ctx SpecContext) {
		definitions := toolSet(ctx, auditRoot(), clickyaichat.CobraToolProviderOptions{})

		policy := api.PermissionPolicy{
			{ToolMatch: api.ToolMatch{Method: api.MatchPatterns{"POST"}}, Policy: api.ToolPolicyDeny},
		}
		resolved, err := captools.ResolveDefinitions(definitions, captools.ResolveOptions{Policy: policy})

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved).To(BeEmpty(), "the method rule must select the tool through its operation")
	})

	It("returns the permission model it was built with for the caller to resolve", func() {
		policy := api.PermissionPolicy{
			{ToolMatch: api.ToolMatch{Verb: api.MatchPatterns{"list"}}, Policy: api.ToolPolicyAllow},
		}
		strategies := []api.PermissionStrategy{api.MCPHintStrategy{}}

		provider, err := clickyaichat.NewCobraToolProvider(clickyaichat.CobraToolProviderOptions{
			Root: auditRoot(), Policy: policy, Strategies: strategies,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(provider.ToolPolicy()).To(Equal(policy))
		Expect(provider.Strategies()).To(HaveLen(1))
	})

	// An unregistered POST reaches captain saying nothing about authority, and
	// captain's default chain is what turns it into an ask.
	It("leaves the answer to captain's strategies", func(ctx SpecContext) {
		definitions := toolSet(ctx, auditRoot(), clickyaichat.CobraToolProviderOptions{})

		resolved, err := captools.ResolveDefinitions(definitions, captools.ResolveOptions{})

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved).To(HaveLen(1))
		Expect(resolved[0].DefaultPermission).To(Equal(api.ToolPolicyAsk))
	})
})
