package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInstallCommand(opts *CommandOptions) *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install as MCP server in Claude Code settings",
		Long: `Register this CLI as an MCP server in .mcp.json.

Adds an mcpServers entry so MCP clients can discover and use this CLI's commands as tools.

Examples:
  app mcp install              # Install in project .mcp.json
  app mcp install --global     # Install in ~/.mcp.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootCmd := cmd
			for rootCmd.Parent() != nil {
				rootCmd = rootCmd.Parent()
			}

			binPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to find executable path: %w", err)
			}
			binPath, _ = filepath.EvalSymlinks(binPath)

			if p, err := exec.LookPath(rootCmd.Use); err == nil {
				binPath = p
			}

			serverName := rootCmd.Use
			entry := map[string]any{
				"command": binPath,
				"args":    []string{"mcp", "serve"},
			}

			settingsPath := claudeSettingsPath(global)

			settings, err := readJSONFile(settingsPath)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to read %s: %w", settingsPath, err)
			}
			if settings == nil {
				settings = map[string]any{}
			}

			servers, _ := settings["mcpServers"].(map[string]any)
			if servers == nil {
				servers = map[string]any{}
			}
			servers[serverName] = entry
			settings["mcpServers"] = servers

			if err := writeJSONFile(settingsPath, settings); err != nil {
				return fmt.Errorf("failed to write %s: %w", settingsPath, err)
			}

			fmt.Fprintf(os.Stderr, "Installed %q MCP server in %s\n", serverName, settingsPath)
			fmt.Fprintf(os.Stderr, "  command: %s mcp serve\n", binPath)
			return nil
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "Install in global Claude Code settings (~/.claude/settings.json)")
	return cmd
}

func claudeSettingsPath(global bool) string {
	if global {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".mcp.json")
	}
	return ".mcp.json"
}

func readJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	return result, json.Unmarshal(data, &result)
}

func writeJSONFile(path string, data map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}
