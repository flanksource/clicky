//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/flanksource/clicky/extensions"
	"github.com/spf13/cobra"
)

// Example demonstrating the Swagger UI serve functionality
func main() {
	// Create your application's root command
	rootCmd := &cobra.Command{
		Use:   "swagger-demo",
		Short: "Demo application with Swagger UI documentation",
		Long: `A demonstration CLI application that showcases how to add
interactive Swagger UI documentation to any Cobra-based CLI using clicky.`,
	}

	// Add your application commands
	rootCmd.AddCommand(newAPICommand())
	rootCmd.AddCommand(newDatabaseCommand())
	rootCmd.AddCommand(newCacheCommand())

	// OPTION 1: Add complete OpenAPI functionality (including serve)
	extensions.CobraExtensions(rootCmd).OpenAPICommand()

	// OPTION 2: Add only serve functionality
	// extensions.CobraExtensions(rootCmd).ServeCommand()

	// OPTION 3: Add with custom OpenAPI configuration
	// import "github.com/flanksource/clicky/rpc"
	// customConfig := &rpc.OpenAPIConfig{
	//     Title:       "Swagger Demo API",
	//     Description: "Interactive API documentation for demo application",
	//     Version:     "2.0.0",
	//     Contact: &rpc.OpenAPIContact{
	//         Name:  "API Team",
	//         Email: "api@example.com",
	//         URL:   "https://example.com/contact",
	//     },
	//     License: &rpc.OpenAPILicense{
	//         Name: "MIT",
	//         URL:  "https://opensource.org/licenses/MIT",
	//     },
	//     Servers: []rpc.OpenAPIServer{
	//         {
	//             URL:         "https://api.example.com/v1",
	//             Description: "Production server",
	//         },
	//         {
	//             URL:         "https://staging-api.example.com/v1",
	//             Description: "Staging server",
	//         },
	//     },
	// }
	// extensions.CobraExtensions(rootCmd).OpenAPICommandWithConfig(customConfig)

	// Execute the CLI
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Example API management commands
func newAPICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "API management commands",
		Long:  "Manage API endpoints, routes, and configurations.",
	}

	// API create endpoint command
	createCmd := &cobra.Command{
		Use:   "create-endpoint",
		Short: "Create a new API endpoint",
		Long:  "Create a new API endpoint with specified configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("path")
			method, _ := cmd.Flags().GetString("method")
			description, _ := cmd.Flags().GetString("description")

			fmt.Printf("Creating API endpoint:\n")
			fmt.Printf("  Path: %s\n", path)
			fmt.Printf("  Method: %s\n", method)
			fmt.Printf("  Description: %s\n", description)
			return nil
		},
	}
	createCmd.Flags().StringP("path", "p", "/", "API endpoint path")
	createCmd.Flags().StringP("method", "m", "GET", "HTTP method (GET, POST, PUT, DELETE)")
	createCmd.Flags().StringP("description", "d", "", "Endpoint description")
	createCmd.MarkFlagRequired("path")

	// API list endpoints command
	listCmd := &cobra.Command{
		Use:   "list-endpoints",
		Short: "List all API endpoints",
		Long:  "Display all configured API endpoints with their details.",
		RunE: func(cmd *cobra.Command, args []string) error {
			filter, _ := cmd.Flags().GetString("filter")
			fmt.Printf("Listing API endpoints (filter: %s)\n", filter)
			return nil
		},
	}
	listCmd.Flags().StringP("filter", "f", "", "Filter endpoints by path or method")

	cmd.AddCommand(createCmd, listCmd)
	return cmd
}

// Example database management commands
func newDatabaseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "database",
		Short: "Database management commands",
		Long:  "Manage database connections, migrations, and data operations.",
	}

	// Database connect command
	connectCmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect to a database",
		Long:  "Establish a connection to the specified database.",
		RunE: func(cmd *cobra.Command, args []string) error {
			host, _ := cmd.Flags().GetString("host")
			port, _ := cmd.Flags().GetInt("port")
			database, _ := cmd.Flags().GetString("database")
			user, _ := cmd.Flags().GetString("user")
			sslMode, _ := cmd.Flags().GetString("ssl-mode")

			fmt.Printf("Connecting to database:\n")
			fmt.Printf("  Host: %s\n", host)
			fmt.Printf("  Port: %d\n", port)
			fmt.Printf("  Database: %s\n", database)
			fmt.Printf("  User: %s\n", user)
			fmt.Printf("  SSL Mode: %s\n", sslMode)
			return nil
		},
	}
	connectCmd.Flags().StringP("host", "h", "localhost", "Database host")
	connectCmd.Flags().IntP("port", "p", 5432, "Database port")
	connectCmd.Flags().StringP("database", "d", "", "Database name")
	connectCmd.Flags().StringP("user", "u", "", "Database user")
	connectCmd.Flags().String("ssl-mode", "prefer", "SSL mode (disable, allow, prefer, require)")
	connectCmd.MarkFlagRequired("database")
	connectCmd.MarkFlagRequired("user")

	// Database migrate command
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		Long:  "Execute pending database migrations to update schema.",
		RunE: func(cmd *cobra.Command, args []string) error {
			direction, _ := cmd.Flags().GetString("direction")
			steps, _ := cmd.Flags().GetInt("steps")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			fmt.Printf("Running database migration:\n")
			fmt.Printf("  Direction: %s\n", direction)
			fmt.Printf("  Steps: %d\n", steps)
			fmt.Printf("  Dry Run: %t\n", dryRun)
			return nil
		},
	}
	migrateCmd.Flags().StringP("direction", "d", "up", "Migration direction (up, down)")
	migrateCmd.Flags().IntP("steps", "s", 0, "Number of migration steps (0 = all)")
	migrateCmd.Flags().Bool("dry-run", false, "Show what would be migrated without executing")

	cmd.AddCommand(connectCmd, migrateCmd)
	return cmd
}

// Example cache management commands
func newCacheCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Cache management commands",
		Long:  "Manage application cache, including Redis and in-memory caches.",
	}

	// Cache set command
	setCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a cache value",
		Long:  "Store a key-value pair in the cache with optional expiration.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]
			ttl, _ := cmd.Flags().GetInt("ttl")
			namespace, _ := cmd.Flags().GetString("namespace")

			fmt.Printf("Setting cache value:\n")
			fmt.Printf("  Key: %s\n", key)
			fmt.Printf("  Value: %s\n", value)
			fmt.Printf("  TTL: %d seconds\n", ttl)
			fmt.Printf("  Namespace: %s\n", namespace)
			return nil
		},
	}
	setCmd.Flags().IntP("ttl", "t", 3600, "Time to live in seconds")
	setCmd.Flags().StringP("namespace", "n", "default", "Cache namespace")

	// Cache get command
	getCmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a cache value",
		Long:  "Retrieve a value from the cache by its key.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			namespace, _ := cmd.Flags().GetString("namespace")

			fmt.Printf("Getting cache value:\n")
			fmt.Printf("  Key: %s\n", key)
			fmt.Printf("  Namespace: %s\n", namespace)
			return nil
		},
	}
	getCmd.Flags().StringP("namespace", "n", "default", "Cache namespace")

	// Cache clear command
	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear cache entries",
		Long:  "Remove cache entries by pattern or clear entire cache.",
		RunE: func(cmd *cobra.Command, args []string) error {
			pattern, _ := cmd.Flags().GetString("pattern")
			namespace, _ := cmd.Flags().GetString("namespace")
			force, _ := cmd.Flags().GetBool("force")

			fmt.Printf("Clearing cache:\n")
			fmt.Printf("  Pattern: %s\n", pattern)
			fmt.Printf("  Namespace: %s\n", namespace)
			fmt.Printf("  Force: %t\n", force)
			return nil
		},
	}
	clearCmd.Flags().StringP("pattern", "p", "*", "Key pattern to clear")
	clearCmd.Flags().StringP("namespace", "n", "default", "Cache namespace")
	clearCmd.Flags().BoolP("force", "f", false, "Force clear without confirmation")

	cmd.AddCommand(setCmd, getCmd, clearCmd)
	return cmd
}

// Usage Examples:
//
// 1. Build the application:
//    go build -o swagger-demo examples/swagger-serve.go
//
// 2. Run regular CLI commands:
//    ./swagger-demo api create-endpoint --path "/users" --method POST --description "Create user"
//    ./swagger-demo database connect --host localhost --database myapp --user admin
//    ./swagger-demo cache set "user:123" "John Doe" --ttl 7200
//
// 3. Generate OpenAPI specification:
//    ./swagger-demo openapi generate --format yaml
//
// 4. Start Swagger UI documentation server:
//    ./swagger-demo openapi serve
//
//    This will start a server at http://localhost:8080 with:
//    - Interactive Swagger UI at: http://localhost:8080/
//    - OpenAPI JSON spec at: http://localhost:8080/api/openapi.json
//    - OpenAPI YAML spec at: http://localhost:8080/api/openapi.yaml
//    - Health check at: http://localhost:8080/health
//
// 5. Advanced serve options:
//    ./swagger-demo openapi serve --port 3000 --open
//    ./swagger-demo openapi serve --title "My API" --description "Custom description"
//    ./swagger-demo openapi serve --host 0.0.0.0 --port 8080 --auto-refresh
//
// 6. Access the documentation:
//    - Open your browser to http://localhost:8080 (or the port you specified)
//    - Explore the interactive API documentation
//    - Try out API endpoints directly from the UI
//    - Download the OpenAPI specification in JSON or YAML format
//
// The serve command provides:
// ✅ Real-time OpenAPI specification generation from CLI commands
// ✅ Interactive Swagger UI with try-it-out functionality
// ✅ Auto-refresh capability for development
// ✅ Multi-format API specification download (JSON/YAML)
// ✅ Health check endpoint for monitoring
// ✅ CORS support for web integration
// ✅ Customizable metadata (title, description, version)
// ✅ Browser auto-launch option
// ✅ Flexible host and port configuration
