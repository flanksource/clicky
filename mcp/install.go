package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// supportedClients lists every MCP client `mcp install` knows how to wire up.
// Order is the order shown in --help and used by --client="all".
var supportedClients = []string{"claude", "codex", "gemini", "copilot", "cursor"}

// supportedTransports lists the transports the install command can write
// into a client config. The MCP server itself supports "stdio" and "sse"
// (handled by Server.Start).
var supportedTransports = []string{"stdio", "sse"}

func newInstallCommandWithOptions(opts *CommandOptions, clientOptions ClientOptions) *cobra.Command {
	var (
		client    string
		global    bool
		transport string
		url       string
		port      int
		prompt    string
		binDir    string
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "install [registered-server]",
		Short: "Install as MCP server in an MCP client's configuration",
		Long: `Register this CLI as an MCP server in the chosen client's config.

Supported clients: ` + strings.Join(supportedClients, ", ") + `
Supported transports: ` + strings.Join(supportedTransports, ", ") + `

Project scope writes to a per-repo config file in the current directory;
--global writes to the user-level config in $HOME.

For sse transport, --url overrides the connection URL (default
http://127.0.0.1:<port>/sse, with --port defaulting to 8080).

Examples:
  app mcp install                              # claude, project, stdio
  app mcp install --global                     # claude, user-level, stdio
  app mcp install --client codex --global      # codex, ~/.codex/config.toml
  app mcp install --client gemini              # gemini, project .gemini/settings.json
  app mcp install --transport sse --port 9000  # claude, sse on :9000
  app mcp install --client copilot --global    # VS Code Copilot user-level mcp.json
  app mcp install filesystem                   # ~/.local/bin/filesystem shortcut
  app mcp install --prompt review.prompt       # prompt-scoped shortcuts`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootCmd := cmd.Root()
			if prompt != "" {
				if len(args) != 0 {
					return fmt.Errorf("--prompt cannot be combined with a server argument")
				}
				if err := rejectInstallFlags(cmd, "prompt shortcuts", "client", "global", "transport", "url", "port"); err != nil {
					return err
				}
				return installPromptShims(cmd, rootCmd.Name(), prompt, binDir, force, clientOptions)
			}
			if len(args) == 1 {
				if err := rejectInstallFlags(cmd, "server shortcuts", "client", "global", "transport", "url", "port"); err != nil {
					return err
				}
				path, err := InstallShim(rootCmd.Name(), args[0], ShimOptions{BinDir: binDir, Force: force})
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Installed MCP shortcut %s\n", path)
				return nil
			}
			if err := rejectInstallFlags(cmd, "client registration", "bin-dir", "force"); err != nil {
				return err
			}
			if !contains(supportedClients, client) {
				return fmt.Errorf("unsupported --client %q (want one of: %s)", client, strings.Join(supportedClients, ", "))
			}
			if !contains(supportedTransports, transport) {
				return fmt.Errorf("unsupported --transport %q (want one of: %s)", transport, strings.Join(supportedTransports, ", "))
			}

			serverName := rootCmd.Name()

			binPath, err := resolveBinaryPath(rootCmd.Name())
			if err != nil {
				return err
			}

			serveArgs := []string{"mcp", "serve"}
			if transport == "sse" {
				serveArgs = append(serveArgs, "--transport", "sse")
				p := port
				if p == 0 {
					p = 8080
				}
				serveArgs = append(serveArgs, "--port", fmt.Sprintf("%d", p))
			}

			endpoint := url
			if transport == "sse" && endpoint == "" {
				p := port
				if p == 0 {
					p = 8080
				}
				endpoint = fmt.Sprintf("http://127.0.0.1:%d/sse", p)
			}

			entry := serverEntry{
				Name:      serverName,
				Command:   binPath,
				Args:      serveArgs,
				Transport: transport,
				URL:       endpoint,
			}

			settingsPath, err := writeClientConfig(client, global, entry)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Installed %q MCP server (%s, %s) in %s\n",
				serverName, client, transport, settingsPath)
			if transport == "stdio" {
				fmt.Fprintf(os.Stderr, "  command: %s %s\n", binPath, strings.Join(serveArgs, " "))
			} else {
				fmt.Fprintf(os.Stderr, "  url: %s\n", endpoint)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&client, "client", "claude",
		"Target MCP client: "+strings.Join(supportedClients, ", "))
	cmd.Flags().BoolVar(&global, "global", false,
		"Install in the user-level (global) config instead of the project config")
	cmd.Flags().StringVar(&transport, "transport", "stdio",
		"Transport to register: stdio or sse")
	cmd.Flags().StringVar(&url, "url", "",
		"SSE endpoint URL (default http://127.0.0.1:<port>/sse)")
	cmd.Flags().IntVar(&port, "port", 0,
		"Port for SSE transport when --url is not set (default 8080)")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Create prompt-scoped server shortcuts from a .prompt file")
	cmd.Flags().StringVar(&binDir, "bin-dir", "", "Shortcut directory (default ~/.local/bin)")
	cmd.Flags().BoolVar(&force, "force", false, "Replace an existing shortcut")

	_ = opts // reserved for future use (e.g. multi-app namespacing)
	return cmd
}

func rejectInstallFlags(cmd *cobra.Command, mode string, flags ...string) error {
	for _, flag := range flags {
		if cmd.Flags().Changed(flag) {
			return fmt.Errorf("--%s does not apply to %s", flag, mode)
		}
	}
	return nil
}

// serverEntry is the resolved view of one install request, used by every
// per-client writer. Args/Command apply to stdio; URL applies to sse.
type serverEntry struct {
	Name      string
	Command   string
	Args      []string
	Transport string
	URL       string
}

// writeClientConfig dispatches to the per-client writer. Returns the path
// the entry was written to so the caller can echo it back to the user.
func writeClientConfig(client string, global bool, entry serverEntry) (string, error) {
	path := clientConfigPath(client, global)
	switch client {
	case "claude":
		return path, writeMcpServersJSON(path, entry, "mcpServers")
	case "gemini":
		return path, writeMcpServersJSON(path, entry, "mcpServers")
	case "copilot":
		// VS Code Copilot uses the key "servers", not "mcpServers".
		return path, writeMcpServersJSON(path, entry, "servers")
	case "cursor":
		return path, writeMcpServersJSON(path, entry, "mcpServers")
	case "codex":
		return path, writeCodexTOML(path, entry)
	default:
		return "", fmt.Errorf("no writer registered for client %q", client)
	}
}

// clientConfigPath returns the on-disk config file for a given client and
// scope. Per-client conventions:
//   - claude:  project=.mcp.json,            global=~/.claude.json
//   - gemini:  project=.gemini/settings.json global=~/.gemini/settings.json
//   - copilot: project=.vscode/mcp.json      global=~/.config/Code/User/mcp.json
//   - cursor:  project=.cursor/mcp.json      global=~/.cursor/mcp.json
//   - codex:   project=.codex/config.toml    global=~/.codex/config.toml
func clientConfigPath(client string, global bool) string {
	home, _ := os.UserHomeDir()
	switch client {
	case "claude":
		if global {
			return filepath.Join(home, ".claude.json")
		}
		return ".mcp.json"
	case "gemini":
		if global {
			return filepath.Join(home, ".gemini", "settings.json")
		}
		return filepath.Join(".gemini", "settings.json")
	case "copilot":
		if global {
			return filepath.Join(home, ".config", "Code", "User", "mcp.json")
		}
		return filepath.Join(".vscode", "mcp.json")
	case "cursor":
		if global {
			return filepath.Join(home, ".cursor", "mcp.json")
		}
		return filepath.Join(".cursor", "mcp.json")
	case "codex":
		if global {
			return filepath.Join(home, ".codex", "config.toml")
		}
		return filepath.Join(".codex", "config.toml")
	}
	return ""
}

// writeMcpServersJSON merges (or creates) entry into the JSON map at path
// under the given top-level key (mcpServers / servers depending on client).
// Existing unrelated content is preserved.
func writeMcpServersJSON(path string, entry serverEntry, key string) error {
	settings, err := readJSONFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	if settings == nil {
		settings = map[string]any{}
	}

	servers, _ := settings[key].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers[entry.Name] = jsonEntryFor(entry)
	settings[key] = servers

	return writeJSONFile(path, settings)
}

// jsonEntryFor builds the per-server JSON object. Stdio carries
// command+args; sse carries url and an explicit type for clients that
// require it (Cursor, modern Claude).
func jsonEntryFor(e serverEntry) map[string]any {
	if e.Transport == "sse" {
		return map[string]any{
			"type": "sse",
			"url":  e.URL,
		}
	}
	return map[string]any{
		"type":    "stdio",
		"command": e.Command,
		"args":    e.Args,
	}
}

// codexSectionRE matches the start of an `[mcp_servers.<name>]` section.
// Used to find and replace an existing block so writes are idempotent.
var codexSectionRE = regexp.MustCompile(`^\s*\[mcp_servers\.([^\]]+)\]\s*$`)

// writeCodexTOML appends or replaces a `[mcp_servers.<name>]` block in the
// codex TOML config. Other top-level sections in the file are left
// completely untouched. We don't full-parse the TOML — we just find the
// boundaries of the named section (its header line, plus every following
// line until the next top-level `[` or EOF) and splice in the new block.
func writeCodexTOML(path string, entry serverEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	stripped := removeCodexSection(existing, entry.Name)
	block := renderCodexBlock(entry)

	out := stripped
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if out != "" {
		out += "\n"
	}
	out += block

	return os.WriteFile(path, []byte(out), 0o644)
}

// removeCodexSection deletes the `[mcp_servers.<name>]` block (header +
// body up to the next top-level section header or EOF). Returns the
// remaining file content with one trailing newline trimmed.
func removeCodexSection(content, name string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	skip := false
	for _, line := range lines {
		if m := codexSectionRE.FindStringSubmatch(line); m != nil {
			if m[1] == name {
				skip = true
				continue
			}
			skip = false
			out = append(out, line)
			continue
		}
		if skip {
			// Any new top-level header (not matched above) ends the skip.
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "[") {
				skip = false
				out = append(out, line)
			}
			continue
		}
		out = append(out, line)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// renderCodexBlock emits a `[mcp_servers.<name>]` section. Stdio entries carry
// command+args; SSE entries carry the endpoint URL.
func renderCodexBlock(e serverEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[mcp_servers.%s]\n", e.Name)
	if e.Transport == "sse" {
		fmt.Fprintf(&b, "url = %q\n", e.URL)
		return b.String()
	}
	fmt.Fprintf(&b, "command = %q\n", e.Command)
	fmt.Fprintf(&b, "args = [")
	for i, a := range e.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q", a)
	}
	b.WriteString("]\n")
	return b.String()
}

// resolveBinaryPath returns the install-friendly path for the running CLI.
// Prefers a $PATH lookup of rootName so installed configs survive between
// builds, falling back to the literal executable path.
func resolveBinaryPath(rootName string) (string, error) {
	binPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to find executable path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
		binPath = resolved
	}
	if p, err := exec.LookPath(rootName); err == nil {
		binPath = p
	}
	return binPath, nil
}

// contains reports whether v is in list. Tiny helper kept local so the
// install command stays free of external deps.
func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}
