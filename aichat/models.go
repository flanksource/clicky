package aichat

import (
	"fmt"
	"sort"
	"strings"
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
	ID        string
	Provider  Provider
	Label     string // human-friendly menu label
	Reasoning bool   // model honours Effort
}

// ModelInfo is the JSON shape served at GET /api/chat/models so a client model
// selector can be data-driven. Configured reports whether the model's provider
// has an API key (selectable) vs merely catalogued.
type ModelInfo struct {
	ID         string `json:"id"`
	Provider   string `json:"provider"`
	Label      string `json:"label"`
	Reasoning  bool   `json:"reasoning"`
	Configured bool   `json:"configured"`
}

// DefaultModelID is the chat backend's default, mirroring captain pkg/ai
// (Anthropic claude-sonnet-4 is captain's NewAnthropic default).
const DefaultModelID = "anthropic/claude-sonnet-4-5"

// catalog is the v1 model menu. Mirrors captain pkg/ai provider defaults so the
// chat agrees with the rest of the stack.
var catalog = []Model{
	{ID: "anthropic/claude-sonnet-4-5", Provider: ProviderAnthropic, Label: "Claude Sonnet 4.5", Reasoning: true},
	{ID: "anthropic/claude-opus-4-1", Provider: ProviderAnthropic, Label: "Claude Opus 4.1", Reasoning: true},
	{ID: "anthropic/claude-3-5-haiku-latest", Provider: ProviderAnthropic, Label: "Claude 3.5 Haiku", Reasoning: false},
	{ID: "openai/gpt-4o", Provider: ProviderOpenAI, Label: "GPT-4o", Reasoning: false},
	{ID: "openai/o3", Provider: ProviderOpenAI, Label: "OpenAI o3", Reasoning: true},
	{ID: "openai/o4-mini", Provider: ProviderOpenAI, Label: "OpenAI o4-mini", Reasoning: true},
	{ID: "googleai/gemini-2.5-pro", Provider: ProviderGoogle, Label: "Gemini 2.5 Pro", Reasoning: true},
	{ID: "googleai/gemini-2.5-flash", Provider: ProviderGoogle, Label: "Gemini 2.5 Flash", Reasoning: true},
}

// Catalog returns the configured model menu.
func Catalog() []Model {
	out := make([]Model, len(catalog))
	copy(out, catalog)
	return out
}

// CatalogInfo returns the model menu annotated with whether each model's
// provider is among the registered providers (i.e. selectable). The order
// mirrors the catalog so the client can render a stable, grouped menu.
func CatalogInfo(registered []Provider) []ModelInfo {
	out := make([]ModelInfo, len(catalog))
	for i, m := range catalog {
		out[i] = ModelInfo{
			ID:         m.ID,
			Provider:   string(m.Provider),
			Label:      m.Label,
			Reasoning:  m.Reasoning,
			Configured: providerRegistered(registered, m.Provider),
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
	for _, m := range catalog {
		if m.ID == id {
			return m, nil
		}
	}
	return Model{}, fmt.Errorf("unknown model %q; available: %s", id, strings.Join(modelIDs(), ", "))
}

// defaultModel picks the model used when a request omits one: the catalog
// default if its provider is configured, otherwise the first catalog model
// whose provider is configured. Returns false when no configured provider has a
// catalog model (caller fails loud).
func defaultModel(registered []Provider) (Model, bool) {
	if def, err := LookupModel(DefaultModelID); err == nil && providerRegistered(registered, def.Provider) {
		return def, true
	}
	for _, m := range catalog {
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
	ids := make([]string, len(catalog))
	for i, m := range catalog {
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
