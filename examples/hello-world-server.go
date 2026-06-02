//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/flanksource/clicky/middleware"
	"github.com/labstack/echo/v4"
)

func main() {
	fmt.Println("🌍 Echo Hello World Server with YAML Configuration")
	fmt.Println("==================================================")

	// Determine configuration file to use
	configFile := getConfigFile()
	fmt.Printf("📄 Loading configuration from: %s\n", configFile)

	// Load middleware configuration from YAML file
	config, err := middleware.LoadConfigFromYAML(configFile)
	if err != nil {
		log.Fatalf("❌ Failed to load configuration: %v", err)
	}
	fmt.Printf("✅ Configuration loaded successfully\n")

	// Create Echo instance
	e := echo.New()
	e.HideBanner = true

	// Apply middleware from configuration
	middleware.ApplyMiddleware(e, config)
	fmt.Printf("🔧 Middleware applied from configuration\n")

	// Setup routes
	setupRoutes(e)
	fmt.Printf("🛤️  Routes configured\n")

	// Start server with graceful shutdown
	port := getPort()
	fmt.Printf("🚀 Starting server on port %s\n", port)
	fmt.Printf("📍 Available endpoints:\n")
	fmt.Printf("   • http://localhost%s/\n", port)
	fmt.Printf("   • http://localhost%s/api/health\n", port)
	fmt.Printf("   • http://localhost%s/api/hello\n", port)
	fmt.Printf("   • http://localhost%s/api/info\n", port)
	fmt.Printf("\n💡 Press Ctrl+C to stop the server\n\n")

	startServerWithGracefulShutdown(e, port)
}

// getConfigFile determines which configuration file to use based on environment
func getConfigFile() string {
	// Check for explicit config file from environment variable
	if configFile := os.Getenv("MIDDLEWARE_CONFIG"); configFile != "" {
		return configFile
	}

	// Check for environment-specific config
	env := os.Getenv("ENV")
	if env == "" {
		env = "development" // Default to development
	}

	configFile := fmt.Sprintf("config/%s.yaml", env)

	// Check if the file exists, fall back to minimal if not
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("⚠️  Config file %s not found, using minimal.yaml\n", configFile)
		return "config/minimal.yaml"
	}

	return configFile
}

// getPort returns the port to listen on, with fallback to :8080
func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return ":" + port
}

// setupRoutes configures all the HTTP routes for the server
func setupRoutes(e *echo.Echo) {
	// Home route
	e.GET("/", homeHandler)

	// API routes
	api := e.Group("/api")
	api.GET("/health", healthHandler)
	api.GET("/hello", helloHandler)
	api.GET("/info", infoHandler)
	api.POST("/echo", echoHandler)
}

// homeHandler serves the home page
func homeHandler(c echo.Context) error {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Echo Hello World Server</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 600px; margin: 0 auto; background: white; padding: 30px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #2c3e50; text-align: center; }
        .endpoints { background: #ecf0f1; padding: 20px; border-radius: 5px; margin: 20px 0; }
        .endpoint { margin: 10px 0; }
        .method { display: inline-block; width: 60px; padding: 2px 8px; border-radius: 3px; font-size: 12px; font-weight: bold; }
        .get { background: #27ae60; color: white; }
        .post { background: #3498db; color: white; }
        a { color: #3498db; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🌍 Echo Hello World Server</h1>
        <p>Welcome to the Echo Hello World server with YAML-based middleware configuration!</p>

        <div class="endpoints">
            <h3>Available Endpoints:</h3>
            <div class="endpoint">
                <span class="method get">GET</span>
                <a href="/api/health">/api/health</a> - Health check endpoint
            </div>
            <div class="endpoint">
                <span class="method get">GET</span>
                <a href="/api/hello">/api/hello</a> - Simple hello message
            </div>
            <div class="endpoint">
                <span class="method get">GET</span>
                <a href="/api/info">/api/info</a> - Server and request information
            </div>
            <div class="endpoint">
                <span class="method post">POST</span>
                <a href="/api/echo">/api/echo</a> - Echo back request data (POST only)
            </div>
        </div>

        <p><strong>Configuration:</strong> This server is configured using YAML files that demonstrate Echo middleware configuration.</p>

        <p><strong>Request ID:</strong> <code id="request-id">` + c.Response().Header().Get("X-Request-ID") + `</code></p>
    </div>
</body>
</html>`
	return c.HTML(http.StatusOK, html)
}

// healthHandler provides a health check endpoint
func healthHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":      "healthy",
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"uptime":      time.Since(startTime).String(),
		"version":     "1.0.0",
		"environment": os.Getenv("ENV"),
		"request_id":  c.Response().Header().Get("X-Request-ID"),
	})
}

// helloHandler provides a simple hello message
func helloHandler(c echo.Context) error {
	name := c.QueryParam("name")
	if name == "" {
		name = "World"
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":    fmt.Sprintf("Hello, %s! 👋", name),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"request_id": c.Response().Header().Get("X-Request-ID"),
		"tip":        "Add ?name=YourName to personalize the greeting",
	})
}

// infoHandler provides detailed request and server information
func infoHandler(c echo.Context) error {
	headers := make(map[string]string)
	for name, values := range c.Request().Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"server": map[string]interface{}{
			"name":        "Echo Hello World Server",
			"version":     "1.0.0",
			"go_version":  "Go " + "1.21+",
			"environment": os.Getenv("ENV"),
			"config_file": os.Getenv("MIDDLEWARE_CONFIG"),
			"uptime":      time.Since(startTime).String(),
		},
		"request": map[string]interface{}{
			"method":       c.Request().Method,
			"path":         c.Request().URL.Path,
			"query_params": c.QueryParams(),
			"remote_addr":  c.Request().RemoteAddr,
			"user_agent":   c.Request().UserAgent(),
			"request_id":   c.Response().Header().Get("X-Request-ID"),
			"headers":      headers,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// echoHandler echoes back the request data (for testing POST requests)
func echoHandler(c echo.Context) error {
	if c.Request().Method != http.MethodPost {
		return c.JSON(http.StatusMethodNotAllowed, map[string]string{
			"error":   "Method not allowed",
			"message": "This endpoint only accepts POST requests",
		})
	}

	// Parse request body
	var requestBody map[string]interface{}
	if err := c.Bind(&requestBody); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Invalid JSON",
			"message": "Request body must be valid JSON",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"echo": map[string]interface{}{
			"received_data":  requestBody,
			"content_type":   c.Request().Header.Get("Content-Type"),
			"content_length": c.Request().ContentLength,
		},
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"request_id": c.Response().Header().Get("X-Request-ID"),
	})
}

var startTime = time.Now()

// startServerWithGracefulShutdown starts the server and handles graceful shutdown
func startServerWithGracefulShutdown(e *echo.Echo, port string) {
	// Start server in a goroutine
	go func() {
		if err := e.Start(port); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	fmt.Printf("\n🛑 Shutting down server...\n")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := e.Shutdown(ctx); err != nil {
		log.Printf("❌ Server forced to shutdown: %v", err)
	} else {
		fmt.Printf("✅ Server stopped gracefully\n")
	}
}

// init function to ensure proper working directory
func init() {
	// Change to the project root directory if we're in the examples directory
	if filepath.Base(os.Args[0]) == "hello-world-server" {
		if wd, err := os.Getwd(); err == nil && filepath.Base(wd) == "examples" {
			if err := os.Chdir(".."); err != nil {
				log.Printf("Warning: Could not change to parent directory: %v", err)
			}
		}
	}
}
