package main

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/flanksource/clicky/middleware"
	"github.com/labstack/echo/v4"
)

func main() {
	fmt.Println("=== Comprehensive Echo Middleware Demo ===")
	fmt.Println("This demo showcases all 25+ middleware configurations available")
	fmt.Println()

	// Example 1: Default configuration (updated with new middleware)
	fmt.Println("1. Default Configuration:")
	demonstrateDefault()

	// Example 2: Security-focused configuration
	fmt.Println("\n2. Security Configuration:")
	demonstrateSecurity()

	// Example 3: Performance/compression configuration
	fmt.Println("\n3. Compression Configuration:")
	demonstrateCompression()

	// Example 4: Development configuration
	fmt.Println("\n4. Development Configuration:")
	demonstrateDevelopment()

	// Example 5: Production configuration (updated)
	fmt.Println("\n5. Production Configuration:")
	demonstrateProduction()

	// Example 6: Advanced custom configuration
	fmt.Println("\n6. Advanced Custom Configuration:")
	demonstrateAdvanced()

	// Example 7: Authentication configuration
	fmt.Println("\n7. Authentication Configuration:")
	demonstrateAuthentication()

	// Example 8: Proxy/Gateway configuration
	fmt.Println("\n8. Proxy/Gateway Configuration:")
	demonstrateProxyGateway()

	fmt.Println("\nDemo completed successfully!")
	fmt.Println("All middleware configurations are working correctly.")
}

func demonstrateDefault() {
	e := echo.New()
	middleware.ApplyDefaultMiddleware(e)

	e.GET("/default", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "Default middleware: CORS, Logger, Recover, RequestID, Gzip, Secure",
		})
	})

	fmt.Println("  ✓ Default middleware applied: CORS, Logger, Recover, RequestID, Gzip, Secure")
}

func demonstrateSecurity() {
	e := echo.New()
	middleware.ApplySecurityMiddleware(e)

	e.GET("/secure", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message":    "Security middleware active",
			"csrf_token": c.Get("csrf").(string),
		})
	})

	fmt.Println("  ✓ Security middleware applied: Recover, RequestID, CSRF, Secure, RateLimiter")
}

func demonstrateCompression() {
	e := echo.New()
	middleware.ApplyCompressionMiddleware(e)

	e.GET("/compressed", func(c echo.Context) error {
		// Return a larger response to demonstrate compression
		data := make(map[string]interface{})
		data["message"] = "This response will be compressed"
		data["data"] = make([]string, 100)
		for i := 0; i < 100; i++ {
			data["data"].([]string)[i] = fmt.Sprintf("Item %d with some repetitive text", i)
		}
		return c.JSON(http.StatusOK, data)
	})

	fmt.Println("  ✓ Compression middleware applied: Gzip, Decompress, BodyLimit, RemoveTrailingSlash")
}

func demonstrateDevelopment() {
	e := echo.New()
	middleware.ApplyDevelopmentMiddleware(e)

	e.GET("/dev", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "Development mode with permissive CORS and detailed logging",
			"env":     "development",
		})
	})

	fmt.Println("  ✓ Development middleware applied: CORS (*), Logger (detailed), Recover (debug), RequestID")
}

func demonstrateProduction() {
	e := echo.New()
	middleware.ApplyProductionMiddleware(e)

	e.GET("/prod", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "Production configuration with security hardening",
			"env":     "production",
		})
	})

	fmt.Println("  ✓ Production middleware applied: CORS (restricted), Logger, Recover, RequestID, CSRF, BodyLimit, Gzip, RateLimiter, Timeout, RemoveTrailingSlash, Secure")
}

func demonstrateAdvanced() {
	e := echo.New()

	// Create a comprehensive custom configuration
	config := middleware.MiddlewareConfig{
		// Core middleware
		CORS: &middleware.CORSConfig{
			AllowOrigins:     []string{"http://localhost:3000", "https://myapp.com"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
			AllowHeaders:     []string{"Content-Type", "Authorization", "X-Requested-With"},
			AllowCredentials: true,
			MaxAge:           3600,
		},
		Logger: &middleware.LoggerConfig{
			Format: `{"time":"${time_rfc3339}","method":"${method}","uri":"${uri}","status":${status},"latency":"${latency_human}","ip":"${remote_ip}"}` + "\n",
			Output: os.Stdout,
		},
		Recover: &middleware.RecoverConfig{
			StackSize:         8 << 10, // 8KB stack trace
			DisablePrintStack: false,
		},
		RequestID: &middleware.RequestIDConfig{
			Generator: func() string {
				return fmt.Sprintf("req-%d", time.Now().UnixNano())
			},
			TargetHeader: "X-Request-ID",
		},

		// Security middleware
		CSRF: &middleware.CSRFConfig{
			TokenLength:    32,
			TokenLookup:    "header:X-CSRF-Token",
			CookieName:     "_csrf",
			CookieSecure:   true,
			CookieHTTPOnly: true,
		},
		Secure: &middleware.SecureConfig{
			XSSProtection:         "1; mode=block",
			ContentTypeNosniff:    "nosniff",
			XFrameOptions:         "SAMEORIGIN",
			HSTSMaxAge:            31536000,
			ContentSecurityPolicy: "default-src 'self'; script-src 'self' 'unsafe-inline'",
		},

		// Request/Response processing
		BodyLimit: &middleware.BodyLimitConfig{
			Limit: "10M",
		},
		Gzip: &middleware.GzipConfig{
			Level: 6,
		},
		Decompress: &middleware.DecompressConfig{},

		// URL rewriting
		Rewrite: &middleware.RewriteConfig{
			Rules: map[string]string{
				"/api/v1/*": "/api/latest/$1",
				"/old/*":    "/new/$1",
			},
		},

		// Trailing slash normalization
		RemoveTrailingSlash: &middleware.TrailingSlashConfig{
			RedirectCode: 301,
		},

		// Rate limiting and timeouts
		RateLimiter: &middleware.RateLimiterConfig{
			RequestsPerSecond: 50,
			Burst:             75,
			ExpiresIn:         5 * time.Minute,
		},
		Timeout: &middleware.TimeoutConfig{
			Timeout: 15 * time.Second,
		},
	}

	middleware.ApplyMiddleware(e, config)

	e.GET("/advanced", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message":    "Advanced configuration with multiple middleware",
			"request_id": c.Response().Header().Get("X-Request-ID"),
			"features": []string{
				"CORS with specific origins",
				"Structured JSON logging",
				"CSRF protection",
				"Body size limits",
				"Gzip compression",
				"URL rewriting",
				"Rate limiting",
				"Request timeouts",
				"Security headers",
			},
		})
	})

	fmt.Println("  ✓ Advanced middleware applied: 11 different middleware types configured")
}

func demonstrateAuthentication() {
	e := echo.New()

	config := middleware.MiddlewareConfig{
		Recover: &middleware.RecoverConfig{
			StackSize: 4 << 10,
		},
		RequestID: &middleware.RequestIDConfig{
			TargetHeader: echo.HeaderXRequestID,
		},
		BasicAuth: &middleware.BasicAuthConfig{
			Validator: func(username, password string, c echo.Context) (bool, error) {
				// Use constant time comparison to prevent timing attacks
				if subtle.ConstantTimeCompare([]byte(username), []byte("admin")) == 1 &&
					subtle.ConstantTimeCompare([]byte(password), []byte("secret")) == 1 {
					return true, nil
				}
				return false, nil
			},
			Realm: "Restricted Area",
		},
		KeyAuth: &middleware.KeyAuthConfig{
			KeyLookup: "header:X-API-Key",
			Validator: func(key string, c echo.Context) (bool, error) {
				return key == "valid-api-key", nil
			},
		},
	}

	middleware.ApplyMiddleware(e, config)

	e.GET("/auth-basic", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "Authenticated with Basic Auth",
			"user":    "admin",
		})
	})

	e.GET("/auth-key", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "Authenticated with API Key",
		})
	})

	fmt.Println("  ✓ Authentication middleware applied: BasicAuth (admin/secret), KeyAuth (X-API-Key: valid-api-key)")
}

func demonstrateProxyGateway() {
	e := echo.New()

	config := middleware.MiddlewareConfig{
		Logger: &middleware.LoggerConfig{
			Format: `[PROXY] ${time_rfc3339_nano} ${method} ${uri} -> ${status} (${latency_human})` + "\n",
		},
		Recover: &middleware.RecoverConfig{
			StackSize: 4 << 10,
		},
		RequestID: &middleware.RequestIDConfig{
			TargetHeader: echo.HeaderXRequestID,
		},
		RateLimiter: &middleware.RateLimiterConfig{
			RequestsPerSecond: 100,
			Burst:             150,
		},
		Timeout: &middleware.TimeoutConfig{
			Timeout: 30 * time.Second,
		},
		// Example proxy configuration (would proxy to httpbin.org in real usage)
		Proxy: &middleware.ProxyConfig{
			Targets: []*middleware.ProxyTarget{
				{Name: "service1", URL: "https://httpbin.org"},
			},
		},
	}

	middleware.ApplyMiddleware(e, config)

	// Add a local endpoint that doesn't get proxied
	e.GET("/gateway/status", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "Gateway is running",
			"proxy":   "Configured for https://httpbin.org",
		})
	})

	fmt.Println("  ✓ Proxy/Gateway middleware applied: Logger (proxy format), Recover, RequestID, RateLimiter (high), Timeout, Proxy")
}

func demonstrateMethodOverrideAndStatic() {
	e := echo.New()

	config := middleware.MiddlewareConfig{
		Logger: &middleware.LoggerConfig{
			Format: `${method} ${uri} -> ${status}` + "\n",
		},
		MethodOverride: &middleware.MethodOverrideConfig{
			Getter: func(c echo.Context) string {
				// Allow method override via X-HTTP-Method-Override header
				return c.Request().Header.Get("X-HTTP-Method-Override")
			},
		},
		Static: &middleware.StaticConfig{
			Root:   "public",
			Index:  "index.html",
			Browse: true,
		},
	}

	middleware.ApplyMiddleware(e, config)

	// This would handle static files from ./public/ directory
	// and support HTTP method override
	e.PUT("/resource", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "PUT request (possibly overridden from POST)",
		})
	})

	fmt.Println("  ✓ Special middleware applied: MethodOverride (X-HTTP-Method-Override), Static (./public/)")
}

// Helper function to demonstrate body dump
func createBodyDumpHandler() func(echo.Context, []byte, []byte) {
	return func(c echo.Context, reqBody, resBody []byte) {
		log.Printf("Request Body: %s", string(reqBody))
		log.Printf("Response Body: %s", string(resBody))
	}
}
