package main

import (
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky/extensions"
	"github.com/flanksource/clicky/rpc"
	"github.com/spf13/cobra"
)

// Example demonstrating the dynamic command execution functionality
func main() {
	// Create your application's root command
	rootCmd := &cobra.Command{
		Use:   "dynamic-demo",
		Short: "Demo application with dynamic command execution",
		Long: `A demonstration CLI application that showcases how to add
dynamic command execution to any Cobra-based CLI using clicky.

This allows HTTP requests to execute CLI commands dynamically.`,
	}

	// Add your application commands
	rootCmd.AddCommand(newUserCommand())
	rootCmd.AddCommand(newConfigCommand())
	rootCmd.AddCommand(newStatusCommand())

	// Add OpenAPI command with executor enabled by default
	config := &rpc.OpenAPIConfig{
		Title:       "Dynamic Demo API",
		Description: "Interactive API with dynamic command execution",
		Version:     "1.0.0",
		Contact: &rpc.OpenAPIContact{
			Name:  "Demo Team",
			Email: "demo@example.com",
		},
	}

	extensions.CobraExtensions(rootCmd).
		OpenAPICommandWithConfig(config)

	// Execute the CLI
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// User management commands
func newUserCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
		Long:  "Manage users in the system.",
	}

	// User create command
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		Long:  "Create a new user with specified details.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			email, _ := cmd.Flags().GetString("email")
			role, _ := cmd.Flags().GetString("role")

			// Validate required fields (demonstrates direct stderr output capture)
			if name == "" {
				fmt.Fprintf(os.Stderr, "❌ Error: name is required\n")
				return fmt.Errorf("name cannot be empty")
			}
			if email == "" {
				fmt.Fprintf(os.Stderr, "❌ Error: email is required\n")
				return fmt.Errorf("email cannot be empty")
			}

			// Write to stdout using direct fmt.Printf (demonstrates global capture)
			fmt.Printf("✅ Created user:\n")
			fmt.Printf("   Name: %s\n", name)
			fmt.Printf("   Email: %s\n", email)
			fmt.Printf("   Role: %s\n", role)
			fmt.Printf("   Created at: %s\n", time.Now().Format(time.RFC3339))

			// Write warnings directly to stderr (demonstrates global stderr capture)
			if role == "admin" {
				fmt.Fprintf(os.Stderr, "⚠️  Warning: Admin privileges granted\n")
			}

			return nil
		},
	}
	createCmd.Flags().StringP("name", "n", "", "User name")
	createCmd.Flags().StringP("email", "e", "", "User email address")
	createCmd.Flags().StringP("role", "r", "user", "User role (user, admin)")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("email")

	// User list command
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all users",
		Long:  "Display all users in the system.",
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			role, _ := cmd.Flags().GetString("role")

			// Output directly to stdout (demonstrates global capture)
			fmt.Printf("📋 Users (limit: %d, role filter: %s):\n", limit, role)
			fmt.Printf("1. Alice Smith (alice@example.com) - admin\n")
			fmt.Printf("2. Bob Johnson (bob@example.com) - user\n")
			fmt.Printf("3. Carol Brown (carol@example.com) - user\n")

			// Warning directly to stderr (demonstrates global stderr capture)
			if role == "" {
				fmt.Fprintf(os.Stderr, "⚠️  Warning: Listing all users without role filter\n")
			}

			return nil
		},
	}
	listCmd.Flags().IntP("limit", "l", 10, "Maximum number of users to list")
	listCmd.Flags().StringP("role", "r", "", "Filter by role")

	// User delete command
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a user",
		Long:  "Remove a user from the system.",
		RunE: func(cmd *cobra.Command, args []string) error {
			userID, _ := cmd.Flags().GetString("id")
			force, _ := cmd.Flags().GetBool("force")

			// Simulate user not found error (demonstrates error with direct stderr output)
			if userID == "999" {
				fmt.Fprintf(os.Stderr, "❌ Error: User not found\n")
				return fmt.Errorf("user with ID %s does not exist", userID)
			}

			// Output directly to stdout
			if force {
				fmt.Printf("🗑️  Force deleted user ID: %s\n", userID)
			} else {
				fmt.Printf("🗑️  Deleted user ID: %s\n", userID)
			}

			// Warning directly to stderr for force delete
			if force {
				fmt.Fprintf(os.Stderr, "⚠️  Warning: Force delete bypassed safety checks\n")
			}

			return nil
		},
	}
	deleteCmd.Flags().StringP("id", "i", "", "User ID to delete")
	deleteCmd.Flags().BoolP("force", "f", false, "Force delete without confirmation")
	deleteCmd.MarkFlagRequired("id")

	cmd.AddCommand(createCmd, listCmd, deleteCmd)
	return cmd
}

// Configuration commands
func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
		Long:  "Manage application configuration settings.",
	}

	// Config get command
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get configuration value",
		Long:  "Retrieve a configuration setting value.",
		RunE: func(cmd *cobra.Command, args []string) error {
			key, _ := cmd.Flags().GetString("key")
			fmt.Printf("🔧 Config[%s] = \"example-value\"\n", key)
			return nil
		},
	}
	getCmd.Flags().StringP("key", "k", "", "Configuration key")
	getCmd.MarkFlagRequired("key")

	// Config set command
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Set configuration value",
		Long:  "Update a configuration setting value.",
		RunE: func(cmd *cobra.Command, args []string) error {
			key, _ := cmd.Flags().GetString("key")
			value, _ := cmd.Flags().GetString("value")
			global, _ := cmd.Flags().GetBool("global")

			scope := "local"
			if global {
				scope = "global"
			}

			fmt.Printf("🔧 Set %s config[%s] = \"%s\"\n", scope, key, value)
			return nil
		},
	}
	setCmd.Flags().StringP("key", "k", "", "Configuration key")
	setCmd.Flags().StringP("value", "v", "", "Configuration value")
	setCmd.Flags().BoolP("global", "g", false, "Set global configuration")
	setCmd.MarkFlagRequired("key")
	setCmd.MarkFlagRequired("value")

	cmd.AddCommand(getCmd, setCmd)
	return cmd
}

// Status command
func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show system status",
		Long:  "Display current system status and health information.",
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			format, _ := cmd.Flags().GetString("format")

			// Output main status directly to stdout
			fmt.Printf("📊 System Status (%s format):\n", format)
			fmt.Printf("   Status: ✅ Healthy\n")
			fmt.Printf("   Uptime: 2h 15m 30s\n")
			fmt.Printf("   Version: 1.0.0\n")

			if verbose {
				fmt.Printf("   Memory: 256MB / 1GB\n")
				fmt.Printf("   CPU: 15%%\n")
				fmt.Printf("   Disk: 4.2GB / 10GB\n")

				// Warning about high disk usage directly to stderr
				fmt.Fprintf(os.Stderr, "⚠️  Warning: Disk usage above 40%%\n")
			}

			// Info message directly to stderr
			fmt.Fprintf(os.Stderr, "ℹ️  Status check completed\n")

			return nil
		},
	}

	cmd.Flags().BoolP("verbose", "v", false, "Show detailed status information")
	cmd.Flags().StringP("format", "f", "text", "Output format (text, json, yaml)")

	return cmd
}

// Usage Examples:
//
// 1. Build the application:
//    go build -o dynamic-demo examples/dynamic-executor.go
//
// 2. Run regular CLI commands:
//    ./dynamic-demo user create --name "John Doe" --email "john@example.com" --role admin
//    ./dynamic-demo config set --key "theme" --value "dark" --global
//    ./dynamic-demo status --verbose
//
// 3. Start server with dynamic execution enabled:
//    ./dynamic-demo openapi serve --enable-executor --port 8080 --open
//
//    This will start a server at http://localhost:8080 with:
//    - Interactive Swagger UI at: http://localhost:8080/
//    - OpenAPI JSON spec at: http://localhost:8080/api/openapi.json
//    - Dynamic command execution endpoints matching the OpenAPI spec
//
// 4. Test dynamic execution via HTTP (with full output capture):
//
//    # Create a user via HTTP POST (captures stdout, stderr, exit code)
//    curl -X POST "http://localhost:8080/api/v1/user" \
//      -H "Content-Type: application/json" \
//      -d '{"flags": {"name": "Alice", "email": "alice@example.com", "role": "admin"}}'
//    # Response includes:
//    # {
//    #   "success": true,
//    #   "message": "Command executed successfully",
//    #   "stdout": "✅ Created user:\n   Name: Alice\n   Email: alice@example.com\n   Role: admin\n   Created at: 2025-01-21T10:30:00Z\n",
//    #   "stderr": "⚠️  Warning: Admin privileges granted\n",
//    #   "exit_code": 0
//    # }
//
//    # Test error handling - user creation without name
//    curl -X POST "http://localhost:8080/api/v1/user" \
//      -H "Content-Type: application/json" \
//      -d '{"flags": {"email": "alice@example.com"}}'
//    # Response includes error output:
//    # {
//    #   "success": false,
//    #   "message": "Command execution failed",
//    #   "stderr": "❌ Error: name is required\n",
//    #   "error": "name cannot be empty",
//    #   "exit_code": 1
//    # }
//
//    # List users via HTTP GET (captures warnings)
//    curl -X GET "http://localhost:8080/api/v1/user?limit=5"
//    # Response includes:
//    # {
//    #   "success": true,
//    #   "stdout": "📋 Users (limit: 5, role filter: ):\n1. Alice Smith...\n",
//    #   "stderr": "⚠️  Warning: Listing all users without role filter\n",
//    #   "exit_code": 0
//    # }
//
//    # Test user not found error
//    curl -X DELETE "http://localhost:8080/api/v1/user?id=999"
//    # Response includes:
//    # {
//    #   "success": false,
//    #   "message": "Command execution failed",
//    #   "stderr": "❌ Error: User not found\n",
//    #   "error": "user with ID 999 does not exist",
//    #   "exit_code": 1
//    # }
//
//    # Get status with verbose output
//    curl -X GET "http://localhost:8080/api/v1/status?verbose=true&format=json"
//    # Response includes:
//    # {
//    #   "success": true,
//    #   "stdout": "📊 System Status (json format):\n   Status: ✅ Healthy\n...",
//    #   "stderr": "⚠️  Warning: Disk usage above 40%\nℹ️  Status check completed\n",
//    #   "exit_code": 0
//    # }
//
// 5. Advanced usage:
//    ./dynamic-demo openapi serve --enable-executor --skip-pre-run=false --title "My API"
//
// Enhanced Features provided by the dynamic executor:
// ✅ HTTP endpoints automatically generated from CLI commands
// ✅ Request parameters mapped to command flags and arguments
// ✅ Pre-run hooks can be skipped for API usage
// ✅ Perfect consistency between OpenAPI documentation and execution
// ✅ JSON request/response format with comprehensive output capture
// ✅ Parameter validation based on command definitions
// ✅ Error handling and proper HTTP status codes
// ✅ CORS support for web integration
// 🆕 Complete stdout/stderr capture and return
// 🆕 Exit code reporting for all command executions
// 🆕 Error responses include any output produced before failure
// 🆕 Separate stdout and stderr fields in JSON responses
// 🆕 Combined output field for backward compatibility
// 🆕 Enhanced debugging capabilities with full command output visibility
//
// The dynamic executor creates REST endpoints that match the OpenAPI specification:
// - POST /api/v1/user (for user create)
// - GET /api/v1/user (for user list)
// - DELETE /api/v1/user (for user delete)
// - GET /api/v1/config (for config get)
// - PUT /api/v1/config (for config set)
// - GET /api/v1/status (for status)
//
// Each endpoint accepts the same parameters as the CLI command, either as:
// - Query parameters (for GET requests)
// - JSON body with "flags" and "args" fields (for POST/PUT/DELETE requests)
