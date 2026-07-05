package aichat

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	capapi "github.com/flanksource/captain/pkg/api"
)

// Provider is a Genkit provider name (the prefix of a "provider/model" id).
type Provider string

const (
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
	ProviderGoogle    Provider = "googleai"
)

// Effort is the per-request reasoning effort, translated per provider through
// Genkit's model config (Anthropic thinking budget, OpenAI reasoning_effort,
// Gemini thinkingConfig). Non-reasoning models ignore it.
type Effort string

const (
	EffortNone   Effort = ""
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
)

// Engine selects which execution path serves a model: the in-process Genkit
// runtime (API providers with caller-injected tools + human approval) or
// captain's consolidated agent framework (claude-agent / codex, which own their
// own tools and run as supervised local subprocesses).
type Engine string

const (
	// EngineGenkit (the zero value) is the Firebase Genkit API path.
	EngineGenkit Engine = ""
	// EngineAgent routes the turn through a captain pkg/ai StreamingProvider.
	EngineAgent Engine = "agent"
)

// Model describes one entry in the chat model menu. For EngineGenkit models ID
// is the full Genkit "provider/model" id passed to ai.WithModelName. For
// EngineAgent models ID is the menu/catalog key (e.g. "claude-agent-sonnet");
// Backend and AgentModel describe how to construct the captain provider.
type Model struct {
	ID            string
	Provider      Provider
	Label         string // human-friendly menu label
	Reasoning     bool   // model honours Effort
	ContextWindow int    // max context tokens, for a usage gauge's denominator

	// Engine selects the execution path. The zero value (EngineGenkit) is the
	// Genkit API path; EngineAgent routes through captain's agent framework.
	Engine Engine
	// Backend is the captain ai.Backend for EngineAgent models (e.g.
	// capapi.BackendClaudeAgent, capapi.BackendCodexCLI). Unused for
	// EngineGenkit.
	Backend capapi.Backend
	// AgentModel is the model slug passed to the captain backend when it differs
	// from ID (e.g. menu id "codex-gpt-5-codex" → backend model "gpt-5-codex").
	// Empty means use ID. Unused for EngineGenkit.
	AgentModel string
}

// ModelInfo is the JSON shape served at GET /api/chat/models so a client model
// selector can be data-driven. Configured reports whether the model's provider
// has an API key (selectable) vs merely catalogued.
type ModelInfo struct {
	ID            string `json:"id"`
	Provider      string `json:"provider"`
	Label         string `json:"label"`
	Reasoning     bool   `json:"reasoning"`
	Configured    bool   `json:"configured"`
	ContextWindow int    `json:"contextWindow"`
}

// DefaultModelID is the chat backend's default, mirroring captain pkg/ai
// (Anthropic Sonnet 5 is captain's default).
const DefaultModelID = "anthropic/claude-sonnet-5"

// defaultCatalog is the model menu: only the latest generally-available model
// per tier for each provider — no preview or superseded entries. Mirrors
// captain pkg/ai's catalog so the chat agrees with the rest of the stack.
//
// Provider currency (reviewed 2026-07-02):
//   - Anthropic: Fable 5 (most capable), Opus 4.8, Sonnet 5, Haiku 4.5. Mythos 5
//     is Project Glasswing invite-only, so it is intentionally excluded.
//   - OpenAI: GPT-5.5 (flagship) and GPT-5.4 mini. GPT-5.6 is preview-only.
//   - Google: Gemini 2.5 Pro is the newest GA Pro (all Gemini 3.x Pro models are
//     still preview), paired with the GA Gemini 3.5 Flash.
var defaultCatalog = []Model{
	{ID: "anthropic/claude-fable-5", Provider: ProviderAnthropic, Label: "Claude Fable 5", Reasoning: true, ContextWindow: 1000000},
	{ID: "anthropic/claude-opus-4-8", Provider: ProviderAnthropic, Label: "Claude Opus 4.8", Reasoning: true, ContextWindow: 1000000},
	{ID: "anthropic/claude-sonnet-5", Provider: ProviderAnthropic, Label: "Claude Sonnet 5", Reasoning: true, ContextWindow: 1000000},
	{ID: "anthropic/claude-haiku-4-5", Provider: ProviderAnthropic, Label: "Claude Haiku 4.5", Reasoning: true, ContextWindow: 200000},
	{ID: "openai/gpt-5.5", Provider: ProviderOpenAI, Label: "GPT-5.5", Reasoning: true, ContextWindow: 1000000},
	{ID: "openai/gpt-5.4-mini", Provider: ProviderOpenAI, Label: "GPT-5.4 mini", Reasoning: true, ContextWindow: 400000},
	{ID: "googleai/gemini-2.5-pro", Provider: ProviderGoogle, Label: "Gemini 2.5 Pro", Reasoning: true, ContextWindow: 1048576},
	{ID: "googleai/gemini-3.5-flash", Provider: ProviderGoogle, Label: "Gemini 3.5 Flash", Reasoning: true, ContextWindow: 1048576},

	// Agent-framework models (captain pkg/ai StreamingProvider). These run a
	// supervised local subprocess that owns its own tools; ids carry the
	// backend prefix captain's InferBackend recognises, and Backend is set
	// explicitly so codex slugs (which look like gpt-*) are not misrouted.
	{ID: "claude-agent-sonnet", Engine: EngineAgent, Backend: capapi.BackendClaudeAgent, Provider: "claude-agent", Label: "Claude Agent · Sonnet", Reasoning: true, ContextWindow: 1000000},
	{ID: "claude-agent-opus", Engine: EngineAgent, Backend: capapi.BackendClaudeAgent, Provider: "claude-agent", Label: "Claude Agent · Opus", Reasoning: true, ContextWindow: 1000000},
	{ID: "claude-agent-haiku", Engine: EngineAgent, Backend: capapi.BackendClaudeAgent, Provider: "claude-agent", Label: "Claude Agent · Haiku", Reasoning: true, ContextWindow: 200000},
	{ID: "codex-gpt-5-codex", Engine: EngineAgent, Backend: capapi.BackendCodexCLI, AgentModel: "gpt-5-codex", Provider: "codex-cli", Label: "Codex · GPT-5", Reasoning: true, ContextWindow: 400000},
}

var (
	modelRegistryMu sync.RWMutex
	catalog         = append([]Model(nil), defaultCatalog...)
)

// Catalog returns the registered model menu.
func Catalog() []Model {
	modelRegistryMu.RLock()
	defer modelRegistryMu.RUnlock()

	out := make([]Model, len(catalog))
	copy(out, catalog)
	return out
}

// RegisterModel adds a model to the global chat model registry, or replaces the
// existing entry with the same ID while preserving its position in the menu.
func RegisterModel(model Model) error {
	return RegisterModels(model)
}

// RegisterModels adds models to the global chat model registry. Existing IDs
// are updated in place; new IDs are appended to the menu.
func RegisterModels(models ...Model) error {
	normalized, err := normalizeModels(models, false)
	if err != nil {
		return err
	}

	modelRegistryMu.Lock()
	defer modelRegistryMu.Unlock()

	for _, model := range normalized {
		updated := false
		for i := range catalog {
			if catalog[i].ID == model.ID {
				catalog[i] = model
				updated = true
				break
			}
		}
		if !updated {
			catalog = append(catalog, model)
		}
	}
	return nil
}

// SetModelCatalog replaces the global chat model registry. Use RegisterModel or
// RegisterModels when you only need to extend the built-in catalog.
func SetModelCatalog(models []Model) error {
	normalized, err := normalizeModels(models, true)
	if err != nil {
		return err
	}

	modelRegistryMu.Lock()
	defer modelRegistryMu.Unlock()
	catalog = append([]Model(nil), normalized...)
	return nil
}

// ResetModelCatalog restores the built-in model registry. It is primarily useful
// for tests that temporarily install custom models.
func ResetModelCatalog() {
	modelRegistryMu.Lock()
	defer modelRegistryMu.Unlock()
	catalog = append([]Model(nil), defaultCatalog...)
}

func normalizeModels(models []Model, rejectDuplicateIDs bool) ([]Model, error) {
	out := make([]Model, 0, len(models))
	seen := make(map[string]bool, len(models))
	for _, model := range models {
		normalized, err := normalizeModel(model)
		if err != nil {
			return nil, err
		}
		if rejectDuplicateIDs && seen[normalized.ID] {
			return nil, fmt.Errorf("duplicate model ID %q", normalized.ID)
		}
		seen[normalized.ID] = true
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeModel(model Model) (Model, error) {
	model.ID = strings.TrimSpace(model.ID)
	model.Label = strings.TrimSpace(model.Label)
	if model.ID == "" {
		return Model{}, fmt.Errorf("model ID is required")
	}
	if model.Engine == EngineAgent {
		if model.Backend == "" {
			return Model{}, fmt.Errorf("agent model %q must set Backend (e.g. claude-agent, codex-cli)", model.ID)
		}
		if model.Provider == "" {
			model.Provider = Provider(model.Backend)
		}
	}
	if model.Provider == "" {
		model.Provider = ProviderOf(model.ID)
	}
	if model.Provider == "" {
		return Model{}, fmt.Errorf("model %q must include a provider, e.g. openai/%s", model.ID, model.ID)
	}
	if model.Label == "" {
		model.Label = model.ID
	}
	return model, nil
}

// CatalogInfo returns the model menu annotated with whether each model's
// provider is among the registered providers (i.e. selectable). The order
// mirrors the catalog so the client can render a stable, grouped menu.
func CatalogInfo(registered []Provider) []ModelInfo {
	models := Catalog()
	out := make([]ModelInfo, len(models))
	for i, m := range models {
		configured := providerRegistered(registered, m.Provider)
		if m.Engine == EngineAgent {
			// Agent models are gated on local backend availability (the CLI/SDK
			// being installed), not on a Genkit API key.
			configured = agentModelConfigured(m)
		}
		out[i] = ModelInfo{
			ID:            m.ID,
			Provider:      string(m.Provider),
			Label:         m.Label,
			Reasoning:     m.Reasoning,
			Configured:    configured,
			ContextWindow: m.ContextWindow,
		}
	}
	return out
}

// LookupModel resolves a "provider/model" id against the catalog. Returns an
// error listing the menu on miss — fail loud, never silently substitute.
func LookupModel(id string) (Model, error) {
	if id == "" {
		id = DefaultModelID
	}
	models := Catalog()
	for _, m := range models {
		if m.ID == id {
			return m, nil
		}
	}
	return Model{}, fmt.Errorf("unknown model %q; available: %s", id, strings.Join(modelIDsFrom(models), ", "))
}

// defaultModel picks the model used when a request omits one: the catalog
// default if its provider is configured, otherwise the first catalog model
// whose provider is configured. Returns false when no configured provider has a
// catalog model (caller fails loud).
func defaultModel(registered []Provider) (Model, bool) {
	models := Catalog()
	for _, m := range models {
		if m.ID == DefaultModelID && providerRegistered(registered, m.Provider) {
			return m, true
		}
	}
	for _, m := range models {
		if m.Engine == EngineGenkit && providerRegistered(registered, m.Provider) {
			return m, true
		}
	}
	return Model{}, false
}

// providerRegistered reports whether p is among the configured providers.
func providerRegistered(registered []Provider, p Provider) bool {
	return slices.Contains(registered, p)
}

// ProviderOf extracts the provider from a "provider/model" id without requiring
// the model to be in the catalog.
func ProviderOf(id string) Provider {
	if i := strings.IndexByte(id, '/'); i > 0 {
		return NormalizeProvider(id[:i])
	}
	return ""
}

// NormalizeProvider maps product/storage aliases onto the Genkit provider
// namespace used by model ids.
func NormalizeProvider(value string) Provider {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "anthropic", "claude":
		return ProviderAnthropic
	case "openai":
		return ProviderOpenAI
	case "google", "gemini", "googleai":
		return ProviderGoogle
	default:
		return Provider(strings.TrimSpace(value))
	}
}

func modelIDs() []string {
	return modelIDsFrom(Catalog())
}

func modelIDsFrom(models []Model) []string {
	ids := make([]string, len(models))
	for i, m := range models {
		ids[i] = m.ID
	}
	sort.Strings(ids)
	return ids
}

// ValidateEffort rejects unknown effort values (fail loud).
func ValidateEffort(e Effort) error {
	switch e {
	case EffortNone, EffortLow, EffortMedium, EffortHigh:
		return nil
	default:
		return fmt.Errorf("invalid reasoning effort %q; want one of: low, medium, high", e)
	}
}
