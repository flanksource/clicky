package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/flanksource/clicky"
)

// ClaudeAgent implements the Agent interface for Claude
type ClaudeAgent struct {
	config      AgentConfig
	taskManager *clicky.TaskManager
}

// NewClaudeAgent creates a new Claude agent
func NewClaudeAgent(config AgentConfig) (*ClaudeAgent, error) {
	if err := ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	tm := clicky.NewTaskManagerWithConcurrency(config.MaxConcurrent)
	if config.Debug || config.Verbose {
		tm.SetVerbose(true)
	}
	
	return &ClaudeAgent{
		config:      config,
		taskManager: tm,
	}, nil
}

// GetType returns the agent type
func (ca *ClaudeAgent) GetType() AgentType {
	return AgentTypeClaude
}

// GetConfig returns the agent configuration
func (ca *ClaudeAgent) GetConfig() AgentConfig {
	return ca.config
}

// GetTaskManager returns the underlying task manager
func (ca *ClaudeAgent) GetTaskManager() *clicky.TaskManager {
	return ca.taskManager
}

// ListModels returns available Claude models
func (ca *ClaudeAgent) ListModels(ctx context.Context) ([]Model, error) {
	// For Claude, we'll return a predefined list of known models
	// In a real implementation, you might query the Claude API for available models
	models := []Model{
		{
			ID:          "claude-3-5-sonnet-20241022",
			Name:        "Claude 3.5 Sonnet",
			Provider:    "anthropic",
			InputPrice:  0.000003,  // $3 per million tokens
			OutputPrice: 0.000015,  // $15 per million tokens
			MaxTokens:   200000,
			Metadata: map[string]string{
				"version": "2024-10-22",
				"type":    "chat",
			},
		},
		{
			ID:          "claude-3-haiku-20240307",
			Name:        "Claude 3 Haiku",
			Provider:    "anthropic",
			InputPrice:  0.00000025, // $0.25 per million tokens
			OutputPrice: 0.00000125, // $1.25 per million tokens
			MaxTokens:   200000,
			Metadata: map[string]string{
				"version": "2024-03-07",
				"type":    "chat",
			},
		},
		{
			ID:          "claude-3-sonnet-20240229",
			Name:        "Claude 3 Sonnet",
			Provider:    "anthropic",
			InputPrice:  0.000003,  // $3 per million tokens
			OutputPrice: 0.000015,  // $15 per million tokens
			MaxTokens:   200000,
			Metadata: map[string]string{
				"version": "2024-02-29",
				"type":    "chat",
			},
		},
		{
			ID:          "claude-3-opus-20240229",
			Name:        "Claude 3 Opus",
			Provider:    "anthropic",
			InputPrice:  0.000015,  // $15 per million tokens
			OutputPrice: 0.000075,  // $75 per million tokens
			MaxTokens:   200000,
			Metadata: map[string]string{
				"version": "2024-02-29",
				"type":    "chat",
			},
		},
	}
	
	return models, nil
}

// ExecutePrompt processes a single prompt
func (ca *ClaudeAgent) ExecutePrompt(ctx context.Context, request PromptRequest) (*PromptResponse, error) {
	var response *PromptResponse
	var err error
	
	task := ca.taskManager.Start(request.Name,
		clicky.WithTimeout(5*time.Minute),
		clicky.WithModel(ca.config.Model),
		clicky.WithPrompt(request.Prompt),
		clicky.WithFunc(func(t *clicky.Task) error {
			t.Infof("Starting Claude request")
			// Start with unknown progress (infinite spinner)
			t.SetProgress(0, 0)
			
			resp, execErr := ca.executeClaude(ctx, request, t)
			if execErr != nil {
				t.Errorf("Claude request failed: %v", execErr)
				return execErr
			}
			
			t.Infof("Completed (%d tokens, $%.6f)", resp.TokensUsed, resp.CostUSD)
			
			response = resp
			return nil
		}))
	
	// Wait for task completion
	for task.Status() == clicky.StatusPending || task.Status() == clicky.StatusRunning {
		select {
		case <-ctx.Done():
			task.Cancel()
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	
	if err = task.Error(); err != nil {
		return nil, err
	}
	
	return response, nil
}

// ExecuteBatch processes multiple prompts
func (ca *ClaudeAgent) ExecuteBatch(ctx context.Context, requests []PromptRequest) (map[string]*PromptResponse, error) {
	results := make(map[string]*PromptResponse)
	resultsChan := make(chan struct {
		name     string
		response *PromptResponse
		err      error
	}, len(requests))
	
	// Create tasks for all requests
	for _, request := range requests {
		req := request // Capture for closure
		
		ca.taskManager.Start(req.Name,
			clicky.WithTimeout(5*time.Minute),
			clicky.WithModel(ca.config.Model),
			clicky.WithPrompt(req.Prompt),
			clicky.WithFunc(func(t *clicky.Task) error {
				t.Infof("Processing request")
				// Start with unknown progress (infinite spinner)
				t.SetProgress(0, 0)
				
				response, err := ca.executeClaude(t.Context(), req, t)
				
				resultsChan <- struct {
					name     string
					response *PromptResponse
					err      error
				}{
					name:     req.Name,
					response: response,
					err:      err,
				}
				
				if err != nil {
					t.Errorf("Failed: %v", err)
					return err
				}
				
				t.Infof("Completed (%d tokens)", response.TokensUsed)
				return nil
			}))
	}
	
	// Collect results
	go func() {
		for i := 0; i < len(requests); i++ {
			select {
			case result := <-resultsChan:
				if result.err == nil {
					results[result.name] = result.response
				}
			case <-ctx.Done():
				ca.taskManager.CancelAll()
				return
			}
		}
	}()
	
	// Wait for all tasks to complete
	exitCode := ca.taskManager.Wait()
	if exitCode != 0 {
		return results, fmt.Errorf("some tasks failed (exit code %d)", exitCode)
	}
	
	return results, nil
}

// executeClaude executes the Claude CLI command
func (ca *ClaudeAgent) executeClaude(ctx context.Context, request PromptRequest, task *clicky.Task) (*PromptResponse, error) {
	args := []string{"-p"}
	
	if ca.config.Model != "" {
		args = append(args, "--model", ca.config.Model)
	}
	
	args = append(args, "--output-format", "json")
	
	if ca.config.StrictMCPConfig {
		args = append(args, "--strict-mcp-config")
	}
	
	// Add context as system message if provided
	prompt := request.Prompt
	if len(request.Context) > 0 {
		var contextParts []string
		for key, value := range request.Context {
			contextParts = append(contextParts, fmt.Sprintf("%s: %s", key, value))
		}
		prompt = fmt.Sprintf("Context:\n%s\n\n%s", strings.Join(contextParts, "\n"), prompt)
	}
	
	args = append(args, prompt)
	
	if task != nil {
		task.Infof("Executing: claude %s", strings.Join(args[:len(args)-1], " "))
	}
	
	startTime := time.Now()
	cmd := exec.CommandContext(ctx, "claude", args...)
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)
	
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("claude CLI cancelled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("claude CLI failed: %w\nOutput: %s", err, string(output))
	}
	
	// Try to parse JSON response
	var claudeResp ClaudeResponse
	if err := json.Unmarshal(output, &claudeResp); err != nil {
		// Fallback to plain text response
		return &PromptResponse{
			Result:     string(output),
			TokensUsed: 0,
			CostUSD:    0,
			DurationMs: int(duration.Milliseconds()),
			Model:      ca.config.Model,
		}, nil
	}
	
	if claudeResp.IsError {
		return nil, fmt.Errorf("claude returned error: %s", claudeResp.Result)
	}
	
	response := &PromptResponse{
		Result:     claudeResp.Result,
		TokensUsed: claudeResp.GetTotalTokens(),
		CostUSD:    claudeResp.TotalCostUSD,
		DurationMs: claudeResp.DurationMs,
		Model:      ca.config.Model,
	}
	
	if task != nil && ca.config.Debug {
		task.Infof("Token usage: input=%d, cache_creation=%d, cache_read=%d, output=%d",
			claudeResp.Usage.InputTokens,
			claudeResp.Usage.CacheCreationInputTokens,
			claudeResp.Usage.CacheReadInputTokens,
			claudeResp.Usage.OutputTokens)
		task.Infof("Cost: $%.6f USD", response.CostUSD)
	}
	
	return response, nil
}

// Close cleans up resources
func (ca *ClaudeAgent) Close() error {
	ca.taskManager.CancelAll()
	return nil
}