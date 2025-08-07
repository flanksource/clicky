package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky/ai"
)

func main() {
	// Check for command line args
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ai_demo.go [simple|batch|commits]")
		os.Exit(1)
	}

	mode := os.Args[1]

	switch mode {
	case "simple":
		demoSimple()
	case "batch":
		demoBatch()
	case "commits":
		demoCommitAnalysis()
	default:
		fmt.Printf("Unknown mode: %s\n", mode)
		fmt.Println("Usage: go run ai_demo.go [simple|batch|commits]")
		os.Exit(1)
	}
}

func demoSimple() {
	fmt.Println("=== Simple Claude Execution Demo ===\n")
	
	options := ai.ClaudeOptions{
		Model:         "claude-3-haiku-20240307",
		MaxConcurrent: 1,
		Debug:         true,
	}

	executor := ai.NewClaudeExecutor(options)
	ctx := context.Background()

	response, err := executor.ExecutePrompt(ctx, "Simple Math", "What is 42 * 17? Reply with just the number.")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nResult: %s\n", response.Result)
	fmt.Printf("Tokens used: %d\n", response.GetTotalTokens())
	fmt.Printf("Cost: $%.6f\n", response.TotalCostUSD)
}

func demoBatch() {
	fmt.Println("=== Batch Processing Demo (Max 3 concurrent) ===\n")
	
	options := ai.ClaudeOptions{
		Model:         "claude-3-haiku-20240307",
		MaxConcurrent: 3,
		Debug:         false,
	}

	executor := ai.NewClaudeExecutor(options)
	ctx := context.Background()

	// Create 10 prompts that will be processed with max 3 concurrent
	prompts := make(map[string]string)
	for i := 1; i <= 10; i++ {
		name := fmt.Sprintf("Task %d", i)
		prompts[name] = fmt.Sprintf("What is %d squared? Reply with just the number.", i)
	}

	fmt.Printf("Processing %d prompts with max concurrency of 3...\n\n", len(prompts))

	responses, err := executor.ExecutePromptBatch(ctx, prompts)
	if err != nil {
		fmt.Printf("Batch execution had errors: %v\n", err)
	}

	fmt.Println("\n=== Results ===")
	totalTokens := 0
	totalCost := 0.0
	for i := 1; i <= 10; i++ {
		name := fmt.Sprintf("Task %d", i)
		if response, ok := responses[name]; ok {
			fmt.Printf("%s: %s (tokens: %d, cost: $%.6f)\n", 
				name, response.Result, response.GetTotalTokens(), response.TotalCostUSD)
			totalTokens += response.GetTotalTokens()
			totalCost += response.TotalCostUSD
		}
	}
	
	fmt.Printf("\nTotal tokens: %d\n", totalTokens)
	fmt.Printf("Total cost: $%.6f\n", totalCost)
}

func demoCommitAnalysis() {
	fmt.Println("=== Commit Analysis Demo ===\n")
	
	options := ai.AnalyzerOptions{
		ClaudeOptions: ai.ClaudeOptions{
			Model:         "claude-3-haiku-20240307",
			MaxConcurrent: 2,
			Debug:         false,
		},
	}

	analyzer := ai.NewCommitAnalyzer(options)
	ctx := context.Background()

	// Create sample commits to analyze
	commits := []ai.CommitInfo{
		{
			Hash:    "abc123def456",
			Message: "feat: add user authentication with JWT tokens",
			Author:  "John Doe",
			Date:    time.Now().Add(-24 * time.Hour),
			FileChanges: []ai.FileChange{
				{Path: "auth/jwt.go", ChangeType: "added", Additions: 150, Deletions: 0},
				{Path: "middleware/auth.go", ChangeType: "modified", Additions: 45, Deletions: 10},
				{Path: "config/auth.yaml", ChangeType: "added", Additions: 25, Deletions: 0},
			},
			Patch: `diff --git a/auth/jwt.go b/auth/jwt.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/auth/jwt.go
@@ -0,0 +1,150 @@
+package auth
+
+import (
+    "time"
+    "github.com/golang-jwt/jwt/v5"
+)
+
+// GenerateToken creates a new JWT token for the user
+func GenerateToken(userID string, expiry time.Duration) (string, error) {
+    claims := jwt.MapClaims{
+        "user_id": userID,
+        "exp":     time.Now().Add(expiry).Unix(),
+    }
+    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
+    return token.SignedString(secretKey)
+}`,
		},
		{
			Hash:    "def789ghi012",
			Message: "fix: resolve memory leak in cache implementation",
			Author:  "Jane Smith",
			Date:    time.Now().Add(-48 * time.Hour),
			FileChanges: []ai.FileChange{
				{Path: "cache/memory.go", ChangeType: "modified", Additions: 15, Deletions: 8},
				{Path: "cache/memory_test.go", ChangeType: "modified", Additions: 30, Deletions: 5},
			},
			Patch: `diff --git a/cache/memory.go b/cache/memory.go
index 1234567..2345678 100644
--- a/cache/memory.go
+++ b/cache/memory.go
@@ -45,8 +45,15 @@ func (c *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
-    c.items[key] = &cacheItem{
-        value: value,
-        expiry: time.Now().Add(ttl),
-    }
+    // Fix: Properly clean up old items before setting new ones
+    if old, exists := c.items[key]; exists {
+        old.cleanup()
+    }
+    
+    item := &cacheItem{
+        value:  value,
+        expiry: time.Now().Add(ttl),
+    }
+    c.items[key] = item
+    c.scheduleCleanup(key, ttl)`,
		},
		{
			Hash:    "ghi345jkl678",
			Message: "docs: update API documentation with examples",
			Author:  "Bob Wilson",
			Date:    time.Now().Add(-72 * time.Hour),
			FileChanges: []ai.FileChange{
				{Path: "README.md", ChangeType: "modified", Additions: 85, Deletions: 20},
				{Path: "docs/api.md", ChangeType: "added", Additions: 250, Deletions: 0},
			},
			Patch: `diff --git a/README.md b/README.md
index 1234567..3456789 100644
--- a/README.md
+++ b/README.md
@@ -10,20 +10,85 @@
-## Usage
-
-See documentation for details.
+## Quick Start
+
+### Installation
+\`\`\`bash
+go get github.com/example/project
+\`\`\`
+
+### Basic Usage
+\`\`\`go
+import "github.com/example/project"
+
+client := project.NewClient()
+result, err := client.DoSomething()
+\`\`\`
+
+For more examples, see [API Documentation](docs/api.md).`,
		},
	}

	fmt.Printf("Analyzing %d commits...\n\n", len(commits))

	results, err := analyzer.AnalyzeCommitsBatch(ctx, commits)
	if err != nil {
		fmt.Printf("Error analyzing commits: %v\n", err)
		// Still show partial results
	}

	fmt.Println("\n=== Analysis Results ===")
	for _, commit := range commits {
		if summary, ok := results[commit.Hash]; ok {
			fmt.Printf("\n%s (%s):\n", commit.Hash[:8], firstLine(commit.Message))
			fmt.Printf("  Summary: %s\n", summary)
		} else {
			fmt.Printf("\n%s: Failed to analyze\n", commit.Hash[:8])
		}
	}
}

func firstLine(s string) string {
	if len(s) > 50 {
		return s[:47] + "..."
	}
	return s
}