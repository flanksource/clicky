package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Prompt represents an MCP prompt that can be provided to AI assistants
type Prompt struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Arguments   []PromptArgument  `json:"arguments,omitempty"`
	Template    string            `json:"template"`
	Tags        []string          `json:"tags,omitempty"`
	Examples    []string          `json:"examples,omitempty"`
}

// PromptArgument represents an argument for a prompt
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// PromptRegistry manages available prompts
type PromptRegistry struct {
	prompts map[string]*Prompt
	config  *Config
}

// NewPromptRegistry creates a new prompt registry
func NewPromptRegistry(config *Config) *PromptRegistry {
	return &PromptRegistry{
		prompts: make(map[string]*Prompt),
		config:  config,
	}
}

// LoadDefaults loads default prompts for common CLI operations
func (r *PromptRegistry) LoadDefaults() {
	// Help and discovery prompts
	r.Register(&Prompt{
		Name:        "help",
		Description: "Get help about available commands",
		Template: `Please help me understand what commands are available and how to use them. 
List the main commands and their purposes.`,
		Tags: []string{"discovery", "help"},
	})
	
	r.Register(&Prompt{
		Name:        "explore",
		Description: "Explore specific functionality",
		Arguments: []PromptArgument{
			{Name: "feature", Description: "Feature or command to explore", Required: true},
		},
		Template: `Help me explore the {{.feature}} functionality. 
Show me what it can do and provide some examples of common use cases.`,
		Tags: []string{"discovery", "learning"},
	})
	
	// Task-oriented prompts
	r.Register(&Prompt{
		Name:        "task",
		Description: "Execute a specific task",
		Arguments: []PromptArgument{
			{Name: "task", Description: "Task description", Required: true},
			{Name: "context", Description: "Additional context", Required: false},
		},
		Template: `I need to {{.task}}.
{{if .context}}Context: {{.context}}{{end}}
Please help me accomplish this using the available tools.`,
		Tags: []string{"task", "execution"},
	})
	
	r.Register(&Prompt{
		Name:        "debug",
		Description: "Debug an issue",
		Arguments: []PromptArgument{
			{Name: "issue", Description: "Issue description", Required: true},
			{Name: "error", Description: "Error message if available", Required: false},
		},
		Template: `I'm experiencing an issue: {{.issue}}
{{if .error}}Error message: {{.error}}{{end}}
Please help me debug and resolve this issue.`,
		Tags: []string{"debugging", "troubleshooting"},
	})
	
	// Analysis prompts
	r.Register(&Prompt{
		Name:        "analyze",
		Description: "Analyze data or output",
		Arguments: []PromptArgument{
			{Name: "target", Description: "What to analyze", Required: true},
			{Name: "aspect", Description: "Specific aspect to focus on", Required: false},
		},
		Template: `Please analyze {{.target}}{{if .aspect}}, focusing on {{.aspect}}{{end}}.
Provide insights and recommendations based on the analysis.`,
		Tags: []string{"analysis", "insights"},
	})
	
	r.Register(&Prompt{
		Name:        "optimize",
		Description: "Optimize performance or configuration",
		Arguments: []PromptArgument{
			{Name: "target", Description: "What to optimize", Required: true},
			{Name: "goal", Description: "Optimization goal", Required: false, Default: "performance"},
		},
		Template: `Help me optimize {{.target}} for {{.goal}}.
Analyze the current state and suggest improvements.`,
		Tags: []string{"optimization", "performance"},
	})
	
	// Workflow prompts
	r.Register(&Prompt{
		Name:        "workflow",
		Description: "Create a workflow for a complex task",
		Arguments: []PromptArgument{
			{Name: "goal", Description: "End goal", Required: true},
			{Name: "constraints", Description: "Any constraints or requirements", Required: false},
		},
		Template: `I want to achieve: {{.goal}}
{{if .constraints}}Constraints: {{.constraints}}{{end}}
Please create a step-by-step workflow using the available tools.`,
		Tags:     []string{"workflow", "automation"},
		Examples: []string{
			"workflow --goal 'deploy application to production' --constraints 'zero downtime'",
			"workflow --goal 'generate monthly reports'",
		},
	})
	
	// Batch operations
	r.Register(&Prompt{
		Name:        "batch",
		Description: "Perform batch operations",
		Arguments: []PromptArgument{
			{Name: "operation", Description: "Operation to perform", Required: true},
			{Name: "targets", Description: "Target items (comma-separated)", Required: true},
		},
		Template: `Please perform the following operation in batch: {{.operation}}
Targets: {{.targets}}
Execute efficiently and report the results.`,
		Tags: []string{"batch", "automation"},
	})
	
	// Monitoring and status
	r.Register(&Prompt{
		Name:        "monitor",
		Description: "Monitor system or process status",
		Arguments: []PromptArgument{
			{Name: "target", Description: "What to monitor", Required: true},
			{Name: "interval", Description: "Check interval", Required: false, Default: "once"},
		},
		Template: `Monitor {{.target}} {{if ne .interval "once"}}every {{.interval}}{{end}}.
Report the current status and any issues found.`,
		Tags: []string{"monitoring", "status"},
	})
	
	// Report generation
	r.Register(&Prompt{
		Name:        "report",
		Description: "Generate a report",
		Arguments: []PromptArgument{
			{Name: "type", Description: "Type of report", Required: true},
			{Name: "period", Description: "Time period", Required: false},
			{Name: "format", Description: "Output format", Required: false, Default: "summary"},
		},
		Template: `Generate a {{.type}} report{{if .period}} for {{.period}}{{end}}.
Format: {{.format}}
Include relevant metrics, trends, and recommendations.`,
		Tags:     []string{"reporting", "analysis"},
		Examples: []string{
			"report --type 'performance' --period 'last week' --format 'detailed'",
			"report --type 'security audit'",
		},
	})
}

// Register adds a prompt to the registry
func (r *PromptRegistry) Register(prompt *Prompt) {
	r.prompts[prompt.Name] = prompt
}

// Get retrieves a prompt by name
func (r *PromptRegistry) Get(name string) (*Prompt, bool) {
	prompt, exists := r.prompts[name]
	return prompt, exists
}

// List returns all available prompts
func (r *PromptRegistry) List() []*Prompt {
	prompts := make([]*Prompt, 0, len(r.prompts))
	for _, p := range r.prompts {
		prompts = append(prompts, p)
	}
	return prompts
}

// ListByTag returns prompts with a specific tag
func (r *PromptRegistry) ListByTag(tag string) []*Prompt {
	var prompts []*Prompt
	for _, p := range r.prompts {
		for _, t := range p.Tags {
			if strings.EqualFold(t, tag) {
				prompts = append(prompts, p)
				break
			}
		}
	}
	return prompts
}

// LoadFromFile loads prompts from a JSON file
func (r *PromptRegistry) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read prompts file: %w", err)
	}
	
	var prompts []*Prompt
	if err := json.Unmarshal(data, &prompts); err != nil {
		return fmt.Errorf("failed to parse prompts file: %w", err)
	}
	
	for _, p := range prompts {
		r.Register(p)
	}
	
	return nil
}

// SaveToFile saves prompts to a JSON file
func (r *PromptRegistry) SaveToFile(path string) error {
	prompts := r.List()
	
	data, err := json.MarshalIndent(prompts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal prompts: %w", err)
	}
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write prompts file: %w", err)
	}
	
	return nil
}

// GetPromptsPath returns the default prompts file path
func GetPromptsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "clicky", "mcp-prompts.json")
}

// ListPromptsResponse represents the MCP prompts/list response
type ListPromptsResponse struct {
	Prompts []Prompt `json:"prompts"`
}

// GetPromptResponse represents the MCP prompts/get response
type GetPromptResponse struct {
	Description string           `json:"description"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Messages    []PromptMessage  `json:"messages"`
}

// PromptMessage represents a message in a prompt
type PromptMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// ToMCPResponse converts a prompt to MCP response format
func (p *Prompt) ToMCPResponse(args map[string]string) *GetPromptResponse {
	// Apply template with arguments
	content := p.Template
	for key, value := range args {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		content = strings.ReplaceAll(content, placeholder, value)
	}
	
	// Handle conditionals (simple implementation)
	// Remove unfilled conditionals
	content = removeUnfilledConditionals(content)
	
	return &GetPromptResponse{
		Description: p.Description,
		Arguments:   p.Arguments,
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: content,
			},
		},
	}
}

// removeUnfilledConditionals removes template conditionals that weren't filled
func removeUnfilledConditionals(content string) string {
	// Simple regex-like removal of {{if ...}}...{{end}} blocks with unfilled variables
	// This is a simplified implementation
	for {
		start := strings.Index(content, "{{if ")
		if start == -1 {
			break
		}
		
		end := strings.Index(content[start:], "{{end}}")
		if end == -1 {
			break
		}
		
		// Check if the condition has unfilled variables (contains {{.)
		block := content[start : start+end+7]
		if strings.Contains(block, "{{.") {
			// Remove the entire conditional block
			content = content[:start] + content[start+end+7:]
		} else {
			// Keep the content between {{if}} and {{end}}
			ifEnd := strings.Index(block, "}}")
			if ifEnd != -1 {
				innerContent := block[ifEnd+2 : len(block)-7]
				content = content[:start] + innerContent + content[start+end+7:]
			} else {
				break
			}
		}
	}
	
	// Clean up any remaining template variables
	for {
		start := strings.Index(content, "{{")
		if start == -1 {
			break
		}
		end := strings.Index(content[start:], "}}")
		if end == -1 {
			break
		}
		content = content[:start] + content[start+end+2:]
	}
	
	// Clean up extra whitespace
	lines := strings.Split(content, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, line)
		}
	}
	
	return strings.Join(cleaned, "\n")
}