# Clicky Framework

A comprehensive Go framework for data formatting, concurrent task execution, web middleware, and CLI-to-API generation. Clicky transforms command-line applications into full-featured systems with beautiful output formatting, robust task management, configurable middleware, and automatic API documentation.

## Core Components

### 🎨 **Formatters** - Advanced Data Formatting
- **Multi-format output**: Pretty tables, JSON, YAML, CSV, Markdown, HTML, PDF, Excel, Tree views
- **Smart struct tags**: Control formatting with `pretty` tags for tables, trees, and styling
- **Tailwind CSS integration**: Built-in support for Tailwind utility classes
- **Dynamic styling**: Fluent API for programmatic content styling
- **Unstructured data**: Handle dynamic data from APIs and user input

### 📊 **Task System** - Concurrent Execution
- **TypedTask & TypedGroup**: Type-safe task execution with structured results
- **Progress tracking**: Visual progress bars with status management
- **Concurrency control**: Manager and group-level limits with semaphores
- **Health system**: Automatic status determination with health mixins
- **Retry logic**: Exponential backoff with configurable error handling
- **Integration ready**: CLI tools, HTTP servers, batch processing

### 🔧 **Middleware** - Echo v4 Web Framework
- **20+ middleware types**: Authentication, security, performance, CORS, logging
- **YAML configuration**: Environment-specific configs with validation
- **CEL integration**: Dynamic request/response processing
- **Authentication**: Basic Auth, JWT, API keys with htpasswd support
- **Security hardening**: CSRF, secure headers, rate limiting
- **Preset configurations**: Development, production, security-focused setups

### 🌐 **OpenAPI & Server** - CLI to API Generation
- **Automatic OpenAPI spec generation**: Convert Cobra commands to OpenAPI 3.0.3
- **Interactive Swagger UI**: Live documentation server with try-it-out functionality
- **Command execution**: HTTP endpoints for running CLI commands
- **Fluent integration**: Simple `.OpenAPICommand()` and `.MCPCommand()` extensions
- **MCP support**: Model Context Protocol for AI assistant integration

## Quick Start

### Basic Data Formatting

```go
package main

import (
    "fmt"
    "github.com/flanksource/clicky"
)

type User struct {
    Name   string `pretty:"label=Full Name,table"`
    Age    int    `pretty:"label=Age,color=blue,table"`
    City   string `pretty:"label=Location,color=gray-500,table"`
    Status string `pretty:"label=Status,sort,table"`
}

func main() {
    users := []User{
        {Name: "Alice", Age: 30, City: "New York", Status: "Active"},
        {Name: "Bob", Age: 25, City: "Los Angeles", Status: "Inactive"},
    }

    // Format as pretty table (default)
    prettyStr, _ := clicky.Format(users, clicky.FormatOptions{Format: "pretty"})
    fmt.Println(prettyStr)

    // Export as different formats
    jsonStr, _ := clicky.Format(users, clicky.FormatOptions{Format: "json"})
    yamlStr, _ := clicky.Format(users, clicky.FormatOptions{Format: "yaml"})
    markdownStr, _ := clicky.Format(users, clicky.FormatOptions{Format: "markdown"})
}
```

### Task Execution

```go
import "github.com/flanksource/clicky/task"

func main() {
    // Start a simple task with progress tracking
    task := task.StartTask("Download Files", func(ctx context.Context, t *task.Task) error {
        t.Infof("Starting download...")
        for i := 0; i <= 100; i += 10 {
            t.SetProgress(i, 100)
            time.Sleep(100 * time.Millisecond)
        }
        t.Infof("Download complete")
        return nil
    })

    result := task.WaitFor()
    if result.Error != nil {
        fmt.Printf("Task failed: %v\n", result.Error)
    }
}
```

### CLI with OpenAPI Documentation

```go
import (
    "github.com/flanksource/clicky/extensions"
    "github.com/spf13/cobra"
)

func main() {
    rootCmd := &cobra.Command{
        Use:   "myapp",
        Short: "My CLI application",
    }

    // Add your commands
    rootCmd.AddCommand(myCommands()...)

    // Add OpenAPI documentation and MCP support
    extensions.CobraExtensions(rootCmd).
        OpenAPICommand().  // Adds 'openapi generate' and 'openapi serve'
        MCPCommand()       // Adds 'mcp serve' for AI integration

    rootCmd.Execute()
}

// Usage:
// myapp openapi serve  # Start Swagger UI at http://localhost:8080
// myapp mcp serve      # Start MCP server for AI assistants
```

## Package Documentation

### Formatters Package

The formatters package provides comprehensive data formatting with multiple output formats and advanced styling.

#### Supported Output Formats

All struct-based data works seamlessly across multiple output formats:

- **`pretty`**: Human-readable tables and formatting (default)
- **`json`**: JSON formatted output with proper indentation
- **`yaml`/`yml`**: YAML formatted output
- **`csv`**: Comma-separated values format for tabular data
- **`markdown`/`md`**: Markdown tables with styling
- **`html`**: HTML tables with Tailwind CSS classes
- **`html-pdf`**: HTML-to-PDF conversion for reports
- **`excel`/`xlsx`**: Excel spreadsheet format
- **`tree`**: Hierarchical tree view for nested data

#### Pretty Tag Reference

The pretty tag system provides comprehensive control over data formatting and styling:

##### Table Tags

```go
type Product struct {
    ID          int     `pretty:"label=Product ID,width=10,table"`
    Name        string  `pretty:"label=Product Name,color=blue,table"`
    Price       float64 `pretty:"label=Price,format=currency,sort,table"`
    InStock     bool    `pretty:"label=Available,color=green,table"`
    CreatedAt   time.Time `pretty:"label=Created,format=date,table"`
}
```

Table tag options:
- **`label=HeaderName`** - Sets the column header name
- **`color=tailwind-class`** - Applies color styling
- **`sort`** - Makes column sortable
- **`width=N`** - Sets column width
- **`format=type`** - date, currency, percentage, etc.
- **`table`** - Marks field for table formatting

##### Tree Formatting

```go
type Organization struct {
    Department *Department `pretty:"tree"`
}

type Department struct {
    Name     string        `json:"name"`
    Manager  string        `json:"manager"`
    Teams    []*Department `json:"teams,omitempty"`
}
```

Tree tag options:
- **`tree`** - Marks field for tree formatting
- **`icon=custom-icon`** - Sets custom icon for nodes

##### Style Integration

```go
type StatusReport struct {
    Message string `pretty:"color=green,font=bold"`
    Level   string `pretty:"color=yellow"`
    Time    string `pretty:"color=gray-500,style=italic"`
}
```

Style tag options:
- **`color=tailwind-class`** - Text colors (red, green, blue, etc.)
- **`font=weight`** - bold, semibold, medium, normal
- **`style=modifier`** - italic, underline, line-through
- **`opacity=level`** - 25, 50, 75, 100

#### Tailwind CSS Support

The formatters package provides comprehensive Tailwind CSS integration:

```go
type StyledContent struct {
    Title   string `pretty:"color=blue-600,font=bold,text=lg"`
    Status  string `pretty:"color=green-500"`
    Footer  string `pretty:"color=gray-500,style=italic"`
}
```

Supported Tailwind utilities:
- **Colors**: `text-{color}-{shade}` (e.g., text-blue-600, text-red-500)
- **Typography**: `font-bold`, `font-semibold`, `italic`, `underline`
- **Spacing**: `p-{value}`, `px-{value}`, `py-{value}`, `pt-{value}`, etc.
- **Opacity**: `opacity-25`, `opacity-50`, `opacity-75`, `opacity-100`

#### Format Options

Control output behavior with `FormatOptions`:

```go
options := clicky.FormatOptions{
    Format:  "markdown",
    NoColor: false,
    Output:  "report.md",
}

// Write to file
err := clicky.FormatToFile(users, options, "report.md")

// Or get as string
result, err := clicky.Format(users, options)
```

### Task System

The task system provides comprehensive concurrent task execution with progress tracking and visual rendering.

#### Core Concepts

##### Task Lifecycle

Tasks progress through well-defined states with visual indicators:
- **StatusPending** (⏳): Task is queued but not yet started
- **StatusRunning** (⟳): Task is currently executing
- **StatusSuccess** (✓): Task completed successfully
- **StatusFailed** (✗): Task failed with an error
- **StatusWarning** (⚠): Task completed with warnings
- **StatusCancelled** (⊘): Task was canceled

##### TypedTask for Type Safety

```go
import "github.com/flanksource/clicky/task"

// Define a typed task that returns a string
task := task.StartTask("Fetch Data", func(ctx context.Context, t *task.Task) (string, error) {
    // Perform work and return typed result
    return "Hello, World!", nil
})

// Get typed result
result, err := task.GetResult()
// result is of type string

// Or wait and get result in one call
wait := task.WaitFor()
if wait.Error != nil {
    // Handle error
}
```

##### TypedGroup for Batch Processing

```go
group := task.NewTypedGroup[UserData]("Load Users")

user1 := group.Add("Load User 1", func(ctx context.Context, t *task.Task) (UserData, error) {
    return loadUser(1)
})

user2 := group.Add("Load User 2", func(ctx context.Context, t *task.Task) (UserData, error) {
    return loadUser(2)
})

// Get all results as map
results, err := group.GetResults()
if err != nil {
    return err
}

for task, userData := range results {
    fmt.Printf("Task %s loaded: %+v\n", task.Name(), userData)
}
```

#### Status and Health System

##### Health Mixin

Results can implement HealthMixin for automatic status determination:

```go
type DatabaseResult struct {
    Connected bool
    Error     error
}

func (r DatabaseResult) Health() task.Health {
    if r.Error != nil {
        return task.HealthError
    }
    if !r.Connected {
        return task.HealthWarning
    }
    return task.HealthOK
}

// Task status will automatically reflect the health
task.SetResult(DatabaseResult{Connected: true})
// Task status becomes StatusSuccess automatically
```

#### Concurrency Control

##### Manager-Level Concurrency

```go
manager := task.NewManager(task.WithMaxConcurrency(10))
// Maximum 10 tasks running simultaneously
```

##### Group-Level Concurrency

```go
group := task.NewGroup("API Calls", task.WithConcurrency(3))
// Maximum 3 concurrent tasks within this group
```

#### Error Handling and Retry

##### Retry Configuration

```go
retryConfig := task.RetryConfig{
    RetryableErrors: []string{"timeout", "connection", "rate limit"},
    BaseDelay:      1 * time.Second,
    MaxDelay:       30 * time.Second,
    BackoffFactor:  2.0,
    JitterFactor:   0.1,
    MaxRetries:     3,
}

task := task.StartTaskWithOptions("Flaky Operation", taskFunc,
    task.WithRetry(retryConfig),
)
```

#### Progress Tracking

```go
task := task.StartTask("Upload Files", func(ctx context.Context, t *task.Task) error {
    files := getFilesToUpload()
    total := len(files)

    for i, file := range files {
        t.SetProgress(i, total)
        t.Infof("Uploading %s...", file.Name)
        err := uploadFile(file)
        if err != nil {
            return err
        }
    }

    t.SetProgress(total, total) // 100% complete
    return nil
})
```

#### Integration Examples

##### CLI Tool Integration

```go
func runCommand(args []string) error {
    manager := task.NewManager(
        task.WithVerbose(verbose),
        task.WithNoProgress(noProgress),
    )

    // Start background tasks
    task1 := task.StartTask("Validate Input", validateInput)
    task2 := task.StartTask("Load Configuration", loadConfig)

    // Wait for prerequisites
    task1.WaitFor()
    task2.WaitFor()

    // Main processing
    mainTask := task.StartTask("Process Data", processData)
    return mainTask.WaitFor().Error
}
```

##### Batch Processing

```go
func processBatch(items []Item) error {
    group := task.NewTypedGroup[ProcessedItem]("Batch Processing",
        task.WithConcurrency(5),
    )

    // Process items concurrently
    for i, item := range items {
        group.Add(fmt.Sprintf("Process Item %d", i+1),
            func(ctx context.Context, t *task.Task) (ProcessedItem, error) {
                return processItem(item)
            })
    }

    // Wait for all processing to complete
    results, err := group.GetResults()
    if err != nil {
        return fmt.Errorf("batch processing failed: %w", err)
    }

    // Handle results
    for task, result := range results {
        fmt.Printf("Task %s: %+v\n", task.Name(), result)
    }

    return nil
}
```

### Middleware System

The middleware package provides a comprehensive, configurable middleware system for Echo v4 with 20+ middleware types, YAML configuration support, and CEL-based dynamic processing.

#### Quick Start

The simplest way to get started is using preset configurations:

```go
import (
    "github.com/flanksource/clicky/middleware"
    "github.com/labstack/echo/v4"
)

func main() {
    e := echo.New()

    // Apply default middleware (CORS, Logger, Recover, RequestID, Gzip, Secure)
    middleware.ApplyDefaultMiddleware(e)

    // Or load from YAML file
    config, err := middleware.LoadConfigFromYAML("middleware.yaml")
    if err != nil {
        log.Fatal(err)
    }
    middleware.ApplyMiddleware(e, config)

    e.Start(":8080")
}
```

#### YAML Configuration

##### Development Configuration

```yaml
# development.yaml - Developer-friendly with permissive CORS
cors:
  allow_origins: ["*"]
  allow_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
  allow_headers: ["*"]

logger:
  format: "[${time_rfc3339_nano}] ${method} ${uri} (${status}) ${latency_human}\n"

recover:
  stack_size: 8192  # Larger stack for debugging
```

##### Production Configuration

```yaml
# production.yaml - Security hardened for production
cors:
  allow_origins: ["https://yourdomain.com"]
  allow_credentials: true

logger:
  format: '{"time":"${time_rfc3339}","method":"${method}","uri":"${uri}","status":${status}}'

csrf:
  token_length: 32
  cookie_secure: true
  cookie_http_only: true

rate_limiter:
  requests_per_second: 20
  burst: 30

secure:
  hsts_max_age: 31536000
  content_security_policy: "default-src 'self'"
```

#### Middleware Categories

##### Core Middleware
- **`cors`**: Cross-Origin Resource Sharing
- **`logger`**: Request logging with customizable formats
- **`recover`**: Panic recovery with stack traces
- **`request_id`**: Unique request identifier generation

##### Security Middleware
- **`basic_auth`**: HTTP Basic Authentication with htpasswd/userpass files
- **`jwt_auth`**: JWT authentication (HMAC/RSA/ECDSA) with CEL validation
- **`key_auth`**: API key authentication
- **`csrf`**: Cross-Site Request Forgery protection
- **`secure`**: Security headers (XSS, HSTS, CSP, etc.)

##### Performance & Reliability
- **`rate_limiter`**: Request rate limiting with memory store
- **`timeout`**: Request timeout middleware
- **`gzip`**: Response compression
- **`body_limit`**: Request body size limiting

#### Authentication Examples

##### Basic Authentication with htpasswd

```yaml
basic_auth:
  htpasswd_file: ".htpasswd"
  realm: "Admin Panel"
```

##### JWT Authentication

```yaml
jwt_auth:
  signing_key: "your-secret-key"
  signing_method: "HS256"
  token_lookup: "header:Authorization"
  token_prefix: "Bearer "
  validation: 'claims.exp > now() && claims.aud == "api"'
```

##### API Key Authentication

```yaml
key_auth:
  key_lookup: "header:X-API-Key"
```

#### CEL Integration

The middleware system includes powerful CEL (Common Expression Language) integration for dynamic request/response processing:

```yaml
interceptors:
  # Authentication guard
  - name: "auth_guard"
    regex: "^/api/.*"
    condition: 'request.method != "OPTIONS"'
    request:
      - 'headers.get("Authorization") == "" ? {status: 401, body: "Unauthorized"} : null'

  # Response headers
  - name: "security_headers"
    regex: ".*"
    response:
      - '{headers: {"X-Content-Type-Options": "nosniff"}}'
```

Available CEL Functions:
- **Request/response manipulation**: `setHeader()`, `returnStatus()`, `returnJSON()`
- **Authentication helpers**: `hasRole()`, `getUser()`, `validateJWT()`
- **Utility functions**: `jsonToXML()` plus all Gomplate functions
- **Variables**: `request`, `response`, `context`, `headers`, `body`, `user`, `claims`

#### Configuration Validation

All configurations are validated before application:

```go
if err := middleware.ValidateConfig(config); err != nil {
    log.Fatalf("Invalid configuration: %v", err)
}
```

Validation checks include:
- CORS credential/origin compatibility
- Rate limiter positive values
- Timeout positive durations
- Body limit format validation
- JWT key file existence
- Proxy target URL validity

### OpenAPI Generation & Documentation Server

The OpenAPI package automatically converts Cobra CLI commands into OpenAPI 3.0.3 specifications and serves interactive Swagger UI documentation.

#### Fluent Integration

The easiest way to add OpenAPI functionality is through the fluent extensions API:

```go
import (
    "github.com/flanksource/clicky/extensions"
    "github.com/spf13/cobra"
)

func main() {
    rootCmd := &cobra.Command{
        Use:   "myapp",
        Short: "My CLI application",
    }

    // Add your application commands
    rootCmd.AddCommand(apiCommands()...)
    rootCmd.AddCommand(databaseCommands()...)

    // Add OpenAPI functionality with fluent interface
    extensions.CobraExtensions(rootCmd).
        OpenAPICommand().        // Adds 'openapi generate' and 'openapi serve'
        MCPCommand()            // Adds 'mcp serve' for AI integration

    // Or add all functionality at once
    // extensions.CobraExtensions(rootCmd).All()

    rootCmd.Execute()
}
```

#### OpenAPI Generation

Generate OpenAPI specifications from your CLI commands:

```bash
# Generate OpenAPI spec as JSON
myapp openapi generate --output api-spec.json

# Generate as YAML with custom metadata
myapp openapi generate \
    --format yaml \
    --title "My API" \
    --version "2.0.0" \
    --server-url "https://api.example.com" \
    --output openapi.yaml

# Output to stdout
myapp openapi generate --format yaml
```

#### Interactive Documentation Server

Start a live documentation server with Swagger UI:

```bash
# Start server with default settings
myapp openapi serve

# Advanced options
myapp openapi serve \
    --port 3000 \
    --host 0.0.0.0 \
    --title "My API Documentation" \
    --description "Interactive API docs" \
    --auto-refresh \
    --open  # Automatically open browser
```

The server provides:
- **Interactive Swagger UI** at `http://localhost:8080/`
- **OpenAPI JSON spec** at `http://localhost:8080/api/openapi.json`
- **OpenAPI YAML spec** at `http://localhost:8080/api/openapi.yaml`
- **Health check** at `http://localhost:8080/health`

#### Command Execution via HTTP

Enable dynamic command execution through HTTP endpoints:

```bash
myapp openapi serve --enable-executor
```

This creates REST endpoints for each CLI command that can be called via HTTP:

```bash
# Execute CLI commands via HTTP
curl -X POST http://localhost:8080/api/v1/database/connect \
  -H "Content-Type: application/json" \
  -d '{"host": "localhost", "database": "myapp", "user": "admin"}'

curl -X GET http://localhost:8080/api/v1/cache/get/user:123
```

#### Custom OpenAPI Configuration

For more control, configure OpenAPI generation programmatically:

```go
import "github.com/flanksource/clicky/rpc"

customConfig := &rpc.OpenAPIConfig{
    Title:       "My API",
    Description: "Comprehensive API documentation",
    Version:     "2.0.0",
    Contact: &rpc.OpenAPIContact{
        Name:  "API Team",
        Email: "api@example.com",
        URL:   "https://example.com/contact",
    },
    License: &rpc.OpenAPILicense{
        Name: "MIT",
        URL:  "https://opensource.org/licenses/MIT",
    },
    Servers: []rpc.OpenAPIServer{
        {
            URL:         "https://api.example.com/v1",
            Description: "Production server",
        },
        {
            URL:         "https://staging-api.example.com/v1",
            Description: "Staging server",
        },
    },
}

extensions.CobraExtensions(rootCmd).
    OpenAPICommandWithConfig(customConfig)
```

#### MCP (Model Context Protocol) Integration

Clicky includes built-in MCP support for AI assistant integration:

```bash
# Start MCP server
myapp mcp serve

# Auto-expose all commands
myapp mcp serve --auto-expose

# View MCP configuration
myapp mcp config
```

Configure Claude Desktop (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "myapp": {
      "command": "/path/to/myapp",
      "args": ["mcp", "serve"]
    }
  }
}
```

When MCP tools are executed, they automatically use Clicky's task system for:
- Visual progress tracking with progress bars
- Concurrent execution control
- Retry logic with exponential backoff
- Timeout protection
- Beautiful styled output

### Themes

```go
theme := Theme{
    Primary:   lipgloss.Color("#8A2BE2"),
    Secondary: lipgloss.Color("#4169E1"),
    Success:   lipgloss.Color("#32CD32"),
    Warning:   lipgloss.Color("#FFD700"),
    Error:     lipgloss.Color("#FF6347"),
    Info:      lipgloss.Color("#00CED1"),
    Muted:     lipgloss.Color("#808080"),
}

parser.Theme = theme
```

## Integration Examples

### Complete CLI Application

Here's a comprehensive example showcasing all Clicky components:

```go
package main

import (
    "context"
    "time"

    "github.com/flanksource/clicky"
    "github.com/flanksource/clicky/extensions"
    "github.com/flanksource/clicky/middleware"
    "github.com/flanksource/clicky/task"
    "github.com/labstack/echo/v4"
    "github.com/spf13/cobra"
)

// Data model with pretty formatting tags
type ServerStatus struct {
    Name     string    `pretty:"label=Server Name,color=blue,table"`
    Status   string    `pretty:"label=Status,color=green,table"`
    Uptime   string    `pretty:"label=Uptime,table"`
    Load     float64   `pretty:"label=Load Average,format=float,table"`
    Updated  time.Time `pretty:"label=Last Updated,format=date,table"`
}

func main() {
    rootCmd := &cobra.Command{
        Use:   "server-manager",
        Short: "Server management CLI with web interface",
    }

    // Add application commands
    rootCmd.AddCommand(newServersCommand())
    rootCmd.AddCommand(newWebServerCommand())

    // Add Clicky extensions
    extensions.CobraExtensions(rootCmd).
        OpenAPICommand().  // Auto-generate API docs and serve Swagger UI
        MCPCommand()       // Enable AI assistant integration

    rootCmd.Execute()
}

func newServersCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "servers",
        Short: "Server management commands",
    }

    // List servers with formatted output
    listCmd := &cobra.Command{
        Use:   "list",
        Short: "List all servers",
        RunE: func(cmd *cobra.Command, args []string) error {
            format, _ := cmd.Flags().GetString("format")

            // Fetch server data with concurrent tasks
            servers, err := fetchServersWithTasks()
            if err != nil {
                return err
            }

            // Format and display using clicky formatters
            output, err := clicky.Format(servers, clicky.FormatOptions{
                Format: format,
            })
            if err != nil {
                return err
            }

            fmt.Println(output)
            return nil
        },
    }
    listCmd.Flags().StringP("format", "f", "pretty", "Output format (pretty, json, yaml, markdown, excel)")

    cmd.AddCommand(listCmd)
    return cmd
}

func fetchServersWithTasks() ([]ServerStatus, error) {
    // Use task system for concurrent data fetching
    group := task.NewTypedGroup[ServerStatus]("Fetch Server Status",
        task.WithConcurrency(3),
    )

    serverNames := []string{"web-01", "db-01", "cache-01"}

    for _, name := range serverNames {
        serverName := name // Capture for closure
        group.Add(fmt.Sprintf("Check %s", serverName),
            func(ctx context.Context, t *task.Task) (ServerStatus, error) {
                t.Infof("Checking server %s...", serverName)

                // Simulate status check with progress
                for i := 0; i <= 100; i += 25 {
                    t.SetProgress(i, 100)
                    time.Sleep(100 * time.Millisecond)
                }

                return ServerStatus{
                    Name:    serverName,
                    Status:  "Running",
                    Uptime:  "5d 12h",
                    Load:    0.75,
                    Updated: time.Now(),
                }, nil
            })
    }

    results, err := group.GetResults()
    if err != nil {
        return nil, err
    }

    // Convert map to slice
    servers := make([]ServerStatus, 0, len(results))
    for _, server := range results {
        servers = append(servers, server)
    }

    return servers, nil
}

func newWebServerCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "web",
        Short: "Start web server with middleware",
        RunE: func(cmd *cobra.Command, args []string) error {
            e := echo.New()

            // Apply middleware from YAML config
            config, err := middleware.LoadConfigFromYAML("middleware.yaml")
            if err != nil {
                // Fallback to default middleware
                middleware.ApplyDefaultMiddleware(e)
            } else {
                middleware.ApplyMiddleware(e, config)
            }

            // Add API routes
            api := e.Group("/api/v1")
            api.GET("/servers", handleListServers)

            return e.Start(":8080")
        },
    }
}

func handleListServers(c echo.Context) error {
    servers, err := fetchServersWithTasks()
    if err != nil {
        return c.JSON(500, map[string]string{"error": err.Error()})
    }
    return c.JSON(200, servers)
}
```

### Usage Examples

```bash
# List servers with pretty table format
server-manager servers list

# Export to different formats
server-manager servers list --format json > servers.json
server-manager servers list --format excel > servers.xlsx
server-manager servers list --format markdown > servers.md

# Start web server with middleware
server-manager web

# Generate OpenAPI documentation
server-manager openapi generate --output api-docs.yaml

# Start interactive API documentation server
server-manager openapi serve --port 3000 --open

# Enable AI assistant integration
server-manager mcp serve
```

## Project Architecture

Clicky is organized into four main packages that work together:

```
github.com/flanksource/clicky/
├── formatters/     # Multi-format data output with styling
│   ├── pretty.go   # Table and tree formatting
│   ├── exports.go  # JSON, YAML, CSV, Excel, PDF output
│   └── styling.go  # Tailwind CSS integration
├── task/          # Concurrent execution with progress tracking
│   ├── manager.go  # Task coordination and limits
│   ├── group.go    # Batch processing with typing
│   └── health.go   # Status and health systems
├── middleware/     # Echo v4 web middleware system
│   ├── config.go   # YAML configuration loading
│   ├── auth.go     # Authentication middleware
│   ├── security.go # Security and CORS middleware
│   └── cel.go      # CEL expression processing
├── rpc/           # OpenAPI generation and HTTP serving
│   ├── openapi.go  # OpenAPI 3.0.3 spec generation
│   ├── serve.go    # Swagger UI documentation server
│   └── executor.go # HTTP command execution
├── extensions/     # Fluent Cobra integration
│   └── cobra.go    # .OpenAPICommand(), .MCPCommand()
└── mcp/           # Model Context Protocol for AI
    ├── server.go   # MCP server implementation
    └── tools.go    # CLI-to-MCP tool conversion
```

### Component Integration

- **Formatters** ↔ **Tasks**: Tasks can output formatted results using any supported format
- **Middleware** ↔ **OpenAPI**: Web servers get automatic API documentation
- **Tasks** ↔ **MCP**: AI tools execute with visual progress tracking
- **Extensions** ↔ **All**: Fluent API provides simple integration points

## Installation

Add Clicky to your Go project:

```bash
go get github.com/flanksource/clicky@latest
```

For specific packages:

```bash
# For formatters only
go get github.com/flanksource/clicky/formatters

# For task system
go get github.com/flanksource/clicky/task

# For Echo middleware
go get github.com/flanksource/clicky/middleware

# For OpenAPI generation
go get github.com/flanksource/clicky/rpc
go get github.com/flanksource/clicky/extensions
```

## Key Dependencies

Clicky builds on proven Go libraries:

- **Core**: `github.com/spf13/cobra` - CLI framework
- **Styling**: `github.com/charmbracelet/lipgloss` - Terminal styling and themes
- **Web**: `github.com/labstack/echo/v4` - High performance web framework
- **Data**: `gopkg.in/yaml.v3`, `github.com/xuri/excelize/v2` - YAML and Excel support
- **PDF**: `github.com/flanksource/maroto/v2` - PDF generation
- **Concurrency**: `golang.org/x/sync` - Advanced synchronization primitives

## Testing

Run the comprehensive test suite:

```bash
# Run all tests
go test -v ./...

# Test specific packages
go test -v ./formatters/...
go test -v ./task/...
go test -v ./middleware/...

# Run integration tests
go test -v -tags=integration ./...
```

## Best Practices

### Security
- Always use HTTPS in production with HTTPSRedirect middleware
- Enable CSRF protection for web applications
- Set restrictive CORS origins (avoid "*" in production)
- Use strong JWT signing keys and rotate them regularly
- Implement rate limiting to prevent abuse

### Performance
- Enable compression with Gzip for API responses
- Set appropriate timeouts to prevent resource exhaustion
- Configure appropriate body limits based on use case
- Use request IDs for tracing and debugging
- Limit task concurrency based on system resources

### Configuration Management
- Use YAML files for environment-specific configs
- Validate configurations before deployment
- Use environment variables for secrets
- Version your middleware and formatting configurations
- Test configurations with unit tests

## Contributing

Clicky is designed to be extensible and welcomes contributions:

### Areas for Contribution

1. **New Formatters**: Add support for additional output formats (XML, Protobuf, etc.)
2. **Enhanced Middleware**: Additional Echo middleware types and CEL functions
3. **Task Features**: Advanced scheduling, dependencies, and workflows
4. **OpenAPI Enhancement**: Support for OpenAPI 3.1, additional specification features
5. **Performance**: Optimize reflection usage and memory allocation
6. **Documentation**: Examples, tutorials, and best practices

### Development Setup

```bash
git clone https://github.com/flanksource/clicky.git
cd clicky
go mod download
go test -v ./...
```

## License

Clicky is released under the MIT License. See [LICENSE](LICENSE) for details.

This framework follows Go best practices and is designed for production use in enterprise environments.
