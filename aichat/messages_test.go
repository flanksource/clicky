package aichat

import (
	"encoding/json"
	"testing"
)

func raw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func boolp(b bool) *bool { return &b }

func TestToGenkitMessagesTextOnly(t *testing.T) {
	msgs := []UIMessage{
		{Role: "user", Parts: []UIPart{{Type: "text", Text: "hello"}}},
	}
	out, resume, err := toGenkitMessages(msgs)
	if err != nil {
		t.Fatalf("toGenkitMessages: %v", err)
	}
	if resume != nil {
		t.Errorf("resume = %v, want nil for a plain user turn", resume)
	}
	if len(out) != 1 || out[0].Role != "user" {
		t.Fatalf("messages = %+v, want one user message", out)
	}
	if !out[0].Content[0].IsText() || out[0].Content[0].Text != "hello" {
		t.Errorf("content = %+v, want text 'hello'", out[0].Content[0])
	}
}

func TestToGenkitMessagesAttachmentBecomesMedia(t *testing.T) {
	msgs := []UIMessage{{Role: "user", Parts: []UIPart{
		{Type: "text", Text: "look"},
		{Type: "file", MediaType: "image/png", URL: "data:image/png;base64,AAAA", Filename: "a.png"},
	}}}
	out, _, err := toGenkitMessages(msgs)
	if err != nil {
		t.Fatalf("toGenkitMessages: %v", err)
	}
	if len(out[0].Content) != 2 {
		t.Fatalf("content parts = %d, want 2 (text + media)", len(out[0].Content))
	}
	media := out[0].Content[1]
	if !media.IsMedia() {
		t.Errorf("second part = %+v, want media", media)
	}
}

func TestToGenkitMessagesResolvedToolBecomesRequestAndResponse(t *testing.T) {
	msgs := []UIMessage{
		{Role: "user", Parts: []UIPart{{Type: "text", Text: "list stacks"}}},
		{Role: "assistant", Parts: []UIPart{
			{Type: "text", Text: "Listing."},
			{
				Type: "dynamic-tool", ToolName: "stack_list", ToolCallID: "call-1",
				State: "output-available", Input: raw(map[string]any{}), Output: raw([]string{"a", "b"}),
			},
		}},
	}
	out, _, err := toGenkitMessages(msgs)
	if err != nil {
		t.Fatalf("toGenkitMessages: %v", err)
	}
	// user, model(text+toolReq), tool(toolResp)
	if len(out) != 3 {
		t.Fatalf("messages = %d, want 3 (user, model, tool)", len(out))
	}
	model := out[1]
	if model.Role != "model" {
		t.Errorf("out[1].Role = %q, want model", model.Role)
	}
	var sawReq bool
	for _, p := range model.Content {
		if p.IsToolRequest() && p.ToolRequest.Name == "stack_list" && p.ToolRequest.Ref == "call-1" {
			sawReq = true
		}
	}
	if !sawReq {
		t.Errorf("model content %+v missing stack_list tool request", model.Content)
	}
	tool := out[2]
	if tool.Role != "tool" || !tool.Content[0].IsToolResponse() {
		t.Errorf("out[2] = %+v, want a tool response message", tool)
	}
}

func TestToGenkitMessagesApprovalResumeApprovedRestarts(t *testing.T) {
	msgs := []UIMessage{
		{Role: "user", Parts: []UIPart{{Type: "text", Text: "restart it"}}},
		{Role: "assistant", Parts: []UIPart{{
			Type: "dynamic-tool", ToolName: "stack_restart", ToolCallID: "call-9",
			State: "approval-responded", Input: raw(map[string]any{"id": "x"}),
			Approval: &Approval{ID: "call-9", Approved: boolp(true)},
		}}},
	}
	if !isApprovalResume(msgs) {
		t.Fatal("isApprovalResume = false, want true for approved tool part")
	}
	out, resume, err := toGenkitMessages(msgs)
	if err != nil {
		t.Fatalf("toGenkitMessages: %v", err)
	}
	// In resume mode the last message stays a model message with a dangling
	// tool request (no trailing tool-response message).
	if out[len(out)-1].Role != "model" {
		t.Errorf("last message role = %q, want model", out[len(out)-1].Role)
	}
	if resume == nil || len(resume.restarts) != 1 || len(resume.responds) != 0 {
		t.Fatalf("resume = %+v, want exactly one restart", resume)
	}
	if resume.restarts[0].ToolRequest.Ref != "call-9" {
		t.Errorf("restart ref = %q, want call-9", resume.restarts[0].ToolRequest.Ref)
	}
	if _, ok := resume.restarts[0].Metadata["resumed"]; !ok {
		t.Errorf("restart part missing 'resumed' metadata: %+v", resume.restarts[0].Metadata)
	}
}

func TestToGenkitMessagesApprovalResumeDeniedResponds(t *testing.T) {
	msgs := []UIMessage{
		{Role: "user", Parts: []UIPart{{Type: "text", Text: "delete it"}}},
		{Role: "assistant", Parts: []UIPart{{
			Type: "dynamic-tool", ToolName: "stack_delete", ToolCallID: "call-7",
			State: "approval-responded", Input: raw(map[string]any{"id": "x"}),
			Approval: &Approval{ID: "call-7", Approved: boolp(false), Reason: "too risky"},
		}}},
	}
	_, resume, err := toGenkitMessages(msgs)
	if err != nil {
		t.Fatalf("toGenkitMessages: %v", err)
	}
	if resume == nil || len(resume.responds) != 1 || len(resume.restarts) != 0 {
		t.Fatalf("resume = %+v, want exactly one respond", resume)
	}
	if resume.responds[0].ToolResponse.Ref != "call-7" {
		t.Errorf("respond ref = %q, want call-7", resume.responds[0].ToolResponse.Ref)
	}
}

func TestTypedToolPartNameFallsBackToTypeSuffix(t *testing.T) {
	p := UIPart{Type: "tool-stack_get", ToolCallID: "c1"}
	if p.toolName() != "stack_get" {
		t.Errorf("toolName = %q, want stack_get", p.toolName())
	}
	if !p.isTool() {
		t.Error("isTool = false for tool- prefixed part")
	}
}
