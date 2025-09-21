package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/flanksource/clicky/extensions"
	"github.com/spf13/cobra"
)

// Example of integrating both OpenAPI and MCP functionality using the fluent API
func main() {
	// Create your root command
	rootCmd := &cobra.Command{
		Use:   "myapp",
		Short: "My application with OpenAPI and MCP support",
		Long: `An example CLI application that demonstrates the fluent API for adding
OpenAPI generation and MCP server functionality to any Cobra-based CLI.`,
	}

	// Add your application commands
	rootCmd.AddCommand(newUserCommand())
	rootCmd.AddCommand(newDataCommand())
	rootCmd.AddCommand(newUtilsCommand())

	// FLUENT API EXAMPLES:

	// Option 1: Add both with default configuration
	extensions.CobraExtensions(rootCmd).All()

	// Option 2: Add individually with fluent chaining
	// extensions.CobraExtensions(rootCmd).
	//     OpenAPICommand().
	//     MCPCommand()

	// Option 3: Add with custom configuration
	// extensions.CobraExtensions(rootCmd).
	//     OpenAPICommandWithConfig(&rpc.OpenAPIConfig{
	//         Title:       "My Custom API",
	//         Description: "API for my awesome application",
	//         Version:     "2.0.0",
	//         Contact: &rpc.OpenAPIContact{
	//             Name:  "API Team",
	//             Email: "api@example.com",
	//         },
	//         Servers: []rpc.OpenAPIServer{
	//             {
	//                 URL:         "https://api.example.com",
	//                 Description: "Production server",
	//             },
	//         },
	//     }).
	//     MCPCommandWithConfig(&mcp.Config{
	//         Name:        "MyApp MCP Server",
	//         Description: "MCP server for MyApp CLI",
	//         Version:     "2.0.0",
	//         Tools: mcp.ToolConfig{
	//             AutoExpose: true,
	//         },
	//         Security: mcp.SecurityConfig{
	//             RequireConfirmation: true,
	//             AuditLog:           true,
	//         },
	//     })

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Example command: User management
func newUserCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
		Long:  `Manage users in the system.`,
	}

	// Add subcommands
	cmd.AddCommand(newUserCreateCommand())
	cmd.AddCommand(newUserListCommand())
	cmd.AddCommand(newUserDeleteCommand())

	return cmd
}

func newUserCreateCommand() *cobra.Command {
	var name, email, role string
	var active bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		Long:  `Create a new user with the specified details.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			status := "inactive"
			if active {
				status = "active"
			}

			fmt.Printf("Creating user:\n")
			fmt.Printf("  Name: %s\n", name)
			fmt.Printf("  Email: %s\n", email)
			fmt.Printf("  Role: %s\n", role)
			fmt.Printf("  Status: %s\n", status)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "User's full name")
	cmd.Flags().StringVarP(&email, "email", "e", "", "User's email address")
	cmd.Flags().StringVarP(&role, "role", "r", "user", "User's role (user, admin, moderator)")
	cmd.Flags().BoolVarP(&active, "active", "a", true, "Whether the user is active")

	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("email")

	return cmd
}

func newUserListCommand() *cobra.Command {
	var role, status string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		Long:  `List users with optional filtering.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Listing users:\n")
			if role != "" {
				fmt.Printf("  Role filter: %s\n", role)
			}
			if status != "" {
				fmt.Printf("  Status filter: %s\n", status)
			}
			fmt.Printf("  Limit: %d\n", limit)
			return nil
		},
	}

	cmd.Flags().StringVarP(&role, "role", "r", "", "Filter by role")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status (active, inactive)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Maximum number of users to return")

	return cmd
}

func newUserDeleteCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <user-id>",
		Short: "Delete a user",
		Long:  `Delete a user by their ID.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			userID := args[0]
			if force {
				fmt.Printf("Force deleting user: %s\n", userID)
			} else {
				fmt.Printf("Deleting user: %s\n", userID)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion without confirmation")

	return cmd
}

// Example command: Data processing
func newDataCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data",
		Short: "Data processing commands",
		Long:  `Process and transform data.`,
	}

	cmd.AddCommand(newDataImportCommand())
	cmd.AddCommand(newDataExportCommand())
	cmd.AddCommand(newDataTransformCommand())

	return cmd
}

func newDataImportCommand() *cobra.Command {
	var source, format string
	var batchSize int
	var validate bool

	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import data from a file",
		Long:  `Import data from various file formats.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file := args[0]
			fmt.Printf("Importing data from: %s\n", file)
			fmt.Printf("  Source: %s\n", source)
			fmt.Printf("  Format: %s\n", format)
			fmt.Printf("  Batch size: %d\n", batchSize)
			fmt.Printf("  Validate: %t\n", validate)
			return nil
		},
	}

	cmd.Flags().StringVarP(&source, "source", "s", "file", "Data source type")
	cmd.Flags().StringVarP(&format, "format", "f", "json", "Data format (json, csv, xml)")
	cmd.Flags().IntVarP(&batchSize, "batch-size", "b", 1000, "Batch size for processing")
	cmd.Flags().BoolVarP(&validate, "validate", "v", true, "Validate data before import")

	return cmd
}

func newDataExportCommand() *cobra.Command {
	var output, format string
	var compress bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export data",
		Long:  `Export data to various formats.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Exporting data:\n")
			fmt.Printf("  Output: %s\n", output)
			fmt.Printf("  Format: %s\n", format)
			fmt.Printf("  Compress: %t\n", compress)
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "export.json", "Output file")
	cmd.Flags().StringVarP(&format, "format", "f", "json", "Export format (json, csv, xml)")
	cmd.Flags().BoolVarP(&compress, "compress", "c", false, "Compress output file")

	return cmd
}

func newDataTransformCommand() *cobra.Command {
	var transformation, input, output string

	cmd := &cobra.Command{
		Use:   "transform",
		Short: "Transform data",
		Long:  `Apply transformations to data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Transforming data:\n")
			fmt.Printf("  Transformation: %s\n", transformation)
			fmt.Printf("  Input: %s\n", input)
			fmt.Printf("  Output: %s\n", output)
			return nil
		},
	}

	cmd.Flags().StringVarP(&transformation, "transform", "t", "", "Transformation to apply")
	cmd.Flags().StringVarP(&input, "input", "i", "", "Input file")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file")

	cmd.MarkFlagRequired("transform")
	cmd.MarkFlagRequired("input")
	cmd.MarkFlagRequired("output")

	return cmd
}

// Example command: Utilities
func newUtilsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "utils",
		Short: "Utility commands",
		Long:  `Various utility functions.`,
	}

	cmd.AddCommand(newUtilsHashCommand())
	cmd.AddCommand(newUtilsEncodeCommand())

	return cmd
}

func newUtilsHashCommand() *cobra.Command {
	var algorithm string
	var file bool

	cmd := &cobra.Command{
		Use:   "hash <input>",
		Short: "Generate hash of input",
		Long:  `Generate hash using various algorithms.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := args[0]
			inputType := "string"
			if file {
				inputType = "file"
			}

			fmt.Printf("Generating %s hash:\n", algorithm)
			fmt.Printf("  Input type: %s\n", inputType)
			fmt.Printf("  Input: %s\n", input)
			return nil
		},
	}

	cmd.Flags().StringVarP(&algorithm, "algorithm", "a", "sha256", "Hash algorithm (md5, sha1, sha256, sha512)")
	cmd.Flags().BoolVarP(&file, "file", "f", false, "Input is a file path")

	return cmd
}

func newUtilsEncodeCommand() *cobra.Command {
	var encoding string
	var decode bool

	cmd := &cobra.Command{
		Use:   "encode <input>",
		Short: "Encode/decode input",
		Long:  `Encode or decode input using various encodings.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := args[0]
			operation := "encode"
			if decode {
				operation = "decode"
			}

			fmt.Printf("%s using %s:\n", strings.Title(operation), encoding)
			fmt.Printf("  Input: %s\n", input)
			return nil
		},
	}

	cmd.Flags().StringVarP(&encoding, "encoding", "e", "base64", "Encoding type (base64, hex, url)")
	cmd.Flags().BoolVarP(&decode, "decode", "d", false, "Decode instead of encode")

	return cmd
}

// Usage Examples:
//
// 1. Build the application:
//    go build -o myapp examples/fluent-integration.go
//
// 2. Run as a regular CLI:
//    ./myapp user create --name "John Doe" --email "john@example.com"
//    ./myapp data import data.json --format json --batch-size 500
//    ./myapp utils hash "hello world" --algorithm sha256
//
// 3. Generate OpenAPI specification:
//    ./myapp openapi generate --title "MyApp API" --format yaml --output api.yaml
//
// 4. Validate OpenAPI specification:
//    ./myapp openapi validate api.yaml
//
// 5. Serve interactive Swagger UI documentation:
//    ./myapp openapi serve
//    ./myapp openapi serve --port 3000 --open --title "MyApp API"
//    ./myapp openapi serve --host 0.0.0.0 --port 8080 --auto-refresh
//
// 6. Run as an MCP server:
//    ./myapp mcp serve
//
// 7. View MCP configuration:
//    ./myapp mcp config
//
// 8. List MCP prompts:
//    ./myapp mcp prompt
//
// 9. Configure for Claude Desktop (add to claude_desktop_config.json):
//    {
//      "mcpServers": {
//        "myapp": {
//          "command": "/path/to/myapp",
//          "args": ["mcp", "serve"]
//        }
//      }
//    }
//
// The fluent API makes it incredibly easy to add OpenAPI, Swagger UI, and MCP functionality
// to any existing Cobra CLI with just one line:
//
//   extensions.CobraExtensions(rootCmd).All()
//
// Or for more control:
//
//   extensions.CobraExtensions(rootCmd).
//       OpenAPICommandWithConfig(myOpenAPIConfig).
//       MCPCommandWithConfig(myMCPConfig)
//
// For just Swagger UI documentation:
//
//   extensions.CobraExtensions(rootCmd).ServeCommand()