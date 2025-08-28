package ai_test

import (
	"context"
	"testing"
	"time"

	"github.com/flanksource/clicky/ai"
)

func TestDefaultConfig(t *testing.T) {
	config := ai.DefaultConfig()

	if config.Type != ai.AgentTypeClaude {
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
		config  ai.AgentConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ai.AgentConfig{
				Type:          ai.AgentTypeClaude,
				Model:         "claude-3-5-sonnet-20241022",
				MaxTokens:     1000,
				MaxConcurrent: 1,
				Temperature:   0.5,
			},
			wantErr: false,
		},
		{
			name: "missing type",
			config: ai.AgentConfig{
				Model:         "claude-3-5-sonnet-20241022",
				MaxTokens:     1000,
				MaxConcurrent: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			config: ai.AgentConfig{
				Type:          ai.AgentType("invalid"),
				Model:         "claude-3-5-sonnet-20241022",
				MaxTokens:     1000,
				MaxConcurrent: 1,
			},
			wantErr: true,
		},
		{
			name: "missing model",
			config: ai.AgentConfig{
				Type:          ai.AgentTypeClaude,
				MaxTokens:     1000,
				MaxConcurrent: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid max tokens",
			config: ai.AgentConfig{
				Type:          ai.AgentTypeClaude,
				Model:         "claude-3-5-sonnet-20241022",
				MaxTokens:     0,
				MaxConcurrent: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid temperature",
			config: ai.AgentConfig{
				Type:          ai.AgentTypeClaude,
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
			err := ai.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgentManager(t *testing.T) {
	config := ai.DefaultConfig()
	am := ai.NewAgentManager(config)
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
	config := ai.AgentConfig{
		Type:          ai.AgentTypeClaude,
		Model:         "claude-3-5-sonnet-20241022",
		MaxTokens:     1000,
		MaxConcurrent: 1,
	}

	agent, err := ai.NewClaudeAgent(config)
	if err != nil {
		t.Fatalf("Failed to create Claude agent: %v", err)
	}
	defer agent.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
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
