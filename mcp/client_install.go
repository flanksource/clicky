package mcp

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// ClientOptions supplies application-specific hooks while keeping MCP client
// transport, persistence, and shim generation reusable by every clicky CLI.
type ClientOptions struct {
	ResolvePrompt PromptResolver
	OpenBrowser   func(string) error
}

// PromptResolver maps an application prompt definition into portable server
// and tool restrictions for offline shortcut generation.
type PromptResolver func(path string) (PromptRestrictions, error)

// PromptRestrictions describes the MCP surface a prompt permits. Empty Servers
// means all registered servers; allow/deny entries match full mcp__server__tool
// names.
type PromptRestrictions struct {
	Name            string
	Disabled        bool
	Servers         []string
	DisabledServers []string
	AllowTools      []string
	DenyTools       []string
}

// ShimOptions controls executable shortcut creation without changing registry
// or contacting the target server.
type ShimOptions struct {
	BinDir     string
	Name       string
	AllowTools []string
	DenyTools  []string
	Force      bool
}

// InstallShim writes an executable that delegates to mcp run. Tool restrictions
// affect the shortcut's help and dispatch surface, not direct mcp run calls.
func InstallShim(appName, serverName string, options ShimOptions) (string, error) {
	if !serverNamePattern.MatchString(serverName) {
		return "", fmt.Errorf("invalid server name %q", serverName)
	}
	registry := NewServerRegistry(appName)
	if _, exists, err := registry.Get(serverName); err != nil {
		return "", err
	} else if !exists {
		return "", fmt.Errorf("MCP server %q is not registered", serverName)
	}
	for _, pattern := range append(append([]string{}, options.AllowTools...), options.DenyTools...) {
		if _, err := path.Match(pattern, "mcp__server__tool"); err != nil {
			return "", fmt.Errorf("invalid tool policy glob %q: %w", pattern, err)
		}
	}

	binDir := options.BinDir
	if binDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		binDir = filepath.Join(home, ".local", "bin")
	}
	name := options.Name
	if name == "" {
		name = serverName
	}
	name = normalizeShortcutName(name)
	if !serverNamePattern.MatchString(name) {
		return "", fmt.Errorf("invalid shortcut name %q", name)
	}
	binPath, err := resolveBinaryPath(appName)
	if err != nil {
		return "", err
	}
	content := renderShim(binPath, serverName, options.AllowTools, options.DenyTools)
	destination := filepath.Join(binDir, name)
	if existing, err := os.ReadFile(destination); err == nil {
		if string(existing) == content {
			if err := os.Chmod(destination, 0o755); err != nil {
				return "", err
			}
			return destination, nil
		}
		if !options.Force {
			return "", fmt.Errorf("shortcut %s already exists; pass --force to replace it", destination)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	tmp := destination + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, destination); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Chmod(destination, 0o755); err != nil {
		return "", err
	}
	return destination, nil
}

func installPromptShims(cmd *cobra.Command, appName, promptPath, binDir string, force bool, options ClientOptions) error {
	if options.ResolvePrompt == nil {
		return fmt.Errorf("%s does not provide prompt-based MCP installation", appName)
	}
	restrictions, err := options.ResolvePrompt(promptPath)
	if err != nil {
		return err
	}
	paths, err := installPromptShortcuts(appName, restrictions, binDir, force)
	if err != nil {
		return err
	}
	for _, installed := range paths {
		fmt.Fprintf(cmd.OutOrStdout(), "Installed prompt-scoped MCP shortcut %s\n", installed)
	}
	return nil
}

func renderShim(binPath, serverName string, allowTools, denyTools []string) string {
	var command strings.Builder
	command.WriteString("#!/bin/sh\nexec ")
	command.WriteString(shellQuote(binPath))
	command.WriteString(" mcp run ")
	command.WriteString(shellQuote(serverName))
	for _, pattern := range allowTools {
		command.WriteString(" --allow-tool ")
		command.WriteString(shellQuote(pattern))
	}
	for _, pattern := range denyTools {
		command.WriteString(" --deny-tool ")
		command.WriteString(shellQuote(pattern))
	}
	command.WriteString(" -- \"$@\"\n")
	return command.String()
}

func installPromptShortcuts(appName string, restrictions PromptRestrictions, binDir string, force bool) ([]string, error) {
	if restrictions.Disabled {
		return nil, fmt.Errorf("prompt disables MCP servers")
	}
	registry := NewServerRegistry(appName)
	names, _, err := registry.List()
	if err != nil {
		return nil, err
	}
	allowed := stringSet(restrictions.Servers)
	disabled := stringSet(restrictions.DisabledServers)
	selected := make([]string, 0, len(names))
	for _, name := range names {
		if disabled[name] || (len(allowed) > 0 && !allowed[name]) {
			continue
		}
		selected = append(selected, name)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("prompt permits no registered MCP servers")
	}
	promptName := normalizeShortcutName(restrictions.Name)
	if promptName == "" {
		promptName = "prompt"
	}
	paths := make([]string, 0, len(selected))
	for _, server := range selected {
		installed, err := InstallShim(appName, server, ShimOptions{
			BinDir: binDir, Name: promptName + "-" + server, Force: force,
			AllowTools: restrictions.AllowTools, DenyTools: restrictions.DenyTools,
		})
		if err != nil {
			return paths, err
		}
		paths = append(paths, installed)
	}
	sort.Strings(paths)
	return paths, nil
}

var shortcutUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func normalizeShortcutName(value string) string {
	value = strings.TrimSuffix(filepath.Base(value), ".prompt")
	value = shortcutUnsafe.ReplaceAllString(value, "-")
	return strings.Trim(value, "-._")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		if value != "" {
			result[value] = true
		}
	}
	return result
}
