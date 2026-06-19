package aichat

import (
	"context"
	"fmt"
	"os"
	"strings"

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

// initGenkit registers every provider plugin whose API key is present in the
// supplied credentials or environment, defaulting the model to DefaultModelID.
// Plugins panic on Init when their key is missing, so an absent key means the
// provider is simply not registered (and selecting one of its models later fails
// loud in LookupModel + generation). Fails loud if no providers are configured.
func initGenkit(ctx context.Context, creds ...ProviderCredential) (*genkit.Genkit, []Provider, error) {
	var plugins []api.Plugin
	registered := configuredProviders(creds)
	keyByProvider := credentialMap(creds)

	if key := firstNonEmptyString(keyByProvider[ProviderAnthropic], os.Getenv("ANTHROPIC_API_KEY")); key != "" {
		plugins = append(plugins, &anthropic.Anthropic{APIKey: key})
	}
	if key := firstNonEmptyString(keyByProvider[ProviderOpenAI], os.Getenv("OPENAI_API_KEY")); key != "" {
		plugins = append(plugins, &openai.OpenAI{APIKey: key})
	}
	if key := firstNonEmptyString(keyByProvider[ProviderGoogle], os.Getenv("GOOGLE_API_KEY"), os.Getenv("GEMINI_API_KEY")); key != "" {
		plugins = append(plugins, &googlegenai.GoogleAI{APIKey: key})
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

const defaultMaxOutputTokens = 4096

// effortConfig builds the provider-specific generation config that translates
// an Effort value into the provider's native reasoning control. Anthropic also
// requires max_tokens on every request, so return a config for Anthropic even
// when no reasoning effort is selected.
func effortConfig(m Model, e Effort) map[string]any {
	switch m.Provider {
	case ProviderOpenAI:
		if !m.Reasoning || e == EffortNone {
			return nil
		}
		// OpenAI o-series: reasoning_effort low|medium|high.
		return map[string]any{"reasoning_effort": string(e)}
	case ProviderGoogle:
		if !m.Reasoning || e == EffortNone {
			return nil
		}
		// Gemini 2.5+: thinkingConfig.thinkingBudget (token budget).
		return map[string]any{"thinkingConfig": map[string]any{"thinkingBudget": geminiThinkingBudget(e)}}
	case ProviderAnthropic:
		cfg := map[string]any{"max_tokens": anthropicMaxTokens(e)}
		if m.Reasoning && e != EffortNone {
			// Anthropic: extended-thinking budget tokens.
			cfg["thinking"] = map[string]any{
				"type":          "enabled",
				"budget_tokens": anthropicThinkingBudget(e),
			}
		}
		return cfg
	default:
		return nil
	}
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

func anthropicMaxTokens(e Effort) int {
	budget := anthropicThinkingBudget(e)
	if budget == 0 {
		return defaultMaxOutputTokens
	}
	// Anthropic's thinking budget is counted inside max_tokens. Leave room for
	// the visible answer in addition to the hidden thinking budget.
	return budget + defaultMaxOutputTokens
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
func generateOptions(m Model, e Effort, system string, msgs []*ai.Message, tools []ai.ToolRef, stream ai.ModelStreamCallback, extra ...ai.GenerateOption) []ai.GenerateOption {
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
	if cfg := effortConfig(m, e); cfg != nil {
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
