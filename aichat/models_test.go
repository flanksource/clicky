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

func TestRegisterModelAddsAndUpdatesCatalog(t *testing.T) {
	t.Cleanup(ResetModelCatalog)

	id := "openai/test-custom-model"
	if err := RegisterModel(Model{ID: id, Label: "Custom", Reasoning: true, ContextWindow: 123}); err != nil {
		t.Fatalf("RegisterModel: %v", err)
	}

	m, err := LookupModel(id)
	if err != nil {
		t.Fatalf("LookupModel(%q): %v", id, err)
	}
	if m.Provider != ProviderOpenAI {
		t.Errorf("Provider = %q, want %q", m.Provider, ProviderOpenAI)
	}
	if m.Label != "Custom" || !m.Reasoning || m.ContextWindow != 123 {
		t.Errorf("model = %+v, want registered values", m)
	}

	if err := RegisterModel(Model{ID: id, Provider: ProviderOpenAI, Label: "Updated", ContextWindow: 456}); err != nil {
		t.Fatalf("RegisterModel update: %v", err)
	}
	m, err = LookupModel(id)
	if err != nil {
		t.Fatalf("LookupModel(%q) after update: %v", id, err)
	}
	if m.Label != "Updated" || m.ContextWindow != 456 || m.Reasoning {
		t.Errorf("updated model = %+v", m)
	}
}

func TestSetModelCatalogAndReset(t *testing.T) {
	t.Cleanup(ResetModelCatalog)

	if err := SetModelCatalog([]Model{{ID: "openai/only-model", Label: "Only"}}); err != nil {
		t.Fatalf("SetModelCatalog: %v", err)
	}
	if _, err := LookupModel("openai/gpt-5.5"); err == nil {
		t.Error("expected built-in model to be absent after SetModelCatalog")
	}
	m, err := LookupModel("openai/only-model")
	if err != nil {
		t.Fatalf("LookupModel custom catalog: %v", err)
	}
	if m.Provider != ProviderOpenAI || m.Label != "Only" {
		t.Errorf("model = %+v", m)
	}

	ResetModelCatalog()
	if _, err := LookupModel("openai/gpt-5.5"); err != nil {
		t.Fatalf("built-in model should be restored: %v", err)
	}
}

func TestRegisterModelValidation(t *testing.T) {
	if err := RegisterModel(Model{}); err == nil {
		t.Error("expected missing ID to fail")
	}
	if err := SetModelCatalog([]Model{{ID: "openai/dup"}, {ID: "openai/dup"}}); err == nil {
		t.Error("expected duplicate IDs to fail in SetModelCatalog")
	}
}

func TestAnthropicCatalogIncludesRequestedModels(t *testing.T) {
	for _, id := range []string{
		"anthropic/claude-haiku-4-5",
		"anthropic/claude-sonnet-5",
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
			cfg := effortConfig(c.model, c.effort, ChatBudget{}, nil)
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
	cfg := effortConfig(Model{Provider: ProviderAnthropic, Reasoning: true}, EffortHigh, ChatBudget{MaxTokens: 1000}, nil)
	if got := cfg["max_tokens"]; got != anthropicThinkingBudget(EffortHigh)+1000 {
		t.Errorf("max_tokens = %v, want thinking budget plus visible output budget", got)
	}
}

func TestEffortConfigAppliesTemperatureAndMaxTokens(t *testing.T) {
	temp := 0.4
	cfg := effortConfig(Model{Provider: ProviderOpenAI}, EffortNone, ChatBudget{MaxTokens: 1200}, &temp)
	if got := cfg["temperature"]; got != temp {
		t.Errorf("temperature = %v, want %v", got, temp)
	}
	if got := cfg["maxOutputTokens"]; got != 1200 {
		t.Errorf("maxOutputTokens = %v, want 1200", got)
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
