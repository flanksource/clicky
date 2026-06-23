package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/spf13/cobra"
)

// Options configures a chat Server.
type Options struct {
	// RootCmd is the Cobra command tree whose operations become AI tools
	// (executed in-process via clicky's RPC executor). Optional.
	RootCmd *cobra.Command
	// MCPServers are external MCP servers consumed as additional tools. Optional.
	MCPServers []MCPServer
	// System is the agent's system prompt. A default is used when empty.
	System string
	// Threads persists conversations for the thread endpoints. When nil, those
	// endpoints report 501 and /api/chat stays stateless. Optional.
	Threads ThreadStore
	// RequireApprovalFor lists tool names that must be approved by the user
	// before they execute (human-in-the-loop). Tools not listed run
	// automatically. Ignored when ApprovalPolicy is set. Optional.
	RequireApprovalFor []string
	// ApprovalPolicy gates tool execution behind user approval with arbitrary
	// logic (e.g. by name prefix or input). Takes precedence over
	// RequireApprovalFor. Optional.
	ApprovalPolicy ApprovalPolicy
	// ToolApprovalPolicy is the metadata-aware approval hook. It takes
	// precedence over ApprovalPolicy and RequireApprovalFor. Optional.
	ToolApprovalPolicy ToolApprovalPolicy
	// CustomTools are app-owned Genkit tools registered alongside clicky RPC and
	// MCP tools. They are useful for structured client-action outputs that are
	// not Cobra commands.
	CustomTools []ToolDefinition
	// SettingsProvider supplies per-request defaults and limits owned by the
	// embedding application. It is evaluated after the request is decoded but
	// before model selection/generation, so it can default the model and reject
	// turns that exceed local budgets.
	SettingsProvider RuntimeSettingsProvider
	// ProviderCredentials supplies per-request upstream provider keys. When set,
	// the server builds a request-local Genkit runtime from those credentials
	// plus any process env keys, so org-scoped connection stores can drive model
	// availability and generation.
	ProviderCredentials ProviderCredentialsProvider
}

const defaultSystem = "You are an operator assistant for this application. " +
	"Use the available tools to answer questions and perform actions on the user's behalf. " +
	"Prefer calling a tool over guessing. Summarize tool results clearly."

// Server is an AI-SDK-compatible chat backend. It serves POST /api/chat as the
// v6 UI Message Stream protocol, backed by Genkit + clicky operations + MCP.
type Server struct {
	g         *genkit.Genkit
	tools     []registeredTool
	providers []Provider
	system    string
	initErr   error
	once      sync.Once
	opts      Options
	approval  approvalPredicate
}

type chatRuntime struct {
	g         *genkit.Genkit
	providers []Provider
	tools     []registeredTool
}

// NewServer builds a chat server. Genkit init and tool discovery happen lazily
// on the first request so construction never fails on missing API keys.
func NewServer(opts Options) *Server {
	system := opts.System
	if system == "" {
		system = defaultSystem
	}
	return &Server{
		system:   system,
		opts:     opts,
		approval: resolveApprovalPolicy(opts.ToolApprovalPolicy, opts.ApprovalPolicy, opts.RequireApprovalFor),
	}
}

// Handler returns the http.Handler serving the chat API: POST /api/chat (the
// streaming turn), GET /api/chat/models (the model menu), and the thread
// endpoints when a ThreadStore is configured. Mount it as a subtree
// (e.g. mux.Handle("/api/chat/", srv.Handler())) so the nested routes resolve.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/chat", s.handleChat)
	mux.HandleFunc("GET /api/chat/models", s.handleModels)
	s.registerThreadRoutes(mux)
	return mux
}

// handleModels serves the model menu annotated with provider availability so a
// client model selector can disable models whose provider is not configured.
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	providers, err := s.availableProviders(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(CatalogInfo(providers)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) availableProviders(ctx context.Context) ([]Provider, error) {
	if s.opts.ProviderCredentials != nil {
		creds, err := s.opts.ProviderCredentials(ctx)
		if err != nil {
			return nil, err
		}
		return configuredProviders(creds), nil
	}
	if err := s.ensureInit(ctx); err != nil {
		return nil, err
	}
	return s.providers, nil
}

func (s *Server) ensureInit(ctx context.Context) error {
	s.once.Do(func() {
		g, providers, err := initGenkit(ctx)
		if err != nil {
			s.initErr = err
			return
		}
		s.g = g
		s.providers = providers
		tools, err := s.buildTools(ctx, g)
		if err != nil {
			s.initErr = err
			return
		}
		s.tools = tools
	})
	return s.initErr
}

func (s *Server) runtime(ctx context.Context) (chatRuntime, error) {
	if s.opts.ProviderCredentials == nil {
		if err := s.ensureInit(ctx); err != nil {
			return chatRuntime{}, err
		}
		return chatRuntime{g: s.g, providers: s.providers, tools: s.tools}, nil
	}
	creds, err := s.opts.ProviderCredentials(ctx)
	if err != nil {
		return chatRuntime{}, err
	}
	g, providers, err := initGenkit(ctx, creds...)
	if err != nil {
		return chatRuntime{}, err
	}
	tools, err := s.buildTools(ctx, g)
	if err != nil {
		return chatRuntime{}, err
	}
	return chatRuntime{g: g, providers: providers, tools: tools}, nil
}

func (s *Server) buildTools(ctx context.Context, g *genkit.Genkit) ([]registeredTool, error) {
	var tools []registeredTool
	if s.opts.RootCmd != nil {
		ts, err := NewClickyToolset(s.opts.RootCmd)
		if err != nil {
			return nil, err
		}
		ts.requireApproval = s.approval
		tools = append(tools, ts.DefineRegisteredTools(g)...)
	}
	customTools, err := DefineCustomTools(g, s.opts.CustomTools)
	if err != nil {
		return nil, err
	}
	tools = append(tools, customTools...)
	mcpTools, err := MCPTools(ctx, g, s.opts.MCPServers)
	if err != nil {
		return nil, err
	}
	for _, tool := range mcpTools {
		tools = append(tools, registeredTool{
			ref:  tool,
			info: ToolInfo{Name: tool.Name()},
		})
	}
	return tools, nil
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	settings, err := s.runtimeSettings(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Enforce local runtime limits (budget/size → 402/413) before initializing the
	// provider runtime, so a deterministic local rejection isn't masked by a 503
	// from runtime setup.
	if err := enforceRuntimeSettings(req, settings); err != nil {
		http.Error(w, err.Error(), statusForRuntimeSettingsError(err))
		return
	}
	rt, err := s.runtime(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	model, err := s.resolveModel(modelIDForRequest(req, settings), rt.providers)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := ValidateEffort(req.ReasoningEffort); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	msgs, resume, err := toGenkitMessages(req.Messages)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	msgs = contextualGenkitMessages(req, msgs)

	if err := s.persistIncoming(ctx, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sse, err := newSSEWriter(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.stream(ctx, sse, rt, model, req.ReasoningEffort, msgs, resume, req.ThreadID, req.ToolPreferences); err != nil {
		// Headers are already sent; surface the error as an SSE error part.
		_ = sse.errorPart(err.Error())
	}
	_ = sse.done()
}

// persistIncoming appends the latest user message to its thread when the request
// carries a thread id and a store is configured. The assistant reply is not
// persisted here; thread history is intended for resuming context, and the
// client owns re-sending the full transcript.
func (s *Server) persistIncoming(ctx context.Context, req ChatRequest) error {
	if req.ThreadID == "" || s.opts.Threads == nil || len(req.Messages) == 0 {
		return nil
	}
	last := req.Messages[len(req.Messages)-1]
	if last.Role != "user" {
		return nil
	}
	return s.opts.Threads.AppendMessage(ctx, req.ThreadID, last)
}

func (s *Server) runtimeSettings(ctx context.Context) (RuntimeSettings, error) {
	if s.opts.SettingsProvider == nil {
		return RuntimeSettings{}, nil
	}
	return s.opts.SettingsProvider(ctx)
}

// resolveModel selects the model for a request. An empty id uses the
// provider-aware default; a named id must be in the catalog AND have its
// provider configured — both failure modes are reported loudly so a request for
// an unconfigured provider does not surface as an opaque generation error.
func (s *Server) resolveModel(id string, providers []Provider) (Model, error) {
	if id == "" {
		m, ok := defaultModel(providers)
		if !ok {
			return Model{}, fmt.Errorf("no catalog model available for configured providers %v", providers)
		}
		return m, nil
	}
	m, err := LookupModel(id)
	if err != nil {
		return Model{}, err
	}
	if !providerRegistered(providers, m.Provider) {
		return Model{}, fmt.Errorf("model %q requires the %q provider, which is not configured (set its API key)", id, m.Provider)
	}
	return m, nil
}

// stream runs one chat turn, translating Genkit stream chunks into v6 SSE parts.
// Genkit auto-executes the tool loop; the callback observes text, tool requests
// and tool responses as they flow across steps. When a gated tool interrupts
// for approval, Generate returns with pending interrupts (no error); each is
// surfaced as a tool-approval-request so the client can approve/deny and resume.
// When resume directives are supplied, the interrupted calls are restarted
// (approved) or responded-to (denied) instead of re-prompting.
func (s *Server) stream(ctx context.Context, sse *sseWriter, rt chatRuntime, model Model, effort Effort, msgs []*ai.Message, resume *resumeDirectives, threadID string, toolPrefs ToolPreferences) error {
	if err := sse.start(); err != nil {
		return err
	}
	if err := sse.startStep(); err != nil {
		return err
	}

	em := &streamEmitter{sse: sse}
	cb := func(_ context.Context, chunk *ai.ModelResponseChunk) error {
		return em.onChunk(chunk)
	}

	ctx = withToolRuntime(ctx, toolRuntimeConfig{
		preferences:     toolPrefs,
		defaultApproval: s.approval,
	})
	opts := generateOptions(model, effort, s.system, msgs, toolsForRequest(rt.tools, toolPrefs), cb, resumeOptions(resume)...)
	resp, err := genkit.Generate(ctx, rt.g, opts...)
	if err != nil {
		return err
	}

	if err := em.closeText(); err != nil {
		return err
	}
	if err := s.emitInterrupts(sse, resp); err != nil {
		return err
	}
	if err := sse.finishStep(); err != nil {
		return err
	}
	return sse.finish(s.usageMetadata(ctx, model, resp, threadID))
}

// usageMetadata captures this turn's token usage + cost, accumulates it onto the
// thread (when persisted), and returns the AI SDK messageMetadata describing
// per-turn usage and the cumulative thread cost. Returns nil when the model
// reported no usage, so no metadata part is emitted.
func (s *Server) usageMetadata(ctx context.Context, model Model, resp *ai.ModelResponse, threadID string) map[string]any {
	if resp == nil || resp.Usage == nil {
		return nil
	}
	u := resp.Usage
	turnCost := costUSD(model.ID, u)
	turn := TurnUsage{InputTokens: u.InputTokens, OutputTokens: u.OutputTokens + u.ThoughtsTokens, CostUSD: turnCost}

	threadCost := turnCost
	if threadID != "" && s.opts.Threads != nil {
		// Accumulation is best-effort: a persistence failure must not abort an
		// otherwise-complete turn, so fall back to this turn's cost.
		if t, err := s.opts.Threads.AddUsage(ctx, threadID, turn); err == nil {
			threadCost = t.TotalCostUsd
		}
	}

	return map[string]any{
		"usage": map[string]any{
			"inputTokens":  u.InputTokens,
			"outputTokens": turn.OutputTokens,
			"totalTokens":  u.TotalTokens,
		},
		"cost":          turnCost,
		"threadCostUsd": threadCost,
		"contextTokens": u.InputTokens,
	}
}

// emitInterrupts surfaces any pending tool-approval interrupts as
// tool-approval-request parts. The tool-input-available part was already emitted
// while the model streamed the tool request; the approval id reuses the tool
// call ref so the client's approval response matches it back by toolCallId.
func (s *Server) emitInterrupts(sse *sseWriter, resp *ai.ModelResponse) error {
	if resp == nil {
		return nil
	}
	for _, p := range resp.Interrupts() {
		if p.ToolRequest == nil {
			continue
		}
		ref := p.ToolRequest.Ref
		if err := sse.toolApprovalRequest(ref, ref); err != nil {
			return err
		}
	}
	return nil
}

// streamEmitter tracks per-turn streaming state: the open text block id and
// seen tool calls, so it emits correctly ordered text-start/-delta/-end and
// tool-input/-output parts.
type streamEmitter struct {
	sse       *sseWriter
	textID    string
	textOpen  bool
	textBlock int
}

func (e *streamEmitter) onChunk(chunk *ai.ModelResponseChunk) error {
	for _, p := range chunk.Content {
		switch {
		case p.IsText() && p.Text != "":
			if err := e.openText(); err != nil {
				return err
			}
			if err := e.sse.textDelta(e.textID, p.Text); err != nil {
				return err
			}
		case p.IsToolRequest() && p.ToolRequest != nil && !p.ToolRequest.Partial:
			if err := e.closeText(); err != nil {
				return err
			}
			tr := p.ToolRequest
			if err := e.sse.toolInputAvailable(tr.Ref, tr.Name, tr.Input); err != nil {
				return err
			}
		case p.IsToolResponse() && p.ToolResponse != nil:
			resp := p.ToolResponse
			if err := e.sse.toolOutputAvailable(resp.Ref, resp.Output); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *streamEmitter) openText() error {
	if e.textOpen {
		return nil
	}
	e.textID = fmt.Sprintf("text-%d", e.textBlock)
	e.textBlock++
	e.textOpen = true
	return e.sse.textStart(e.textID)
}

func (e *streamEmitter) closeText() error {
	if !e.textOpen {
		return nil
	}
	e.textOpen = false
	return e.sse.textEnd(e.textID)
}
