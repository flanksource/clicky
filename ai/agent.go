package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/clicky/ai/cache"

	"github.com/spf13/pflag"
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

type Tokens struct {
	Input  int     `json:"input,omitempty"`
	Output int     `json:"output,omitempty"`
	Cost   float64 `json:"cost,omitempty"`
}

func (t Tokens) Add(other Tokens) Tokens {
	return Tokens{
		Input:  t.Input + other.Input,
		Output: t.Output + other.Output,
		Cost:   t.Cost + other.Cost,
	}
}

func (t Tokens) Total() int {
	return t.Input + t.Output
}

func (c Cost) Add(other Cost) Cost {
	model := c.Model
	if model == "" {
		model = other.Model
	}
	return Cost{
		Model:        model,
		InputTokens:  c.InputTokens + other.InputTokens,
		OutputTokens: c.OutputTokens + other.OutputTokens,
		TotalTokens:  c.TotalTokens + other.TotalTokens,
		InputCost:    c.InputCost + other.InputCost,
		OutputCost:   c.OutputCost + other.OutputCost,
	}
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

func GetDefaultAgent() (Agent, error) {

	manager := NewAgentManager(DefaultConfig())
	return manager.GetDefaultAgent()
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

var defaultConfig AgentConfig = AgentConfig{
	Type:          AgentTypeClaude,
	Model:         "claude-haiku-4-5",
	MaxTokens:     10000,
	MaxConcurrent: 4,
	Debug:         false,
	Verbose:       false,
	// Temperature:   0.2,
	StrictMCPConfig: true,
	CacheTTL:        24 * time.Hour, // Default 24 hour TTL
	NoCache:         false,
}

// DefaultConfig returns a default agent configuration
func DefaultConfig() AgentConfig {
	return defaultConfig
}

// BindFlags adds AI-related flags to the flag set
func BindFlags(flags *pflag.FlagSet) {

	agentType := string(defaultConfig.Type)
	flags.StringVar(&agentType, "agent", agentType, "AI agent type (claude, aider)")
	flags.BoolVar(&defaultConfig.Debug, "ai-debug", defaultConfig.Debug, "Enable AI debug output")
	flags.BoolVar(&defaultConfig.Verbose, "ai-verbose", defaultConfig.Verbose, "Enable AI verbose logging")
	flags.StringVar(&defaultConfig.Model, "ai-model", defaultConfig.Model, "AI model to use")
	flags.IntVar(&defaultConfig.MaxTokens, "ai-max-tokens", defaultConfig.MaxTokens, "Maximum tokens per request")
	flags.IntVar(&defaultConfig.MaxConcurrent, "ai-max-concurrent", defaultConfig.MaxConcurrent, "Maximum concurrent AI requests")
	flags.Float64Var(&defaultConfig.Temperature, "ai-temperature", defaultConfig.Temperature, "AI temperature (0.0-2.0)")
	flags.BoolVar(&defaultConfig.StrictMCPConfig, "ai-strict-mcp", defaultConfig.StrictMCPConfig, "Use strict MCP configuration (Claude only)")

	// Cache configuration flags
	flags.DurationVar(&defaultConfig.CacheTTL, "ai-cache-ttl", defaultConfig.CacheTTL, "AI cache TTL (e.g., 24h, 7d)")
	flags.BoolVar(&defaultConfig.NoCache, "ai-no-cache", defaultConfig.NoCache, "Disable AI response caching")
	flags.StringVar(&defaultConfig.CacheDBPath, "ai-cache-db", defaultConfig.CacheDBPath, "Path to AI cache database (default: ~/.cache/clicky-ai.db)")
	flags.StringVar(&defaultConfig.ProjectName, "ai-project", defaultConfig.ProjectName, "Project name for cache grouping")

	// Add convenience flags (these will be handled by the calling code)
	flags.Bool("aider", false, "Use Aider agent (shorthand for --agent=aider)")
	flags.Bool("claude", false, "Use Claude agent (shorthand for --agent=claude)")

	// Update config type after parsing (caller needs to handle this)
	defaultConfig.Type = AgentType(agentType)
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
