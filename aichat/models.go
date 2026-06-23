package aichat

import (
	"fmt"
	"sort"
	"strings"
	"sync"
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

// Model describes one entry in the chat model menu. ID is the full Genkit
// "provider/model" id passed to ai.WithModelName.
type Model struct {
	ID            string
	Provider      Provider
	Label         string // human-friendly menu label
	Reasoning     bool   // model honours Effort
	ContextWindow int    // max context tokens, for a usage gauge's denominator
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
// (Anthropic claude-sonnet-4 is captain's NewAnthropic default).
const DefaultModelID = "anthropic/claude-sonnet-4-5"

// defaultCatalog is the v1 model menu. Mirrors captain pkg/ai provider defaults
// so the chat agrees with the rest of the stack.
var defaultCatalog = []Model{
	{ID: "anthropic/claude-sonnet-4-6", Provider: ProviderAnthropic, Label: "Claude Sonnet 4.6", Reasoning: true, ContextWindow: 200000},
	{ID: "anthropic/claude-opus-4-8", Provider: ProviderAnthropic, Label: "Claude Opus 4.8", Reasoning: true, ContextWindow: 200000},
	{ID: "anthropic/claude-haiku-4-5", Provider: ProviderAnthropic, Label: "Claude Haiku 4.5", Reasoning: true, ContextWindow: 200000},
	{ID: "anthropic/claude-sonnet-4-5", Provider: ProviderAnthropic, Label: "Claude Sonnet 4.5", Reasoning: true, ContextWindow: 200000},
	{ID: "anthropic/claude-opus-4-1", Provider: ProviderAnthropic, Label: "Claude Opus 4.1", Reasoning: true, ContextWindow: 200000},
	{ID: "anthropic/claude-3-5-haiku-latest", Provider: ProviderAnthropic, Label: "Claude 3.5 Haiku", Reasoning: false, ContextWindow: 200000},
	{ID: "openai/gpt-4o", Provider: ProviderOpenAI, Label: "GPT-4o", Reasoning: false, ContextWindow: 128000},
	{ID: "openai/o3", Provider: ProviderOpenAI, Label: "OpenAI o3", Reasoning: true, ContextWindow: 200000},
	{ID: "openai/o4-mini", Provider: ProviderOpenAI, Label: "OpenAI o4-mini", Reasoning: true, ContextWindow: 200000},
	{ID: "googleai/gemini-2.5-pro", Provider: ProviderGoogle, Label: "Gemini 2.5 Pro", Reasoning: true, ContextWindow: 1048576},
	{ID: "googleai/gemini-2.5-flash", Provider: ProviderGoogle, Label: "Gemini 2.5 Flash", Reasoning: true, ContextWindow: 1048576},
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
		out[i] = ModelInfo{
			ID:            m.ID,
			Provider:      string(m.Provider),
			Label:         m.Label,
			Reasoning:     m.Reasoning,
			Configured:    providerRegistered(registered, m.Provider),
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
		if providerRegistered(registered, m.Provider) {
			return m, true
		}
	}
	return Model{}, false
}

// providerRegistered reports whether p is among the configured providers.
func providerRegistered(registered []Provider, p Provider) bool {
	for _, r := range registered {
		if r == p {
			return true
		}
	}
	return false
}

// ProviderOf extracts the provider from a "provider/model" id without requiring
// the model to be in the catalog.
func ProviderOf(id string) Provider {
	if i := strings.IndexByte(id, '/'); i > 0 {
		return Provider(id[:i])
	}
	return ""
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
