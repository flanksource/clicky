package mcp

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// CommandOptions holds options for MCP command creation
type CommandOptions struct {
	ConfigPath   string
	Verbose      bool
	Transport    string
	Address      string
	Port         int
	AutoExpose   bool
}

// NewCommand creates the MCP command group that can be added to any cobra CLI
func NewCommand() *cobra.Command {
	opts := &CommandOptions{}
	
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
	mcpCmd.AddCommand(newConfigCommand(opts))
	
	// Global MCP flags
	mcpCmd.PersistentFlags().StringVar(&opts.ConfigPath, "config", "", "Path to MCP configuration file")
	mcpCmd.PersistentFlags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Enable verbose output")
	
	return mcpCmd
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
				configPath = GetConfigPath()
			}
			
			config, err := LoadConfig(configPath)
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
				configPath = GetConfigPath()
			}
			
			config, err := LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}
			
			displayConfig(config, configPath)
			return nil
		},
	}
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