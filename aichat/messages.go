package aichat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/firebase/genkit/go/ai"
)

// ChatRequest is the body POSTed by the AI SDK DefaultChatTransport. messages is
// the running UIMessage list; model and reasoningEffort are optional per-request
// overrides carried in the transport `body`.
type ChatRequest struct {
	ID              string      `json:"id,omitempty"`
	Messages        []UIMessage `json:"messages"`
	Model           string      `json:"model,omitempty"`
	ReasoningEffort Effort      `json:"reasoningEffort,omitempty"`
	// ThreadID, when set (carried in the transport body), names the persisted
	// conversation this turn belongs to. Distinct from ID, which is the AI SDK
	// client chat id and not a server thread.
	ThreadID string `json:"threadId,omitempty"`
}

// UIMessage is the AI SDK v6 client message: a role plus typed parts.
type UIMessage struct {
	ID    string   `json:"id,omitempty"`
	Role  string   `json:"role"`
	Parts []UIPart `json:"parts"`
}

// UIPart is one part of a UIMessage. It models the union of the part shapes we
// consume: text, reasoning, file (multimodal input), and tool parts (typed
// `tool-<name>` or `dynamic-tool`, which is how clicky operations surface).
type UIPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`

	// file parts
	MediaType string `json:"mediaType,omitempty"`
	URL       string `json:"url,omitempty"`
	Filename  string `json:"filename,omitempty"`

	// tool parts
	ToolName   string          `json:"toolName,omitempty"`
	ToolCallID string          `json:"toolCallId,omitempty"`
	State      string          `json:"state,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	Output     json.RawMessage `json:"output,omitempty"`
	Approval   *Approval       `json:"approval,omitempty"`
}

// Approval is the AI SDK v6 tool-approval envelope carried on a tool part. When
// the user responds, Approved is set; until then it is nil.
type Approval struct {
	ID       string `json:"id"`
	Approved *bool  `json:"approved,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

func (p UIPart) isTool() bool {
	return p.Type == "dynamic-tool" || strings.HasPrefix(p.Type, "tool-")
}

// toolName returns the tool name for a tool part: the explicit toolName for
// dynamic tools, otherwise the suffix after the `tool-` type prefix.
func (p UIPart) toolName() string {
	if p.ToolName != "" {
		return p.ToolName
	}
	if strings.HasPrefix(p.Type, "tool-") {
		return strings.TrimPrefix(p.Type, "tool-")
	}
	return ""
}

// pendingApproval reports whether this tool part carries a user approval
// decision that the backend has not yet acted on (the resume trigger).
func (p UIPart) pendingApproval() bool {
	return p.State == "approval-responded" && p.Approval != nil && p.Approval.Approved != nil
}

// decodeJSON unmarshals a raw tool input/output into a generic value. An empty
// payload decodes to nil so a missing input is not an error.
func decodeJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

// resumeDirectives carries the genkit Restart/Respond parts that resolve the
// interrupted tool requests of an approval resume.
type resumeDirectives struct {
	restarts []*ai.Part // WithToolRestarts — re-execute (approved)
	responds []*ai.Part // WithToolResponses — synthetic output (denied/already-run)
}

func (d *resumeDirectives) empty() bool {
	return d == nil || (len(d.restarts) == 0 && len(d.responds) == 0)
}

// isApprovalResume reports whether the conversation is resuming after a tool
// approval: the last message is an assistant turn whose tool parts carry user
// approval decisions.
func isApprovalResume(msgs []UIMessage) bool {
	if len(msgs) == 0 {
		return false
	}
	last := msgs[len(msgs)-1]
	if last.Role != "assistant" {
		return false
	}
	for _, p := range last.Parts {
		if p.isTool() && p.pendingApproval() {
			return true
		}
	}
	return false
}

// toGenkitMessages converts the UIMessage history into Genkit messages and, when
// the last turn is a tool-approval resume, the directives that resolve the
// interrupted tool requests. In resume mode the last assistant message is left
// as a model message with dangling tool requests (no tool-response message) so
// Genkit's resume path can match the Restart/Respond directives by ref.
func toGenkitMessages(msgs []UIMessage) ([]*ai.Message, *resumeDirectives, error) {
	resume := isApprovalResume(msgs)
	dirs := &resumeDirectives{}
	var out []*ai.Message
	for i, m := range msgs {
		last := i == len(msgs)-1
		switch m.Role {
		case "user":
			if parts := userParts(m); len(parts) > 0 {
				out = append(out, ai.NewUserMessage(parts...))
			}
		case "system":
			if t := textOf(m); t != "" {
				out = append(out, ai.NewSystemTextMessage(t))
			}
		case "assistant":
			model, toolResp := assistantParts(m, resume && last, dirs)
			if len(model) > 0 {
				out = append(out, ai.NewModelMessage(model...))
			}
			if len(toolResp) > 0 {
				out = append(out, ai.NewMessage(ai.RoleTool, nil, toolResp...))
			}
		default:
			return nil, nil, fmt.Errorf("unknown message role %q", m.Role)
		}
	}
	if len(out) == 0 {
		return nil, nil, fmt.Errorf("no messages with content")
	}
	if !resume {
		dirs = nil
	}
	return out, dirs, nil
}

// userParts converts a user message into text and media (attachment) parts.
func userParts(m UIMessage) []*ai.Part {
	var parts []*ai.Part
	for _, p := range m.Parts {
		switch {
		case p.Type == "text" && p.Text != "":
			parts = append(parts, ai.NewTextPart(p.Text))
		case p.Type == "file" && p.URL != "":
			parts = append(parts, ai.NewMediaPart(p.MediaType, p.URL))
		}
	}
	return parts
}

// assistantParts converts an assistant message into model-message parts (text +
// tool requests) and, for resolved tool calls, the tool-response parts that
// belong in a following tool-role message. When resuming, no tool-response
// parts are emitted; instead the pending tool requests are resolved into the
// supplied resumeDirectives.
func assistantParts(m UIMessage, resuming bool, dirs *resumeDirectives) (model, toolResp []*ai.Part) {
	for _, p := range m.Parts {
		switch {
		case p.Type == "text" && p.Text != "":
			model = append(model, ai.NewTextPart(p.Text))
		case p.isTool():
			name := p.toolName()
			if name == "" || p.ToolCallID == "" {
				// Older/partial UI message streams can contain malformed tool parts
				// (for example a `tool-` part without a name). Provider APIs reject
				// empty tool names, so drop the bad part rather than poisoning the next
				// chat turn.
				continue
			}
			req := &ai.ToolRequest{Name: name, Ref: p.ToolCallID, Input: decodeJSON(p.Input)}
			model = append(model, ai.NewToolRequestPart(req))
			if resuming {
				addResumeDirective(p, dirs)
			} else if p.State == "output-available" && len(p.Output) > 0 {
				toolResp = append(toolResp, ai.NewToolResponsePart(&ai.ToolResponse{
					Name: name, Ref: p.ToolCallID, Output: decodeJSON(p.Output),
				}))
			}
		}
	}
	return model, toolResp
}

// addResumeDirective resolves one tool part of a resumed assistant turn: an
// approved tool restarts (re-executes), a denied tool gets a synthetic "denied"
// response, and an already-resolved tool replays its stored output.
func addResumeDirective(p UIPart, dirs *resumeDirectives) {
	switch {
	case p.State == "approval-responded" && p.Approval != nil && p.Approval.Approved != nil && *p.Approval.Approved:
		restart := ai.NewToolRequestPart(&ai.ToolRequest{
			Name: p.toolName(), Ref: p.ToolCallID, Input: decodeJSON(p.Input),
		})
		restart.Metadata = map[string]any{"resumed": map[string]any{}}
		dirs.restarts = append(dirs.restarts, restart)
	case p.State == "approval-responded" && p.Approval != nil:
		reason := p.Approval.Reason
		if reason == "" {
			reason = "denied by user"
		}
		dirs.responds = append(dirs.responds, ai.NewToolResponsePart(&ai.ToolResponse{
			Name: p.toolName(), Ref: p.ToolCallID, Output: map[string]any{"denied": true, "reason": reason},
		}))
	case p.State == "output-available" && len(p.Output) > 0:
		dirs.responds = append(dirs.responds, ai.NewToolResponsePart(&ai.ToolResponse{
			Name: p.toolName(), Ref: p.ToolCallID, Output: decodeJSON(p.Output),
		}))
	}
}

func textOf(m UIMessage) string {
	var s string
	for _, p := range m.Parts {
		if p.Type == "text" {
			s += p.Text
		}
	}
	return s
}
