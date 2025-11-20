package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/icons"
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

type CostInterface interface {
	GetTotalCost() Cost
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

func (a AgentConfig) Pretty() api.Text {
	t := api.Text{}.Append(string(a.Type))
	if a.Model != "" {
		t = t.Space().Append(a.Model, "font-mono")
	}
	if a.NoCache {
		t = t.Space().Append(" No Cache", "text-orange-500")
	}
	return t
}

// PromptRequest represents a request to process a prompt
type PromptRequest struct {
	Context          map[string]string `json:"context,omitempty"`
	Name             string            `json:"name"`
	Prompt           string            `json:"prompt"`
	StructuredOutput interface{}       `json:"structured_output,omitempty"` // Schema for structured JSON output
}

// TypedPromptRequest represents a type-safe request with structured output
type TypedPromptRequest[T any] struct {
	Context map[string]string `json:"context,omitempty"`
	Name    string            `json:"name"`
	Prompt  string            `json:"prompt"`
}

// PromptResponse represents the response from processing a prompt
type PromptResponse struct {
	Request        PromptRequest `json:"request,omitempty"`
	Result         string        `json:"result"`
	StructuredData interface{}   `json:"structured_data,omitempty"` // Populated if structured output was requested
	Costs          Costs         `json:"costs,omitempty"`
	Model          string        `json:"model,omitempty"`
	Error          string        `json:"error,omitempty"`
	// Total wall-clock duration of the request
	Duration time.Duration `json:"duration,omitempty"`
	// Duration spent in the model processing as reported by the API
	DurationModel time.Duration `json:"duration_model,omitempty"`
	CacheHit      bool          `json:"cache_hit,omitempty"`
}

// TypedPromptResponse represents a type-safe response with structured output
type TypedPromptResponse[T any] struct {
	Request       TypedPromptRequest[T] `json:"request,omitempty"`
	Data          T                     `json:"data"`
	Costs         Costs                 `json:"costs,omitempty"`
	Model         string                `json:"model,omitempty"`
	Error         string                `json:"error,omitempty"`
	Duration      time.Duration         `json:"duration,omitempty"`
	DurationModel time.Duration         `json:"duration_model,omitempty"`
	CacheHit      bool                  `json:"cache_hit,omitempty"`
}

func (tr TypedPromptResponse[T]) IsOK() bool {
	return tr.Error == ""
}

func (pr PromptResponse) IsOK() bool {
	return pr.Error == ""
}

func (pr PromptResponse) PrettyFull() api.Text {
	t := pr.Pretty()
	t = t.NewLine().Append(pr.Result)
	return t
}

func (pr PromptResponse) Pretty() api.Text {
	t := api.Text{}.Append(pr.Request.Name).Space().Add(icons.ArrowDoubleRight).Space()

	if pr.CacheHit {
		t = t.Add(icons.Check).Append(" Cache Hit", "text-green-600")
	}
	if !pr.IsOK() {
		t = t.Space().Add(icons.Error).Append(pr.Error, "text-red-500")
	}
	t = t.Space().Append(pr.Model, "font-mono")
	if pr.Duration.Milliseconds() > 0 {
		t = t.Space().Append(" (", "text-muted").Append(pr.Duration, "text-orange-500").Append(")", "text-muted")
	}
	t = t.Space().Append(pr.Costs.Sum().Pretty())
	return t

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

	GetCosts() Costs

	// Close cleans up resources
	Close() error
}

// ExecutePromptTyped is a generic helper that executes a prompt with type-safe structured output.
// It automatically cleans up the JSON response before unmarshaling into the target type.
func ExecutePromptTyped[T any](ctx context.Context, agent Agent, request TypedPromptRequest[T]) (*TypedPromptResponse[T], error) {
	// Convert to regular PromptRequest
	var schema T
	promptReq := PromptRequest{
		Context:          request.Context,
		Name:             request.Name,
		Prompt:           request.Prompt,
		StructuredOutput: &schema,
	}

	// Execute the request
	resp, err := agent.ExecutePrompt(ctx, promptReq)
	if err != nil {
		return &TypedPromptResponse[T]{
			Request: request,
			Error:   err.Error(),
		}, err
	}

	// Convert to typed response
	typedResp := &TypedPromptResponse[T]{
		Request:       request,
		Costs:         resp.Costs,
		Model:         resp.Model,
		Duration:      resp.Duration,
		DurationModel: resp.DurationModel,
		CacheHit:      resp.CacheHit,
		Error:         resp.Error,
	}

	// If there's structured data, convert it
	if resp.StructuredData != nil {
		if data, ok := resp.StructuredData.(*T); ok {
			typedResp.Data = *data
		} else {
			// Try to marshal and unmarshal through JSON
			jsonData, marshalErr := json.Marshal(resp.StructuredData)
			if marshalErr != nil {
				typedResp.Error = fmt.Sprintf("failed to marshal structured data: %v", marshalErr)
				return typedResp, marshalErr
			}
			if unmarshalErr := json.Unmarshal(jsonData, &typedResp.Data); unmarshalErr != nil {
				typedResp.Error = fmt.Sprintf("failed to unmarshal structured data to target type: %v", unmarshalErr)
				return typedResp, unmarshalErr
			}
		}
	}

	return typedResp, nil
}
