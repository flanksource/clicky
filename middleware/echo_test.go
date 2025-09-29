package middleware

import (
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test that default config has expected middleware enabled
	assert.NotNil(t, config.CORS, "CORS should be enabled in default config")
	assert.NotNil(t, config.Logger, "Logger should be enabled in default config")
	assert.NotNil(t, config.Recover, "Recover should be enabled in default config")
	assert.NotNil(t, config.RequestID, "RequestID should be enabled in default config")
	assert.NotNil(t, config.Secure, "Secure should be enabled in default config")

	// JWT, RateLimiter, and Timeout should be disabled in default config
	assert.Nil(t, config.RateLimiter, "RateLimiter should be disabled in default config")
	assert.Nil(t, config.Timeout, "Timeout should be disabled in default config")
}

func TestMinimalConfig(t *testing.T) {
	config := MinimalConfig()

	// Test that minimal config only has essential middleware
	assert.NotNil(t, config.Recover, "Recover should be enabled in minimal config")
	assert.NotNil(t, config.RequestID, "RequestID should be enabled in minimal config")

	// All other middleware should be disabled
	assert.Nil(t, config.CORS, "CORS should be disabled in minimal config")
	assert.Nil(t, config.Logger, "Logger should be disabled in minimal config")
	assert.Nil(t, config.RateLimiter, "RateLimiter should be disabled in minimal config")
	assert.Nil(t, config.Timeout, "Timeout should be disabled in minimal config")
	assert.Nil(t, config.Secure, "Secure should be disabled in minimal config")
}

func TestProductionConfig(t *testing.T) {
	config := ProductionConfig()

	// Test that production config has comprehensive middleware
	assert.NotNil(t, config.CORS, "CORS should be enabled in production config")
	assert.NotNil(t, config.Logger, "Logger should be enabled in production config")
	assert.NotNil(t, config.Recover, "Recover should be enabled in production config")
	assert.NotNil(t, config.RequestID, "RequestID should be enabled in production config")
	assert.NotNil(t, config.RateLimiter, "RateLimiter should be enabled in production config")
	assert.NotNil(t, config.Timeout, "Timeout should be enabled in production config")
	assert.NotNil(t, config.Secure, "Secure should be enabled in production config")

	// Test specific production values
	assert.Equal(t, 20.0, config.RateLimiter.RequestsPerSecond, "Production should have rate limit of 20 req/s")
	assert.Equal(t, 30*time.Second, config.Timeout.Timeout, "Production should have 30s timeout")
}

func TestEchoMiddlewareWithEmptyConfig(t *testing.T) {
	config := MiddlewareConfig{} // Empty config

	middlewares := EchoMiddleware(config)

	// Should return empty slice when no middleware is configured
	assert.Empty(t, middlewares, "Empty config should return no middleware")
}

func TestEchoMiddlewareWithCORSOnly(t *testing.T) {
	config := MiddlewareConfig{
		CORS: &CORSConfig{
			AllowOrigins: []string{"http://localhost:3000"},
			AllowMethods: []string{"GET", "POST"},
		},
	}

	middlewares := EchoMiddleware(config)

	// Should return exactly one middleware function
	assert.Len(t, middlewares, 1, "CORS-only config should return exactly one middleware")
}

func TestApplyMiddleware(t *testing.T) {
	e := echo.New()
	config := MinimalConfig()

	// Test that ApplyMiddleware doesn't panic
	require.NotPanics(t, func() {
		ApplyMiddleware(e, config)
	}, "ApplyMiddleware should not panic")
}

func TestApplyDefaultMiddleware(t *testing.T) {
	e := echo.New()

	// Test that ApplyDefaultMiddleware doesn't panic
	require.NotPanics(t, func() {
		ApplyDefaultMiddleware(e)
	}, "ApplyDefaultMiddleware should not panic")
}

func TestApplyMinimalMiddleware(t *testing.T) {
	e := echo.New()

	// Test that ApplyMinimalMiddleware doesn't panic
	require.NotPanics(t, func() {
		ApplyMinimalMiddleware(e)
	}, "ApplyMinimalMiddleware should not panic")
}

func TestApplyProductionMiddleware(t *testing.T) {
	e := echo.New()

	// Test that ApplyProductionMiddleware doesn't panic
	require.NotPanics(t, func() {
		ApplyProductionMiddleware(e)
	}, "ApplyProductionMiddleware should not panic")
}

func TestCORSConfigValues(t *testing.T) {
	config := MiddlewareConfig{
		CORS: &CORSConfig{
			AllowOrigins:     []string{"http://example.com"},
			AllowMethods:     []string{"GET", "POST", "PUT"},
			AllowHeaders:     []string{"Authorization", "Content-Type"},
			AllowCredentials: true,
			ExposeHeaders:    []string{"X-Total-Count"},
			MaxAge:           3600,
		},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 1, "Should create exactly one middleware")
}

func TestRateLimiterConfigValues(t *testing.T) {
	config := MiddlewareConfig{
		RateLimiter: &RateLimiterConfig{
			RequestsPerSecond: 10.0,
			Burst:             20,
			ExpiresIn:         5 * time.Minute,
		},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 1, "Should create exactly one middleware")
}

func TestCustomRequestIDGenerator(t *testing.T) {
	customIDGenerated := false
	config := MiddlewareConfig{
		RequestID: &RequestIDConfig{
			Generator: func() string {
				customIDGenerated = true
				return "custom-id-123"
			},
			TargetHeader: "X-My-Request-ID",
		},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 1, "Should create exactly one middleware")

	// The custom generator should be set but not called during middleware creation
	assert.False(t, customIDGenerated, "Custom generator should not be called during middleware creation")
}

// Test new preset configurations
func TestSecurityConfig(t *testing.T) {
	config := SecurityConfig()

	// Test that security config has appropriate middleware enabled
	assert.NotNil(t, config.Recover, "Recover should be enabled in security config")
	assert.NotNil(t, config.RequestID, "RequestID should be enabled in security config")
	assert.NotNil(t, config.CSRF, "CSRF should be enabled in security config")
	assert.NotNil(t, config.Secure, "Secure should be enabled in security config")
	assert.NotNil(t, config.RateLimiter, "RateLimiter should be enabled in security config")

	// Security-focused should not have CORS (restrictive by default)
	assert.Nil(t, config.CORS, "CORS should not be enabled in security config by default")
	assert.Nil(t, config.Logger, "Logger should not be enabled in security config by default")
}

func TestCompressionConfig(t *testing.T) {
	config := CompressionConfig()

	// Test that compression config has appropriate middleware
	assert.NotNil(t, config.Gzip, "Gzip should be enabled in compression config")
	assert.NotNil(t, config.Decompress, "Decompress should be enabled in compression config")
	assert.NotNil(t, config.BodyLimit, "BodyLimit should be enabled in compression config")
	assert.NotNil(t, config.RemoveTrailingSlash, "RemoveTrailingSlash should be enabled in compression config")

	// Verify specific configuration values
	assert.Equal(t, 6, config.Gzip.Level, "Gzip level should be 6 for balanced compression")
	assert.Equal(t, "10M", config.BodyLimit.Limit, "Body limit should be 10M")
}

func TestDevelopmentConfig(t *testing.T) {
	config := DevelopmentConfig()

	// Test that development config has appropriate middleware
	assert.NotNil(t, config.CORS, "CORS should be enabled in development config")
	assert.NotNil(t, config.Logger, "Logger should be enabled in development config")
	assert.NotNil(t, config.Recover, "Recover should be enabled in development config")
	assert.NotNil(t, config.RequestID, "RequestID should be enabled in development config")

	// Should be permissive for development
	assert.Contains(t, config.CORS.AllowOrigins, "*", "CORS should allow all origins in development")
	assert.Equal(t, 8<<10, config.Recover.StackSize, "Recover should have larger stack for debugging")
}

func TestProxyGatewayConfig(t *testing.T) {
	config := ProxyGatewayConfig()

	// Test that proxy config has appropriate middleware
	assert.NotNil(t, config.Logger, "Logger should be enabled in proxy config")
	assert.NotNil(t, config.Recover, "Recover should be enabled in proxy config")
	assert.NotNil(t, config.RequestID, "RequestID should be enabled in proxy config")
	assert.NotNil(t, config.RateLimiter, "RateLimiter should be enabled in proxy config")
	assert.NotNil(t, config.Timeout, "Timeout should be enabled in proxy config")

	// Should have higher rate limits for gateway scenarios
	assert.Equal(t, 100.0, config.RateLimiter.RequestsPerSecond, "Should have high rate limit for gateway")
	assert.Equal(t, 60*time.Second, config.Timeout.Timeout, "Should have longer timeout for proxy")
}

// Test individual middleware configurations
func TestBasicAuthMiddleware(t *testing.T) {
	config := MiddlewareConfig{
		BasicAuth: &BasicAuthConfig{
			Validator: func(username, password string, c echo.Context) (bool, error) {
				return username == "test" && password == "pass", nil
			},
			Realm: "Test Realm",
		},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 1, "Should create exactly one middleware")
}

func TestCSRFMiddleware(t *testing.T) {
	config := MiddlewareConfig{
		CSRF: &CSRFConfig{
			TokenLength:    32,
			TokenLookup:    "header:X-CSRF-Token",
			CookieName:     "_csrf",
			CookieSecure:   true,
			CookieHTTPOnly: true,
		},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 1, "Should create exactly one middleware")
}

func TestGzipMiddleware(t *testing.T) {
	config := MiddlewareConfig{
		Gzip: &GzipConfig{
			Level: 6,
		},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 1, "Should create exactly one middleware")
}

func TestBodyLimitMiddleware(t *testing.T) {
	config := MiddlewareConfig{
		BodyLimit: &BodyLimitConfig{
			Limit: "1M",
		},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 1, "Should create exactly one middleware")
}

func TestRewriteMiddleware(t *testing.T) {
	config := MiddlewareConfig{
		Rewrite: &RewriteConfig{
			Rules: map[string]string{
				"/old/*": "/new/$1",
			},
		},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 1, "Should create exactly one middleware")
}

func TestRedirectMiddleware(t *testing.T) {
	config := MiddlewareConfig{
		HTTPSRedirect: &RedirectConfig{
			Code: 301,
		},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 1, "Should create exactly one middleware")
}

func TestTrailingSlashMiddleware(t *testing.T) {
	config := MiddlewareConfig{
		RemoveTrailingSlash: &TrailingSlashConfig{
			RedirectCode: 301,
		},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 1, "Should create exactly one middleware")
}

func TestMultipleRedirectMiddleware(t *testing.T) {
	config := MiddlewareConfig{
		HTTPSRedirect: &RedirectConfig{Code: 301},
		WWWRedirect:   &RedirectConfig{Code: 302},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 2, "Should create two redirect middlewares")
}

func TestProxyMiddleware(t *testing.T) {
	config := MiddlewareConfig{
		Proxy: &ProxyConfig{
			Targets: []*ProxyTarget{
				{Name: "service1", URL: "http://localhost:8081"},
			},
		},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 1, "Should create exactly one middleware")
}

func TestContextTimeoutMiddleware(t *testing.T) {
	config := MiddlewareConfig{
		ContextTimeout: &ContextTimeoutConfig{
			Timeout: 30 * time.Second,
		},
	}

	middlewares := EchoMiddleware(config)
	assert.Len(t, middlewares, 1, "Should create exactly one middleware")
}

// Test comprehensive configuration
func TestComprehensiveConfiguration(t *testing.T) {
	config := MiddlewareConfig{
		// Core middleware
		CORS:      &CORSConfig{AllowOrigins: []string{"*"}},
		Logger:    &LoggerConfig{Format: "${method} ${uri}\n"},
		Recover:   &RecoverConfig{StackSize: 4 << 10},
		RequestID: &RequestIDConfig{TargetHeader: echo.HeaderXRequestID},

		// Security middleware
		CSRF:   &CSRFConfig{TokenLength: 32},
		Secure: &SecureConfig{XSSProtection: "1; mode=block"},

		// Performance middleware
		Gzip:      &GzipConfig{Level: 6},
		BodyLimit: &BodyLimitConfig{Limit: "10M"},

		// URL processing
		Rewrite: &RewriteConfig{
			Rules: map[string]string{"/old/*": "/new/$1"},
		},
		RemoveTrailingSlash: &TrailingSlashConfig{RedirectCode: 301},

		// Rate limiting
		RateLimiter: &RateLimiterConfig{
			RequestsPerSecond: 10,
			Burst:             15,
		},
	}

	middlewares := EchoMiddleware(config)
	assert.GreaterOrEqual(t, len(middlewares), 10, "Should create multiple middleware")
}

// Test all helper functions
func TestApplySecurityMiddleware(t *testing.T) {
	e := echo.New()
	require.NotPanics(t, func() {
		ApplySecurityMiddleware(e)
	}, "ApplySecurityMiddleware should not panic")
}

func TestApplyCompressionMiddleware(t *testing.T) {
	e := echo.New()
	require.NotPanics(t, func() {
		ApplyCompressionMiddleware(e)
	}, "ApplyCompressionMiddleware should not panic")
}

func TestApplyDevelopmentMiddleware(t *testing.T) {
	e := echo.New()
	require.NotPanics(t, func() {
		ApplyDevelopmentMiddleware(e)
	}, "ApplyDevelopmentMiddleware should not panic")
}

func TestApplyProxyGatewayMiddleware(t *testing.T) {
	e := echo.New()
	require.NotPanics(t, func() {
		ApplyProxyGatewayMiddleware(e)
	}, "ApplyProxyGatewayMiddleware should not panic")
}
