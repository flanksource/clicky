package aichat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	capapi "github.com/flanksource/captain/pkg/api"
)

// fakeStreamProvider is a captain StreamingProvider test double: it replays a
// scripted event slice and records the request it received. It also implements
// io.Closer so the provider-pool tests can observe teardown.
type fakeStreamProvider struct {
	model     string
	backend   capapi.Backend
	events    []capapi.Event
	streamErr error

	mu     sync.Mutex
	closed bool
}

func (f *fakeStreamProvider) GetModel() string           { return f.model }
func (f *fakeStreamProvider) GetBackend() capapi.Backend { return f.backend }
func (f *fakeStreamProvider) Execute(context.Context, capapi.Spec) (*capapi.Response, error) {
	return nil, fmt.Errorf("fakeStreamProvider.Execute not used")
}

func (f *fakeStreamProvider) ExecuteStream(_ context.Context, _ capapi.Spec) (<-chan capapi.Event, error) {
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	ch := make(chan capapi.Event, len(f.events))
	for _, ev := range f.events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func (f *fakeStreamProvider) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	return nil
}

// userMsg builds a one-text-part user UIMessage.
func userMsg(text string) UIMessage {
	return UIMessage{Role: "user", Parts: []UIPart{{Type: "text", Text: text}}}
}

// postChat drives one POST /api/chat turn through the full handler (routing
// included) and returns the recorder.
func postChat(t *testing.T, s *Server, req ChatRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

// parseSSEParts extracts the JSON part objects from a v6 SSE body, skipping the
// [DONE] terminator.
func parseSSEParts(t *testing.T, body string) []map[string]any {
	t.Helper()
	var parts []map[string]any
	for _, line := range strings.Split(body, "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		if data == "[DONE]" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			t.Fatalf("bad SSE data %q: %v", data, err)
		}
		parts = append(parts, m)
	}
	return parts
}

func partTypes(parts []map[string]any) []string {
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i], _ = p["type"].(string)
	}
	return out
}

func firstPartOfType(parts []map[string]any, typ string) map[string]any {
	for _, p := range parts {
		if p["type"] == typ {
			return p
		}
	}
	return nil
}

func containsType(parts []map[string]any, typ string) bool {
	return firstPartOfType(parts, typ) != nil
}

// agentServer builds a server whose agent engine uses the supplied fake,
// recording every Config the factory is asked to build.
func agentServer(opts Options, fake *fakeStreamProvider, configs *[]capapi.Config) *Server {
	// Register a stable test agent model so LookupModel routes these requests to
	// the agent path regardless of the concrete ids captain's catalog ships. The
	// upsert is idempotent and process-global; a fake provider factory serves the
	// turn, so no real backend is contacted.
	_ = RegisterModel(Model{ID: "claude-agent-sonnet", Backend: capapi.BackendClaudeAgent, Label: "Claude Agent · Sonnet (test)", Reasoning: true, ContextWindow: 200000})
	opts.AgentProviderFactory = func(cfg capapi.Config) (capapi.StreamingProvider, error) {
		if configs != nil {
			*configs = append(*configs, cfg)
		}
		return fake, nil
	}
	return NewServer(opts)
}

func TestAgentChatStreamsEventsAsSSE(t *testing.T) {
	fake := &fakeStreamProvider{
		model:   "claude-agent-sonnet",
		backend: capapi.BackendClaudeAgent,
		events: []capapi.Event{
			{Kind: capapi.EventSystem, SessionID: "sess-1"},
			{Kind: capapi.EventText, Text: "Hello "},
			{Kind: capapi.EventText, Text: "world"},
			{Kind: capapi.EventThinking, Text: "let me think"},
			{Kind: capapi.EventToolUse, ToolCallID: "call-1", Tool: "Read", Input: map[string]any{"file": "main.go"}},
			{Kind: capapi.EventToolResult, ToolCallID: "call-1", Text: "package main", Success: true},
			{Kind: capapi.EventResult, SessionID: "sess-1", CostUSD: 0.0123,
				Usage: &capapi.Usage{InputTokens: 100, OutputTokens: 40, ReasoningTokens: 10}},
		},
	}
	var configs []capapi.Config
	s := agentServer(Options{System: "be helpful"}, fake, &configs)
	defer s.Close()

	w := postChat(t, s, ChatRequest{Model: "claude-agent-sonnet", Messages: []UIMessage{userMsg("hi")}})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	parts := parseSSEParts(t, w.Body.String())

	for _, want := range []string{"start", "start-step", "text-start", "text-delta", "text-end",
		"reasoning-start", "reasoning-delta", "reasoning-end", "tool-input-available",
		"tool-output-available", "finish-step", "finish"} {
		if !containsType(parts, want) {
			t.Errorf("missing SSE part %q; got %v", want, partTypes(parts))
		}
	}

	// Text deltas reproduce the streamed text in order.
	var text strings.Builder
	for _, p := range parts {
		if p["type"] == "text-delta" {
			text.WriteString(p["delta"].(string))
		}
	}
	if got := text.String(); got != "Hello world" {
		t.Errorf("text = %q, want %q", got, "Hello world")
	}

	tool := firstPartOfType(parts, "tool-input-available")
	if tool["toolName"] != "Read" {
		t.Errorf("tool name = %v, want Read", tool["toolName"])
	}
	if tool["toolCallId"] != "call-1" {
		t.Errorf("tool call id = %v, want call-1 (the backend id, not synthetic)", tool["toolCallId"])
	}

	out := firstPartOfType(parts, "tool-output-available")
	if out["toolCallId"] != "call-1" {
		t.Errorf("tool-output call id = %v, want call-1 (correlated to its call)", out["toolCallId"])
	}
	if output, _ := out["output"].(map[string]any); output["output"] != "package main" {
		t.Errorf("tool output = %v, want real result 'package main'", out["output"])
	}

	// Backend is selected explicitly from the catalog (not inferred from the id).
	if len(configs) != 1 || configs[0].Model.Backend != capapi.BackendClaudeAgent {
		t.Fatalf("provider configs = %+v, want one claude-agent config", configs)
	}

	finish := firstPartOfType(parts, "finish")
	meta, ok := finish["messageMetadata"].(map[string]any)
	if !ok {
		t.Fatalf("finish has no messageMetadata: %v", finish)
	}
	if meta["providerSessionId"] != "sess-1" {
		t.Errorf("providerSessionId = %v, want sess-1", meta["providerSessionId"])
	}
	usage := meta["usage"].(map[string]any)
	if usage["inputTokens"].(float64) != 100 {
		t.Errorf("inputTokens = %v, want 100", usage["inputTokens"])
	}
	if usage["outputTokens"].(float64) != 40 {
		t.Errorf("outputTokens = %v, want 40", usage["outputTokens"])
	}
	if usage["reasoningTokens"].(float64) != 10 {
		t.Errorf("reasoningTokens = %v, want 10", usage["reasoningTokens"])
	}
	if usage["totalTokens"].(float64) != 150 {
		t.Errorf("totalTokens = %v, want 150", usage["totalTokens"])
	}
	if meta["cost"].(float64) != 0.0123 {
		t.Errorf("cost = %v, want 0.0123", meta["cost"])
	}
}

func TestAgentToolResultErrorIsFlagged(t *testing.T) {
	fake := &fakeStreamProvider{
		model:   "claude-agent-sonnet",
		backend: capapi.BackendClaudeAgent,
		events: []capapi.Event{
			{Kind: capapi.EventToolUse, ToolCallID: "c9", Tool: "Bash", Input: map[string]any{"command": "false"}},
			{Kind: capapi.EventToolResult, ToolCallID: "c9", Text: "permission denied", Success: false},
			{Kind: capapi.EventResult, Usage: &capapi.Usage{InputTokens: 1, OutputTokens: 1}},
		},
	}
	s := agentServer(Options{}, fake, nil)
	defer s.Close()

	w := postChat(t, s, ChatRequest{Model: "claude-agent-sonnet", Messages: []UIMessage{userMsg("run it")}})
	out := firstPartOfType(parseSSEParts(t, w.Body.String()), "tool-output-available")
	if out == nil || out["toolCallId"] != "c9" {
		t.Fatalf("missing correlated tool output; got %v", out)
	}
	output := out["output"].(map[string]any)
	if output["output"] != "permission denied" || output["isError"] != true {
		t.Errorf("errored output = %v, want text + isError true", output)
	}
}

func TestAgentDanglingToolCallIsClosed(t *testing.T) {
	// A tool call with no matching result must still get a synthetic output so
	// the UI card does not hang pending.
	fake := &fakeStreamProvider{
		model:   "claude-agent-sonnet",
		backend: capapi.BackendClaudeAgent,
		events: []capapi.Event{
			{Kind: capapi.EventToolUse, ToolCallID: "cX", Tool: "Read", Input: map[string]any{"file": "x"}},
			{Kind: capapi.EventResult, Usage: &capapi.Usage{InputTokens: 1, OutputTokens: 1}},
		},
	}
	s := agentServer(Options{}, fake, nil)
	defer s.Close()

	w := postChat(t, s, ChatRequest{Model: "claude-agent-sonnet", Messages: []UIMessage{userMsg("read x")}})
	out := firstPartOfType(parseSSEParts(t, w.Body.String()), "tool-output-available")
	if out == nil || out["toolCallId"] != "cX" {
		t.Fatalf("dangling tool call was not closed; got %v", out)
	}
	if output := out["output"].(map[string]any); output["status"] != "executed" {
		t.Errorf("dangling output = %v, want synthetic status executed", output)
	}
}

func TestAgentChatSurfacesEventErrorAsSSE(t *testing.T) {
	fake := &fakeStreamProvider{
		model:   "claude-agent-sonnet",
		backend: capapi.BackendClaudeAgent,
		events: []capapi.Event{
			{Kind: capapi.EventText, Text: "partial"},
			{Kind: capapi.EventError, Error: "model overloaded"},
		},
	}
	s := agentServer(Options{}, fake, nil)
	defer s.Close()

	w := postChat(t, s, ChatRequest{Model: "claude-agent-sonnet", Messages: []UIMessage{userMsg("hi")}})
	parts := parseSSEParts(t, w.Body.String())

	errPart := firstPartOfType(parts, "error")
	if errPart == nil {
		t.Fatalf("expected an error part; got %v", partTypes(parts))
	}
	if !strings.Contains(errPart["errorText"].(string), "model overloaded") {
		t.Errorf("errorText = %v, want it to contain 'model overloaded'", errPart["errorText"])
	}
	// The open text block is closed before the error so the stream stays well-formed.
	if !containsType(parts, "text-end") {
		t.Errorf("expected text-end before error; got %v", partTypes(parts))
	}
}

func TestAgentSessionResumeReusesProvider(t *testing.T) {
	resultEvents := []capapi.Event{
		{Kind: capapi.EventSystem, SessionID: "sess-9"},
		{Kind: capapi.EventText, Text: "ok"},
		{Kind: capapi.EventResult, SessionID: "sess-9",
			Usage: &capapi.Usage{InputTokens: 5, OutputTokens: 2}},
	}
	fake := &fakeStreamProvider{model: "claude-agent-sonnet", backend: capapi.BackendClaudeAgent, events: resultEvents}

	var factoryCalls int
	s := NewServer(Options{
		AgentProviderFactory: func(cfg capapi.Config) (capapi.StreamingProvider, error) {
			factoryCalls++
			return fake, nil
		},
	})
	defer s.Close()

	// First (stateless) turn mints a session and returns it in finish metadata.
	w1 := postChat(t, s, ChatRequest{Model: "claude-agent-sonnet", Messages: []UIMessage{userMsg("first")}})
	meta1 := firstPartOfType(parseSSEParts(t, w1.Body.String()), "finish")["messageMetadata"].(map[string]any)
	sid := meta1["providerSessionId"].(string)
	if sid != "sess-9" {
		t.Fatalf("first turn session id = %q, want sess-9", sid)
	}

	// Second turn echoes the session id and must reuse the same live provider.
	w2 := postChat(t, s, ChatRequest{Model: "claude-agent-sonnet", ProviderSessionID: sid, Messages: []UIMessage{userMsg("second")}})
	if w2.Code != http.StatusOK {
		t.Fatalf("second turn status = %d", w2.Code)
	}
	if factoryCalls != 1 {
		t.Errorf("factory called %d times, want 1 (provider reused across turns)", factoryCalls)
	}
}

func TestAgentRequestAppliesSafeDefault(t *testing.T) {
	s := NewServer(Options{System: "sys"})
	defer s.Close()

	req := ChatRequest{
		Messages: []UIMessage{userMsg("ignored"), {Role: "assistant", Parts: []UIPart{{Type: "text", Text: "reply"}}}, userMsg("the real prompt")},
		Context:  "editor shows foo.go",
	}
	air := s.agentRequest(req, "resume-123")

	if !strings.Contains(air.Prompt.User, "the real prompt") {
		t.Errorf("prompt = %q, want it to include the latest user message", air.Prompt.User)
	}
	if !strings.Contains(air.Prompt.User, "editor shows foo.go") {
		t.Errorf("prompt = %q, want it to include the UI context", air.Prompt.User)
	}
	if air.Prompt.System != "sys" {
		t.Errorf("system = %q, want sys", air.Prompt.System)
	}
	if air.SessionID != "resume-123" {
		t.Errorf("sessionID = %q, want resume-123", air.SessionID)
	}
	if air.Permissions.HasPreset(capapi.PresetEdit) {
		t.Errorf("Edit should default false")
	}
	if strings.Join(air.Permissions.Tools.Allow, ",") != strings.Join(defaultAgentReadOnlyTools, ",") {
		t.Errorf("AllowedTools = %v, want read-only default %v", air.Permissions.Tools.Allow, defaultAgentReadOnlyTools)
	}
}

func TestAgentRequestEditOptInSkipsReadOnlyDefault(t *testing.T) {
	s := NewServer(Options{Agent: AgentOptions{Edit: true, Cwd: "/repo"}})
	defer s.Close()

	air := s.agentRequest(ChatRequest{Messages: []UIMessage{userMsg("go")}}, "")
	if !air.Permissions.HasPreset(capapi.PresetEdit) {
		t.Errorf("Edit should be true")
	}
	if air.Cwd() != "/repo" {
		t.Errorf("Cwd = %q, want /repo", air.Cwd())
	}
	if len(air.Permissions.Tools.Allow) != 0 {
		t.Errorf("AllowedTools = %v, want empty (edit opt-in lets the backend curate)", air.Permissions.Tools.Allow)
	}
}

func TestAgentRequestCarriesBudgetAndTemperature(t *testing.T) {
	temp := 0.2
	s := NewServer(Options{})
	defer s.Close()

	air := s.agentRequest(ChatRequest{
		Messages:    []UIMessage{userMsg("go")},
		Temperature: &temp,
		Budget:      ChatBudget{Cost: 1.5, MaxTokens: 2048},
	}, "")
	if air.Model.Temperature == nil || *air.Model.Temperature != temp {
		t.Fatalf("temperature = %v, want %v", air.Model.Temperature, temp)
	}
	if air.Budget.Cost != 1.5 || air.Budget.MaxTokens != 2048 {
		t.Fatalf("budget = %+v, want cost 1.5 max 2048", air.Budget)
	}
}

// TestAgentModelsInCatalog verifies the captain-owned catalog (consumed via the
// aichat aliases) carries both agent and Genkit models, that IsAgent() tracks the
// backend kind, and that every id round-trips through LookupModel with the slug
// the captain backend receives.
func TestAgentModelsInCatalog(t *testing.T) {
	var sawAgent, sawGenkit bool
	for _, model := range Catalog() {
		got, err := LookupModel(model.ID)
		if err != nil {
			t.Fatalf("LookupModel(%q): %v", model.ID, err)
		}
		if got.IsAgent() != (model.Backend.Kind() == "cli") {
			t.Errorf("%s IsAgent = %v, but backend %q kind = %q", model.ID, got.IsAgent(), model.Backend, model.Backend.Kind())
		}
		if model.IsAgent() {
			sawAgent = true
			if want := firstNonEmptyString(model.AgentModel, model.ID); captainModel(model) != want {
				t.Errorf("%s captainModel = %q, want %q", model.ID, captainModel(model), want)
			}
		} else {
			sawGenkit = true
		}
	}
	if !sawAgent || !sawGenkit {
		t.Fatalf("catalog should contain both agent and genkit models: agent=%v genkit=%v", sawAgent, sawGenkit)
	}
}
