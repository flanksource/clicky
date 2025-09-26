/*
Package middleware provides a comprehensive, configurable middleware system for Echo v4.

This package offers 20+ middleware types with YAML/JSON configuration support,
CEL-based dynamic processing, and multiple preset configurations for different
deployment scenarios (development, production, security-focused, etc.).

The middleware system emphasizes configuration-as-code through YAML files,
making it easy to manage complex middleware setups across different environments.

# Quick Start

The simplest way to get started is using one of the preset configurations:

	e := echo.New()
	middleware.ApplyDefaultMiddleware(e) // CORS, Logger, Recover, RequestID, Gzip, Secure

Or load configuration from a YAML file:

	config, err := middleware.LoadConfigFromYAML("middleware.yaml")
	if err != nil {
		log.Fatal(err)
	}
	middleware.ApplyMiddleware(e, config)

# Configuration System

The middleware system supports multiple configuration approaches:

1. Preset configurations for common scenarios
2. YAML/JSON file-based configuration (recommended)
3. Programmatic configuration with Go structs
4. Dynamic configuration with CEL expressions

# YAML Configuration Examples

# Minimal Configuration

Load from examples/middleware/minimal.yaml:

	# Minimal setup with just essentials
	recover:
	  stack_size: 4096
	request_id:
	  target_header: "X-Request-ID"

# Development Configuration

Load from examples/middleware/development.yaml:

	# Developer-friendly with permissive CORS
	cors:
	  allow_origins: ["*"]
	  allow_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
	  allow_headers: ["*"]

	logger:
	  format: "[${time_rfc3339_nano}] ${method} ${uri} (${status}) ${latency_human}\n"

	recover:
	  stack_size: 8192  # Larger stack for debugging

# Production Configuration

Load from examples/middleware/production.yaml:

	# Security hardened for production
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

# Loading YAML Configuration

	config, err := middleware.LoadConfigFromYAML("middleware.yaml")
	if err != nil {
		log.Fatalf("Failed to load middleware config: %v", err)
	}

	// Validate configuration
	if err := middleware.ValidateConfig(config); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Apply to Echo instance
	middleware.ApplyMiddleware(e, config)

# Middleware Categories

# Core Middleware

Essential middleware for most applications:

	cors:           # Cross-Origin Resource Sharing
	logger:         # Request logging with customizable formats
	recover:        # Panic recovery with stack traces
	request_id:     # Unique request identifier generation

# Security Middleware

Authentication and security hardening:

	basic_auth:     # HTTP Basic Authentication with htpasswd/userpass files
	jwt_auth:       # JWT authentication (HMAC/RSA/ECDSA) with CEL validation
	key_auth:       # API key authentication
	csrf:           # Cross-Site Request Forgery protection
	secure:         # Security headers (XSS, HSTS, CSP, etc.)

# Request/Response Processing

Content handling and transformation:

	body_dump:      # Request/response body logging
	body_limit:     # Request body size limiting
	gzip:           # Response compression
	decompress:     # Request decompression
	interceptors:   # Generic CEL-based request/response processing

# Performance and Reliability

Rate limiting and timeouts:

	rate_limiter:   # Request rate limiting with memory store
	timeout:        # Request timeout middleware
	context_timeout: # Context-based timeout with custom handlers

# Authentication Examples

# Basic Authentication with htpasswd

From examples/middleware/authentication.yaml:

	basic_auth:
	  htpasswd_file: ".htpasswd"
	  realm: "Admin Panel"

# JWT Authentication

From examples/middleware/authentication.yaml:

	jwt_auth:
	  signing_key: "your-secret-key"
	  signing_method: "HS256"
	  token_lookup: "header:Authorization"
	  token_prefix: "Bearer "
	  validation: 'claims.exp > now() && claims.aud == "api"'

# API Key Authentication

	key_auth:
	  key_lookup: "header:X-API-Key"

# CEL Integration

The middleware system includes powerful CEL (Common Expression Language)
integration for dynamic request/response processing through interceptors.

# CEL Interceptors Example

From examples/middleware/cel-interceptors.yaml:

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

# Available CEL Functions

Request/response manipulation: setHeader(), returnStatus(), returnJSON()

Authentication helpers: hasRole(), getUser(), validateJWT()

Utility functions: jsonToXML() plus all Gomplate functions

Variables: request, response, context, headers, body, user, claims

# Configuration Validation

All configurations are validated before application:

	if err := middleware.ValidateConfig(config); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

Validation checks include:
- CORS credential/origin compatibility
- Rate limiter positive values
- Timeout positive durations
- Body limit format validation
- JWT key file existence
- Proxy target URL validity

# Error Handling

The middleware system provides:
- Comprehensive configuration validation with detailed error messages
- Graceful fallbacks for optional middleware
- Structured error responses with proper HTTP status codes
- Debug information in development, secure handling in production

# Example Files

Complete YAML configuration examples are available in examples/middleware/:

- minimal.yaml          - Essential middleware only
- development.yaml      - Developer-friendly settings
- production.yaml       - Production security hardening
- security.yaml         - Security-focused subset
- authentication.yaml   - All authentication methods
- cel-interceptors.yaml - Advanced CEL examples
- comprehensive.yaml    - All middleware types demo

# Programmatic Usage

While YAML configuration is recommended, programmatic configuration is also supported:

	config := middleware.MiddlewareConfig{
		CORS: &middleware.CORSConfig{
			AllowOrigins:     []string{"https://example.com"},
			AllowCredentials: true,
		},
		RateLimiter: &middleware.RateLimiterConfig{
			RequestsPerSecond: 10,
			Burst:             15,
		},
	}
	middleware.ApplyMiddleware(e, config)

# Testing

The package includes comprehensive test coverage and utilities:

	func TestMyMiddleware(t *testing.T) {
		e := echo.New()
		config, _ := middleware.LoadConfigFromYAML("test-config.yaml")
		middleware.ApplyMiddleware(e, config)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.NotEmpty(t, rec.Header().Get("X-Request-Id"))
	}

# Best Practices

# Security

  - Always use HTTPS in production with HTTPSRedirect
  - Enable CSRF protection for web applications
  - Set restrictive CORS origins (avoid "*" in production)
  - Use strong JWT signing keys and rotate them regularly
  - Implement rate limiting to prevent abuse

# Performance

  - Enable compression with Gzip for API responses
  - Set appropriate timeouts to prevent resource exhaustion
  - Configure appropriate body limits based on use case
  - Use request IDs for tracing and debugging

# Configuration Management

  - Use YAML files for environment-specific configs
  - Validate configurations before deployment
  - Use environment variables for secrets
  - Version your middleware configurations
  - Test configurations with unit tests

For detailed API reference and additional examples, see the complete configuration
options in the source code and example files.
*/
package middleware