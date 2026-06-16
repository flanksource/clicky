package aichat

import "testing"

func TestLookupModelDefault(t *testing.T) {
	m, err := LookupModel("")
	if err != nil {
		t.Fatalf("LookupModel(\"\"): %v", err)
	}
	if m.ID != DefaultModelID {
		t.Errorf("default = %q, want %q", m.ID, DefaultModelID)
	}
}

func TestLookupModelUnknownFailsLoud(t *testing.T) {
	if _, err := LookupModel("openai/does-not-exist"); err == nil {
		t.Error("expected error for unknown model, got nil")
	}
}

func TestAnthropicCatalogIncludesRequestedModels(t *testing.T) {
	for _, id := range []string{
		"anthropic/claude-haiku-4-5",
		"anthropic/claude-sonnet-4-6",
		"anthropic/claude-opus-4-8",
	} {
		m, err := LookupModel(id)
		if err != nil {
			t.Fatalf("LookupModel(%q): %v", id, err)
		}
		if m.Provider != ProviderAnthropic {
			t.Errorf("LookupModel(%q).Provider = %q, want %q", id, m.Provider, ProviderAnthropic)
		}
	}
}

func TestEffortConfigPerProvider(t *testing.T) {
	cases := []struct {
		name    string
		model   Model
		effort  Effort
		wantKey string
		wantNil bool
	}{
		{"openai-high", Model{Provider: ProviderOpenAI, Reasoning: true}, EffortHigh, "reasoning_effort", false},
		{"gemini-medium", Model{Provider: ProviderGoogle, Reasoning: true}, EffortMedium, "thinkingConfig", false},
		{"anthropic-low", Model{Provider: ProviderAnthropic, Reasoning: true}, EffortLow, "thinking", false},
		{"anthropic-no-effort-still-sets-max-tokens", Model{Provider: ProviderAnthropic, Reasoning: true}, EffortNone, "max_tokens", false},
		{"non-reasoning", Model{Provider: ProviderOpenAI, Reasoning: false}, EffortHigh, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := effortConfig(c.model, c.effort)
			if c.wantNil {
				if cfg != nil {
					t.Errorf("cfg = %v, want nil", cfg)
				}
				return
			}
			if _, ok := cfg[c.wantKey]; !ok {
				t.Errorf("cfg %v missing key %q", cfg, c.wantKey)
			}
		})
	}
}

func TestAnthropicEffortConfigSetsMaxTokens(t *testing.T) {
	cfg := effortConfig(Model{Provider: ProviderAnthropic, Reasoning: true}, EffortHigh)
	if got := cfg["max_tokens"]; got != anthropicThinkingBudget(EffortHigh)+defaultMaxOutputTokens {
		t.Errorf("max_tokens = %v, want thinking budget plus visible output budget", got)
	}
}

func TestDefaultModelPrefersConfiguredDefault(t *testing.T) {
	m, ok := defaultModel([]Provider{ProviderAnthropic, ProviderGoogle})
	if !ok {
		t.Fatal("expected a model when default provider is configured")
	}
	if m.ID != DefaultModelID {
		t.Errorf("default = %q, want %q", m.ID, DefaultModelID)
	}
}

func TestDefaultModelFallsBackToConfiguredProvider(t *testing.T) {
	// Anthropic (the catalog default's provider) is NOT configured; only Google.
	m, ok := defaultModel([]Provider{ProviderGoogle})
	if !ok {
		t.Fatal("expected a Google model when only Google is configured")
	}
	if m.Provider != ProviderGoogle {
		t.Errorf("provider = %q, want %q", m.Provider, ProviderGoogle)
	}
}

func TestDefaultModelNoneConfigured(t *testing.T) {
	if _, ok := defaultModel(nil); ok {
		t.Error("expected no model when no providers configured")
	}
}

func TestValidateEffort(t *testing.T) {
	for _, e := range []Effort{EffortNone, EffortLow, EffortMedium, EffortHigh} {
		if err := ValidateEffort(e); err != nil {
			t.Errorf("ValidateEffort(%q) = %v, want nil", e, err)
		}
	}
	if err := ValidateEffort(Effort("extreme")); err == nil {
		t.Error("expected error for invalid effort")
	}
}
