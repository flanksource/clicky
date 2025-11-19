package middleware

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gopkg.in/yaml.v3"
)

// MiddlewareConfig contains configuration for all available Echo middleware.
// This struct provides a unified way to configure all Echo v4 middleware types
// through either programmatic configuration or YAML/JSON file loading.
type MiddlewareConfig struct {
	// Core middleware - Essential middleware for most applications
	CORS      *CORSConfig      `json:"cors,omitempty" yaml:"cors,omitempty"`             // Cross-Origin Resource Sharing
	Logger    *LoggerConfig    `json:"logger,omitempty" yaml:"logger,omitempty"`         // Request logging
	Recover   *RecoverConfig   `json:"recover,omitempty" yaml:"recover,omitempty"`       // Panic recovery
	RequestID *RequestIDConfig `json:"request_id,omitempty" yaml:"request_id,omitempty"` // Request ID generation

	// Security middleware - Authentication and authorization
	BasicAuth *BasicAuthConfig `json:"basic_auth,omitempty" yaml:"basic_auth,omitempty"` // HTTP Basic Authentication
	JWTAuth   *JWTAuthConfig   `json:"jwt_auth,omitempty" yaml:"jwt_auth,omitempty"`     // JWT Token Authentication
	KeyAuth   *KeyAuthConfig   `json:"key_auth,omitempty" yaml:"key_auth,omitempty"`     // API Key Authentication
	CSRF      *CSRFConfig      `json:"csrf,omitempty" yaml:"csrf,omitempty"`             // Cross-Site Request Forgery protection
	Secure    *SecureConfig    `json:"secure,omitempty" yaml:"secure,omitempty"`         // Security headers (XSS, HSTS, etc.)

	// Request/Response interceptors - CEL-powered processing
	Interceptors []*InterceptorConfig `json:"interceptors,omitempty" yaml:"interceptors,omitempty"` // Generic request/response interceptors

	// Request/Response processing middleware - Content handling
	BodyDump   *BodyDumpConfig   `json:"body_dump,omitempty" yaml:"body_dump,omitempty"`   // Request/response body logging
	BodyLimit  *BodyLimitConfig  `json:"body_limit,omitempty" yaml:"body_limit,omitempty"` // Request body size limiting
	Gzip       *GzipConfig       `json:"gzip,omitempty" yaml:"gzip,omitempty"`             // Response compression
	Decompress *DecompressConfig `json:"decompress,omitempty" yaml:"decompress,omitempty"` // Request decompression

	// Routing and static middleware - URL and file handling
	Static         *StaticConfig         `json:"static,omitempty" yaml:"static,omitempty"`                   // Static file serving
	MethodOverride *MethodOverrideConfig `json:"method_override,omitempty" yaml:"method_override,omitempty"` // HTTP method override
	Proxy          *ProxyConfig          `json:"proxy,omitempty" yaml:"proxy,omitempty"`                     // Reverse proxy
	Rewrite        *RewriteConfig        `json:"rewrite,omitempty" yaml:"rewrite,omitempty"`                 // URL rewriting

	// Redirect middleware - URL redirection
	HTTPSRedirect    *RedirectConfig `json:"https_redirect,omitempty" yaml:"https_redirect,omitempty"`         // HTTP to HTTPS redirect
	WWWRedirect      *RedirectConfig `json:"www_redirect,omitempty" yaml:"www_redirect,omitempty"`             // WWW subdomain redirect
	HTTPSWWWRedirect *RedirectConfig `json:"https_www_redirect,omitempty" yaml:"https_www_redirect,omitempty"` // Combined HTTPS+WWW redirect

	// URL normalization middleware - Trailing slash handling
	AddTrailingSlash    *TrailingSlashConfig `json:"add_trailing_slash,omitempty" yaml:"add_trailing_slash,omitempty"`       // Add trailing slash to URLs
	RemoveTrailingSlash *TrailingSlashConfig `json:"remove_trailing_slash,omitempty" yaml:"remove_trailing_slash,omitempty"` // Remove trailing slash from URLs

	// Rate limiting and timeout middleware - Performance and reliability
	RateLimiter    *RateLimiterConfig    `json:"rate_limiter,omitempty" yaml:"rate_limiter,omitempty"`       // Request rate limiting
	Timeout        *TimeoutConfig        `json:"timeout,omitempty" yaml:"timeout,omitempty"`                 // Request timeout
	ContextTimeout *ContextTimeoutConfig `json:"context_timeout,omitempty" yaml:"context_timeout,omitempty"` // Context-based timeout

	// Advanced logging middleware - Structured logging
	RequestLogger *RequestLoggerConfig `json:"request_logger,omitempty" yaml:"request_logger,omitempty"` // Advanced structured logging
}

// CORSConfig configures Cross-Origin Resource Sharing (CORS) middleware.
// CORS enables secure cross-domain data transfers by controlling access from browsers.
// See: https://echo.labstack.com/docs/middleware/cors
type CORSConfig struct {
	// AllowOrigins defines a list of allowed origin domains for cross-origin requests.
	// Examples: ["https://example.com", "https://api.example.com"]
	// Use ["*"] to allow all origins (not recommended for production with credentials)
	// Default: ["*"]
	AllowOrigins []string `json:"allow_origins,omitempty" yaml:"allow_origins,omitempty"`

	// AllowMethods specifies which HTTP methods are allowed in cross-origin requests.
	// Examples: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
	// Default: ["GET", "HEAD", "PUT", "PATCH", "POST", "DELETE"]
	AllowMethods []string `json:"allow_methods,omitempty" yaml:"allow_methods,omitempty"`

	// AllowHeaders specifies which request headers can be used during the actual request.
	// Examples: ["Content-Type", "Authorization", "X-Requested-With"]
	// Use ["*"] to allow all headers (requires specific origin, not wildcard)
	// Default: []
	AllowHeaders []string `json:"allow_headers,omitempty" yaml:"allow_headers,omitempty"`

	// AllowCredentials indicates whether the response can be exposed when credentials flag is true.
	// When true, AllowOrigins cannot contain "*" wildcard.
	// Examples: true (for authenticated requests), false (for public APIs)
	// Default: false
	AllowCredentials bool `json:"allow_credentials,omitempty" yaml:"allow_credentials,omitempty"`

	// ExposeHeaders indicates which headers are safe to expose to the API of a CORS API specification.
	// Examples: ["Content-Length", "X-Total-Count", "X-Request-ID"]
	// Default: []
	ExposeHeaders []string `json:"expose_headers,omitempty" yaml:"expose_headers,omitempty"`

	// MaxAge indicates how long (in seconds) the results of a preflight request can be cached.
	// Examples: 86400 (24 hours), 3600 (1 hour)
	// Default: 0 (no caching)
	MaxAge int `json:"max_age,omitempty" yaml:"max_age,omitempty"`
}

// LoggerConfig configures request logging middleware.
// The Logger middleware logs HTTP requests with customizable format and output.
// See: https://echo.labstack.com/docs/middleware/logger
type LoggerConfig struct {
	// Format defines the log format template using Echo's template syntax.
	// Available variables: time_unix, time_rfc3339, remote_ip, uri, host, method, status, error, latency, bytes_in, bytes_out
	// Examples: "${method} ${uri} ${status} ${latency_human}\n"
	//           "{\"time\":\"${time_rfc3339}\",\"method\":\"${method}\",\"uri\":\"${uri}\",\"status\":${status}}\n"
	// Default: JSON format with comprehensive request details
	Format string `json:"format,omitempty" yaml:"format,omitempty"`

	// CustomTimeFormat specifies a custom time format for ${time_custom} variable.
	// Uses Go's time format layout ("2006-01-02 15:04:05").
	// Examples: "2006-01-02 15:04:05", "Jan _2 15:04:05"
	// Default: ""
	CustomTimeFormat string `json:"custom_time_format,omitempty" yaml:"custom_time_format,omitempty"`

	// Output specifies the output destination for log messages (io.Writer).
	// Cannot be serialized to JSON/YAML, must be set programmatically.
	// Examples: os.Stdout, os.Stderr, file handle
	// Default: os.Stdout
	Output interface{} `json:"-" yaml:"-"`

	// Template is deprecated in favor of Format field.
	// Kept for backwards compatibility.
	Template string `json:"template,omitempty" yaml:"template,omitempty"`

	// CustomTags allows adding custom variables to the log format.
	// Examples: map[string]string{"app": "myapp", "version": "1.0.0"}
	// Default: nil
	CustomTags map[string]string `json:"custom_tags,omitempty" yaml:"custom_tags,omitempty"`
}

// RecoverConfig configures panic recovery middleware.
// Recover middleware recovers from panics anywhere in the chain and handles them gracefully.
// See: https://echo.labstack.com/docs/middleware/recover
type RecoverConfig struct {
	// StackSize sets the maximum size of the stack trace in bytes.
	// Examples: 4096 (4KB), 8192 (8KB)
	// Default: 4KB
	StackSize int `json:"stack_size,omitempty" yaml:"stack_size,omitempty"`

	// DisableStackAll disables printing stack trace of all goroutines.
	// Examples: true (show only current goroutine), false (show all)
	// Default: false
	DisableStackAll bool `json:"disable_stack_all,omitempty" yaml:"disable_stack_all,omitempty"`

	// DisablePrintStack disables printing the stack trace.
	// Examples: true (no stack trace), false (print stack trace)
	// Default: false
	DisablePrintStack bool `json:"disable_print_stack,omitempty" yaml:"disable_print_stack,omitempty"`

	// DisableErrorHandler disables the centralized error handler for panics.
	// Examples: true (handle panics inline), false (use global error handler)
	// Default: false
	DisableErrorHandler bool `json:"disable_error_handler,omitempty" yaml:"disable_error_handler,omitempty"`
}

// RequestIDConfig configures request ID generation middleware.
// RequestID generates a unique identifier for each request for tracing and logging.
// See: https://echo.labstack.com/docs/middleware/request-id
type RequestIDConfig struct {
	// Generator is a function that generates unique request IDs.
	// Cannot be serialized to JSON/YAML, must be set programmatically.
	// Default: generates random 32-character hex string
	Generator func() string `json:"-" yaml:"-"`

	// TargetHeader specifies which header to store the request ID in.
	// Examples: "X-Request-ID", "X-Correlation-ID", "X-Trace-ID"
	// Default: "X-Request-Id"
	TargetHeader string `json:"target_header,omitempty" yaml:"target_header,omitempty"`

	// Handler is a function called after the request ID is generated.
	// Cannot be serialized to JSON/YAML, must be set programmatically.
	// Default: nil
	Handler func(echo.Context, string) `json:"-" yaml:"-"`
}

// RateLimiterConfig configures request rate limiting middleware.
// RateLimiter controls the rate of requests to prevent abuse and ensure fair usage.
// See: https://echo.labstack.com/docs/middleware/rate-limiter
type RateLimiterConfig struct {
	// RequestsPerSecond sets the rate limit as requests per second.
	// Examples: 10.0 (10 req/s), 100.0 (100 req/s), 0.1 (1 req per 10s)
	// Default: 20.0
	RequestsPerSecond float64 `json:"requests_per_second" yaml:"requests_per_second"`

	// Burst sets the maximum burst size (number of requests that can exceed the rate).
	// Examples: 30 (allow bursts up to 30), 1 (strict rate limiting)
	// Default: RequestsPerSecond * 1.5
	Burst int `json:"burst" yaml:"burst"`

	// ExpiresIn sets how long rate limit entries are kept in memory.
	// Examples: 5*time.Minute, 1*time.Hour
	// YAML format: "5m", "1h", "30s"
	// Default: 5 minutes
	ExpiresIn time.Duration `json:"expires_in,omitempty" yaml:"expires_in,omitempty"`
}

// TimeoutConfig configures request timeout middleware.
// Timeout middleware cancels requests that take longer than the specified duration.
// See: https://echo.labstack.com/docs/middleware/timeout
type TimeoutConfig struct {
	// Timeout sets the maximum duration for request processing.
	// Examples: 30*time.Second, 5*time.Minute
	// YAML format: "30s", "5m", "1h"
	// Default: 30 seconds
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// SecureConfig configures security headers middleware.
// Secure middleware adds various security headers to protect against common attacks.
// See: https://echo.labstack.com/docs/middleware/secure
type SecureConfig struct {
	// XSSProtection sets the X-XSS-Protection header to prevent XSS attacks.
	// Examples: "1; mode=block", "1", "0"
	// Default: "1; mode=block"
	XSSProtection string `json:"xss_protection,omitempty" yaml:"xss_protection,omitempty"`

	// ContentTypeNosniff sets the X-Content-Type-Options header.
	// Examples: "nosniff"
	// Default: "nosniff"
	ContentTypeNosniff string `json:"content_type_nosniff,omitempty" yaml:"content_type_nosniff,omitempty"`

	// XFrameOptions sets the X-Frame-Options header to prevent clickjacking.
	// Examples: "DENY", "SAMEORIGIN", "ALLOW-FROM https://example.com"
	// Default: "SAMEORIGIN"
	XFrameOptions string `json:"x_frame_options,omitempty" yaml:"x_frame_options,omitempty"`

	// HSTSMaxAge sets the max-age value for the Strict-Transport-Security header (seconds).
	// Examples: 31536000 (1 year), 63072000 (2 years)
	// Default: 0 (disabled)
	HSTSMaxAge int `json:"hsts_max_age,omitempty" yaml:"hsts_max_age,omitempty"`

	// HSTSExcludeSubdomains excludes subdomains from HSTS policy.
	// Examples: true (exclude subdomains), false (include subdomains)
	// Default: false
	HSTSExcludeSubdomains bool `json:"hsts_exclude_subdomains,omitempty" yaml:"hsts_exclude_subdomains,omitempty"`

	// ContentSecurityPolicy sets the Content-Security-Policy header.
	// Examples: "default-src 'self'", "default-src 'self'; script-src 'self' 'unsafe-inline'"
	// Default: ""
	ContentSecurityPolicy string `json:"content_security_policy,omitempty" yaml:"content_security_policy,omitempty"`

	// CSPReportOnly sets Content-Security-Policy-Report-Only instead of CSP.
	// Examples: true (report only), false (enforce policy)
	// Default: false
	CSPReportOnly bool `json:"csp_report_only,omitempty" yaml:"csp_report_only,omitempty"`

	// HSTSPreloadEnabled adds the preload directive to HSTS header.
	// Examples: true (enable preload), false (disable preload)
	// Default: false
	HSTSPreloadEnabled bool `json:"hsts_preload_enabled,omitempty" yaml:"hsts_preload_enabled,omitempty"`

	// ReferrerPolicy sets the Referrer-Policy header.
	// Examples: "no-referrer", "strict-origin", "same-origin"
	// Default: ""
	ReferrerPolicy string `json:"referrer_policy,omitempty" yaml:"referrer_policy,omitempty"`
}

// BasicAuthConfig configures HTTP Basic Authentication middleware.
// BasicAuth middleware provides HTTP Basic Authentication using username/password credentials
// with support for htpasswd files, simple user=password files, and CEL-based validation.
// See: https://echo.labstack.com/docs/middleware/basic-auth
type BasicAuthConfig struct {
	// Validator is a function that validates username and password credentials.
	// Cannot be serialized to JSON/YAML, must be set programmatically.
	// Example: func(username, password string, c echo.Context) (bool, error) {
	//   return username == "admin" && password == "secret", nil
	// }
	Validator func(string, string, echo.Context) (bool, error) `json:"-" yaml:"-"`

	// HtpasswdFile path to Apache htpasswd file for user authentication.
	// Supports bcrypt, SHA1, and MD5 hashing algorithms.
	// Example: "auth/.htpasswd"
	HtpasswdFile string `json:"htpasswd_file,omitempty" yaml:"htpasswd_file,omitempty"`

	// UserpassFile path to simple user=password text file (one per line).
	// Passwords are stored in plain text. Use only for development.
	// Example: "auth/users.txt" with content like: "admin=secret123\nuser=password\n"
	UserpassFile string `json:"userpass_file,omitempty" yaml:"userpass_file,omitempty"`

	// Validation CEL expression for additional user validation logic.
	// Available variables: user (string), password (string), context (echo.Context)
	// Examples: 'user.startsWith("admin")', 'user.matches("[a-z]+") && len(password) >= 8'
	Validation string `json:"validation,omitempty" yaml:"validation,omitempty"`

	// Realm sets the authentication realm displayed in the browser's login prompt.
	// Examples: "Restricted Area", "Admin Panel", "API Access"
	// Default: "Restricted"
	Realm string `json:"realm,omitempty" yaml:"realm,omitempty"`
}

// JWTAuthConfig configures JWT (JSON Web Token) authentication middleware.
// JWT middleware provides stateless authentication using signed tokens with configurable validation.
type JWTAuthConfig struct {
	// SigningKey is the secret key used to validate JWT tokens (for HMAC algorithms).
	// For RSA/ECDSA algorithms, use SigningKeyFile instead.
	// Examples: "my-secret-key", "${JWT_SECRET}"
	SigningKey string `json:"signing_key,omitempty" yaml:"signing_key,omitempty"`

	// SigningKeyFile path to file containing signing key (for RSA/ECDSA algorithms).
	// Examples: "keys/jwt-rsa.pem", "keys/jwt-ec.pem"
	SigningKeyFile string `json:"signing_key_file,omitempty" yaml:"signing_key_file,omitempty"`

	// SigningMethod specifies the JWT signing algorithm.
	// Supported: "HS256", "HS384", "HS512", "RS256", "RS384", "RS512", "ES256", "ES384", "ES512"
	// Examples: "HS256", "RS256"
	// Default: "HS256"
	SigningMethod string `json:"signing_method,omitempty" yaml:"signing_method,omitempty"`

	// TokenLookup specifies where to look for the JWT token.
	// Examples: "header:Authorization", "query:token", "form:token", "cookie:token"
	// Default: "header:Authorization"
	TokenLookup string `json:"token_lookup,omitempty" yaml:"token_lookup,omitempty"`

	// TokenPrefix specifies the prefix to strip from the token value.
	// Examples: "Bearer ", "Token ", "" (no prefix)
	// Default: "Bearer "
	TokenPrefix string `json:"token_prefix,omitempty" yaml:"token_prefix,omitempty"`

	// Validation CEL expression for additional token/claims validation.
	// Available variables: token (jwt.Token), claims (jwt.MapClaims), context (echo.Context)
	// Examples: 'claims.exp > now()', 'claims.aud == "api" && claims.iss == "myapp"'
	Validation string `json:"validation,omitempty" yaml:"validation,omitempty"`

	// ErrorHandler handles JWT validation errors.
	// Cannot be serialized to JSON/YAML, must be set programmatically.
	// Default: returns HTTP 401 Unauthorized
	ErrorHandler func(error, echo.Context) error `json:"-" yaml:"-"`

	// SuccessHandler handles successful JWT validation.
	// Cannot be serialized to JSON/YAML, must be set programmatically.
	// Default: stores user claims in context and continues
	SuccessHandler func(echo.Context) error `json:"-" yaml:"-"`
}

// KeyAuthConfig configures API Key Authentication middleware.
// KeyAuth middleware provides authentication using API keys from headers, query parameters, or forms.
// See: https://echo.labstack.com/docs/middleware/key-auth
type KeyAuthConfig struct {
	// KeyLookup specifies where to look for the API key.
	// Examples: "header:X-API-Key", "query:api_key", "form:api_key", "header:Authorization"
	// Default: "header:Authorization"
	KeyLookup string `json:"key_lookup,omitempty" yaml:"key_lookup,omitempty"`

	// AuthScheme sets the authentication scheme for Authorization header.
	// Examples: "Bearer", "Token", "" (no scheme)
	// Default: "Bearer"
	AuthScheme string `json:"auth_scheme,omitempty" yaml:"auth_scheme,omitempty"`

	// Validator is a function that validates the provided API key.
	// Cannot be serialized to JSON/YAML, must be set programmatically.
	// Example: func(key string, c echo.Context) (bool, error) {
	//   return key == "valid-api-key", nil
	// }
	Validator func(string, echo.Context) (bool, error) `json:"-" yaml:"-"`

	// ErrorHandler handles authentication errors.
	// Cannot be serialized to JSON/YAML, must be set programmatically.
	// Default: returns HTTP 401 Unauthorized
	ErrorHandler func(error, echo.Context) error `json:"-" yaml:"-"`
}

// CSRFConfig configures Cross-Site Request Forgery protection middleware.
// CSRF middleware protects against Cross-Site Request Forgery attacks using tokens.
// See: https://echo.labstack.com/docs/middleware/csrf
type CSRFConfig struct {
	// TokenLength sets the length of the CSRF token in bytes.
	// Examples: 32 (32 bytes = 256 bits), 16 (16 bytes = 128 bits)
	// Default: 32
	TokenLength uint8 `json:"token_length,omitempty" yaml:"token_length,omitempty"`

	// TokenLookup specifies where to look for the CSRF token.
	// Examples: "header:X-CSRF-Token", "form:_token", "query:csrf"
	// Default: "header:X-CSRF-Token"
	TokenLookup string `json:"token_lookup,omitempty" yaml:"token_lookup,omitempty"`

	// ContextKey sets the key used to store the token in the request context.
	// Examples: "csrf", "_csrf_token"
	// Default: "csrf"
	ContextKey string `json:"context_key,omitempty" yaml:"context_key,omitempty"`

	// CookieName sets the name of the CSRF cookie.
	// Examples: "_csrf", "csrf-token"
	// Default: "_csrf"
	CookieName string `json:"cookie_name,omitempty" yaml:"cookie_name,omitempty"`

	// CookieDomain sets the domain attribute of the CSRF cookie.
	// Examples: ".example.com", "api.example.com"
	// Default: ""
	CookieDomain string `json:"cookie_domain,omitempty" yaml:"cookie_domain,omitempty"`

	// CookiePath sets the path attribute of the CSRF cookie.
	// Examples: "/", "/api"
	// Default: ""
	CookiePath string `json:"cookie_path,omitempty" yaml:"cookie_path,omitempty"`

	// CookieMaxAge sets the max-age attribute of the CSRF cookie (seconds).
	// Examples: 3600 (1 hour), 86400 (24 hours)
	// Default: 86400 (24 hours)
	CookieMaxAge int `json:"cookie_max_age,omitempty" yaml:"cookie_max_age,omitempty"`

	// CookieSecure sets the secure attribute of the CSRF cookie.
	// Examples: true (HTTPS only), false (HTTP and HTTPS)
	// Default: false
	CookieSecure bool `json:"cookie_secure,omitempty" yaml:"cookie_secure,omitempty"`

	// CookieHTTPOnly sets the HttpOnly attribute of the CSRF cookie.
	// Examples: true (no JavaScript access), false (JavaScript accessible)
	// Default: false
	CookieHTTPOnly bool `json:"cookie_http_only,omitempty" yaml:"cookie_http_only,omitempty"`
}

// BodyDumpConfig configures BodyDump middleware
type BodyDumpConfig struct {
	Handler func(echo.Context, []byte, []byte) `json:"-"` // Body dump handler function, not serializable
}

// BodyLimitConfig configures request body size limiting middleware.
// BodyLimit middleware limits the size of request bodies to prevent abuse.
// See: https://echo.labstack.com/docs/middleware/body-limit
type BodyLimitConfig struct {
	// Limit sets the maximum allowed size for request bodies.
	// Examples: "1M" (1 megabyte), "500K" (500 kilobytes), "2G" (2 gigabytes)
	// Supported units: B, K, M, G, T, P
	// Default: "32M"
	Limit string `json:"limit" yaml:"limit"`
}

// GzipConfig configures response compression middleware.
// Gzip middleware compresses HTTP responses to reduce bandwidth usage.
// See: https://echo.labstack.com/docs/middleware/gzip
type GzipConfig struct {
	// Level sets the compression level for gzip.
	// Examples: 1 (fastest), 6 (balanced), 9 (best compression), -1 (default)
	// Range: -1 (default), 0 (no compression), 1-9 (compression levels)
	// Default: -1 (default compression)
	Level int `json:"level,omitempty" yaml:"level,omitempty"`
}

// DecompressConfig configures Decompress middleware
type DecompressConfig struct {
	GzipDecompressPool interface{} `json:"-"` // Pool for gzip decompression, not serializable
}

// StaticConfig configures Static middleware
type StaticConfig struct {
	Root       string          `json:"root,omitempty"`        // Root directory for static files
	Index      string          `json:"index,omitempty"`       // Index file name
	Browse     bool            `json:"browse,omitempty"`      // Enable directory browsing
	HTML5      bool            `json:"html5,omitempty"`       // Enable HTML5 mode
	IgnoreBase bool            `json:"ignore_base,omitempty"` // Ignore base of URL path
	Filesystem http.FileSystem `json:"-"`                     // Custom filesystem, not serializable
}

// MethodOverrideConfig configures MethodOverride middleware
type MethodOverrideConfig struct {
	Getter func(echo.Context) string `json:"-"` // Method getter function, not serializable
}

// RequestLoggerConfig configures RequestLogger middleware
type RequestLoggerConfig struct {
	LogStatus        bool                                                     `json:"log_status,omitempty"`
	LogURI           bool                                                     `json:"log_uri,omitempty"`
	LogError         bool                                                     `json:"log_error,omitempty"`
	LogLatency       bool                                                     `json:"log_latency,omitempty"`
	LogProtocol      bool                                                     `json:"log_protocol,omitempty"`
	LogRemoteIP      bool                                                     `json:"log_remote_ip,omitempty"`
	LogHost          bool                                                     `json:"log_host,omitempty"`
	LogMethod        bool                                                     `json:"log_method,omitempty"`
	LogUserAgent     bool                                                     `json:"log_user_agent,omitempty"`
	LogRequestID     bool                                                     `json:"log_request_id,omitempty"`
	LogReferer       bool                                                     `json:"log_referer,omitempty"`
	LogContentLength bool                                                     `json:"log_content_length,omitempty"`
	LogResponseSize  bool                                                     `json:"log_response_size,omitempty"`
	HandleError      bool                                                     `json:"handle_error,omitempty"`
	LogValuesFunc    func(echo.Context, middleware.RequestLoggerValues) error `json:"-"` // Log values function, not serializable
}

// ProxyConfig configures Proxy middleware
type ProxyConfig struct {
	Targets        []*ProxyTarget             `json:"targets,omitempty"`       // List of targets
	Balancer       interface{}                `json:"-"`                       // Load balancer, not serializable
	Rewrite        map[string]string          `json:"rewrite,omitempty"`       // URL rewrite rules
	RegexRewrite   map[string]string          `json:"regex_rewrite,omitempty"` // Regex rewrite rules
	Timeout        time.Duration              `json:"timeout,omitempty"`       // Request timeout
	ModifyResponse func(*http.Response) error `json:"-"`                       // Response modifier, not serializable
}

// ProxyTarget represents a proxy target
type ProxyTarget struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url"`
}

// RewriteConfig configures Rewrite middleware
type RewriteConfig struct {
	Rules map[string]string `json:"rules,omitempty"` // URL rewrite rules (from -> to)
}

// RedirectConfig configures redirect middleware (HTTPSRedirect, WWWRedirect, etc.)
type RedirectConfig struct {
	Code int `json:"code,omitempty"` // HTTP redirect status code (301, 302, etc.)
}

// TrailingSlashConfig configures TrailingSlash middleware
type TrailingSlashConfig struct {
	RedirectCode int `json:"redirect_code,omitempty"` // HTTP redirect status code (default 301)
}

// ContextTimeoutConfig configures ContextTimeout middleware
type ContextTimeoutConfig struct {
	Timeout                    time.Duration                   `json:"timeout"` // Request timeout
	ErrorHandler               func(error, echo.Context) error `json:"-"`       // Timeout error handler, not serializable
	OnTimeoutRouteErrorHandler func(error, echo.Context)       `json:"-"`       // Timeout route error handler, not serializable
}

// DefaultConfig returns a MiddlewareConfig with sensible defaults
func DefaultConfig() MiddlewareConfig {
	return MiddlewareConfig{
		CORS: &CORSConfig{
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"*"},
			AllowCredentials: false,
			MaxAge:           86400, // 24 hours
		},
		Logger: &LoggerConfig{
			Format: `{"time":"${time_rfc3339}","level":"info","method":"${method}","uri":"${uri}","status":${status},"latency_human":"${latency_human}","bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n",
		},
		Recover: &RecoverConfig{
			StackSize:           4 << 10, // 4 KB
			DisableStackAll:     false,
			DisablePrintStack:   false,
			DisableErrorHandler: false,
		},
		RequestID: &RequestIDConfig{
			TargetHeader: echo.HeaderXRequestID,
		},
		Gzip: &GzipConfig{
			Level: -1, // Default compression
		},
		Secure: &SecureConfig{
			XSSProtection:      "1; mode=block",
			ContentTypeNosniff: "nosniff",
			XFrameOptions:      "DENY",
			HSTSMaxAge:         31536000, // 1 year
			ReferrerPolicy:     "strict-origin-when-cross-origin",
		},
	}
}

// MinimalConfig returns a MiddlewareConfig with only essential middleware
func MinimalConfig() MiddlewareConfig {
	return MiddlewareConfig{
		Recover: &RecoverConfig{
			StackSize: 4 << 10,
		},
		RequestID: &RequestIDConfig{
			TargetHeader: echo.HeaderXRequestID,
		},
	}
}

// ProductionConfig returns a MiddlewareConfig suitable for production use
func ProductionConfig() MiddlewareConfig {
	return MiddlewareConfig{
		CORS: &CORSConfig{
			AllowOrigins:     []string{"https://yourdomain.com"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
			AllowCredentials: true,
			MaxAge:           86400,
		},
		Logger: &LoggerConfig{
			Format: `{"time":"${time_rfc3339}","level":"info","method":"${method}","uri":"${uri}","status":${status},"latency_human":"${latency_human}","remote_ip":"${remote_ip}","user_agent":"${user_agent}"}` + "\n",
		},
		Recover: &RecoverConfig{
			StackSize:           4 << 10,
			DisableStackAll:     false,
			DisablePrintStack:   false,
			DisableErrorHandler: false,
		},
		RequestID: &RequestIDConfig{
			TargetHeader: echo.HeaderXRequestID,
		},
		CSRF: &CSRFConfig{
			TokenLength:    32,
			TokenLookup:    "header:X-CSRF-Token",
			ContextKey:     "csrf",
			CookieName:     "_csrf",
			CookieMaxAge:   86400, // 24 hours
			CookieSecure:   true,
			CookieHTTPOnly: true,
		},
		BodyLimit: &BodyLimitConfig{
			Limit: "32M", // 32 MB request body limit
		},
		Gzip: &GzipConfig{
			Level: 6, // Good compression/speed balance
		},
		RateLimiter: &RateLimiterConfig{
			RequestsPerSecond: 20,
			Burst:             30,
			ExpiresIn:         10 * time.Minute,
		},
		Timeout: &TimeoutConfig{
			Timeout: 30 * time.Second,
		},
		RemoveTrailingSlash: &TrailingSlashConfig{
			RedirectCode: 301,
		},
		Secure: &SecureConfig{
			XSSProtection:         "1; mode=block",
			ContentTypeNosniff:    "nosniff",
			XFrameOptions:         "DENY",
			HSTSMaxAge:            31536000,
			HSTSExcludeSubdomains: false,
			HSTSPreloadEnabled:    true,
			ReferrerPolicy:        "strict-origin-when-cross-origin",
		},
	}
}

// SecurityConfig returns a MiddlewareConfig focused on security middleware
func SecurityConfig() MiddlewareConfig {
	return MiddlewareConfig{
		Recover: &RecoverConfig{
			StackSize: 4 << 10,
		},
		RequestID: &RequestIDConfig{
			TargetHeader: echo.HeaderXRequestID,
		},
		CSRF: &CSRFConfig{
			TokenLength:    32,
			TokenLookup:    "header:X-CSRF-Token",
			CookieSecure:   true,
			CookieHTTPOnly: true,
		},
		Secure: &SecureConfig{
			XSSProtection:         "1; mode=block",
			ContentTypeNosniff:    "nosniff",
			XFrameOptions:         "DENY",
			HSTSMaxAge:            31536000,
			HSTSPreloadEnabled:    true,
			ContentSecurityPolicy: "default-src 'self'",
		},
		RateLimiter: &RateLimiterConfig{
			RequestsPerSecond: 10,
			Burst:             15,
		},
	}
}

// CompressionConfig returns a MiddlewareConfig focused on compression and performance
func CompressionConfig() MiddlewareConfig {
	return MiddlewareConfig{
		Gzip: &GzipConfig{
			Level: 6,
		},
		Decompress: &DecompressConfig{},
		BodyLimit: &BodyLimitConfig{
			Limit: "10M",
		},
		RemoveTrailingSlash: &TrailingSlashConfig{
			RedirectCode: 301,
		},
	}
}

// DevelopmentConfig returns a MiddlewareConfig suitable for development
func DevelopmentConfig() MiddlewareConfig {
	return MiddlewareConfig{
		CORS: &CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders: []string{"*"},
		},
		Logger: &LoggerConfig{
			Format: `[${time_rfc3339_nano}] ${method} ${uri} (${status}) ${latency_human}` + "\n",
		},
		Recover: &RecoverConfig{
			StackSize: 8 << 10, // Larger stack for debugging
		},
		RequestID: &RequestIDConfig{
			TargetHeader: echo.HeaderXRequestID,
		},
	}
}

// ProxyGatewayConfig returns a MiddlewareConfig for proxy/gateway scenarios
func ProxyGatewayConfig() MiddlewareConfig {
	return MiddlewareConfig{
		Logger: &LoggerConfig{
			Format: `{"time":"${time_rfc3339}","method":"${method}","uri":"${uri}","status":${status},"latency_human":"${latency_human}","remote_ip":"${remote_ip}"}` + "\n",
		},
		Recover: &RecoverConfig{
			StackSize: 4 << 10,
		},
		RequestID: &RequestIDConfig{
			TargetHeader: echo.HeaderXRequestID,
		},
		RateLimiter: &RateLimiterConfig{
			RequestsPerSecond: 100,
			Burst:             150,
		},
		Timeout: &TimeoutConfig{
			Timeout: 60 * time.Second,
		},
	}
}

// LoadConfigFromYAML loads middleware configuration from a YAML file.
// This function reads and parses a YAML file containing middleware configuration,
// validates the parsed data, and returns a MiddlewareConfig struct.
//
// Example usage:
//
//	config, err := middleware.LoadConfigFromYAML("config/middleware.yaml")
//	if err != nil {
//	  log.Fatalf("Failed to load config: %v", err)
//	}
//
// Parameters:
//
//	filename - Path to the YAML configuration file
//
// Returns:
//
//	MiddlewareConfig - Parsed and validated configuration
//	error - Any error that occurred during loading or validation
func LoadConfigFromYAML(filename string) (MiddlewareConfig, error) {
	var config MiddlewareConfig

	// Read the YAML file
	data, err := os.ReadFile(filename)
	if err != nil {
		return config, fmt.Errorf("failed to read YAML file '%s': %w", filename, err)
	}

	// Parse YAML content
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to parse YAML file '%s': %w", filename, err)
	}

	// Validate the parsed configuration
	if err := ValidateConfig(config); err != nil {
		return config, fmt.Errorf("configuration validation failed for '%s': %w", filename, err)
	}

	return config, nil
}

// SaveConfigToYAML saves middleware configuration to a YAML file.
// This function serializes a MiddlewareConfig struct to YAML format
// and writes it to the specified file with proper formatting.
//
// Example usage:
//
//	config := middleware.DefaultConfig()
//	err := middleware.SaveConfigToYAML(config, "config/middleware.yaml")
//	if err != nil {
//	  log.Fatalf("Failed to save config: %v", err)
//	}
//
// Parameters:
//
//	config - The middleware configuration to save
//	filename - Path where the YAML file should be written
//
// Returns:
//
//	error - Any error that occurred during serialization or file writing
func SaveConfigToYAML(config MiddlewareConfig, filename string) error {
	// Serialize to YAML with proper formatting
	data, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write to file with readable permissions
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write YAML file '%s': %w", filename, err)
	}

	return nil
}

// ValidateConfig validates a middleware configuration for common issues.
// This function performs various validation checks to ensure the configuration
// is valid and can be safely used with Echo middleware.
//
// Example usage:
//
//	if err := middleware.ValidateConfig(config); err != nil {
//	  log.Fatalf("Invalid config: %v", err)
//	}
//
// Parameters:
//
//	config - The middleware configuration to validate
//
// Returns:
//
//	error - Description of validation errors, or nil if valid
func ValidateConfig(config MiddlewareConfig) error {
	// Validate CORS configuration
	if config.CORS != nil {
		if config.CORS.AllowCredentials {
			for _, origin := range config.CORS.AllowOrigins {
				if origin == "*" {
					return fmt.Errorf("CORS: allow_credentials cannot be true when allow_origins contains '*'")
				}
			}
		}
		if config.CORS.MaxAge < 0 {
			return fmt.Errorf("CORS: max_age cannot be negative")
		}
	}

	// Validate RateLimiter configuration
	if config.RateLimiter != nil {
		if config.RateLimiter.RequestsPerSecond <= 0 {
			return fmt.Errorf("RateLimiter: requests_per_second must be greater than 0")
		}
		if config.RateLimiter.Burst <= 0 {
			return fmt.Errorf("RateLimiter: burst must be greater than 0")
		}
	}

	// Validate Timeout configuration
	if config.Timeout != nil {
		if config.Timeout.Timeout <= 0 {
			return fmt.Errorf("timeout: timeout must be greater than 0")
		}
	}

	// Validate ContextTimeout configuration
	if config.ContextTimeout != nil {
		if config.ContextTimeout.Timeout <= 0 {
			return fmt.Errorf("ContextTimeout: timeout must be greater than 0")
		}
	}

	// Validate BodyLimit configuration
	if config.BodyLimit != nil {
		if config.BodyLimit.Limit == "" {
			return fmt.Errorf("BodyLimit: limit cannot be empty")
		}
		// Basic format validation for size strings
		if len(config.BodyLimit.Limit) < 2 {
			return fmt.Errorf("BodyLimit: limit format invalid (e.g., '10M', '1G')")
		}
	}

	// Validate Gzip configuration
	if config.Gzip != nil {
		if config.Gzip.Level < -1 || config.Gzip.Level > 9 {
			return fmt.Errorf("gzip: level must be between -1 and 9")
		}
	}

	// Validate Secure configuration
	if config.Secure != nil {
		if config.Secure.HSTSMaxAge < 0 {
			return fmt.Errorf("secure: hsts_max_age cannot be negative")
		}
	}

	// Validate CSRF configuration
	if config.CSRF != nil {
		if config.CSRF.TokenLength == 0 {
			return fmt.Errorf("CSRF: token_length must be greater than 0")
		}
		if config.CSRF.CookieMaxAge < 0 {
			return fmt.Errorf("CSRF: cookie_max_age cannot be negative")
		}
	}

	// Validate Proxy configuration
	if config.Proxy != nil {
		if len(config.Proxy.Targets) == 0 {
			return fmt.Errorf("proxy: at least one target must be specified")
		}
		for i, target := range config.Proxy.Targets {
			if target == nil {
				return fmt.Errorf("proxy: target %d cannot be nil", i)
			}
			if target.URL == "" {
				return fmt.Errorf("proxy: target %d URL cannot be empty", i)
			}
		}
	}

	return nil
}

// InterceptorConfig configures generic request/response interceptors with CEL-based processing.
// Interceptors provide flexible middleware capabilities using CEL expressions for conditional
// logic and request/response transformation.
type InterceptorConfig struct {
	// Name is a descriptive name for the interceptor (used in logs and debugging).
	// Examples: "api_auth", "admin_guard", "response_transformer"
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Regex pattern to match request paths for selective application.
	// Uses Go regexp syntax for flexible path matching.
	// Examples: "^/api/.*", "^/admin/.*", ".*\\.(jpg|png|gif)$"
	Regex string `json:"regex,omitempty" yaml:"regex,omitempty"`

	// Condition optional CEL expression for additional conditional logic.
	// Available variables: request (echo.Request), response (echo.Response), context (echo.Context)
	// Examples: 'request.method != "OPTIONS"', 'request.header.get("X-API-Key") != ""'
	// If empty, interceptor always applies to matching paths
	Condition string `json:"condition,omitempty" yaml:"condition,omitempty"`

	// Request array of CEL expressions for request processing.
	// First non-null return value is used for early response or request modification.
	// Available variables: request, context, headers (map), body (string)
	// Return format: {status: 401, headers: {...}, body: "..."}
	// Examples:
	//   - 'headers.get("Authorization") == "" ? {status: 401, body: "Unauthorized"} : null'
	//   - '{headers: {"X-Processed": "true"}}'
	Request []string `json:"request,omitempty" yaml:"request,omitempty"`

	// Response array of CEL expressions for response processing.
	// First non-null return value is used for response transformation.
	// Available variables: request, response, context, headers (map), body (string)
	// Return format: {headers: {...}, body: "...", status: 200}
	// Examples:
	//   - 'headers.get("Content-Type").contains("json") ? {headers: {"Cache-Control": "no-cache"}} : null'
	//   - 'response.status >= 400 ? null : {headers: {"X-Success": "true"}}'
	Response []string `json:"response,omitempty" yaml:"response,omitempty"`
}
