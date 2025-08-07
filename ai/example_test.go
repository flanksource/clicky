package ai

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestClaudeExecutor(t *testing.T) {
	// Skip if not in CI or explicit test mode
	if testing.Short() {
		t.Skip("Skipping Claude API test in short mode")
	}

	options := ClaudeOptions{
		Model:         "claude-3-haiku-20240307",
		MaxConcurrent: 2,
		Debug:         true,
	}

	executor := NewClaudeExecutor(options)
	ctx := context.Background()

	// Test single prompt execution
	t.Run("SinglePrompt", func(t *testing.T) {
		response, err := executor.ExecutePrompt(ctx, "Math question", "What is 2+2? Reply with just the number.")
		if err != nil {
			t.Fatalf("Failed to execute prompt: %v", err)
		}

		if response.Result == "" {
			t.Error("Expected non-empty result")
		}

		fmt.Printf("Response: %s\n", response.Result)
		fmt.Printf("Tokens used: %d\n", response.GetTotalTokens())
	})

	// Test batch execution with concurrency control
	t.Run("BatchPrompts", func(t *testing.T) {
		prompts := map[string]string{
			"Math 1":   "What is 10 + 5? Reply with just the number.",
			"Math 2":   "What is 20 - 8? Reply with just the number.",
			"Math 3":   "What is 3 * 7? Reply with just the number.",
			"Math 4":   "What is 100 / 4? Reply with just the number.",
			"Math 5":   "What is 2^3? Reply with just the number.",
		}

		executor := NewClaudeExecutor(options)
		responses, err := executor.ExecutePromptBatch(ctx, prompts)
		if err != nil {
			t.Errorf("Batch execution had errors: %v", err)
		}

		for name, response := range responses {
			fmt.Printf("%s: %s (tokens: %d)\n", name, response.Result, response.GetTotalTokens())
		}

		if len(responses) != len(prompts) {
			t.Errorf("Expected %d responses, got %d", len(prompts), len(responses))
		}
	})

	// Test cancellation
	t.Run("Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		executor := NewClaudeExecutor(options)
		
		// This should be cancelled before completion
		_, err := executor.ExecutePrompt(ctx, "Long task", "Write a 1000 word essay about space exploration.")
		
		if err == nil {
			t.Error("Expected cancellation error")
		}
	})
}

func ExampleClaudeExecutor() {
	options := ClaudeOptions{
		Model:         "claude-3-haiku-20240307",
		MaxConcurrent: 3,
		Debug:         false,
	}

	executor := NewClaudeExecutor(options)
	ctx := context.Background()

	// Execute multiple prompts with concurrency control
	prompts := map[string]string{
		"Greeting":    "Say hello in French",
		"Translation": "Translate 'goodbye' to Spanish",
		"Math":        "What is the square root of 144?",
	}

	responses, err := executor.ExecutePromptBatch(ctx, prompts)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	for name, response := range responses {
		fmt.Printf("%s: %s\n", name, response.Result)
	}
}