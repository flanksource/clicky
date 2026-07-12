package aichat

import (
	"fmt"
	"slices"

	capai "github.com/flanksource/captain/pkg/ai"
)

// Provider is a Genkit provider name (the prefix of a "provider/model" id). It
// keys the Genkit plugin registration and the per-request "configured" overlay;
// the model catalog itself is owned by captain (see Model below).
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

// Model and ModelInfo are captain's catalog types. captain owns the catalog (it
// is keyed on Backend, which is captain data) and aichat consumes it via these
// aliases, so the chat menu never drifts from `captain whoami`. An agent model
// is identified by Model.IsAgent(); the Genkit provider for an API model is
// captain's BackendToProvider(Model.Backend).
type Model = capai.Model

// ModelInfo is the JSON shape served at GET /api/chat/models.
type ModelInfo = capai.ModelInfo

// DefaultModelID mirrors captain's catalog default (Anthropic Sonnet 5).
const DefaultModelID = capai.DefaultModelID

// Catalog returns captain's static model menu.
func Catalog() []Model { return capai.Catalog() }

// LookupModel resolves an id against captain's catalog for the generation path.
// It uses the static catalog (not the live probe) so resolution is deterministic
// and hermetic; the served menu (handleModels) uses the live catalog for display,
// which is a superset on any host whose probe only adds live provider models.
func LookupModel(id string) (Model, error) { return capai.LookupModel(id) }

// RegisterModel / RegisterModels / SetModelCatalog / ResetModelCatalog delegate
// to captain's catalog for embedders that extend or replace the menu.
func RegisterModel(model Model) error        { return capai.RegisterModel(model) }
func RegisterModels(models ...Model) error   { return capai.RegisterModels(models...) }
func SetModelCatalog(models []Model) error   { return capai.SetModelCatalog(models) }
func ResetModelCatalog()                     { capai.ResetModelCatalog() }

// modelProvider is the Genkit provider (plugin key) for a catalog model.
func modelProvider(m Model) Provider { return Provider(capai.BackendToProvider(m.Backend)) }

// providerRegistered reports whether p is among the configured providers.
func providerRegistered(registered []Provider, p Provider) bool {
	return slices.Contains(registered, p)
}

// defaultModel picks the model used when a request omits one: the catalog
// default if its provider is configured, otherwise the first API-backed catalog
// model whose provider is configured. Returns false when no configured provider
// has a catalog model (caller fails loud).
func defaultModel(registered []Provider) (Model, bool) {
	models := Catalog()
	for _, m := range models {
		if m.ID == DefaultModelID && providerRegistered(registered, modelProvider(m)) {
			return m, true
		}
	}
	for _, m := range models {
		if !m.IsAgent() && providerRegistered(registered, modelProvider(m)) {
			return m, true
		}
	}
	return Model{}, false
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
