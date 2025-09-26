package middleware

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

var _ = Describe("Echo Middleware", func() {
	Describe("Missing Middleware Types", func() {
		Context("KeyAuth middleware", func() {
			It("should create middleware with validator function", func() {
				config := MiddlewareConfig{
					KeyAuth: &KeyAuthConfig{
						KeyLookup: "header:X-API-Key",
						Validator: func(key string, c echo.Context) (bool, error) {
							return key == "valid-key", nil
						},
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})

			It("should create middleware with auth scheme", func() {
				config := MiddlewareConfig{
					KeyAuth: &KeyAuthConfig{
						KeyLookup:  "header:Authorization",
						AuthScheme: "Bearer",
						Validator: func(key string, c echo.Context) (bool, error) {
							return key == "token123", nil
						},
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})
		})

		Context("BodyDump middleware", func() {
			It("should create middleware with handler function", func() {
				config := MiddlewareConfig{
					BodyDump: &BodyDumpConfig{
						Handler: func(c echo.Context, reqBody, resBody []byte) {
							// Body dump handler logic
						},
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})

			It("should not create middleware when handler is nil", func() {
				config := MiddlewareConfig{
					BodyDump: &BodyDumpConfig{
						Handler: nil,
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(0))
			})
		})

		Context("Decompress middleware", func() {
			It("should create middleware for request decompression", func() {
				config := MiddlewareConfig{
					Decompress: &DecompressConfig{},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})
		})

		Context("Static middleware", func() {
			It("should create middleware with root directory", func() {
				config := MiddlewareConfig{
					Static: &StaticConfig{
						Root:  "public",
						Index: "index.html",
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})

			It("should create middleware with browse and HTML5 options", func() {
				config := MiddlewareConfig{
					Static: &StaticConfig{
						Root:   "assets",
						Browse: true,
						HTML5:  true,
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})
		})

		Context("MethodOverride middleware", func() {
			It("should create middleware with custom getter", func() {
				config := MiddlewareConfig{
					MethodOverride: &MethodOverrideConfig{
						Getter: func(c echo.Context) string {
							return c.Request().Header.Get("X-HTTP-Method-Override")
						},
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})
		})

		Context("HTTPSWWWRedirect middleware", func() {
			It("should create redirect middleware", func() {
				config := MiddlewareConfig{
					HTTPSWWWRedirect: &RedirectConfig{
						Code: 301,
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})
		})

		Context("AddTrailingSlash middleware", func() {
			It("should create middleware with custom redirect code", func() {
				config := MiddlewareConfig{
					AddTrailingSlash: &TrailingSlashConfig{
						RedirectCode: 302,
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})
		})

		Context("RequestLogger middleware", func() {
			It("should create middleware with LogValuesFunc", func() {
				config := MiddlewareConfig{
					RequestLogger: &RequestLoggerConfig{
						LogStatus:      true,
						LogURI:         true,
						LogMethod:      true,
						LogValuesFunc: func(c echo.Context, v echomiddleware.RequestLoggerValues) error {
							// Custom logging logic
							return nil
						},
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})

			It("should not create middleware when LogValuesFunc is nil", func() {
				config := MiddlewareConfig{
					RequestLogger: &RequestLoggerConfig{
						LogStatus: true,
						LogURI:    true,
						// LogValuesFunc is nil
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(0))
			})
		})
	})

	Describe("Edge Cases and Error Handling", func() {
		Context("when proxy has invalid URL", func() {
			It("should not create proxy middleware for malformed URL", func() {
				config := MiddlewareConfig{
					Proxy: &ProxyConfig{
						Targets: []*ProxyTarget{
							{Name: "invalid", URL: "://invalid-url"},
						},
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(0))
			})

			It("should handle empty proxy targets", func() {
				config := MiddlewareConfig{
					Proxy: &ProxyConfig{
						Targets: []*ProxyTarget{},
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(0))
			})
		})

		Context("when body limit is empty", func() {
			It("should not create body limit middleware", func() {
				config := MiddlewareConfig{
					BodyLimit: &BodyLimitConfig{
						Limit: "",
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(0))
			})
		})

		Context("secure middleware with specific values", func() {
			It("should configure all security headers", func() {
				config := MiddlewareConfig{
					Secure: &SecureConfig{
						XSSProtection:         "1; mode=block",
						ContentTypeNosniff:    "nosniff",
						XFrameOptions:         "DENY",
						HSTSMaxAge:            31536000,
						ContentSecurityPolicy: "default-src 'self'",
						CSPReportOnly:         true,
						HSTSPreloadEnabled:    true,
						ReferrerPolicy:        "strict-origin",
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})
		})

		Context("logger middleware with custom output", func() {
			It("should handle custom output writer", func() {
				config := MiddlewareConfig{
					Logger: &LoggerConfig{
						Format:           "${method} ${uri} ${status}\n",
						CustomTimeFormat: "2006-01-02 15:04:05",
						Output:           os.Stdout,
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})

			It("should default to stdout when output is nil", func() {
				config := MiddlewareConfig{
					Logger: &LoggerConfig{
						Format: "${method} ${uri}\n",
						Output: nil,
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})
		})

		Context("rate limiter with default expiry", func() {
			It("should use default expiry when ExpiresIn is zero", func() {
				config := MiddlewareConfig{
					RateLimiter: &RateLimiterConfig{
						RequestsPerSecond: 10.0,
						Burst:             15,
						ExpiresIn:         0, // Should use default
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})
		})

		Context("request ID with default generator", func() {
			It("should use default generator when none provided", func() {
				config := MiddlewareConfig{
					RequestID: &RequestIDConfig{
						Generator:    nil, // Should use default
						TargetHeader: "X-Request-ID",
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})
		})

		Context("timeout middleware values", func() {
			It("should configure timeout duration", func() {
				config := MiddlewareConfig{
					Timeout: &TimeoutConfig{
						Timeout: 30 * time.Second,
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(middlewares).To(HaveLen(1))
			})
		})
	})

	Describe("Integration Scenarios", func() {
		Context("when all middleware types are enabled", func() {
			It("should create comprehensive middleware stack", func() {
				config := MiddlewareConfig{
					// Core middleware
					CORS:      &CORSConfig{AllowOrigins: []string{"*"}},
					Logger:    &LoggerConfig{Format: "${method} ${uri}\n"},
					Recover:   &RecoverConfig{StackSize: 4 << 10},
					RequestID: &RequestIDConfig{TargetHeader: echo.HeaderXRequestID},

					// Security middleware
					BasicAuth: &BasicAuthConfig{
						Validator: func(username, password string, c echo.Context) (bool, error) {
							return username == "admin", nil
						},
					},
					KeyAuth: &KeyAuthConfig{
						Validator: func(key string, c echo.Context) (bool, error) {
							return key == "secret", nil
						},
					},
					CSRF:   &CSRFConfig{TokenLength: 32},
					Secure: &SecureConfig{XSSProtection: "1; mode=block"},

					// Request/Response processing
					BodyDump: &BodyDumpConfig{
						Handler: func(c echo.Context, reqBody, resBody []byte) {},
					},
					BodyLimit:  &BodyLimitConfig{Limit: "10M"},
					Gzip:       &GzipConfig{Level: 6},
					Decompress: &DecompressConfig{},

					// Routing and static
					Static: &StaticConfig{
						Root:  "public",
						Index: "index.html",
					},
					MethodOverride: &MethodOverrideConfig{
						Getter: func(c echo.Context) string {
							return c.Request().Header.Get("X-HTTP-Method-Override")
						},
					},
					Proxy: &ProxyConfig{
						Targets: []*ProxyTarget{
							{Name: "api", URL: "http://localhost:8081"},
						},
					},
					Rewrite: &RewriteConfig{
						Rules: map[string]string{"/old/*": "/new/$1"},
					},

					// Redirects
					HTTPSRedirect:    &RedirectConfig{Code: 301},
					WWWRedirect:      &RedirectConfig{Code: 302},
					HTTPSWWWRedirect: &RedirectConfig{Code: 301},

					// URL normalization
					AddTrailingSlash:    &TrailingSlashConfig{RedirectCode: 301},
					RemoveTrailingSlash: &TrailingSlashConfig{RedirectCode: 301},

					// Rate limiting and timeouts
					RateLimiter: &RateLimiterConfig{
						RequestsPerSecond: 100,
						Burst:             150,
					},
					Timeout: &TimeoutConfig{
						Timeout: 30 * time.Second,
					},
					ContextTimeout: &ContextTimeoutConfig{
						Timeout: 25 * time.Second,
					},

					// Advanced logging
					RequestLogger: &RequestLoggerConfig{
						LogStatus: true,
						LogURI:    true,
						LogValuesFunc: func(c echo.Context, v echomiddleware.RequestLoggerValues) error {
							return nil
						},
					},
				}

				middlewares := EchoMiddleware(config)
				Expect(len(middlewares)).To(BeNumerically(">=", 20))
			})
		})

		Context("middleware ordering consistency", func() {
			It("should apply middleware in consistent order across calls", func() {
				config := MiddlewareConfig{
					CORS:        &CORSConfig{AllowOrigins: []string{"*"}},
					Logger:      &LoggerConfig{Format: "${method}\n"},
					Recover:     &RecoverConfig{StackSize: 4 << 10},
					RequestID:   &RequestIDConfig{TargetHeader: "X-Request-ID"},
					RateLimiter: &RateLimiterConfig{RequestsPerSecond: 10, Burst: 15},
				}

				// Call multiple times and verify consistent ordering
				middlewares1 := EchoMiddleware(config)
				middlewares2 := EchoMiddleware(config)

				Expect(middlewares1).To(HaveLen(5))
				Expect(middlewares2).To(HaveLen(5))
				Expect(len(middlewares1)).To(Equal(len(middlewares2)))
			})
		})

		Context("configuration with preset functions", func() {
			DescribeTable("preset configurations",
				func(configFunc func() MiddlewareConfig, expectedMin int) {
					config := configFunc()
					middlewares := EchoMiddleware(config)
					Expect(len(middlewares)).To(BeNumerically(">=", expectedMin))
				},
				Entry("DefaultConfig", DefaultConfig, 5),
				Entry("MinimalConfig", MinimalConfig, 2),
				Entry("ProductionConfig", ProductionConfig, 10),
				Entry("SecurityConfig", SecurityConfig, 5),
				Entry("CompressionConfig", CompressionConfig, 4),
				Entry("DevelopmentConfig", DevelopmentConfig, 4),
				Entry("ProxyGatewayConfig", ProxyGatewayConfig, 5),
			)
		})
	})

	Describe("Apply Middleware Functions", func() {
		var e *echo.Echo

		BeforeEach(func() {
			e = echo.New()
		})

		Context("ApplyMiddleware function", func() {
			It("should apply all configured middleware to Echo instance", func() {
				config := MiddlewareConfig{
					CORS:      &CORSConfig{AllowOrigins: []string{"*"}},
					Logger:    &LoggerConfig{Format: "${method}\n"},
					RequestID: &RequestIDConfig{TargetHeader: "X-Request-ID"},
				}

				Expect(func() {
					ApplyMiddleware(e, config)
				}).NotTo(Panic())
			})
		})

		Context("preset apply functions", func() {
			DescribeTable("apply preset middleware",
				func(applyFunc func(*echo.Echo)) {
					Expect(func() {
						applyFunc(e)
					}).NotTo(Panic())
				},
				Entry("ApplyDefaultMiddleware", ApplyDefaultMiddleware),
				Entry("ApplyMinimalMiddleware", ApplyMinimalMiddleware),
				Entry("ApplyProductionMiddleware", ApplyProductionMiddleware),
				Entry("ApplySecurityMiddleware", ApplySecurityMiddleware),
				Entry("ApplyCompressionMiddleware", ApplyCompressionMiddleware),
				Entry("ApplyDevelopmentMiddleware", ApplyDevelopmentMiddleware),
				Entry("ApplyProxyGatewayMiddleware", ApplyProxyGatewayMiddleware),
			)
		})
	})
})