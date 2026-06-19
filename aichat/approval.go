package aichat

import (
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/flanksource/clicky/rpc"
)

// approvalKey is the metadata key under which a tool's interrupt carries its
// approval request, mirrored back to the client as a tool-approval-request.
const approvalKey = "approval"

// ToolMode controls how a tool is exposed for one request.
type ToolMode string

const (
	ToolModeEnabled  ToolMode = "enabled"
	ToolModeAsk      ToolMode = "ask"
	ToolModeDisabled ToolMode = "disabled"
)

// ToolPreferences carries the clicky-ui tool preference payload. The UI sends
// "enabled", "ask", or "disabled"; "auto" and "off" are accepted for older
// callers that used those labels.
type ToolPreferences map[string]ToolMode

func normalizeToolMode(mode ToolMode) (ToolMode, bool) {
	switch ToolMode(strings.ToLower(strings.TrimSpace(string(mode)))) {
	case ToolModeEnabled, "auto":
		return ToolModeEnabled, true
	case ToolModeAsk:
		return ToolModeAsk, true
	case ToolModeDisabled, "off":
		return ToolModeDisabled, true
	default:
		return "", false
	}
}

// ToolInfo describes the concrete tool call being considered for approval.
type ToolInfo struct {
	Name          string
	OperationName string
	Method        string
	Path          string
	ClickyVerb    string
	ClickyScope   string
	Operation     *rpc.RPCOperation
}

func toolInfo(name string, op *rpc.RPCOperation) ToolInfo {
	info := ToolInfo{Name: name, Operation: op}
	if op == nil {
		return info
	}
	info.OperationName = op.Name
	info.Method = strings.ToUpper(op.Method)
	info.Path = op.Path
	if op.Clicky != nil {
		info.ClickyVerb = op.Clicky.Verb
		info.ClickyScope = op.Clicky.Scope
	}
	return info
}

// ApprovalPolicy reports whether a tool call must be approved by the user
// before it executes. Returning true pauses the call for human-in-the-loop
// approval; false runs it automatically.
type ApprovalPolicy func(toolName string, input any) bool

// ToolApprovalPolicy is the metadata-aware approval hook. It takes precedence
// over ApprovalPolicy when both are configured.
type ToolApprovalPolicy func(tool ToolInfo, input any) bool

// approvalPredicate is the internal alias used to gate tool execution.
type approvalPredicate func(tool ToolInfo, input any) bool

// resolveApprovalPolicy picks the effective gate: an explicit policy wins,
// otherwise an exact-name list, otherwise nil (auto-approve everything).
func resolveApprovalPolicy(toolPolicy ToolApprovalPolicy, policy ApprovalPolicy, names []string) approvalPredicate {
	if toolPolicy != nil {
		return approvalPredicate(toolPolicy)
	}
	if policy != nil {
		return func(tool ToolInfo, input any) bool {
			return policy(tool.Name, input)
		}
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
	return func(tool ToolInfo, _ any) bool {
		return set[tool.Name]
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
