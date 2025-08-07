package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky/ai"
)

func main() {
	fmt.Printf("=== AI Agent Demo ===\n\n")

	// Create default config
	config := ai.DefaultConfig()
	
	// Override with environment or args
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "aider":
			config.Type = ai.AgentTypeAider
			config.Model = "gpt-4"
		case "claude":
			config.Type = ai.AgentTypeClaude
			config.Model = "claude-3-5-sonnet-20241022"
		case "list-models":
			demoListModels(config)
			return
		case "batch":
			demoBatch(config)
			return
		}
	}
	
	fmt.Printf("Using %s agent with model: %s\n\n", config.Type, config.Model)

	// Get the configured agent
	am := ai.NewAgentManager(config)
	defer am.Close()
	
	agent, err := am.GetDefaultAgent()
	if err != nil {
		fmt.Printf("Failed to create agent: %v\n", err)
		return
	}
	
	fmt.Printf("Successfully created %s agent\n", agent.GetType())
	
	// If we have a prompt argument, execute it
	if len(os.Args) > 2 {
		prompt := os.Args[2]
		fmt.Printf("\n--- Executing Prompt ---\n")
		fmt.Printf("Prompt: %s\n\n", prompt)
		
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		
		request := ai.PromptRequest{
			Name:   "Demo Request",
			Prompt: prompt,
		}
		
		response, err := agent.ExecutePrompt(ctx, request)
		if err != nil {
			fmt.Printf("Error executing prompt: %v\n", err)
			return
		}
		
		// Give a moment for the user to see the completed task status
		time.Sleep(2 * time.Second)
		
		fmt.Printf("--- Response ---\n")
		fmt.Printf("Result: %s\n", response.Result)
		if response.TokensUsed > 0 {
			fmt.Printf("Tokens: %d\n", response.TokensUsed)
		}
		if response.CostUSD > 0 {
			fmt.Printf("Cost: $%.6f\n", response.CostUSD)
		}
		if response.DurationMs > 0 {
			fmt.Printf("Duration: %dms\n", response.DurationMs)
		}
	} else {
		fmt.Printf("\nUsage: %s [mode] [prompt]\n", os.Args[0])
		fmt.Printf("  mode: claude, aider, list-models, or batch (default: claude)\n")
		fmt.Printf("  prompt: text to send to the AI (required unless mode is list-models or batch)\n")
		fmt.Printf("\nExamples:\n")
		fmt.Printf("  %s claude \"What is the capital of France?\"\n", os.Args[0])
		fmt.Printf("  %s aider \"How do I fix this bug?\"\n", os.Args[0])
		fmt.Printf("  %s list-models\n", os.Args[0])
		fmt.Printf("  %s batch\n", os.Args[0])
	}
}

func demoListModels(config ai.AgentConfig) {
	fmt.Printf("--- Listing Available Models ---\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := ai.ListModels(ctx, &config); err != nil {
		fmt.Printf("Error listing models: %v\n", err)
	}
}

func demoBatch(config ai.AgentConfig) {
	fmt.Printf("=== Batch Processing Demo (Max 3 concurrent) ===\n\n")
	
	config.MaxConcurrent = 3
	config.Debug = false
	
	am := ai.NewAgentManager(config)
	defer am.Close()
	
	agent, err := am.GetDefaultAgent()
	if err != nil {
		fmt.Printf("Failed to create agent: %v\n", err)
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create 5 prompts that will be processed with max 3 concurrent
	requests := make([]ai.PromptRequest, 0, 5)
	for i := 1; i <= 5; i++ {
		requests = append(requests, ai.PromptRequest{
			Name:   fmt.Sprintf("Math Task %d", i),
			Prompt: fmt.Sprintf("What is %d squared? Reply with just the number.", i),
		})
	}

	fmt.Printf("Processing %d prompts with max concurrency of %d...\n\n", len(requests), config.MaxConcurrent)

	responses, err := agent.ExecuteBatch(ctx, requests)
	if err != nil {
		fmt.Printf("Batch execution had errors: %v\n", err)
	}

	// Give a moment for the user to see all completed task statuses
	time.Sleep(3 * time.Second)

	fmt.Println("\n=== Results ===")
	totalTokens := 0
	totalCost := 0.0
	
	for _, request := range requests {
		if response, ok := responses[request.Name]; ok {
			fmt.Printf("%s: %s", request.Name, response.Result)
			if response.TokensUsed > 0 {
				fmt.Printf(" (tokens: %d", response.TokensUsed)
				totalTokens += response.TokensUsed
			}
			if response.CostUSD > 0 {
				fmt.Printf(", cost: $%.6f", response.CostUSD)
				totalCost += response.CostUSD
			}
			fmt.Printf(")\n")
		} else {
			fmt.Printf("%s: Failed\n", request.Name)
		}
	}
	
	if totalTokens > 0 {
		fmt.Printf("\nTotal tokens: %d\n", totalTokens)
	}
	if totalCost > 0 {
		fmt.Printf("Total cost: $%.6f\n", totalCost)
	}
}