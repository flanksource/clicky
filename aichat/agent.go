package aichat

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	_ "github.com/flanksource/captain/pkg/ai/provider" // registers the agent backends
	capapi "github.com/flanksource/captain/pkg/api"
	"github.com/flanksource/commons-db/shell"
)

// AgentOptions configures the captain agent engine (EngineAgent models). The
// zero value is a safe, read-mostly posture: agent backends run with only
// read-only tools until the embedder opts into edits.
type AgentOptions struct {
	// Cwd is the working directory for agent subprocesses (the repo/workspace the
	// agent operates on). Empty means the server process's cwd.
	Cwd string
	// PermissionMode maps to captain ai.Request.PermissionMode (e.g.
	// "acceptEdits", "bypassPermissions"). Empty keeps the safe default.
	PermissionMode string
	// AllowedTools restricts the agent's tools (claude-agent allowlist). When
	// empty with Edit false and PermissionMode unset, a read-only default is
	// applied.
	AllowedTools []string
	// Edit opts into edits: acceptEdits + a curated Read/Edit/Write/Glob/Grep
	// allowlist (captain ai.Request.Edit).
	Edit bool
	// MaxTurns caps agentic turns per request (0 = backend default).
	MaxTurns int
	// BudgetUSD caps spend per session (captain ai.Config.BudgetUSD; 0 = none).
	BudgetUSD float64
	// IdleTTL is how long an idle pooled provider is kept before eviction
	// (0 = defaultAgentIdleTTL).
	IdleTTL time.Duration
}

// defaultAgentReadOnlyTools is the safe-default allowlist applied when the
// embedder has not opted into edits — it keeps claude-agent from running with
// its permissive bypassPermissions default. codex ignores AllowedTools but is
// read-only by default, so this is a no-op there.
var defaultAgentReadOnlyTools = []string{"Read", "Glob", "Grep"}

// defaultAgentProviderFactory builds an agent provider from captain's registry
// and asserts it streams (every agent backend does).
func defaultAgentProviderFactory(cfg capapi.Config) (capapi.StreamingProvider, error) {
	prov, err := capapi.NewProvider(cfg)
	if err != nil {
		return nil, err
	}
	sp, ok := prov.(capapi.StreamingProvider)
	if !ok {
		return nil, fmt.Errorf("backend %q does not support streaming", cfg.Model.Backend)
	}
	return sp, nil
}

// captainModel returns the model slug passed to the captain backend: AgentModel
// when set, otherwise the menu ID.
func captainModel(m Model) string {
	if m.AgentModel != "" {
		return m.AgentModel
	}
	return m.ID
}

// latestUserText returns the text of the most recent user message — the prompt
// for an agent turn. The live agent session carries prior history, so only the
// new message is sent rather than the whole transcript.
func latestUserText(msgs []UIMessage) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return textOf(msgs[i])
		}
	}
	return ""
}

// agentSessionInfo resolves how a request maps onto a pooled session.
type agentSessionInfo struct {
	key      string // provider pool key
	resumeID string // captain session id to resume on a cold provider
	threaded bool   // request carries a ThreadID (session persisted server-side)
	pending  bool   // first stateless turn: key is temporary, rekey after the turn
}

func (s *Server) agentSession(ctx context.Context, req ChatRequest) agentSessionInfo {
	if req.ThreadID != "" {
		info := agentSessionInfo{key: "thread:" + req.ThreadID, threaded: true}
		if s.opts.Threads != nil {
			if t, err := s.opts.Threads.Get(ctx, req.ThreadID); err == nil {
				info.resumeID = t.ProviderSessionID
			}
		}
		return info
	}
	if req.ProviderSessionID != "" {
		return agentSessionInfo{key: "session:" + req.ProviderSessionID, resumeID: req.ProviderSessionID}
	}
	return agentSessionInfo{key: s.pool.pendingKey(), pending: true}
}

// agentRequest builds the captain request for one turn, applying the safe
// read-only default when the embedder has not opted into edits.
func (s *Server) agentRequest(req ChatRequest, resumeID string) capapi.Spec {
	prompt := latestUserText(req.Messages)
	if cp := contextPrompt(req); cp != "" {
		prompt = cp + "\n\n" + prompt
	}
	perms := capapi.Permissions{
		Mode:  capapi.PermissionMode(s.opts.Agent.PermissionMode),
		Tools: capapi.Tools{Allow: s.opts.Agent.AllowedTools},
	}
	if s.opts.Agent.Edit {
		perms.Presets = append(perms.Presets, capapi.PresetEdit)
	}
	if !s.opts.Agent.Edit && perms.Mode == "" && len(perms.Tools.Allow) == 0 {
		perms.Tools.Allow = defaultAgentReadOnlyTools
	}
	return capapi.Spec{
		Prompt:      capapi.Prompt{User: prompt, System: s.system},
		Model:       capapi.Model{Effort: capapi.Effort(req.ReasoningEffort), Temperature: req.Temperature},
		Budget:      capapi.Budget{Cost: req.Budget.Cost, MaxTokens: req.Budget.MaxTokens, MaxTurns: s.opts.Agent.MaxTurns},
		Permissions: perms,
		Setup:       &shell.Setup{Cwd: s.opts.Agent.Cwd},
		SessionID:   resumeID,
	}
}

// serveAgentChat is the HTTP entry point for an EngineAgent turn. It mirrors the
// Genkit handleChat tail: validate, persist the incoming message, then stream.
func (s *Server) serveAgentChat(w http.ResponseWriter, r *http.Request, model Model, req ChatRequest) {
	ctx := r.Context()
	if err := ValidateEffort(req.ReasoningEffort); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.persistIncoming(ctx, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse, err := newSSEWriter(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.streamAgent(ctx, sse, model, req); err != nil {
		_ = sse.errorPart(err.Error())
	}
	_ = sse.done()
}

// streamAgent runs one agent turn: acquire (or create) the session's provider,
// stream its events as SSE, and persist the captain session id for resumption.
func (s *Server) streamAgent(ctx context.Context, sse *sseWriter, model Model, req ChatRequest) error {
	// Cancel the provider's stream if we stop draining early (e.g. on an event
	// error), so its goroutine does not block on a full channel.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sess := s.agentSession(ctx, req)
	budgetCost := s.opts.Agent.BudgetUSD
	if req.Budget.Cost > 0 {
		budgetCost = req.Budget.Cost
	}
	entry, err := s.pool.acquire(sess.key, capapi.Config{
		Model:  capapi.Model{Name: captainModel(model), Backend: model.Backend},
		Budget: capapi.Budget{Cost: budgetCost, MaxTokens: req.Budget.MaxTokens},
	})
	if err != nil {
		return err
	}
	entry.turn.Lock()
	defer entry.turn.Unlock()

	resumeID := entry.sessionID
	if resumeID == "" {
		resumeID = sess.resumeID
	}
	// Reject a turn that carries only context (no new user message): contextPrompt
	// would otherwise make air.Prompt non-empty, sending the agent context with no
	// instruction. The contract is latest-user-message-only.
	if strings.TrimSpace(latestUserText(req.Messages)) == "" {
		return fmt.Errorf("no user message to send to the agent")
	}
	air := s.agentRequest(req, resumeID)

	if err := sse.start(); err != nil {
		return err
	}
	if err := sse.startStep(); err != nil {
		return err
	}

	ch, err := entry.provider.ExecuteStream(ctx, air)
	if err != nil {
		return err
	}
	res, drainErr := drainAgentEvents(sse, ch)

	sid := res.sessionID
	if sid == "" {
		sid = resumeID
	}
	s.pool.touch(entry)
	if res.sessionID != "" {
		entry.sessionID = res.sessionID
	}
	s.persistAgentSession(ctx, sess, req.ThreadID, sid)

	if drainErr != nil {
		return drainErr
	}
	if err := sse.finishStep(); err != nil {
		return err
	}
	return sse.finish(s.agentUsageMetadata(ctx, req, model, res, sid))
}

// persistAgentSession records the captain session id so a later turn resumes it:
// threaded turns persist to the store; a first stateless turn rekeys the live
// provider to the session id the client will echo next time.
func (s *Server) persistAgentSession(ctx context.Context, sess agentSessionInfo, threadID, sid string) {
	if sid == "" {
		return
	}
	if sess.threaded {
		if s.opts.Threads != nil {
			_ = s.opts.Threads.SetProviderSession(ctx, threadID, sid)
		}
		return
	}
	if sess.pending {
		s.pool.rekey(sess.key, "session:"+sid)
	}
}

// agentUsageMetadata builds the v6 finish messageMetadata for an agent turn:
// the captain session id (so the client can resume) plus per-turn usage/cost,
// accumulating onto the thread when persisted. Returns nil when there is nothing
// to report.
func (s *Server) agentUsageMetadata(ctx context.Context, req ChatRequest, model Model, res agentStreamResult, sid string) map[string]any {
	meta := map[string]any{}
	if sid != "" {
		meta["providerSessionId"] = sid
	}
	if res.usage != nil {
		usage := captainUsageBreakdown(res.usage)
		cost := costForUsage(captainModel(model), usage)
		turnCost := cost.TotalUsd
		if res.cost > 0 {
			turnCost = res.cost
			if cost.TotalUsd == 0 {
				cost.TotalUsd = res.cost
			}
		}
		turn := TurnUsage{
			InputTokens:      usage.InputTokens,
			OutputTokens:     usage.OutputTokens,
			ReasoningTokens:  usage.ReasoningTokens,
			CacheReadTokens:  usage.CacheReadTokens,
			CacheWriteTokens: usage.CacheWriteTokens,
			CostUSD:          turnCost,
		}
		threadCost := turnCost
		if req.ThreadID != "" && s.opts.Threads != nil {
			// Best-effort: a persistence failure must not abort a complete turn.
			if t, err := s.opts.Threads.AddUsage(ctx, req.ThreadID, turn); err == nil {
				threadCost = t.TotalCostUsd
			}
		}
		meta["usage"] = usage
		meta["costBreakdown"] = cost
		meta["cost"] = turnCost
		meta["threadCostUsd"] = threadCost
		meta["contextTokens"] = usage.InputTokens
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}
