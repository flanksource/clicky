package aichat

import (
	"context"
	"fmt"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/anthropic"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
)

// ProviderCredential is a request-scoped API key for one upstream AI provider.
// Embedders can use this to resolve org-owned provider keys from their own
// connection store instead of relying only on process environment variables.
type ProviderCredential struct {
	Provider Provider
	APIKey   string
}

// ProviderCredentialsProvider returns API keys available to the current request.
type ProviderCredentialsProvider func(context.Context) ([]ProviderCredential, error)

// providerEnvKeys lists the environment variables consulted for each provider's
// API key, in priority order. A request-scoped credential always takes
// precedence over these.
var providerEnvKeys = map[Provider][]string{
	ProviderAnthropic: {"ANTHROPIC_API_KEY"},
	ProviderOpenAI:    {"OPENAI_API_KEY"},
	ProviderGoogle:    {"GOOGLE_API_KEY", "GEMINI_API_KEY"},
}

// providerOrder is the stable order in which providers are resolved, registered,
// and reported.
var providerOrder = []Provider{ProviderAnthropic, ProviderOpenAI, ProviderGoogle}

// initGenkit registers every provider plugin whose API key is present in the
// supplied credentials or environment, defaulting the model to DefaultModelID.
// Plugins panic on Init when their key is missing, so an absent key means the
// provider is simply not registered (and selecting one of its models later fails
// loud in LookupModel + generation). Fails loud if no providers are configured.
func initGenkit(ctx context.Context, creds ...ProviderCredential) (*genkit.Genkit, []Provider, error) {
	keyByProvider := credentialMap(creds)
	var plugins []api.Plugin
	var registered []Provider

	for _, provider := range providerOrder {
		key := resolveProviderKey(provider, keyByProvider)
		if key == "" {
			continue
		}
		switch provider {
		case ProviderAnthropic:
			plugins = append(plugins, &anthropic.Anthropic{APIKey: key})
		case ProviderOpenAI:
			plugins = append(plugins, &openai.OpenAI{APIKey: key})
		case ProviderGoogle:
			plugins = append(plugins, &googlegenai.GoogleAI{APIKey: key})
		}
		registered = append(registered, provider)
	}

	if len(plugins) == 0 {
		return nil, nil, fmt.Errorf("no AI provider configured: set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GOOGLE_API_KEY/GEMINI_API_KEY")
	}

	g := genkit.Init(ctx,
		genkit.WithPlugins(plugins...),
		genkit.WithDefaultModel(DefaultModelID),
	)
	return g, registered, nil
}

// credentialMap indexes request-scoped credentials by provider, keeping the
// first non-empty key supplied for each provider.
func credentialMap(creds []ProviderCredential) map[Provider]string {
	m := make(map[Provider]string, len(creds))
	for _, c := range creds {
		if c.APIKey == "" {
			continue
		}
		if _, ok := m[c.Provider]; !ok {
			m[c.Provider] = c.APIKey
		}
	}
	return m
}

// resolveProviderKey returns the API key for a provider, preferring a
// request-scoped credential over the provider's environment variables.
func resolveProviderKey(provider Provider, keyByProvider map[Provider]string) string {
	candidates := []string{keyByProvider[provider]}
	for _, env := range providerEnvKeys[provider] {
		candidates = append(candidates, os.Getenv(env))
	}
	return firstNonEmptyString(candidates...)
}

// configuredProviders returns the providers that have an API key available
// (from creds or the environment), in stable provider order. It mirrors the
// registration logic in initGenkit so callers see the same set of providers the
// server would actually register.
func configuredProviders(creds []ProviderCredential) []Provider {
	keyByProvider := credentialMap(creds)
	var providers []Provider
	for _, provider := range providerOrder {
		if resolveProviderKey(provider, keyByProvider) != "" {
			providers = append(providers, provider)
		}
	}
	return providers
}

// firstNonEmptyString returns the first non-empty value, or "" if all are empty.
func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

const defaultMaxOutputTokens = 4096

// effortConfig builds the provider-specific generation config that translates
// an Effort value into the provider's native reasoning control. Anthropic also
// requires max_tokens on every request, so return a config for Anthropic even
// when no reasoning effort is selected.
func effortConfig(m Model, e Effort, budget ChatBudget, temperature *float64) map[string]any {
	cfg := map[string]any{}
	switch m.Provider {
	case ProviderOpenAI:
		if m.Reasoning && e != EffortNone {
			// OpenAI o-series: reasoning_effort low|medium|high.
			cfg["reasoning_effort"] = string(e)
		}
	case ProviderGoogle:
		if m.Reasoning && e != EffortNone {
			// Gemini 2.5+: thinkingConfig.thinkingBudget (token budget).
			cfg["thinkingConfig"] = map[string]any{"thinkingBudget": geminiThinkingBudget(e)}
		}
	case ProviderAnthropic:
		cfg["max_tokens"] = anthropicMaxTokens(e, budget.MaxTokens)
		if m.Reasoning && e != EffortNone {
			// Anthropic: extended-thinking budget tokens.
			cfg["thinking"] = map[string]any{
				"type":          "enabled",
				"budget_tokens": anthropicThinkingBudget(e),
			}
		}
	default:
		return nil
	}
	if temperature != nil {
		cfg["temperature"] = *temperature
	}
	if budget.MaxTokens > 0 && m.Provider != ProviderAnthropic {
		cfg["maxOutputTokens"] = budget.MaxTokens
	}
	if len(cfg) == 0 {
		return nil
	}
	return cfg
}

func anthropicThinkingBudget(e Effort) int {
	switch e {
	case EffortLow:
		return 2048
	case EffortMedium:
		return 8192
	case EffortHigh:
		return 24576
	default:
		return 0
	}
}

func anthropicMaxTokens(e Effort, maxOutputTokens int) int {
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultMaxOutputTokens
	}
	budget := anthropicThinkingBudget(e)
	if budget == 0 {
		return maxOutputTokens
	}
	// Anthropic's thinking budget is counted inside max_tokens. Leave room for
	// the visible answer in addition to the hidden thinking budget.
	return budget + maxOutputTokens
}

func geminiThinkingBudget(e Effort) int {
	switch e {
	case EffortLow:
		return 2048
	case EffortMedium:
		return 8192
	case EffortHigh:
		return 24576
	default:
		return 0
	}
}

// generateOptions assembles the Generate options for a chat turn: model,
// messages, tools, system prompt, optional effort config, and the streaming
// callback.
func generateOptions(m Model, e Effort, budget ChatBudget, temperature *float64, system string, msgs []*ai.Message, tools []ai.ToolRef, stream ai.ModelStreamCallback, extra ...ai.GenerateOption) []ai.GenerateOption {
	opts := []ai.GenerateOption{
		ai.WithModelName(m.ID),
		ai.WithMessages(msgs...),
	}
	if system != "" {
		opts = append(opts, ai.WithSystem(system))
	}
	if len(tools) > 0 {
		opts = append(opts, ai.WithTools(tools...))
	}
	if cfg := effortConfig(m, e, budget, temperature); cfg != nil {
		opts = append(opts, ai.WithConfig(cfg))
	}
	if stream != nil {
		opts = append(opts, ai.WithStreaming(stream))
	}
	// Resume directives (WithToolRestarts/WithToolResponses) and any other
	// per-turn options come last.
	opts = append(opts, extra...)
	return opts
}

// resumeOptions translates approval resume directives into genkit Generate
// options. Returns nil when there is nothing to resume.
func resumeOptions(dirs *resumeDirectives) []ai.GenerateOption {
	if dirs.empty() {
		return nil
	}
	var opts []ai.GenerateOption
	if len(dirs.restarts) > 0 {
		opts = append(opts, ai.WithToolRestarts(dirs.restarts...))
	}
	if len(dirs.responds) > 0 {
		opts = append(opts, ai.WithToolResponses(dirs.responds...))
	}
	return opts
}
