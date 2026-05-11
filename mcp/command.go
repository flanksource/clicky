package mcp

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/clicky/formatters"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// CommandOptions holds options for MCP command creation
type CommandOptions struct {
	ConfigPath    string
	Transport     string
	Address       string
	Port          int
	AutoExpose    bool
	InitialConfig *Config
}

// NewCommand creates the MCP command group that can be added to any cobra CLI
func NewCommand() *cobra.Command {
	return NewCommandWithConfig(nil)
}

// NewCommandWithConfig creates the MCP command group with custom configuration
func NewCommandWithConfig(config *Config) *cobra.Command {
	opts := &CommandOptions{}

	// Store initial config for merging in serve command
	if config != nil {
		opts.InitialConfig = config
		opts.AutoExpose = config.Tools.AutoExpose
		opts.Transport = config.Transport.Type
		opts.Address = config.Transport.Address
		opts.Port = config.Transport.Port
	}

	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP (Model Context Protocol) server management",
		Long: `Manage MCP servers and expose CLI commands as MCP tools.

The MCP command group provides functionality to:
- Run the CLI as an MCP server
- Configure tool exposure settings`,
	}

	// Add subcommands
	mcpCmd.AddCommand(newServeCommand(opts))
	mcpCmd.AddCommand(newInstallCommand(opts))
	mcpCmd.AddCommand(newConfigCommand(opts))
	mcpCmd.AddCommand(newPromptCommand(opts))
	mcpCmd.AddCommand(newToolsCommand(opts))

	// Global MCP flags. Verbose output is controlled by clicky's existing
	// --loglevel/-v flag and the VERBOSE/DEBUG env vars honored by the server,
	// so we don't redeclare a --verbose flag here (it would clash with -v
	// shorthand on hosts that bind clicky.Flags on the root command).
	mcpCmd.PersistentFlags().StringVar(&opts.ConfigPath, "config", "", "Path to MCP configuration file")

	return mcpCmd
}

// rootAppName walks up to the root cobra command and returns its Name(),
// which is the host application's CLI name (e.g. "gavel"). Used to namespace
// per-app config so multiple clicky-based MCP servers don't share one file.
func rootAppName(cmd *cobra.Command) string {
	root := cmd
	for root.Parent() != nil {
		root = root.Parent()
	}
	return root.Name()
}

// newServeCommand creates the serve subcommand
func newServeCommand(opts *CommandOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run CLI as an MCP server",
		Long: `Start the CLI as an MCP server, exposing selected commands as MCP tools.

The server implements the MCP protocol and can be connected to by MCP clients
like Claude Desktop, Cursor, or other AI assistants.

Examples:
  app mcp serve                    # Start with default configuration
  app mcp serve --auto-expose      # Expose all commands
  app mcp serve --transport http --port 8080  # Use HTTP transport`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			configPath := opts.ConfigPath
			if configPath == "" {
				configPath = GetConfigPathFor(rootAppName(cmd))
			}

			config, err := loadCommandConfig(configPath, opts.InitialConfig)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Apply command-line overrides
			if opts.Transport != "" {
				config.Transport.Type = opts.Transport
			}
			if opts.Address != "" {
				config.Transport.Address = opts.Address
			}
			if opts.Port > 0 {
				config.Transport.Port = opts.Port
			}
			if opts.AutoExpose {
				config.Tools.AutoExpose = true
			}

			// Get root command (we need to traverse up to find it)
			rootCmd := cmd
			for rootCmd.Parent() != nil {
				rootCmd = rootCmd.Parent()
			}

			// Create and initialize MCP server
			server := NewMCPServer(config, rootCmd)
			if err := server.Initialize(); err != nil {
				return fmt.Errorf("failed to initialize MCP server: %w", err)
			}

			// Display startup information
			displayServerInfo(config)

			// Set up context with cancellation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle shutdown signals
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigChan
				fmt.Fprintf(os.Stderr, "\nShutting down MCP server...\n")
				cancel()
			}()

			// Start the server
			fmt.Fprintf(os.Stderr, "Starting MCP server...\n")
			return server.Start(ctx)
		},
	}

	// Serve command flags
	cmd.Flags().StringVar(&opts.Transport, "transport", "", "Transport type (stdio, http)")
	cmd.Flags().StringVar(&opts.Address, "address", "", "HTTP server address")
	cmd.Flags().IntVar(&opts.Port, "port", 0, "HTTP server port")
	cmd.Flags().BoolVar(&opts.AutoExpose, "auto-expose", false, "Auto-expose all commands as tools")

	return cmd
}

// newConfigCommand creates the config subcommand
func newConfigCommand(opts *CommandOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Manage MCP server configuration",
		Long: `Display or modify MCP server configuration.

The configuration controls which commands are exposed as MCP tools,
security settings, and transport options.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := opts.ConfigPath
			if configPath == "" {
				configPath = GetConfigPathFor(rootAppName(cmd))
			}

			config, err := loadCommandConfig(configPath, opts.InitialConfig)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			displayConfig(config, configPath)
			return nil
		},
	}
}

// newToolsCommand creates the tools subcommand: pretty-prints the tools and
// parameters that this MCP server would expose for the host CLI, honoring
// the same Exclude/Include/IgnoreParams/Format settings used at runtime.
func newToolsCommand(opts *CommandOptions) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Show the MCP tools this CLI exposes, with parameters and defaults",
		Long: `Render the catalogue of MCP tools that 'mcp serve' would publish.

The output reflects the same Exclude / Include / IgnoreParams settings
configured by the host application or via the on-disk config file, so it
doubles as a dry-run for what AI clients will see.

Output uses the configured runtime format; pass --format to override.`,
		Example: `  app mcp tools                # ANSI-colored catalogue (default)
  app mcp tools --format markdown
  app mcp tools --format plain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := opts.ConfigPath
			if configPath == "" {
				configPath = GetConfigPathFor(rootAppName(cmd))
			}
			config, err := loadCommandConfig(configPath, opts.InitialConfig)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			rootCmd := cmd.Root()

			registry := NewToolRegistry(config)
			if err := registry.RegisterCommandTree(rootCmd); err != nil {
				return fmt.Errorf("failed to register command tree: %w", err)
			}

			renderOpts := config.Tools.Format
			if renderOpts == nil {
				renderOpts = &formatters.FormatOptions{Pretty: true}
			}
			if cmd.Flags().Changed("format") {
				renderOpts = &formatters.FormatOptions{Format: format}
			}
			out := RenderDiscoverToolsString(registry.GetTools(), renderOpts)
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "", "Output format: pretty, markdown, or plain")
	return cmd
}

func loadCommandConfig(configPath string, initial *Config) (*Config, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	mergeInitialConfig(config, initial)
	return config, nil
}

// mergeInitialConfig applies host-provided MCP defaults from
// NewCommandWithConfig. These values intentionally win over the persisted file:
// they describe the embedding application contract, while the file supplies
// user-managed additions.
func mergeInitialConfig(config *Config, initial *Config) {
	if config == nil || initial == nil {
		return
	}
	if initial.Name != "" {
		config.Name = initial.Name
	}
	if initial.Description != "" {
		config.Description = initial.Description
	}
	if initial.Version != "" {
		config.Version = initial.Version
	}
	if initial.Transport.Type != "" {
		config.Transport.Type = initial.Transport.Type
	}
	if initial.Transport.Address != "" {
		config.Transport.Address = initial.Transport.Address
	}
	if initial.Transport.Port != 0 {
		config.Transport.Port = initial.Transport.Port
	}

	config.Security.RequireConfirmation = initial.Security.RequireConfirmation
	config.Security.AuditLog = initial.Security.AuditLog
	if initial.Security.TimeoutSeconds != 0 {
		config.Security.TimeoutSeconds = initial.Security.TimeoutSeconds
	}
	config.Security.AllowedCommands = appendUniqueStrings(config.Security.AllowedCommands, initial.Security.AllowedCommands...)
	config.Security.BlockedCommands = appendUniqueStrings(config.Security.BlockedCommands, initial.Security.BlockedCommands...)

	if initial.Tools.AutoExpose {
		config.Tools.AutoExpose = true
	}
	config.Tools.Exclude = appendUniqueStrings(config.Tools.Exclude, initial.Tools.Exclude...)
	config.Tools.Include = appendUniqueStrings(config.Tools.Include, initial.Tools.Include...)
	config.Tools.IgnoredParams = append(config.Tools.IgnoredParams, initial.Tools.IgnoredParams...)
	if initial.Tools.Format != nil {
		config.Tools.Format = initial.Tools.Format
	}
	for name, desc := range initial.Tools.Descriptions {
		if config.Tools.Descriptions == nil {
			config.Tools.Descriptions = map[string]string{}
		}
		config.Tools.Descriptions[name] = desc
	}
}

func appendUniqueStrings(base []string, values ...string) []string {
	for _, value := range values {
		found := false
		for _, existing := range base {
			if existing == value {
				found = true
				break
			}
		}
		if !found {
			base = append(base, value)
		}
	}
	return base
}

// displayServerInfo shows server startup information
func displayServerInfo(config *Config) {
	// Define styles
	primaryColor := lipgloss.Color("14")
	accentColor := lipgloss.Color("12")
	successColor := lipgloss.Color("10")
	mutedColor := lipgloss.Color("8")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Padding(1, 2).
		MarginBottom(1)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Render("🔧 MCP Server")

	content := []string{
		title,
		"",
		fmt.Sprintf("Name: %s", config.Name),
		fmt.Sprintf("Version: %s", config.Version),
		fmt.Sprintf("Transport: %s", config.Transport.Type),
	}

	if config.Transport.Type == "http" {
		content = append(content,
			fmt.Sprintf("Address: %s:%d", config.Transport.Address, config.Transport.Port))
	}

	content = append(content, "",
		fmt.Sprintf("Auto-expose: %s", boolToStatus(config.Tools.AutoExpose, successColor, mutedColor)),
		fmt.Sprintf("Require confirmation: %s", boolToStatus(config.Security.RequireConfirmation, successColor, mutedColor)),
		fmt.Sprintf("Audit logging: %s", boolToStatus(config.Security.AuditLog, successColor, mutedColor)),
	)

	if len(config.Tools.Include) > 0 {
		content = append(content, "", "Included commands:")
		for _, cmd := range config.Tools.Include {
			content = append(content, fmt.Sprintf("  • %s", cmd))
		}
	}

	if len(config.Tools.Exclude) > 0 {
		content = append(content, "", "Excluded commands:")
		for _, cmd := range config.Tools.Exclude {
			content = append(content, fmt.Sprintf("  • %s", cmd))
		}
	}

	fmt.Fprintf(os.Stderr, "%s\n", boxStyle.Render(strings.Join(content, "\n")))
}

// displayConfig shows the current configuration
func displayConfig(config *Config, configPath string) {
	// Define styles
	primaryColor := lipgloss.Color("14")
	accentColor := lipgloss.Color("12")
	successColor := lipgloss.Color("10")
	mutedColor := lipgloss.Color("8")

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		MarginBottom(1)

	accentStyle := lipgloss.NewStyle().Foreground(accentColor)
	mutedStyle := lipgloss.NewStyle().Foreground(mutedColor)

	fmt.Println(titleStyle.Render("MCP Server Configuration"))
	fmt.Printf("Config file: %s\n\n", mutedStyle.Render(configPath))

	// Server settings
	fmt.Println(accentStyle.Render("Server Settings:"))
	fmt.Printf("  Name: %s\n", config.Name)
	fmt.Printf("  Description: %s\n", config.Description)
	fmt.Printf("  Version: %s\n\n", config.Version)

	// Transport settings
	fmt.Println(accentStyle.Render("Transport Settings:"))
	fmt.Printf("  Type: %s\n", config.Transport.Type)
	if config.Transport.Type == "http" {
		fmt.Printf("  Address: %s\n", config.Transport.Address)
		fmt.Printf("  Port: %d\n", config.Transport.Port)
	}
	fmt.Println()

	// Security settings
	fmt.Println(accentStyle.Render("Security Settings:"))
	fmt.Printf("  Require confirmation: %s\n", boolToStatus(config.Security.RequireConfirmation, successColor, mutedColor))
	fmt.Printf("  Audit logging: %s\n", boolToStatus(config.Security.AuditLog, successColor, mutedColor))
	fmt.Printf("  Timeout: %ds\n\n", config.Security.TimeoutSeconds)

	// Tool settings
	fmt.Println(accentStyle.Render("Tool Settings:"))
	fmt.Printf("  Auto-expose: %s\n", boolToStatus(config.Tools.AutoExpose, successColor, mutedColor))

	if len(config.Tools.Include) > 0 {
		fmt.Println("  Included commands:")
		for _, cmd := range config.Tools.Include {
			fmt.Printf("    • %s\n", cmd)
		}
	}

	if len(config.Tools.Exclude) > 0 {
		fmt.Println("  Excluded commands:")
		for _, cmd := range config.Tools.Exclude {
			fmt.Printf("    • %s\n", cmd)
		}
	}
}

// boolToStatus converts a boolean to a status string
func boolToStatus(b bool, enabledColor, disabledColor lipgloss.Color) string {
	if b {
		return lipgloss.NewStyle().Foreground(enabledColor).Render("enabled")
	}
	return lipgloss.NewStyle().Foreground(disabledColor).Render("disabled")
}

// newPromptCommand creates the prompt subcommand
func newPromptCommand(opts *CommandOptions) *cobra.Command {
	var listTags bool
	var byTag string
	var showExample bool
	var savePath string

	cmd := &cobra.Command{
		Use:   "prompt [name]",
		Short: "Manage and test MCP prompts",
		Long: `Manage MCP prompts that help AI assistants understand how to use your CLI tools.

Prompts provide pre-configured templates that guide AI assistants in:
- Discovering available tools and their usage
- Understanding appropriate arguments for each tool
- Learning common workflows and best practices
- Combining tools for complex tasks

Examples:
  app mcp prompt                       # List all available prompts
  app mcp prompt --list-tags           # List all prompt tags
  app mcp prompt --tag workflow        # List prompts tagged as 'workflow'
  app mcp prompt --save custom.json    # Save prompts to file

Tool discovery has moved: use 'app mcp tools' (or call the
'discover-tools' MCP tool) instead of 'app mcp prompt discover-tools'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			configPath := opts.ConfigPath
			if configPath == "" {
				configPath = GetConfigPathFor(rootAppName(cmd))
			}

			config, err := LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Get root command
			rootCmd := cmd
			for rootCmd.Parent() != nil {
				rootCmd = rootCmd.Parent()
			}

			// Create prompt registry
			promptRegistry := NewPromptRegistry(config)
			promptRegistry.LoadDefaults()

			// Try to load custom prompts
			promptsPath := GetPromptsPathFor(rootAppName(cmd))
			if _, err := os.Stat(promptsPath); err == nil {
				if err := promptRegistry.LoadFromFile(promptsPath); err != nil {
					fmt.Printf("Warning: failed to load prompts from %s: %v\n", promptsPath, err)
				}
			}

			// Handle save option
			if savePath != "" {
				if err := promptRegistry.SaveToFile(savePath); err != nil {
					return fmt.Errorf("failed to save prompts: %w", err)
				}
				fmt.Printf("Prompts saved to %s\n", savePath)
				return nil
			}

			// Handle list tags
			if listTags {
				tags := make(map[string]int)
				for _, p := range promptRegistry.List() {
					for _, tag := range p.Tags {
						tags[tag]++
					}
				}

				fmt.Println("Available prompt tags:")
				for tag, count := range tags {
					fmt.Printf("  • %s (%d prompts)\n", tag, count)
				}
				return nil
			}

			// Handle filter by tag
			var prompts []*Prompt
			if byTag != "" {
				prompts = promptRegistry.ListByTag(byTag)
				if len(prompts) == 0 {
					return fmt.Errorf("no prompts found with tag: %s", byTag)
				}
			} else if len(args) == 0 {
				prompts = promptRegistry.List()
			}

			// Handle specific prompt
			if len(args) > 0 {
				promptName := args[0]

				// Get regular prompt
				prompt, exists := promptRegistry.Get(promptName)
				if !exists {
					return fmt.Errorf("prompt not found: %s", promptName)
				}

				// Display prompt details
				fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")).
					Render(prompt.Name))
				fmt.Printf("Description: %s\n", prompt.Description)

				if len(prompt.Tags) > 0 {
					fmt.Printf("Tags: %s\n", strings.Join(prompt.Tags, ", "))
				}

				if len(prompt.Arguments) > 0 {
					fmt.Println("\nArguments:")
					for _, arg := range prompt.Arguments {
						req := ""
						if arg.Required {
							req = " (required)"
						}
						fmt.Printf("  • %s: %s%s\n", arg.Name, arg.Description, req)
						if arg.Default != "" {
							fmt.Printf("    Default: %s\n", arg.Default)
						}
					}
				}

				fmt.Println("\nTemplate:")
				fmt.Println(prompt.Template)

				if len(prompt.Examples) > 0 && showExample {
					fmt.Println("\nExamples:")
					for _, ex := range prompt.Examples {
						fmt.Printf("  %s\n", ex)
					}
				}

				return nil
			}

			// List prompts
			fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")).
				Render("Available MCP Prompts"))
			fmt.Println()

			// Group by tags
			byTagMap := make(map[string][]*Prompt)
			untagged := []*Prompt{}

			for _, p := range prompts {
				if len(p.Tags) == 0 {
					untagged = append(untagged, p)
				} else {
					for _, tag := range p.Tags {
						byTagMap[tag] = append(byTagMap[tag], p)
					}
				}
			}

			// Display by category
			displayedPrompts := make(map[string]bool)

			// Show discovery prompts first
			if discoveryPrompts, ok := byTagMap["discovery"]; ok {
				fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render("Discovery:"))
				for _, p := range discoveryPrompts {
					if !displayedPrompts[p.Name] {
						fmt.Printf("  • %s - %s\n",
							lipgloss.NewStyle().Bold(true).Render(p.Name),
							p.Description)
						displayedPrompts[p.Name] = true
					}
				}
				fmt.Println()
			}

			// Show task prompts
			if taskPrompts, ok := byTagMap["task"]; ok {
				fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render("Tasks:"))
				for _, p := range taskPrompts {
					if !displayedPrompts[p.Name] {
						fmt.Printf("  • %s - %s\n",
							lipgloss.NewStyle().Bold(true).Render(p.Name),
							p.Description)
						displayedPrompts[p.Name] = true
					}
				}
				fmt.Println()
			}

			// Show other categories
			for tag, tagPrompts := range byTagMap {
				if tag == "discovery" || tag == "task" {
					continue
				}

				fmt.Printf("%s:\n", lipgloss.NewStyle().Foreground(lipgloss.Color("12")).
					Render(cases.Title(language.English).String(tag)))
				for _, p := range tagPrompts {
					if !displayedPrompts[p.Name] {
						fmt.Printf("  • %s - %s\n",
							lipgloss.NewStyle().Bold(true).Render(p.Name),
							p.Description)
						displayedPrompts[p.Name] = true
					}
				}
				fmt.Println()
			}

			// Show untagged
			if len(untagged) > 0 {
				fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render("Other:"))
				for _, p := range untagged {
					if !displayedPrompts[p.Name] {
						fmt.Printf("  • %s - %s\n",
							lipgloss.NewStyle().Bold(true).Render(p.Name),
							p.Description)
					}
				}
				fmt.Println()
			}

			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).
				Render("Use 'mcp prompt <name>' to see details about a specific prompt"))

			return nil
		},
	}

	// Add flags
	cmd.Flags().BoolVar(&listTags, "list-tags", false, "List all available tags")
	cmd.Flags().StringVar(&byTag, "tag", "", "Filter prompts by tag")
	cmd.Flags().BoolVar(&showExample, "examples", false, "Show examples for prompts")
	cmd.Flags().StringVar(&savePath, "save", "", "Save prompts to JSON file")

	return cmd
}
