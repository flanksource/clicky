package aichat

import "github.com/firebase/genkit/go/ai"

// approvalKey is the metadata key under which a tool's interrupt carries its
// approval request, mirrored back to the client as a tool-approval-request.
const approvalKey = "approval"

// ApprovalPolicy reports whether a tool call must be approved by the user
// before it executes. Returning true pauses the call for human-in-the-loop
// approval; false runs it automatically.
type ApprovalPolicy func(toolName string, input any) bool

// approvalPredicate is the internal alias used to gate tool execution.
type approvalPredicate = ApprovalPolicy

// resolveApprovalPolicy picks the effective gate: an explicit policy wins,
// otherwise an exact-name list, otherwise nil (auto-approve everything).
func resolveApprovalPolicy(policy ApprovalPolicy, names []string) approvalPredicate {
	if policy != nil {
		return policy
	}
	return requireApprovalFor(names)
}

// requireApprovalFor builds a predicate that requires approval for exactly the
// named tools. An empty list yields nil (auto-approve everything).
func requireApprovalFor(names []string) approvalPredicate {
	if len(names) == 0 {
		return nil
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return func(toolName string, _ any) bool {
		return set[toolName]
	}
}

// interruptForApproval signals genkit to pause this tool call pending user
// approval. The metadata is preserved on the interrupted tool request and is
// what we translate into a tool-approval-request on the wire.
func interruptForApproval(tc *ai.ToolContext, toolName string) error {
	return tc.Interrupt(&ai.InterruptOptions{
		Metadata: map[string]any{approvalKey: map[string]any{"toolName": toolName}},
	})
}
