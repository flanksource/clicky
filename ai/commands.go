package ai

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

// ListModels lists all available models from configured AI agents
func ListModels(ctx context.Context, config *AgentConfig) error {
	am := NewAgentManager(*config)
	defer am.Close()

	fmt.Printf("Available AI Models:\n\n")

	// Get models from all agents
	allModels, err := am.ListAllModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	if len(allModels) == 0 {
		fmt.Printf("No AI agents available. Make sure Claude CLI or Aider is installed.\n")
		return nil
	}

	// Create a tab writer for formatted output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "AGENT\tMODEL ID\tNAME\tPROVIDER\tMAX TOKENS\tINPUT PRICE\tOUTPUT PRICE\n")
	fmt.Fprintf(w, "-----\t--------\t----\t--------\t----------\t-----------\t------------\n")

	// Sort agent types for consistent output
	agentTypes := make([]AgentType, 0, len(allModels))
	for agentType := range allModels {
		agentTypes = append(agentTypes, agentType)
	}
	sort.Slice(agentTypes, func(i, j int) bool {
		return string(agentTypes[i]) < string(agentTypes[j])
	})

	// Display models grouped by agent
	for _, agentType := range agentTypes {
		models := allModels[agentType]

		// Sort models by ID for consistent output
		sort.Slice(models, func(i, j int) bool {
			return models[i].ID < models[j].ID
		})

		for _, model := range models {
			inputPrice := formatPrice(model.InputPrice)
			outputPrice := formatPrice(model.OutputPrice)
			maxTokens := formatTokens(model.MaxTokens)

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				strings.ToUpper(string(agentType)),
				model.ID,
				model.Name,
				model.Provider,
				maxTokens,
				inputPrice,
				outputPrice)
		}
	}

	w.Flush()

	// Show current configuration
	fmt.Printf("\nCurrent Configuration:\n")
	fmt.Printf("  Default Agent: %s\n", config.Type)
	if config.Model != "" {
		fmt.Printf("  Default Model: %s\n", config.Model)
	}
	if config.MaxTokens > 0 {
		fmt.Printf("  Max Tokens: %d\n", config.MaxTokens)
	}
	if config.Temperature > 0 {
		fmt.Printf("  Temperature: %.2f\n", config.Temperature)
	}

	return nil
}

// formatPrice formats a price per token for display
func formatPrice(price float64) string {
	if price == 0 {
		return "N/A"
	}
	// Convert to price per million tokens for readability
	pricePerMillion := price * 1000000
	if pricePerMillion >= 1 {
		return fmt.Sprintf("$%.2f/1M", pricePerMillion)
	} else {
		return fmt.Sprintf("$%.3f/1M", pricePerMillion)
	}
}

// formatTokens formats token count for display
func formatTokens(tokens int) string {
	if tokens == 0 {
		return "N/A"
	}
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	} else if tokens >= 1000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1000)
	} else {
		return fmt.Sprintf("%d", tokens)
	}
}
