package ai

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Type != AgentTypeClaude {
		t.Errorf("Expected default agent type to be Claude, got %s", config.Type)
	}

	if config.MaxConcurrent <= 0 {
		t.Errorf("Expected positive max concurrent, got %d", config.MaxConcurrent)
	}

	if config.Model == "" {
		t.Error("Expected default model to be set")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  AgentConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: AgentConfig{
				Type:          AgentTypeClaude,
				Model:         "claude-3-5-sonnet-20241022",
				MaxTokens:     1000,
				MaxConcurrent: 1,
				Temperature:   0.5,
			},
			wantErr: false,
		},
		{
			name: "missing type",
			config: AgentConfig{
				Model:         "claude-3-5-sonnet-20241022",
				MaxTokens:     1000,
				MaxConcurrent: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			config: AgentConfig{
				Type:          "invalid",
				Model:         "claude-3-5-sonnet-20241022",
				MaxTokens:     1000,
				MaxConcurrent: 1,
			},
			wantErr: true,
		},
		{
			name: "missing model",
			config: AgentConfig{
				Type:          AgentTypeClaude,
				MaxTokens:     1000,
				MaxConcurrent: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid max tokens",
			config: AgentConfig{
				Type:          AgentTypeClaude,
				Model:         "claude-3-5-sonnet-20241022",
				MaxTokens:     0,
				MaxConcurrent: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid temperature",
			config: AgentConfig{
				Type:          AgentTypeClaude,
				Model:         "claude-3-5-sonnet-20241022",
				MaxTokens:     1000,
				MaxConcurrent: 1,
				Temperature:   3.0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgentManager(t *testing.T) {
	config := DefaultConfig()
	am := NewAgentManager(config)
	defer am.Close()

	// Test getting default agent
	agent, err := am.GetDefaultAgent()
	if err != nil {
		t.Fatalf("Failed to get default agent: %v", err)
	}

	if agent.GetType() != config.Type {
		t.Errorf("Expected agent type %s, got %s", config.Type, agent.GetType())
	}

	// Test getting the same agent again (should be cached)
	agent2, err := am.GetAgent(config.Type)
	if err != nil {
		t.Fatalf("Failed to get cached agent: %v", err)
	}

	if agent != agent2 {
		t.Error("Expected cached agent to be the same instance")
	}
}

func TestClaudeAgentListModels(t *testing.T) {
	config := AgentConfig{
		Type:          AgentTypeClaude,
		Model:         "claude-3-5-sonnet-20241022",
		MaxTokens:     1000,
		MaxConcurrent: 1,
	}

	agent, err := NewClaudeAgent(config)
	if err != nil {
		t.Fatalf("Failed to create Claude agent: %v", err)
	}
	defer agent.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	models, err := agent.ListModels(ctx)
	if err != nil {
		t.Fatalf("Failed to list models: %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected at least one model")
	}

	// Check that we have expected Claude models
	foundHaiku := false
	for _, model := range models {
		if model.ID == "claude-3-haiku-20240307" {
			foundHaiku = true
			if model.Provider != "anthropic" {
				t.Errorf("Expected provider 'anthropic', got %s", model.Provider)
			}
			break
		}
	}

	if !foundHaiku {
		t.Error("Expected to find Claude 3 Haiku model")
	}
}

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		name  string
		price float64
		want  string
	}{
		{"zero", 0, "N/A"},
		{"small", 0.000003, "$3.00/1M"},
		{"very small", 0.00000025, "$0.250/1M"},
		{"large", 0.000075, "$75.00/1M"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPrice(tt.price)
			if got != tt.want {
				t.Errorf("formatPrice(%f) = %s, want %s", tt.price, got, tt.want)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		name   string
		tokens int
		want   string
	}{
		{"zero", 0, "N/A"},
		{"small", 100, "100"},
		{"thousands", 8192, "8.2K"},
		{"millions", 200000, "200.0K"},
		{"large millions", 2000000, "2.0M"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTokens(tt.tokens)
			if got != tt.want {
				t.Errorf("formatTokens(%d) = %s, want %s", tt.tokens, got, tt.want)
			}
		})
	}
}

// Helper function to check if a string contains another string
func containsString(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		findSubstring(haystack, needle) >= 0
}

func findSubstring(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
