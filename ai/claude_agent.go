package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/ai/cache"
)

// ClaudeAgent implements the Agent interface for Claude
type ClaudeAgent struct {
	config AgentConfig
	cache  *cache.Cache
}

// NewClaudeAgent creates a new Claude agent
func NewClaudeAgent(config AgentConfig) (*ClaudeAgent, error) {
	if err := ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	// Configure global task manager settings
	clicky.SetGlobalMaxConcurrency(config.MaxConcurrent)
	if config.Debug || config.Verbose {
		clicky.SetGlobalVerbose(true)
	}
	
	agent := &ClaudeAgent{
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
				fmt.Fprintf(os.Stderr, "Warning: Failed to initialize AI cache: %v\n", err)
			}
		} else {
			agent.cache = c
		}
	}
	
	return agent, nil
}

// GetType returns the agent type
func (ca *ClaudeAgent) GetType() AgentType {
	return AgentTypeClaude
}

// GetConfig returns the agent configuration
func (ca *ClaudeAgent) GetConfig() AgentConfig {
	return ca.config
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
	
	task := clicky.StartGlobalTask(request.Name,
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
	
	// Store tasks to wait for them properly  
	var tasks []*clicky.Task
	
	// Create tasks for all requests
	for _, request := range requests {
		req := request // Capture for closure
		
		task := clicky.StartGlobalTask(req.Name,
			clicky.WithTimeout(5*time.Minute),
			clicky.WithModel(ca.config.Model),
			clicky.WithPrompt(req.Prompt),
			clicky.WithFunc(func(t *clicky.Task) error {
				t.Infof("Processing request")
				// Start with unknown progress (infinite spinner)
				t.SetProgress(0, 0)
				
				response, err := ca.executeClaude(t.Context(), req, t)
				
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
				
				t.Infof("Completed (%d tokens)", response.TokensUsed)
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
				Model: ca.config.Model,
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

// executeClaude executes the Claude CLI command with caching
func (ca *ClaudeAgent) executeClaude(ctx context.Context, request PromptRequest, task *clicky.Task) (*PromptResponse, error) {
	// Add context to prompt if provided
	prompt := request.Prompt
	if len(request.Context) > 0 {
		var contextParts []string
		for key, value := range request.Context {
			contextParts = append(contextParts, fmt.Sprintf("%s: %s", key, value))
		}
		prompt = fmt.Sprintf("Context:\n%s\n\n%s", strings.Join(contextParts, "\n"), prompt)
	}
	
	// Check cache first
	if ca.cache != nil {
		entry, err := ca.cache.Get(prompt, ca.config.Model, ca.config.Temperature, ca.config.MaxTokens)
		if err == nil && entry != nil && entry.Error == "" {
			// Cache hit
			if task != nil {
				task.Infof("Cache hit for prompt (saved $%.6f)", entry.CostUSD)
			}
			return &PromptResponse{
				Result:           entry.Response,
				TokensUsed:       entry.TokensTotal,
				TokensInput:      entry.TokensInput,
				TokensOutput:     entry.TokensOutput,
				TokensCacheRead:  entry.TokensCacheRead,
				TokensCacheWrite: entry.TokensCacheWrite,
				CostUSD:          0, // No cost for cached response
				DurationMs:       0, // No time for cached response
				CacheHit:         true,
				Model:            ca.config.Model,
			}, nil
		}
	}
	
	// Build Claude CLI arguments
	args := []string{"-p"}
	
	if ca.config.Model != "" {
		args = append(args, "--model", ca.config.Model)
	}
	
	args = append(args, "--output-format", "json")
	
	if ca.config.StrictMCPConfig {
		args = append(args, "--strict-mcp-config")
	}
	
	args = append(args, prompt)
	
	if task != nil {
		task.Infof("Executing: claude %s", strings.Join(args[:len(args)-1], " "))
	}
	
	startTime := time.Now()
	cmd := exec.CommandContext(ctx, "claude", args...)
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)
	
	// Prepare cache entry
	cacheEntry := &cache.Entry{
		Model:       ca.config.Model,
		Prompt:      prompt,
		Temperature: ca.config.Temperature,
		MaxTokens:   ca.config.MaxTokens,
		DurationMS:  int64(duration.Milliseconds()),
		ProjectName: ca.config.ProjectName,
		TaskName:    request.Name,
		SessionID:   ca.config.SessionID,
		CreatedAt:   time.Now(),
	}
	
	if err != nil {
		errMsg := fmt.Sprintf("claude CLI failed: %v", err)
		if ctx.Err() != nil {
			errMsg = fmt.Sprintf("claude CLI cancelled: %v", ctx.Err())
		}
		
		// Cache the error
		cacheEntry.Error = errMsg
		if ca.cache != nil {
			ca.cache.Set(cacheEntry)
		}
		
		return nil, fmt.Errorf("%s\nOutput: %s", errMsg, string(output))
	}
	
	// Try to parse JSON response
	var claudeResp ClaudeResponse
	if err := json.Unmarshal(output, &claudeResp); err != nil {
		// Fallback to plain text response
		response := &PromptResponse{
			Result:     string(output),
			TokensUsed: 0,
			CostUSD:    0,
			DurationMs: int(duration.Milliseconds()),
			Model:      ca.config.Model,
		}
		
		// Cache the response
		cacheEntry.Response = response.Result
		if ca.cache != nil {
			ca.cache.Set(cacheEntry)
		}
		
		return response, nil
	}
	
	if claudeResp.IsError {
		errMsg := fmt.Sprintf("claude returned error: %s", claudeResp.Result)
		cacheEntry.Error = errMsg
		if ca.cache != nil {
			ca.cache.Set(cacheEntry)
		}
		return nil, fmt.Errorf(errMsg)
	}
	
	// Build response with detailed token information
	response := &PromptResponse{
		Result:           claudeResp.Result,
		TokensUsed:       claudeResp.GetTotalTokens(),
		TokensInput:      claudeResp.Usage.InputTokens,
		TokensOutput:     claudeResp.Usage.OutputTokens,
		TokensCacheRead:  claudeResp.Usage.CacheReadInputTokens,
		TokensCacheWrite: claudeResp.Usage.CacheCreationInputTokens,
		CostUSD:          claudeResp.TotalCostUSD,
		DurationMs:       claudeResp.DurationMs,
		Model:            ca.config.Model,
		CacheHit:         false,
	}
	
	// Update cache entry with successful response
	cacheEntry.Response = response.Result
	cacheEntry.TokensInput = response.TokensInput
	cacheEntry.TokensOutput = response.TokensOutput
	cacheEntry.TokensCacheRead = response.TokensCacheRead
	cacheEntry.TokensCacheWrite = response.TokensCacheWrite
	cacheEntry.TokensTotal = response.TokensUsed
	cacheEntry.CostUSD = response.CostUSD
	
	// Save to cache
	if ca.cache != nil {
		if err := ca.cache.Set(cacheEntry); err != nil && ca.config.Debug {
			fmt.Fprintf(os.Stderr, "Warning: Failed to cache response: %v\n", err)
		}
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
	// Cancel any tasks this agent started
	clicky.CancelAllGlobalTasks()
	
	// Close cache if initialized
	if ca.cache != nil {
		return ca.cache.Close()
	}
	
	return nil
}