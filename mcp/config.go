package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/flanksource/clicky/formatters"
)

// Config holds MCP server configuration
type Config struct {
	// Server settings
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`

	// Transport configuration
	Transport TransportConfig `json:"transport"`

	// Security settings
	Security SecurityConfig `json:"security"`

	// Tool exposure settings
	Tools ToolsConfig `json:"tools"`
}

// TransportConfig defines how the MCP server communicates
type TransportConfig struct {
	Type    string `json:"type"`    // "stdio" or "http"
	Address string `json:"address"` // For HTTP transport
	Port    int    `json:"port"`    // For HTTP transport
}

// SecurityConfig defines security and permission settings
type SecurityConfig struct {
	// Require user confirmation for tool invocations
	RequireConfirmation bool `json:"require_confirmation"`

	// Allowed commands (if empty, all commands are allowed)
	AllowedCommands []string `json:"allowed_commands"`

	// Blocked commands
	BlockedCommands []string `json:"blocked_commands"`

	// Enable audit logging
	AuditLog bool `json:"audit_log"`

	// Maximum execution time for tools
	TimeoutSeconds int `json:"timeout_seconds"`
}

// ToolsConfig defines which cobra commands to expose as MCP tools
type ToolsConfig struct {
	// Auto-expose all commands
	AutoExpose bool `json:"auto_expose"`

	// Include pattern for command names
	Include []string `json:"include"`

	// Exclude pattern for command names
	Exclude []string `json:"exclude"`

	// Override descriptions for specific commands
	Descriptions map[string]string `json:"descriptions,omitempty"`

	// IgnoredParams strips parameters (flags or positional names) from
	// the MCP-facing tool schema. Each rule applies to tools whose name
	// matches ToolGlob (path.Match semantics; "*" matches all). Names
	// in Params may include the "--" prefix or omit it.
	// MCP-only — does not affect OpenAPI generation.
	IgnoredParams []IgnoredParamRule `json:"ignored_params,omitempty"`

	// Format overrides the format/color settings of every tool execution
	// by setting the corresponding cobra flags on the command before it
	// runs. Useful for forcing Markdown without ANSI for AI clients.
	Format *formatters.FormatOptions `json:"format,omitempty"`
}

// IgnoredParamRule describes a per-tool parameter exclusion. ToolGlob
// uses path.Match syntax, so "*" matches everything, "ai *" matches all
// subcommands of "ai", and an exact name like "status" matches only itself.
type IgnoredParamRule struct {
	ToolGlob string   `json:"tool_glob"`
	Params   []string `json:"params"`
}

// DefaultConfig returns a secure default configuration
func DefaultConfig() *Config {
	return &Config{
		Name:        "clicky-mcp-server",
		Description: "Clicky MCP Server - exposes CLI commands as MCP tools",
		Version:     "1.0.0",
		Transport: TransportConfig{
			Type: "stdio",
		},
		Security: SecurityConfig{
			RequireConfirmation: true,
			AuditLog:            true,
			TimeoutSeconds:      30,
		},
		Tools: ToolsConfig{
			AutoExpose: false,
			Include:    []string{".*"},  // Include all by default
			Exclude:    []string{"mcp"}, // Exclude MCP commands themselves
		},
	}
}

// LoadConfig loads configuration from file, creating default if not found
func LoadConfig(configPath string) (*Config, error) {
	// Create config directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// If config file doesn't exist, create default
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultConfig()
		if err := SaveConfig(config, configPath); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
		return config, nil
	}

	// Load existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfigPath returns the default config file path for the legacy
// shared "clicky" namespace. New callers should use GetConfigPathFor with
// the host application's name so multiple clicky-based MCP servers don't
// stomp on each other's config files.
func GetConfigPath() string {
	return GetConfigPathFor("clicky")
}

// GetConfigPathFor returns the default config file path for the given
// application name. A clicky-based MCP server should pass its CLI name
// (e.g. "gavel") so each app gets its own ~/.config/<app>/mcp-config.json.
func GetConfigPathFor(appName string) string {
	if appName == "" {
		appName = "clicky"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", appName, "mcp-config.json")
}
