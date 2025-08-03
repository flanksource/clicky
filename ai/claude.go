package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/flanksource/clicky"
	flanksourceContext "github.com/flanksource/commons/context"
)

// ClaudeOptions contains configuration for Claude API calls
type ClaudeOptions struct {
	Model            string
	MaxTokens        int
	Debug            bool
	StrictMCPConfig  bool
	OutputFormat     string
	MaxConcurrent    int
}

// ClaudeResponse represents the response from Claude CLI
type ClaudeResponse struct {
	Type         string  `json:"type"`
	Result       string  `json:"result"`
	IsError      bool    `json:"is_error"`
	DurationMs   int     `json:"duration_ms"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		OutputTokens             int `json:"output_tokens"`
	} `json:"usage"`
}

// ClaudeExecutor manages Claude API calls with TaskManager integration
type ClaudeExecutor struct {
	options     ClaudeOptions
	taskManager *clicky.TaskManager
}

// NewClaudeExecutor creates a new Claude executor
func NewClaudeExecutor(options ClaudeOptions) *ClaudeExecutor {
	tm := clicky.NewTaskManagerWithConcurrency(options.MaxConcurrent)
	if options.Debug {
		tm.SetVerbose(true)
	}
	
	return &ClaudeExecutor{
		options:     options,
		taskManager: tm,
	}
}

// GetTaskManager returns the underlying task manager
func (ce *ClaudeExecutor) GetTaskManager() *clicky.TaskManager {
	return ce.taskManager
}

// ExecutePrompt executes a single Claude prompt with progress tracking
func (ce *ClaudeExecutor) ExecutePrompt(ctx context.Context, name string, prompt string) (*ClaudeResponse, error) {
	task := ce.taskManager.Start(name, clicky.WithFunc(func(ctx flanksourceContext.Context, t *clicky.Task) error {
		t.Infof("Starting Claude API call")
		t.SetProgress(10, 100)
		
		response, err := ce.executeClaudeCLI(ctx, prompt, t)
		if err != nil {
			t.Errorf("Claude API failed: %v", err)
			return err
		}
		
		t.SetProgress(100, 100)
		t.Infof("Received response (%d tokens)", response.GetTotalTokens())
		
		// Store response in task context
		t.SetStatus(fmt.Sprintf("%s (completed)", name))
		return nil
	}))
	
	// Wait for this specific task
	for task.Status() == clicky.StatusPending || task.Status() == clicky.StatusRunning {
		select {
		case <-ctx.Done():
			task.Cancel()
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	
	if err := task.Error(); err != nil {
		return nil, err
	}
	
	// Execute again to get the actual response (since we can't store it in the task)
	return ce.executeClaudeCLI(ctx, prompt, nil)
}

// ExecutePromptBatch executes multiple prompts in parallel with concurrency control
func (ce *ClaudeExecutor) ExecutePromptBatch(ctx context.Context, prompts map[string]string) (map[string]*ClaudeResponse, error) {
	results := make(map[string]*ClaudeResponse)
	resultsChan := make(chan struct {
		name     string
		response *ClaudeResponse
		err      error
	}, len(prompts))
	
	// Create tasks for all prompts
	for name, prompt := range prompts {
		taskName := name
		taskPrompt := prompt
		
		ce.taskManager.Start(taskName, 
			clicky.WithTimeout(5*time.Minute),
			clicky.WithFunc(func(ctx flanksourceContext.Context, t *clicky.Task) error {
				t.Infof("Processing prompt")
				t.SetProgress(0, 100)
				
				response, err := ce.executeClaudeCLI(t.Context(), taskPrompt, t)
				
				resultsChan <- struct {
					name     string
					response *ClaudeResponse
					err      error
				}{
					name:     taskName,
					response: response,
					err:      err,
				}
				
				if err != nil {
					t.Errorf("Failed: %v", err)
					return err
				}
				
				t.SetProgress(100, 100)
				t.Infof("Completed (%d tokens)", response.GetTotalTokens())
				return nil
			}))
	}
	
	// Collect results
	go func() {
		for i := 0; i < len(prompts); i++ {
			select {
			case result := <-resultsChan:
				if result.err == nil {
					results[result.name] = result.response
				}
			case <-ctx.Done():
				ce.taskManager.CancelAll()
				return
			}
		}
	}()
	
	// Wait for all tasks to complete
	exitCode := ce.taskManager.Wait()
	if exitCode != 0 {
		return results, fmt.Errorf("some tasks failed (exit code %d)", exitCode)
	}
	
	return results, nil
}

// executeClaudeCLI executes the Claude CLI command
func (ce *ClaudeExecutor) executeClaudeCLI(ctx context.Context, prompt string, task *clicky.Task) (*ClaudeResponse, error) {
	args := []string{"-p"}
	
	if ce.options.Model != "" {
		args = append(args, "--model", ce.options.Model)
	}
	
	if ce.options.OutputFormat != "" {
		args = append(args, "--output-format", ce.options.OutputFormat)
	} else {
		args = append(args, "--output-format", "json")
	}
	
	if ce.options.StrictMCPConfig {
		args = append(args, "--strict-mcp-config")
	}
	
	args = append(args, prompt)
	
	if task != nil {
		task.Infof("Executing: claude %s", strings.Join(args[:2], " "))
		task.SetProgress(30, 100)
	}
	
	cmd := exec.CommandContext(ctx, "claude", args...)
	output, err := cmd.CombinedOutput()
	
	if task != nil {
		task.SetProgress(80, 100)
	}
	
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("claude CLI cancelled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("claude CLI failed: %w\nOutput: %s", err, string(output))
	}
	
	// Try to parse JSON response
	var response ClaudeResponse
	if err := json.Unmarshal(output, &response); err != nil {
		// Fallback to plain text response
		return &ClaudeResponse{
			Type:   "text",
			Result: string(output),
		}, nil
	}
	
	if response.IsError {
		return nil, fmt.Errorf("claude returned error: %s", response.Result)
	}
	
	if task != nil && ce.options.Debug {
		task.Infof("Token usage: input=%d, cache_creation=%d, cache_read=%d, output=%d",
			response.Usage.InputTokens,
			response.Usage.CacheCreationInputTokens,
			response.Usage.CacheReadInputTokens,
			response.Usage.OutputTokens)
		task.Infof("Cost: $%.6f USD", response.TotalCostUSD)
	}
	
	return &response, nil
}

// GetTotalTokens returns the total billable tokens (excluding cache reads)
func (r *ClaudeResponse) GetTotalTokens() int {
	return r.Usage.InputTokens +
		r.Usage.CacheCreationInputTokens +
		r.Usage.OutputTokens
}

// GetAllTokens returns all tokens including cache reads
func (r *ClaudeResponse) GetAllTokens() int {
	return r.GetTotalTokens() + r.Usage.CacheReadInputTokens
}