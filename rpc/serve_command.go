package rpc

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}

	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func DefaultServeConfig() *ServeConfig {
	return &ServeConfig{
		Host: "localhost", Port: 8080, Title: "CLI API",
		Description: "Generated API documentation from CLI commands", Version: "1.0.0",
	}
}

func newOpenAPIServeCommand(defaultConfig *OpenAPIConfig) *cobra.Command {
	serveConfig := DefaultServeConfig()
	cmd := &cobra.Command{
		Use: "serve", Short: "Start an HTTP server with interactive API documentation",
		Long: `Start an HTTP server that serves interactive API documentation for the CLI.

This command generates an OpenAPI specification from the current CLI command structure
and serves it through a web interface. The documentation is
generated dynamically and reflects the current state of the CLI commands.`,
		Example: `  myapp openapi serve
  myapp openapi serve --port 3000 --open
  myapp openapi serve --title "My API" --description "Custom API documentation"
  myapp openapi serve --host 0.0.0.0 --port 8080 --auto-refresh`,
		// Exemption from the no-manual-RunE rule: serve bootstraps the server
		// that hosts generated entity commands, so it cannot itself be one.
		RunE: func(cmd *cobra.Command, _ []string) error {
			if serveConfig.Port < 1 || serveConfig.Port > 65535 {
				return fmt.Errorf("invalid port number: %d (must be between 1 and 65535)", serveConfig.Port)
			}
			if strings.TrimSpace(serveConfig.Host) == "" {
				return fmt.Errorf("host cannot be empty")
			}

			openAPIConfig := &OpenAPIConfig{
				Title: serveConfig.Title, Description: serveConfig.Description, Version: serveConfig.Version,
			}
			if defaultConfig != nil {
				openAPIConfig.Contact = defaultConfig.Contact
				openAPIConfig.License = defaultConfig.License
				openAPIConfig.Servers = defaultConfig.Servers
				openAPIConfig.Tags = defaultConfig.Tags
			}
			return NewSwaggerServer(serveConfig, cmd.Root(), openAPIConfig).Start(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&serveConfig.Host, "host", serveConfig.Host, "Host to bind the server to")
	cmd.Flags().IntVarP(&serveConfig.Port, "port", "p", serveConfig.Port, "Port to bind the server to")
	cmd.Flags().StringVar(&serveConfig.Title, "title", serveConfig.Title, "API documentation title")
	cmd.Flags().StringVar(&serveConfig.Description, "description", serveConfig.Description, "API documentation description")
	cmd.Flags().StringVar(&serveConfig.Version, "version", serveConfig.Version, "API version")
	cmd.Flags().BoolVar(&serveConfig.AutoRefresh, "auto-refresh", serveConfig.AutoRefresh, "Enable auto-refresh of documentation")
	cmd.Flags().BoolVar(&serveConfig.Open, "open", serveConfig.Open, "Automatically open browser")
	cmd.Flags().BoolVar(&serveConfig.HideErrorDetails, "hide-error-details", serveConfig.HideErrorDetails, "Hide unclassified error details from API responses")

	var enableExecutor bool
	var skipPreRun bool
	cmd.Flags().BoolVar(&enableExecutor, "enable-executor", false, "Enable dynamic command execution via HTTP endpoints")
	cmd.Flags().BoolVar(&skipPreRun, "skip-pre-run", true, "Skip pre-run hooks during command execution")
	cmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		if enableExecutor {
			serveConfig.Executor = &ExecutorConfig{
				Enabled: enableExecutor, SkipPreRun: skipPreRun, PathPrefix: "/api/v1",
			}
		}
		return nil
	}
	return cmd
}
