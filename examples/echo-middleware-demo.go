//go:build ignore

package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/flanksource/clicky/middleware"
	"github.com/labstack/echo/v4"
)

func main() {
	// Create Echo instance
	e := echo.New()

	// Example 1: Using default configuration
	fmt.Println("=== Example 1: Default Configuration ===")
	demonstrateDefaultConfig(e)

	// Example 2: Using minimal configuration
	fmt.Println("\n=== Example 2: Minimal Configuration ===")
	demonstrateMinimalConfig()

	// Example 3: Using production configuration
	fmt.Println("\n=== Example 3: Production Configuration ===")
	demonstrateProductionConfig()

	// Example 4: Custom configuration
	fmt.Println("\n=== Example 4: Custom Configuration ===")
	demonstrateCustomConfig()

	// Example 5: Selective middleware
	fmt.Println("\n=== Example 5: Selective Middleware ===")
	demonstrateSelectiveMiddleware()

	// Start server (commented out for demo)
	// e.Logger.Fatal(e.Start(":8080"))
}

func demonstrateDefaultConfig(e *echo.Echo) {
	// Apply default middleware using helper function
	middleware.ApplyDefaultMiddleware(e)

	// Add a simple route
	e.GET("/default", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "Default middleware applied",
			"path":    "/default",
		})
	})

	fmt.Println("Default middleware applied: CORS, Logger, Recover, RequestID, Secure")
}

func demonstrateMinimalConfig() {
	e := echo.New()

	// Apply minimal middleware
	middleware.ApplyMinimalMiddleware(e)

	e.GET("/minimal", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "Minimal middleware applied",
			"path":    "/minimal",
		})
	})

	fmt.Println("Minimal middleware applied: Recover, RequestID only")
}

func demonstrateProductionConfig() {
	e := echo.New()

	// Apply production middleware
	middleware.ApplyProductionMiddleware(e)

	e.GET("/production", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "Production middleware applied",
			"path":    "/production",
		})
	})

	fmt.Println("Production middleware applied: CORS, Logger, Recover, RequestID, RateLimiter, Timeout, Secure")
}

func demonstrateCustomConfig() {
	e := echo.New()

	// Create custom configuration
	config := middleware.MiddlewareConfig{
		// Enable CORS with specific origins
		CORS: &middleware.CORSConfig{
			AllowOrigins:     []string{"http://localhost:3000", "https://myapp.com"},
			AllowMethods:     []string{"GET", "POST", "PUT"},
			AllowHeaders:     []string{"Content-Type", "Authorization"},
			AllowCredentials: true,
			MaxAge:           3600, // 1 hour
		},

		// Custom logger format
		Logger: &middleware.LoggerConfig{
			Format: `[${time_rfc3339_nano}] ${method} ${uri} (${status}) ${latency_human}` + "\n",
			Output: os.Stdout,
		},

		// Recover with custom stack size
		Recover: &middleware.RecoverConfig{
			StackSize:         8 << 10, // 8 KB
			DisablePrintStack: false,
		},

		// RequestID with custom generator
		RequestID: &middleware.RequestIDConfig{
			Generator: func() string {
				return fmt.Sprintf("req-%d", time.Now().UnixNano())
			},
			TargetHeader: "X-Custom-Request-ID",
		},

		// Rate limiter: 10 requests per second, burst of 15
		RateLimiter: &middleware.RateLimiterConfig{
			RequestsPerSecond: 10,
			Burst:             15,
			ExpiresIn:         5 * time.Minute,
		},

		// Timeout of 10 seconds
		Timeout: &middleware.TimeoutConfig{
			Timeout: 10 * time.Second,
		},

		// Custom security headers
		Secure: &middleware.SecureConfig{
			XSSProtection:      "1; mode=block",
			ContentTypeNosniff: "nosniff",
			XFrameOptions:      "SAMEORIGIN",
			HSTSMaxAge:         31536000, // 1 year
			ReferrerPolicy:     "no-referrer",
		},
	}

	// Apply custom middleware
	middleware.ApplyMiddleware(e, config)

	e.GET("/custom", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message":    "Custom middleware applied",
			"request_id": c.Response().Header().Get("X-Custom-Request-ID"),
			"path":       "/custom",
		})
	})

	fmt.Println("Custom middleware applied with specific configurations for each middleware type")
}

func demonstrateSelectiveMiddleware() {
	e := echo.New()

	// Only enable specific middleware
	config := middleware.MiddlewareConfig{
		// Only CORS and Logger
		CORS: &middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET", "POST"},
		},
		Logger: &middleware.LoggerConfig{
			Format: `{"method":"${method}","uri":"${uri}","status":${status}}` + "\n",
		},
		// RequestID enabled, others disabled by not setting them
		RequestID: &middleware.RequestIDConfig{
			TargetHeader: echo.HeaderXRequestID,
		},
	}

	// Apply selective middleware
	middleware.ApplyMiddleware(e, config)

	e.GET("/selective", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "Only CORS, Logger, and RequestID middleware applied",
			"path":    "/selective",
		})
	})

	fmt.Println("Selective middleware applied: CORS, Logger, RequestID only")
}

func demonstrateAdvancedUsage() {
	e := echo.New()

	// Get middleware functions for manual application
	config := middleware.DefaultConfig()
	middlewareFuncs := middleware.EchoMiddleware(config)

	// Apply middleware functions individually with custom logic
	for i, mw := range middlewareFuncs {
		fmt.Printf("Applying middleware #%d\n", i+1)
		e.Use(mw)
	}

	// Add routes
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status":     "healthy",
			"timestamp":  time.Now().Format(time.RFC3339),
			"request_id": c.Response().Header().Get(echo.HeaderXRequestID),
		})
	})

	e.POST("/data", func(c echo.Context) error {
		var data map[string]interface{}
		if err := c.Bind(&data); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid JSON")
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"received":  data,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	fmt.Println("Advanced usage: Middleware functions applied individually")
	fmt.Println("Available routes:")
	fmt.Println("  GET  /health - Health check endpoint")
	fmt.Println("  POST /data   - Data endpoint")
	fmt.Println("\nTo start the server, uncomment: e.Logger.Fatal(e.Start(\":8080\"))")
}
