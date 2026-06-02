//go:build ignore

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/commons/duration"
	"github.com/spf13/cobra"
)

// Example: User listing with filtering
type ListUsersOptions struct {
	Role   string            `flag:"role" help:"Filter by user role" short:"r"`
	Limit  int               `flag:"limit" help:"Maximum results to return" default:"50"`
	Active bool              `flag:"active" help:"Only show active users" default:"true"`
	Since  time.Time         `flag:"since" help:"Created since (supports now-7d)" default:"now-30d"`
	Tags   []string          `flag:"tags" help:"Filter by tags (supports @file)" stdin:"true"`
	MaxAge duration.Duration `flag:"max-age" help:"Maximum account age" default:"365d"`
}

type User struct {
	ID     int      `json:"id"`
	Name   string   `json:"name"`
	Role   string   `json:"role"`
	Active bool     `json:"active"`
	Tags   []string `json:"tags"`
}

func listUsers(opts ListUsersOptions) (any, error) {
	// Simulate user data
	users := []User{
		{ID: 1, Name: "Alice", Role: "admin", Active: true, Tags: []string{"prod", "critical"}},
		{ID: 2, Name: "Bob", Role: "user", Active: true, Tags: []string{"dev"}},
		{ID: 3, Name: "Charlie", Role: "user", Active: false, Tags: []string{"staging"}},
	}

	// Filter users based on options
	var filtered []User
	for _, user := range users {
		if opts.Role != "" && user.Role != opts.Role {
			continue
		}
		if opts.Active && !user.Active {
			continue
		}
		if len(opts.Tags) > 0 {
			hasTag := false
			for _, tag := range opts.Tags {
				for _, userTag := range user.Tags {
					if tag == userTag {
						hasTag = true
						break
					}
				}
			}
			if !hasTag {
				continue
			}
		}
		filtered = append(filtered, user)
		if len(filtered) >= opts.Limit {
			break
		}
	}

	return filtered, nil
}

// Example: Data cleanup with duration
type CleanupOptions struct {
	MaxAge duration.Duration `flag:"max-age" help:"Delete items older than" default:"30d" short:"m"`
	DryRun bool              `flag:"dry-run" help:"Preview without deleting" default:"false"`
	Path   string            `flag:"path" help:"Path to clean" default:"/tmp"`
}

func cleanup(opts CleanupOptions) (any, error) {
	result := map[string]any{
		"path":    opts.Path,
		"max_age": opts.MaxAge.String(),
		"dry_run": opts.DryRun,
		"deleted": 42, // Simulated count
	}
	return result, nil
}

// Example: Query logs with time range
type QueryLogsOptions struct {
	Since time.Time `flag:"since" help:"Start time" default:"now-1h"`
	Until time.Time `flag:"until" help:"End time" default:"now"`
	Level string    `flag:"level" help:"Log level filter" default:"info" short:"l"`
	Query string    `flag:"query" help:"Search query" stdin:"true"`
}

func queryLogs(opts QueryLogsOptions) (any, error) {
	logs := []map[string]any{
		{
			"timestamp": time.Now().Add(-30 * time.Minute),
			"level":     "info",
			"message":   "Application started",
		},
		{
			"timestamp": time.Now().Add(-15 * time.Minute),
			"level":     "error",
			"message":   "Connection failed",
		},
	}
	return logs, nil
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "demo",
		Short: "Demo of clicky.AddCommand",
		Long:  `Demonstrates automatic flag binding, file loading, and stdin support`,
	}

	// Add list users command
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List users with filtering",
		Long: `List users with various filtering options.

Examples:
  # List all users
  demo list

  # List admin users only
  demo list --role admin

  # Filter by tags from file
  demo list --tags @tags.txt

  # Filter by tags from stdin
  echo -e "prod\ncritical" | demo list

  # List users created in last 7 days
  demo list --since now-7d`,
	}
	clicky.AddCommand(listCmd, ListUsersOptions{}, listUsers)
	rootCmd.AddCommand(listCmd)

	// Add cleanup command
	cleanupCmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up old data",
		Long: `Delete old data based on age.

Examples:
  # Preview cleanup (dry-run)
  demo cleanup --dry-run

  # Clean up items older than 60 days
  demo cleanup --max-age 60d

  # Clean specific path
  demo cleanup --path /var/log --max-age 7d`,
	}
	clicky.AddCommand(cleanupCmd, CleanupOptions{}, cleanup)
	rootCmd.AddCommand(cleanupCmd)

	// Add query logs command
	queryCmd := &cobra.Command{
		Use:   "query",
		Short: "Query application logs",
		Long: `Search and filter application logs by time range and level.

Examples:
  # Query last hour
  demo query

  # Query last 24 hours, error level
  demo query --since now-24h --level error

  # Query with search from stdin
  echo "connection error" | demo query`,
	}
	clicky.AddCommand(queryCmd, QueryLogsOptions{}, queryLogs)
	rootCmd.AddCommand(queryCmd)

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Usage Examples:
//
// 1. Build the demo:
//    go build -o demo examples/cobra-command-demo.go
//
// 2. List all users:
//    ./demo list
//    ./demo list --json
//    ./demo list --yaml
//
// 3. List with filters:
//    ./demo list --role admin --limit 10
//    ./demo list -r user --active=false
//
// 4. List with file input:
//    echo -e "prod\ncritical" > tags.txt
//    ./demo list --tags @tags.txt
//
// 5. List with stdin:
//    echo -e "prod\ncritical" | ./demo list
//
// 6. Cleanup with duration:
//    ./demo cleanup --max-age 60d
//    ./demo cleanup -m 2w --dry-run
//
// 7. Query logs with time range:
//    ./demo query --since now-24h
//    ./demo query --since now-7d --until now-1d
//    ./demo query -l error
//
// 8. Different output formats:
//    ./demo list --json
//    ./demo list --yaml
//    ./demo list --pretty
//    ./demo cleanup --json | jq '.deleted'
