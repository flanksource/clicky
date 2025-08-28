package ai

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/clicky/ai/cache"
)

// AgentType represents the type of AI agent
type AgentType string

// Supported agent types
const (
	// AgentTypeClaude represents the Claude AI agent
	AgentTypeClaude AgentType = "claude"
	// AgentTypeAider represents the Aider AI agent
	AgentTypeAider AgentType = "aider"
)

// Model represents an AI model
type Model struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Provider    string            `json:"provider"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	InputPrice  float64           `json:"input_price_per_token,omitempty"`
	OutputPrice float64           `json:"output_price_per_token,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
}

// AgentConfig holds configuration for AI agents
type AgentConfig struct {
	Type            AgentType     `json:"type"`
	Model           string        `json:"model"`
	CacheDBPath     string        `json:"cache_db_path,omitempty"`
	ProjectName     string        `json:"project_name,omitempty"`
	SessionID       string        `json:"session_id,omitempty"`
	CacheTTL        time.Duration `json:"cache_ttl,omitempty"`
	Temperature     float64       `json:"temperature,omitempty"`
	MaxTokens       int           `json:"max_tokens"`
	MaxConcurrent   int           `json:"max_concurrent"`
	Debug           bool          `json:"debug"`
	Verbose         bool          `json:"verbose"`
	StrictMCPConfig bool          `json:"strict_mcp_config"`
	NoCache         bool          `json:"no_cache,omitempty"`
}

// PromptRequest represents a request to process a prompt
type PromptRequest struct {
	Context map[string]string `json:"context,omitempty"`
	Name    string            `json:"name"`
	Prompt  string            `json:"prompt"`
}

// PromptResponse represents the response from processing a prompt
type PromptResponse struct {
	Result           string  `json:"result"`
	Model            string  `json:"model,omitempty"`
	Error            string  `json:"error,omitempty"`
	CostUSD          float64 `json:"cost_usd"`
	TokensUsed       int     `json:"tokens_used"`
	TokensInput      int     `json:"tokens_input,omitempty"`
	TokensOutput     int     `json:"tokens_output,omitempty"`
	TokensCacheRead  int     `json:"tokens_cache_read,omitempty"`
	TokensCacheWrite int     `json:"tokens_cache_write,omitempty"`
	DurationMs       int     `json:"duration_ms"`
	CacheHit         bool    `json:"cache_hit,omitempty"`
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

	// Close cleans up resources
	Close() error
}

// AgentManager manages AI agents
type AgentManager struct {
	agents map[AgentType]Agent
	cache  *cache.Cache
	config AgentConfig
}

// NewAgentManager creates a new agent manager
func NewAgentManager(config AgentConfig) *AgentManager {
	am := &AgentManager{
		agents: make(map[AgentType]Agent),
		config: config,
	}

	// Initialize cache if not disabled
	if !config.NoCache {
		cacheConfig := cache.Config{
			TTL:     config.CacheTTL,
			NoCache: config.NoCache,
			DBPath:  config.CacheDBPath,
			Debug:   config.Debug,
		}

		c, err := cache.New(cacheConfig)
		if err != nil {
			// Log error but continue without cache
			if config.Debug {
				fmt.Printf("Warning: Failed to initialize AI cache: %v\n", err)
			}
		} else {
			am.cache = c
		}
	}

	return am
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
func (am *AgentManager) ListAllModels(ctx context.Context) map[AgentType][]Model {
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

	return results
}

// GetCache returns the cache instance
func (am *AgentManager) GetCache() *cache.Cache {
	return am.cache
}

// Close closes all agents and the cache
func (am *AgentManager) Close() error {
	var errs []string

	// Close all agents
	for _, agent := range am.agents {
		if err := agent.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// Close cache
	if am.cache != nil {
		if err := am.cache.Close(); err != nil {
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
		CacheTTL:      24 * time.Hour, // Default 24 hour TTL
		NoCache:       false,
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

	// Cache configuration flags
	flags.DurationVar(&config.CacheTTL, "ai-cache-ttl", config.CacheTTL, "AI cache TTL (e.g., 24h, 7d)")
	flags.BoolVar(&config.NoCache, "ai-no-cache", config.NoCache, "Disable AI response caching")
	flags.StringVar(&config.CacheDBPath, "ai-cache-db", config.CacheDBPath, "Path to AI cache database (default: ~/.cache/clicky-ai.db)")
	flags.StringVar(&config.ProjectName, "ai-project", config.ProjectName, "Project name for cache grouping")

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
