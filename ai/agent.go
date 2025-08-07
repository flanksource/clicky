package ai

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/flanksource/clicky"
)

// AgentType represents the type of AI agent
type AgentType string

const (
	AgentTypeClaude AgentType = "claude"
	AgentTypeAider  AgentType = "aider"
)

// Model represents an AI model
type Model struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Provider    string            `json:"provider"`
	InputPrice  float64           `json:"input_price_per_token,omitempty"`
	OutputPrice float64           `json:"output_price_per_token,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AgentConfig holds configuration for AI agents
type AgentConfig struct {
	Type            AgentType `json:"type"`
	Model           string    `json:"model"`
	MaxTokens       int       `json:"max_tokens"`
	MaxConcurrent   int       `json:"max_concurrent"`
	Debug           bool      `json:"debug"`
	Verbose         bool      `json:"verbose"`
	StrictMCPConfig bool      `json:"strict_mcp_config"`
	Temperature     float64   `json:"temperature,omitempty"`
}

// PromptRequest represents a request to process a prompt
type PromptRequest struct {
	Name    string            `json:"name"`
	Prompt  string            `json:"prompt"`
	Context map[string]string `json:"context,omitempty"`
}

// PromptResponse represents the response from processing a prompt
type PromptResponse struct {
	Result       string  `json:"result"`
	TokensUsed   int     `json:"tokens_used"`
	CostUSD      float64 `json:"cost_usd"`
	DurationMs   int     `json:"duration_ms"`
	CacheHit     bool    `json:"cache_hit,omitempty"`
	Model        string  `json:"model,omitempty"`
	Error        string  `json:"error,omitempty"`
}

// Agent interface defines the contract for AI agents
type Agent interface {
	// GetType returns the agent type
	GetType() AgentType
	
	// GetConfig returns the agent configuration
	GetConfig() AgentConfig
	
	// ListModels returns available models for this agent
	ListModels(ctx context.Context) ([]Model, error)
	
	// ExecutePrompt processes a single prompt
	ExecutePrompt(ctx context.Context, request PromptRequest) (*PromptResponse, error)
	
	// ExecuteBatch processes multiple prompts
	ExecuteBatch(ctx context.Context, requests []PromptRequest) (map[string]*PromptResponse, error)
	
	// GetTaskManager returns the underlying task manager for progress tracking
	GetTaskManager() *clicky.TaskManager
	
	// Close cleans up resources
	Close() error
}

// AgentManager manages AI agents
type AgentManager struct {
	agents map[AgentType]Agent
	config AgentConfig
}

// NewAgentManager creates a new agent manager
func NewAgentManager(config AgentConfig) *AgentManager {
	return &AgentManager{
		agents: make(map[AgentType]Agent),
		config: config,
	}
}

// GetAgent returns an agent of the specified type, creating it if needed
func (am *AgentManager) GetAgent(agentType AgentType) (Agent, error) {
	if agent, exists := am.agents[agentType]; exists {
		return agent, nil
	}
	
	// Create new agent
	var agent Agent
	var err error
	
	switch agentType {
	case AgentTypeClaude:
		agent, err = NewClaudeAgent(am.config)
	case AgentTypeAider:
		agent, err = NewAiderAgent(am.config)
	default:
		return nil, fmt.Errorf("unsupported agent type: %s", agentType)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to create %s agent: %w", agentType, err)
	}
	
	am.agents[agentType] = agent
	return agent, nil
}

// GetDefaultAgent returns the default agent based on config
func (am *AgentManager) GetDefaultAgent() (Agent, error) {
	return am.GetAgent(am.config.Type)
}

// ListAllModels returns models from all available agents
func (am *AgentManager) ListAllModels(ctx context.Context) (map[AgentType][]Model, error) {
	results := make(map[AgentType][]Model)
	
	for _, agentType := range []AgentType{AgentTypeClaude, AgentTypeAider} {
		agent, err := am.GetAgent(agentType)
		if err != nil {
			// Skip agents that can't be created
			continue
		}
		
		models, err := agent.ListModels(ctx)
		if err != nil {
			// Skip agents that can't list models
			continue
		}
		
		results[agentType] = models
	}
	
	return results, nil
}

// Close closes all agents
func (am *AgentManager) Close() error {
	var errs []string
	
	for _, agent := range am.agents {
		if err := agent.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("errors closing agents: %s", strings.Join(errs, "; "))
	}
	
	return nil
}

// DefaultConfig returns a default agent configuration
func DefaultConfig() AgentConfig {
	return AgentConfig{
		Type:          AgentTypeClaude,
		Model:         "claude-3-5-sonnet-20241022",
		MaxTokens:     10000,
		MaxConcurrent: 3,
		Debug:         false,
		Verbose:       false,
		Temperature:   0.2,
	}
}

// BindFlags adds AI-related flags to the flag set
func BindFlags(flags *flag.FlagSet, config *AgentConfig) {
	agentType := string(config.Type)
	flags.StringVar(&agentType, "agent", agentType, "AI agent type (claude, aider)")
	flags.BoolVar(&config.Debug, "ai-debug", config.Debug, "Enable AI debug output")
	flags.BoolVar(&config.Verbose, "ai-verbose", config.Verbose, "Enable AI verbose logging")
	flags.StringVar(&config.Model, "ai-model", config.Model, "AI model to use")
	flags.IntVar(&config.MaxTokens, "ai-max-tokens", config.MaxTokens, "Maximum tokens per request")
	flags.IntVar(&config.MaxConcurrent, "ai-max-concurrent", config.MaxConcurrent, "Maximum concurrent AI requests")
	flags.Float64Var(&config.Temperature, "ai-temperature", config.Temperature, "AI temperature (0.0-2.0)")
	flags.BoolVar(&config.StrictMCPConfig, "ai-strict-mcp", config.StrictMCPConfig, "Use strict MCP configuration (Claude only)")
	
	// Add convenience flags (these will be handled by the calling code)
	flags.Bool("aider", false, "Use Aider agent (shorthand for --agent=aider)")
	flags.Bool("claude", false, "Use Claude agent (shorthand for --agent=claude)")
	
	// Update config type after parsing (caller needs to handle this)
	config.Type = AgentType(agentType)
}

// ValidateConfig validates the agent configuration
func ValidateConfig(config AgentConfig) error {
	if config.Type == "" {
		return fmt.Errorf("agent type is required")
	}
	
	if config.Type != AgentTypeClaude && config.Type != AgentTypeAider {
		return fmt.Errorf("unsupported agent type: %s (supported: claude, aider)", config.Type)
	}
	
	if config.Model == "" {
		return fmt.Errorf("model is required")
	}
	
	if config.MaxTokens <= 0 {
		return fmt.Errorf("max tokens must be positive")
	}
	
	if config.MaxConcurrent <= 0 {
		return fmt.Errorf("max concurrent must be positive")
	}
	
	if config.Temperature < 0 || config.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}
	
	return nil
}