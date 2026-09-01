package aichat_test

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	captools "github.com/flanksource/captain/pkg/ai/tools"
	"github.com/flanksource/captain/pkg/api"
	"github.com/flanksource/clicky"
	clickyaichat "github.com/flanksource/clicky/aichat"
	"github.com/spf13/cobra"
)

type greetOptions struct {
	Name string `flag:"name" help:"who to greet"`
}

type greetResult struct {
	Message string `json:"message"`
}

var _ = Describe("Cobra tool provider", func() {
	It("projects runnable RPC operations into executable Captain tools", func(ctx SpecContext) {
		root := &cobra.Command{Use: "example"}
		command := clicky.AddCommand(root, greetOptions{}, func(options greetOptions) (greetResult, error) {
			return greetResult{Message: "hello " + options.Name}, nil
		})
		clicky.AnnotateTool(command, clicky.MCPToolHints{
			Group: "examples.read", Parent: "Examples", DefaultPermission: clicky.ToolPermissionOn,
		})

		provider, err := clickyaichat.NewCobraToolProvider(clickyaichat.CobraToolProviderOptions{Root: root})
		Expect(err).NotTo(HaveOccurred())
		set, err := provider.ToolSet(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(set.Definitions).To(HaveLen(1))
		Expect(set.Catalog).To(HaveLen(1))

		definition := set.Definitions[0]
		Expect(definition).To(MatchFields(IgnoreExtras, Fields{
			"Name":              Equal("greet"),
			"Group":             Equal("examples.read"),
			"Parent":            Equal("Examples"),
			"DefaultPermission": Equal(api.ToolPolicyAllow),
		}))
		// The operation travels whole rather than as a string map, so captain reads
		// the method from the model clicky already has.
		Expect(definition.Operation).NotTo(BeNil())
		Expect(definition.Operation.Method).To(Equal("POST"))
		Expect(definition.Annotations).NotTo(HaveKey("clicky/method"))
		Expect(definition.InputSchema).To(HaveKeyWithValue("type", "object"))
		output, err := definition.Handler(context.Background(), map[string]any{"name": "world"})
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal(greetResult{Message: "hello world"}))

		entry := set.Catalog[0]
		Expect(entry).To(MatchFields(IgnoreExtras, Fields{
			"Name":              Equal("greet"),
			"Source":            Equal("clicky"),
			"Group":             Equal("examples.read"),
			"PreferenceKey":     Equal("examples.read"),
			"DefaultPermission": Equal(captools.ToolPolicyAllow),
			"OperationName":     Equal("greet"),
		}))
	})

	It("omits hidden and Cobra builtin operations", func(ctx SpecContext) {
		root := &cobra.Command{Use: "example"}
		root.AddCommand(
			&cobra.Command{Use: "visible", Run: func(*cobra.Command, []string) {}},
			&cobra.Command{Use: "hidden", Hidden: true, Run: func(*cobra.Command, []string) {}},
		)
		root.InitDefaultCompletionCmd()
		root.InitDefaultHelpCmd()

		provider, err := clickyaichat.NewCobraToolProvider(clickyaichat.CobraToolProviderOptions{Root: root})
		Expect(err).NotTo(HaveOccurred())
		set, err := provider.ToolSet(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(set.Definitions).To(HaveLen(1))
		Expect(set.Definitions[0].Name).To(Equal("visible"))
	})

	It("normalizes operation names to Captain's provider-safe alphabet", func(ctx SpecContext) {
		root := &cobra.Command{Use: "example"}
		root.AddCommand(&cobra.Command{Use: "unsafe.name", Run: func(*cobra.Command, []string) {}})

		provider, err := clickyaichat.NewCobraToolProvider(clickyaichat.CobraToolProviderOptions{Root: root})
		Expect(err).NotTo(HaveOccurred())
		set, err := provider.ToolSet(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(set.Definitions).To(HaveLen(1))
		Expect(set.Definitions[0].Name).To(Equal("unsafe_name"))
	})

	It("publishes an output schema for operations with a declared response type", func(ctx SpecContext) {
		root := &cobra.Command{Use: "example"}
		clicky.AddCommand(root, greetOptions{}, func(options greetOptions) (greetResult, error) {
			return greetResult{Message: "hello " + options.Name}, nil
		})

		provider, err := clickyaichat.NewCobraToolProvider(clickyaichat.CobraToolProviderOptions{Root: root})
		Expect(err).NotTo(HaveOccurred())
		set, err := provider.ToolSet(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(set.Catalog).To(HaveLen(1))

		outputSchema := set.Catalog[0].OutputSchema
		Expect(outputSchema).To(HaveKeyWithValue("type", "object"))
		Expect(outputSchema).To(HaveKeyWithValue("properties",
			HaveKeyWithValue("message", HaveKeyWithValue("type", "string"))),
			"the schema describes the greetResult return type")
	})

	// An agent backend calls tools back over a loopback MCP server whose requests
	// are rooted at context.Background(), so the handler's own context carries
	// none of the caller's request-scoped values. ToolSet is the last place those
	// values are still in hand, so they must be captured there and re-attached —
	// while cancellation still comes from the call, since the agent turn outlives
	// the HTTP request that built the tool set.
	It("carries request-scoped values from ToolSet into handlers invoked later", func() {
		type scopeKey struct{}
		var seen any
		var deadline bool

		root := &cobra.Command{Use: "example"}
		clicky.AddCommandWithContext(root, greetOptions{},
			func(ctx context.Context, options greetOptions) (greetResult, error) {
				seen = ctx.Value(scopeKey{})
				_, deadline = ctx.Deadline()
				return greetResult{Message: "hello " + options.Name}, nil
			})

		scoped, cancelScope := context.WithCancel(context.WithValue(context.Background(), scopeKey{}, "tenant-x"))
		provider, err := clickyaichat.NewCobraToolProvider(clickyaichat.CobraToolProviderOptions{Root: root})
		Expect(err).NotTo(HaveOccurred())
		set, err := provider.ToolSet(scoped)
		Expect(err).NotTo(HaveOccurred())
		// The request that built the tool set is over before the agent calls back.
		cancelScope()

		call, cancelCall := context.WithTimeout(context.Background(), time.Minute)
		defer cancelCall()
		output, err := set.Definitions[0].Handler(call, map[string]any{"name": "world"})

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal(greetResult{Message: "hello world"}))
		Expect(seen).To(Equal("tenant-x"), "request-scoped values must reach the handler")
		Expect(deadline).To(BeTrue(), "lifetime must come from the call, not the captured scope")
	})

	It("omits outputSchema for operations without a declared response type", func(ctx SpecContext) {
		root := &cobra.Command{Use: "example"}
		root.AddCommand(&cobra.Command{Use: "visible", Run: func(*cobra.Command, []string) {}})

		provider, err := clickyaichat.NewCobraToolProvider(clickyaichat.CobraToolProviderOptions{Root: root})
		Expect(err).NotTo(HaveOccurred())
		set, err := provider.ToolSet(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(set.Catalog).To(HaveLen(1))
		Expect(set.Catalog[0].OutputSchema).To(BeNil())

		// The frontend degrades to value heuristics on a missing key, so the
		// absence has to survive marshalling — not just be an empty map.
		encoded, err := json.Marshal(set.Catalog[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(string(encoded)).NotTo(ContainSubstring("outputSchema"))
	})
})
