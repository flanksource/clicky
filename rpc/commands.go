package rpc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewOpenAPICommand creates the OpenAPI command group that can be added to any cobra CLI
func NewOpenAPICommand() *cobra.Command {
	return newOpenAPICommand(nil)
}

// NewOpenAPICommandWithConfig creates the OpenAPI command group with custom configuration
func NewOpenAPICommandWithConfig(config *OpenAPIConfig) *cobra.Command {
	return newOpenAPICommand(config)
}

// newOpenAPICommand creates the OpenAPI command group
func newOpenAPICommand(defaultConfig *OpenAPIConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "openapi",
		Short: "Generate OpenAPI specifications",
		Long: `Generate OpenAPI 3.0.3 specifications from CLI command structures.

The openapi command converts CLI commands and their parameters into OpenAPI specifications
that can be used for API documentation, client generation, and testing.`,
	}

	// Add subcommands
	cmd.AddCommand(newOpenAPIGenerateCommand(defaultConfig))
	cmd.AddCommand(newOpenAPIValidateCommand())
	cmd.AddCommand(newOpenAPIServeCommand(defaultConfig))

	return cmd
}

// newOpenAPIGenerateCommand creates the generate subcommand
func newOpenAPIGenerateCommand(defaultConfig *OpenAPIConfig) *cobra.Command {
	var (
		outputFile  string
		format      string
		title       string
		description string
		version     string
		serverURL   string
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate OpenAPI specification from CLI commands",
		Long: `Generate an OpenAPI 3.0.3 specification from the current CLI command structure.

This command analyzes the CLI commands and their parameters to generate a comprehensive
OpenAPI specification that can be used for API documentation and client generation.`,
		Example: `  myapp openapi generate --output api-spec.json
  myapp openapi generate --format yaml --title "My API" --version "2.0.0"
  myapp openapi generate --server-url "https://api.example.com" --output openapi.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the root command to convert
			rootCmd := cmd.Root()

			// Configure OpenAPI generation
			config := &OpenAPIConfig{
				Title:       title,
				Description: description,
				Version:     version,
			}

			// Apply default config if provided
			if defaultConfig != nil {
				if config.Title == "CLI API" && defaultConfig.Title != "" {
					config.Title = defaultConfig.Title
				}
				if config.Description == "Generated API from CLI commands" && defaultConfig.Description != "" {
					config.Description = defaultConfig.Description
				}
				if config.Version == "1.0.0" && defaultConfig.Version != "" {
					config.Version = defaultConfig.Version
				}
				if defaultConfig.Contact != nil {
					config.Contact = defaultConfig.Contact
				}
				if defaultConfig.License != nil {
					config.License = defaultConfig.License
				}
				if len(defaultConfig.Servers) > 0 && serverURL == "" {
					config.Servers = defaultConfig.Servers
				}
				if len(defaultConfig.Tags) > 0 {
					config.Tags = defaultConfig.Tags
				}
			}

			// Add server if specified via flag
			if serverURL != "" {
				config.Servers = []OpenAPIServer{
					{
						URL:         serverURL,
						Description: "API Server",
					},
				}
			}

			// Generate OpenAPI spec from root command
			generator := NewOpenAPIGenerator(config)
			spec, err := generator.GenerateFromCobra(rootCmd)
			if err != nil {
				return fmt.Errorf("failed to generate OpenAPI spec: %w", err)
			}

			// Convert to desired format
			var output []byte
			switch strings.ToLower(format) {
			case "yaml", "yml":
				output, err = spec.ToYAML()
			case "json":
				output, err = spec.ToJSON()
			default:
				return fmt.Errorf("unsupported format: %s (supported: json, yaml)", format)
			}

			if err != nil {
				return fmt.Errorf("failed to marshal OpenAPI spec: %w", err)
			}

			// Output to file or stdout
			if outputFile != "" {
				// Ensure output directory exists
				if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
					return fmt.Errorf("failed to create output directory: %w", err)
				}

				// Write to file
				if err := os.WriteFile(outputFile, output, 0644); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}

				fmt.Printf("OpenAPI specification written to %s\n", outputFile)
			} else {
				// Output to stdout
				fmt.Print(string(output))
			}

			return nil
		},
	}

	// Set default values from config or fallback defaults
	defaultTitle := "CLI API"
	defaultDescription := "Generated API from CLI commands"
	defaultVersion := "1.0.0"

	if defaultConfig != nil {
		if defaultConfig.Title != "" {
			defaultTitle = defaultConfig.Title
		}
		if defaultConfig.Description != "" {
			defaultDescription = defaultConfig.Description
		}
		if defaultConfig.Version != "" {
			defaultVersion = defaultConfig.Version
		}
	}

	// Add flags
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for the OpenAPI specification")
	cmd.Flags().StringVarP(&format, "format", "f", "json", "Output format (json, yaml)")
	cmd.Flags().StringVar(&title, "title", defaultTitle, "API title")
	cmd.Flags().StringVar(&description, "description", defaultDescription, "API description")
	cmd.Flags().StringVar(&version, "version", defaultVersion, "API version")
	cmd.Flags().StringVar(&serverURL, "server-url", "", "API server URL")

	return cmd
}

// newOpenAPIValidateCommand creates the validate subcommand
func newOpenAPIValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate an OpenAPI specification file",
		Long: `Validate an OpenAPI specification file for compliance with OpenAPI 3.0 standards.

This command checks the OpenAPI specification for structural correctness, required fields,
and adherence to OpenAPI 3.0 specification requirements.`,
		Example: `  myapp openapi validate api-spec.json
  myapp openapi validate openapi.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specFile := args[0]

			// Read the specification file
			data, err := os.ReadFile(specFile)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", specFile, err)
			}

			// Parse the specification
			var spec OpenAPISpec
			ext := strings.ToLower(filepath.Ext(specFile))

			switch ext {
			case ".json":
				if err := json.Unmarshal(data, &spec); err != nil {
					return fmt.Errorf("failed to parse JSON: %w", err)
				}
			case ".yaml", ".yml":
				if err := yaml.Unmarshal(data, &spec); err != nil {
					return fmt.Errorf("failed to parse YAML: %w", err)
				}
			default:
				return fmt.Errorf("unsupported file extension: %s (supported: .json, .yaml, .yml)", ext)
			}

			// Validate the specification
			validator := NewOpenAPIValidator()
			result := validator.Validate(&spec)

			if result.Valid {
				fmt.Printf("✓ OpenAPI specification '%s' is valid\n", specFile)
				return nil
			}

			// Print validation errors
			fmt.Printf("✗ OpenAPI specification '%s' has %d validation error(s):\n", specFile, len(result.Errors))
			for i, err := range result.Errors {
				fmt.Printf("  %d. %s\n", i+1, err.Error())
			}

			return fmt.Errorf("validation failed")
		},
	}

	return cmd
}
