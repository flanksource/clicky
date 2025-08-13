package ai

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/flanksource/clicky"
	flanksourceContext "github.com/flanksource/commons/context"
)

// AiderAgent implements the Agent interface for Aider
type AiderAgent struct {
	config AgentConfig
}

// AiderResponse represents the response from Aider
type AiderResponse struct {
	Result    string `json:"result"`
	FilesChanged []string `json:"files_changed,omitempty"`
	Error     string `json:"error,omitempty"`
}

// NewAiderAgent creates a new Aider agent
func NewAiderAgent(config AgentConfig) (*AiderAgent, error) {
	if err := ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	// Check if aider is available
	if _, err := exec.LookPath("aider"); err != nil {
		return nil, fmt.Errorf("aider not found in PATH: %w", err)
	}
	
	// Configure global task manager settings
	clicky.SetGlobalMaxConcurrency(config.MaxConcurrent)
	if config.Debug || config.Verbose {
		clicky.SetGlobalVerbose(true)
	}
	
	return &AiderAgent{
		config: config,
	}, nil
}

// GetType returns the agent type
func (aa *AiderAgent) GetType() AgentType {
	return AgentTypeAider
}

// GetConfig returns the agent configuration
func (aa *AiderAgent) GetConfig() AgentConfig {
	return aa.config
}


// ListModels returns available Aider models
func (aa *AiderAgent) ListModels(ctx context.Context) ([]Model, error) {
	// Get models from aider
	cmd := exec.CommandContext(ctx, "aider", "--models")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get aider models: %w", err)
	}
	
	// Parse the output to extract model information
	models := parseAiderModels(string(output))
	
	return models, nil
}

// ExecutePrompt processes a single prompt
func (aa *AiderAgent) ExecutePrompt(ctx context.Context, request PromptRequest) (*PromptResponse, error) {
	var response *PromptResponse
	var err error
	
	task := clicky.StartGlobalTask(request.Name,
		clicky.WithTimeout(10*time.Minute), // Aider might need more time
		clicky.WithModel(aa.config.Model),
		clicky.WithPrompt(request.Prompt),
		clicky.WithFunc(func(ctx flanksourceContext.Context, t *clicky.Task) error {
			t.Infof("Starting Aider request")
			// Start with unknown progress (infinite spinner)
			t.SetProgress(0, 0)
			
			resp, execErr := aa.executeAider(ctx, request, t)
			if execErr != nil {
				t.Errorf("Aider request failed: %v", execErr)
				return execErr
			}
			
			t.Infof("Completed")
			
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
func (aa *AiderAgent) ExecuteBatch(ctx context.Context, requests []PromptRequest) (map[string]*PromptResponse, error) {
	results := make(map[string]*PromptResponse)
	resultsChan := make(chan struct {
		name     string
		response *PromptResponse
		err      error
	}, len(requests))
	
	// Store tasks to wait for them properly
	var tasks []*clicky.Task
	
	// Create tasks for all requests
	for _, request := range requests {
		req := request // Capture for closure
		
		task := clicky.StartGlobalTask(req.Name,
			clicky.WithTimeout(10*time.Minute),
			clicky.WithModel(aa.config.Model),
			clicky.WithPrompt(req.Prompt),
			clicky.WithFunc(func(ctx flanksourceContext.Context, t *clicky.Task) error {
				t.Infof("Processing request")
				// Start with unknown progress (infinite spinner)
				t.SetProgress(0, 0)
				
				response, err := aa.executeAider(t.Context(), req, t)
				
				// Always send result to channel, even if there's an error
				select {
				case resultsChan <- struct {
					name     string
					response *PromptResponse
					err      error
				}{
					name:     req.Name,
					response: response,
					err:      err,
				}:
				case <-t.Context().Done():
					return t.Context().Err()
				}
				
				if err != nil {
					// Log error but don't fail the task - let it complete as a warning
					t.Warnf("Failed: %v", err)
					return nil // Return nil so task shows as completed with warning
				}
				
				t.Infof("Completed")
				return nil
			}))
		
		tasks = append(tasks, task)
	}
	
	// Wait for all tasks to complete and collect results concurrently
	go func() {
		for _, task := range tasks {
			for task.Status() == clicky.StatusPending || task.Status() == clicky.StatusRunning {
				select {
				case <-ctx.Done():
					return
				case <-time.After(10 * time.Millisecond):
					// Continue polling
				}
			}
		}
		close(resultsChan) // Close channel when all tasks complete
	}()
	
	// Collect results from channel
	for result := range resultsChan {
		if result.err == nil {
			results[result.name] = result.response
		} else {
			// Include failed responses with error information
			results[result.name] = &PromptResponse{
				Error: result.err.Error(),
				Model: aa.config.Model,
			}
		}
	}
	
	// Check for context cancellation
	if ctx.Err() != nil {
		clicky.CancelAllGlobalTasks()
		return results, ctx.Err()
	}
	
	return results, nil
}

// executeAider executes the Aider command
func (aa *AiderAgent) executeAider(ctx context.Context, request PromptRequest, task *clicky.Task) (*PromptResponse, error) {
	args := []string{
		"--yes", // Auto-confirm changes
		"--no-git", // Don't auto-commit
	}
	
	if aa.config.Model != "" {
		args = append(args, "--model", aa.config.Model)
	}
	
	// Set temperature if specified
	if aa.config.Temperature > 0 {
		args = append(args, "--temperature", fmt.Sprintf("%.2f", aa.config.Temperature))
	}
	
	// Add message
	args = append(args, "--message", request.Prompt)
	
	if task != nil {
		task.Infof("Executing: aider %s", strings.Join(args, " "))
	}
	
	startTime := time.Now()
	cmd := exec.CommandContext(ctx, "aider", args...)
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)
	
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("aider cancelled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("aider failed: %w\nOutput: %s", err, string(output))
	}
	
	// Parse the output
	result := strings.TrimSpace(string(output))
	
	response := &PromptResponse{
		Result:     result,
		TokensUsed: 0, // Aider doesn't provide token usage info
		CostUSD:    0, // Aider doesn't provide cost info
		DurationMs: int(duration.Milliseconds()),
		Model:      aa.config.Model,
	}
	
	if task != nil && aa.config.Debug {
		task.Infof("Aider output length: %d characters", len(result))
		task.Infof("Duration: %v", duration)
	}
	
	return response, nil
}

// Close cleans up resources
func (aa *AiderAgent) Close() error {
	clicky.CancelAllGlobalTasks()
	return nil
}

// parseAiderModels parses the output from `aider --models` command
func parseAiderModels(output string) []Model {
	models := []Model{}
	lines := strings.Split(output, "\n")
	
	// Common models that Aider supports
	knownModels := map[string]Model{
		"gpt-4": {
			ID:       "gpt-4",
			Name:     "GPT-4",
			Provider: "openai",
			MaxTokens: 8192,
		},
		"gpt-4-turbo": {
			ID:       "gpt-4-turbo",
			Name:     "GPT-4 Turbo",
			Provider: "openai",
			MaxTokens: 128000,
		},
		"gpt-3.5-turbo": {
			ID:       "gpt-3.5-turbo",
			Name:     "GPT-3.5 Turbo",
			Provider: "openai",
			MaxTokens: 16384,
		},
		"claude-3-opus": {
			ID:       "claude-3-opus-20240229",
			Name:     "Claude 3 Opus",
			Provider: "anthropic",
			MaxTokens: 200000,
		},
		"claude-3-sonnet": {
			ID:       "claude-3-sonnet-20240229",
			Name:     "Claude 3 Sonnet",
			Provider: "anthropic",
			MaxTokens: 200000,
		},
		"claude-3-haiku": {
			ID:       "claude-3-haiku-20240307",
			Name:     "Claude 3 Haiku",
			Provider: "anthropic",
			MaxTokens: 200000,
		},
	}
	
	// Look for model names in the output
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Try to match against known models
		for key, model := range knownModels {
			if strings.Contains(strings.ToLower(line), strings.ToLower(key)) {
				models = append(models, model)
				break
			}
		}
		
		// Also try to extract any model ID directly from the line
		fields := strings.Fields(line)
		if len(fields) > 0 {
			modelID := fields[0]
			// If it's not in our known models, add it as a generic model
			if _, exists := knownModels[modelID]; !exists && !containsModel(models, modelID) {
				models = append(models, Model{
					ID:       modelID,
					Name:     modelID,
					Provider: "unknown",
				})
			}
		}
	}
	
	// If no models found from parsing, return default set
	if len(models) == 0 {
		for _, model := range knownModels {
			models = append(models, model)
		}
	}
	
	return models
}

// containsModel checks if a model with the given ID already exists in the slice
func containsModel(models []Model, id string) bool {
	for _, model := range models {
		if model.ID == id {
			return true
		}
	}
	return false
}