package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

// EchoMiddleware configures and returns Echo middleware based on the provided configuration
func EchoMiddleware(config MiddlewareConfig) []echo.MiddlewareFunc {
	var middlewares []echo.MiddlewareFunc

	// Apply CORS middleware
	if config.CORS != nil {
		corsConfig := middleware.CORSConfig{}

		if len(config.CORS.AllowOrigins) > 0 {
			corsConfig.AllowOrigins = config.CORS.AllowOrigins
		}
		if len(config.CORS.AllowMethods) > 0 {
			corsConfig.AllowMethods = config.CORS.AllowMethods
		}
		if len(config.CORS.AllowHeaders) > 0 {
			corsConfig.AllowHeaders = config.CORS.AllowHeaders
		}
		if len(config.CORS.ExposeHeaders) > 0 {
			corsConfig.ExposeHeaders = config.CORS.ExposeHeaders
		}
		if config.CORS.MaxAge > 0 {
			corsConfig.MaxAge = config.CORS.MaxAge
		}
		corsConfig.AllowCredentials = config.CORS.AllowCredentials

		middlewares = append(middlewares, middleware.CORSWithConfig(corsConfig))
	}

	// Apply Logger middleware
	if config.Logger != nil {
		loggerConfig := middleware.LoggerConfig{}

		if config.Logger.Format != "" {
			loggerConfig.Format = config.Logger.Format
		}
		if config.Logger.CustomTimeFormat != "" {
			loggerConfig.CustomTimeFormat = config.Logger.CustomTimeFormat
		}
		if config.Logger.Output != nil {
			if writer, ok := config.Logger.Output.(io.Writer); ok {
				loggerConfig.Output = writer
			}
		} else {
			loggerConfig.Output = os.Stdout
		}
		// Note: Template field is not available in Echo v4 logger config

		// config.Logger preserves echo's format-string request logging, which the
		// non-deprecated RequestLogger middleware (config.RequestLogger) does not offer.
		middlewares = append(middlewares, middleware.LoggerWithConfig(loggerConfig)) //nolint:staticcheck // deprecated echo API retained for format-string logging
	}

	// Apply Recover middleware
	if config.Recover != nil {
		recoverConfig := middleware.RecoverConfig{}

		if config.Recover.StackSize > 0 {
			recoverConfig.StackSize = config.Recover.StackSize
		}
		recoverConfig.DisableStackAll = config.Recover.DisableStackAll
		recoverConfig.DisablePrintStack = config.Recover.DisablePrintStack
		recoverConfig.DisableErrorHandler = config.Recover.DisableErrorHandler

		middlewares = append(middlewares, middleware.RecoverWithConfig(recoverConfig))
	}

	// Apply RequestID middleware
	if config.RequestID != nil {
		requestIDConfig := middleware.RequestIDConfig{}

		if config.RequestID.Generator != nil {
			requestIDConfig.Generator = config.RequestID.Generator
		} else {
			// Default generator for random 32-character string
			requestIDConfig.Generator = func() string {
				b := make([]byte, 16)
				if _, err := rand.Read(b); err != nil {
					// Fallback to timestamp-based ID if random fails
					return fmt.Sprintf("%d", time.Now().UnixNano())
				}
				return hex.EncodeToString(b)
			}
		}

		if config.RequestID.TargetHeader != "" {
			requestIDConfig.TargetHeader = config.RequestID.TargetHeader
		}

		if config.RequestID.Handler != nil {
			requestIDConfig.RequestIDHandler = config.RequestID.Handler
		}

		middlewares = append(middlewares, middleware.RequestIDWithConfig(requestIDConfig))
	}

	// Apply RateLimiter middleware
	if config.RateLimiter != nil {
		rateLimiterConfig := middleware.RateLimiterConfig{}

		// Create memory store
		storeConfig := middleware.RateLimiterMemoryStoreConfig{
			Rate:      rate.Limit(config.RateLimiter.RequestsPerSecond),
			Burst:     config.RateLimiter.Burst,
			ExpiresIn: config.RateLimiter.ExpiresIn,
		}

		if config.RateLimiter.ExpiresIn == 0 {
			storeConfig.ExpiresIn = middleware.DefaultRateLimiterMemoryStoreConfig.ExpiresIn
		}

		rateLimiterConfig.Store = middleware.NewRateLimiterMemoryStoreWithConfig(storeConfig)

		middlewares = append(middlewares, middleware.RateLimiterWithConfig(rateLimiterConfig))
	}

	// Apply Timeout middleware
	if config.Timeout != nil {
		// config.Timeout uses echo's Timeout middleware (hard deadline → 503); the
		// non-deprecated ContextTimeout (config.ContextTimeout) has different,
		// context-cancellation semantics, so it is not a drop-in replacement.
		timeoutConfig := middleware.TimeoutConfig{ //nolint:staticcheck // deprecated echo API retained for hard-timeout semantics
			Timeout: config.Timeout.Timeout,
		}

		middlewares = append(middlewares, middleware.TimeoutWithConfig(timeoutConfig)) //nolint:staticcheck // see TimeoutConfig above
	}

	// Apply Secure middleware
	if config.Secure != nil {
		secureConfig := middleware.SecureConfig{}

		if config.Secure.XSSProtection != "" {
			secureConfig.XSSProtection = config.Secure.XSSProtection
		}
		if config.Secure.ContentTypeNosniff != "" {
			secureConfig.ContentTypeNosniff = config.Secure.ContentTypeNosniff
		}
		if config.Secure.XFrameOptions != "" {
			secureConfig.XFrameOptions = config.Secure.XFrameOptions
		}
		if config.Secure.HSTSMaxAge > 0 {
			secureConfig.HSTSMaxAge = config.Secure.HSTSMaxAge
		}
		secureConfig.HSTSExcludeSubdomains = config.Secure.HSTSExcludeSubdomains
		if config.Secure.ContentSecurityPolicy != "" {
			secureConfig.ContentSecurityPolicy = config.Secure.ContentSecurityPolicy
		}
		secureConfig.CSPReportOnly = config.Secure.CSPReportOnly
		secureConfig.HSTSPreloadEnabled = config.Secure.HSTSPreloadEnabled
		if config.Secure.ReferrerPolicy != "" {
			secureConfig.ReferrerPolicy = config.Secure.ReferrerPolicy
		}

		middlewares = append(middlewares, middleware.SecureWithConfig(secureConfig))
	}

	// Apply BasicAuth middleware
	if config.BasicAuth != nil {
		basicAuthConfig := middleware.BasicAuthConfig{}
		if config.BasicAuth.Validator != nil {
			basicAuthConfig.Validator = config.BasicAuth.Validator
		}
		if config.BasicAuth.Realm != "" {
			basicAuthConfig.Realm = config.BasicAuth.Realm
		}
		middlewares = append(middlewares, middleware.BasicAuthWithConfig(basicAuthConfig))
	}

	// Apply KeyAuth middleware
	if config.KeyAuth != nil {
		keyAuthConfig := middleware.KeyAuthConfig{}
		if config.KeyAuth.KeyLookup != "" {
			keyAuthConfig.KeyLookup = config.KeyAuth.KeyLookup
		}
		if config.KeyAuth.AuthScheme != "" {
			keyAuthConfig.AuthScheme = config.KeyAuth.AuthScheme
		}
		if config.KeyAuth.Validator != nil {
			keyAuthConfig.Validator = config.KeyAuth.Validator
		}
		if config.KeyAuth.ErrorHandler != nil {
			keyAuthConfig.ErrorHandler = config.KeyAuth.ErrorHandler
		}
		middlewares = append(middlewares, middleware.KeyAuthWithConfig(keyAuthConfig))
	}

	// Apply CSRF middleware
	if config.CSRF != nil {
		csrfConfig := middleware.CSRFConfig{}
		if config.CSRF.TokenLength > 0 {
			csrfConfig.TokenLength = config.CSRF.TokenLength
		}
		if config.CSRF.TokenLookup != "" {
			csrfConfig.TokenLookup = config.CSRF.TokenLookup
		}
		if config.CSRF.ContextKey != "" {
			csrfConfig.ContextKey = config.CSRF.ContextKey
		}
		if config.CSRF.CookieName != "" {
			csrfConfig.CookieName = config.CSRF.CookieName
		}
		if config.CSRF.CookieDomain != "" {
			csrfConfig.CookieDomain = config.CSRF.CookieDomain
		}
		if config.CSRF.CookiePath != "" {
			csrfConfig.CookiePath = config.CSRF.CookiePath
		}
		if config.CSRF.CookieMaxAge > 0 {
			csrfConfig.CookieMaxAge = config.CSRF.CookieMaxAge
		}
		csrfConfig.CookieSecure = config.CSRF.CookieSecure
		csrfConfig.CookieHTTPOnly = config.CSRF.CookieHTTPOnly
		middlewares = append(middlewares, middleware.CSRFWithConfig(csrfConfig))
	}

	// Apply BodyDump middleware
	if config.BodyDump != nil && config.BodyDump.Handler != nil {
		bodyDumpConfig := middleware.BodyDumpConfig{
			Handler: config.BodyDump.Handler,
		}
		middlewares = append(middlewares, middleware.BodyDumpWithConfig(bodyDumpConfig))
	}

	// Apply BodyLimit middleware
	if config.BodyLimit != nil && config.BodyLimit.Limit != "" {
		middlewares = append(middlewares, middleware.BodyLimit(config.BodyLimit.Limit))
	}

	// Apply Gzip middleware
	if config.Gzip != nil {
		gzipConfig := middleware.GzipConfig{}
		if config.Gzip.Level != 0 {
			gzipConfig.Level = config.Gzip.Level
		}
		middlewares = append(middlewares, middleware.GzipWithConfig(gzipConfig))
	}

	// Apply Decompress middleware
	if config.Decompress != nil {
		middlewares = append(middlewares, middleware.Decompress())
	}

	// Apply Static middleware
	if config.Static != nil {
		staticConfig := middleware.StaticConfig{}
		if config.Static.Root != "" {
			staticConfig.Root = config.Static.Root
		}
		if config.Static.Index != "" {
			staticConfig.Index = config.Static.Index
		}
		staticConfig.Browse = config.Static.Browse
		staticConfig.HTML5 = config.Static.HTML5
		staticConfig.IgnoreBase = config.Static.IgnoreBase
		if config.Static.Filesystem != nil {
			staticConfig.Filesystem = config.Static.Filesystem
		}
		middlewares = append(middlewares, middleware.StaticWithConfig(staticConfig))
	}

	// Apply MethodOverride middleware
	if config.MethodOverride != nil {
		methodOverrideConfig := middleware.MethodOverrideConfig{}
		if config.MethodOverride.Getter != nil {
			methodOverrideConfig.Getter = config.MethodOverride.Getter
		}
		middlewares = append(middlewares, middleware.MethodOverrideWithConfig(methodOverrideConfig))
	}

	// Apply Proxy middleware
	if config.Proxy != nil && len(config.Proxy.Targets) > 0 {
		// For now, use the basic proxy with first target
		// Complex proxy configuration would require more setup
		if len(config.Proxy.Targets) > 0 && config.Proxy.Targets[0] != nil && config.Proxy.Targets[0].URL != "" {
			if targetURL, err := url.Parse(config.Proxy.Targets[0].URL); err == nil {
				middlewares = append(middlewares, middleware.Proxy(middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
					{URL: targetURL},
				})))
			}
		}
	}

	// Apply Rewrite middleware
	if config.Rewrite != nil && len(config.Rewrite.Rules) > 0 {
		rewriteConfig := middleware.RewriteConfig{
			Rules: config.Rewrite.Rules,
		}
		middlewares = append(middlewares, middleware.RewriteWithConfig(rewriteConfig))
	}

	// Apply redirect middleware
	if config.HTTPSRedirect != nil {
		redirectConfig := middleware.RedirectConfig{}
		if config.HTTPSRedirect.Code > 0 {
			redirectConfig.Code = config.HTTPSRedirect.Code
		}
		middlewares = append(middlewares, middleware.HTTPSRedirectWithConfig(redirectConfig))
	}

	if config.WWWRedirect != nil {
		redirectConfig := middleware.RedirectConfig{}
		if config.WWWRedirect.Code > 0 {
			redirectConfig.Code = config.WWWRedirect.Code
		}
		middlewares = append(middlewares, middleware.WWWRedirectWithConfig(redirectConfig))
	}

	if config.HTTPSWWWRedirect != nil {
		redirectConfig := middleware.RedirectConfig{}
		if config.HTTPSWWWRedirect.Code > 0 {
			redirectConfig.Code = config.HTTPSWWWRedirect.Code
		}
		middlewares = append(middlewares, middleware.HTTPSWWWRedirectWithConfig(redirectConfig))
	}

	// Apply trailing slash middleware
	if config.AddTrailingSlash != nil {
		trailingSlashConfig := middleware.TrailingSlashConfig{}
		if config.AddTrailingSlash.RedirectCode > 0 {
			trailingSlashConfig.RedirectCode = config.AddTrailingSlash.RedirectCode
		}
		middlewares = append(middlewares, middleware.AddTrailingSlashWithConfig(trailingSlashConfig))
	}

	if config.RemoveTrailingSlash != nil {
		trailingSlashConfig := middleware.TrailingSlashConfig{}
		if config.RemoveTrailingSlash.RedirectCode > 0 {
			trailingSlashConfig.RedirectCode = config.RemoveTrailingSlash.RedirectCode
		}
		middlewares = append(middlewares, middleware.RemoveTrailingSlashWithConfig(trailingSlashConfig))
	}

	// Apply ContextTimeout middleware
	if config.ContextTimeout != nil {
		contextTimeoutConfig := middleware.ContextTimeoutConfig{
			Timeout: config.ContextTimeout.Timeout,
		}
		if config.ContextTimeout.ErrorHandler != nil {
			contextTimeoutConfig.ErrorHandler = config.ContextTimeout.ErrorHandler
		}
		middlewares = append(middlewares, middleware.ContextTimeoutWithConfig(contextTimeoutConfig))
	}

	// Apply RequestLogger middleware
	if config.RequestLogger != nil && config.RequestLogger.LogValuesFunc != nil {
		requestLoggerConfig := middleware.RequestLoggerConfig{}
		requestLoggerConfig.LogStatus = config.RequestLogger.LogStatus
		requestLoggerConfig.LogURI = config.RequestLogger.LogURI
		requestLoggerConfig.LogError = config.RequestLogger.LogError
		requestLoggerConfig.LogLatency = config.RequestLogger.LogLatency
		requestLoggerConfig.LogProtocol = config.RequestLogger.LogProtocol
		requestLoggerConfig.LogRemoteIP = config.RequestLogger.LogRemoteIP
		requestLoggerConfig.LogHost = config.RequestLogger.LogHost
		requestLoggerConfig.LogMethod = config.RequestLogger.LogMethod
		requestLoggerConfig.LogUserAgent = config.RequestLogger.LogUserAgent
		requestLoggerConfig.LogRequestID = config.RequestLogger.LogRequestID
		requestLoggerConfig.LogReferer = config.RequestLogger.LogReferer
		requestLoggerConfig.LogContentLength = config.RequestLogger.LogContentLength
		requestLoggerConfig.LogResponseSize = config.RequestLogger.LogResponseSize
		requestLoggerConfig.HandleError = config.RequestLogger.HandleError
		requestLoggerConfig.LogValuesFunc = config.RequestLogger.LogValuesFunc
		middlewares = append(middlewares, middleware.RequestLoggerWithConfig(requestLoggerConfig))
	}

	return middlewares
}

// ApplyMiddleware applies the configured middleware to an Echo instance
func ApplyMiddleware(e *echo.Echo, config MiddlewareConfig) {
	middlewares := EchoMiddleware(config)
	for _, mw := range middlewares {
		e.Use(mw)
	}
}

// ApplyDefaultMiddleware applies default middleware configuration to an Echo instance
func ApplyDefaultMiddleware(e *echo.Echo) {
	ApplyMiddleware(e, DefaultConfig())
}

// ApplyMinimalMiddleware applies minimal middleware configuration to an Echo instance
func ApplyMinimalMiddleware(e *echo.Echo) {
	ApplyMiddleware(e, MinimalConfig())
}

// ApplyProductionMiddleware applies production-ready middleware configuration to an Echo instance
func ApplyProductionMiddleware(e *echo.Echo) {
	ApplyMiddleware(e, ProductionConfig())
}

// ApplySecurityMiddleware applies security-focused middleware configuration to an Echo instance
func ApplySecurityMiddleware(e *echo.Echo) {
	ApplyMiddleware(e, SecurityConfig())
}

// ApplyCompressionMiddleware applies compression-focused middleware configuration to an Echo instance
func ApplyCompressionMiddleware(e *echo.Echo) {
	ApplyMiddleware(e, CompressionConfig())
}

// ApplyDevelopmentMiddleware applies development-friendly middleware configuration to an Echo instance
func ApplyDevelopmentMiddleware(e *echo.Echo) {
	ApplyMiddleware(e, DevelopmentConfig())
}

// ApplyProxyGatewayMiddleware applies proxy/gateway middleware configuration to an Echo instance
func ApplyProxyGatewayMiddleware(e *echo.Echo) {
	ApplyMiddleware(e, ProxyGatewayConfig())
}
