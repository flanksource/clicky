package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/labstack/echo/v4"
)

// InterceptorResult represents the result of a CEL expression evaluation
type InterceptorResult struct {
	Status  int                    `json:"status,omitempty"`
	Headers map[string]string      `json:"headers,omitempty"`
	Body    string                 `json:"body,omitempty"`
	HTML    string                 `json:"html,omitempty"`
	JSON    map[string]interface{} `json:"json,omitempty"`
}

// InterceptorMiddleware creates generic request/response interceptor middleware
func InterceptorMiddleware(interceptors []*InterceptorConfig, celEngine *CELEngine) echo.MiddlewareFunc {
	if len(interceptors) == 0 || celEngine == nil {
		// Return no-op middleware if no interceptors configured
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	// Compile regex patterns
	compiledInterceptors := make([]*compiledInterceptor, 0, len(interceptors))
	for _, config := range interceptors {
		compiled, err := compileInterceptor(config)
		if err != nil {
			panic(fmt.Sprintf("Failed to compile interceptor '%s': %v", config.Name, err))
		}
		compiledInterceptors = append(compiledInterceptors, compiled)
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path

			// Process request interceptors
			for _, interceptor := range compiledInterceptors {
				if interceptor.matches(path) {
					result, err := interceptor.processRequest(c, celEngine)
					if err != nil {
						return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Interceptor error: %v", err))
					}

					if result != nil {
						// Early return with custom response
						return sendInterceptorResponse(c, result)
					}
				}
			}

			// Create response recorder to capture response for processing
			recorder := &responseRecorder{
				ResponseWriter: c.Response().Writer,
				statusCode:     http.StatusOK,
				body:           &bytes.Buffer{},
			}
			c.Response().Writer = recorder

			// Call next handler
			err := next(c)
			if err != nil {
				return err
			}

			// Process response interceptors
			for _, interceptor := range compiledInterceptors {
				if interceptor.matches(path) {
					result, interceptErr := interceptor.processResponse(c, celEngine, recorder)
					if interceptErr != nil {
						// Log error but don't fail the request
						c.Logger().Errorf("Response interceptor error: %v", interceptErr)
						continue
					}

					if result != nil {
						// Apply response transformation
						return applyResponseTransformation(c, result, recorder)
					}
				}
			}

			// Write original response if no transformations applied
			return writeOriginalResponse(c, recorder)
		}
	}
}

// compiledInterceptor holds the compiled version of an interceptor config
type compiledInterceptor struct {
	name      string
	regex     *regexp.Regexp
	condition string
	request   []string
	response  []string
}

// compileInterceptor compiles an interceptor configuration
func compileInterceptor(config *InterceptorConfig) (*compiledInterceptor, error) {
	compiled := &compiledInterceptor{
		name:      config.Name,
		condition: config.Condition,
		request:   config.Request,
		response:  config.Response,
	}

	// Compile regex pattern
	if config.Regex != "" {
		regex, err := regexp.Compile(config.Regex)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern '%s': %w", config.Regex, err)
		}
		compiled.regex = regex
	}

	return compiled, nil
}

// matches checks if the interceptor should be applied to the given path
func (ci *compiledInterceptor) matches(path string) bool {
	if ci.regex == nil {
		return true // No regex means match all paths
	}
	return ci.regex.MatchString(path)
}

// processRequest processes request interceptors and returns first non-null result
func (ci *compiledInterceptor) processRequest(c echo.Context, celEngine *CELEngine) (*InterceptorResult, error) {
	// Check condition first
	if ci.condition != "" {
		variables := CreateRequestVariables(c)
		valid, err := celEngine.EvaluateCondition(ci.condition, variables)
		if err != nil {
			return nil, fmt.Errorf("condition evaluation error: %w", err)
		}
		if !valid {
			return nil, nil // Skip this interceptor
		}
	}

	// Process request expressions
	if len(ci.request) > 0 {
		variables := CreateRequestVariables(c)
		result, err := celEngine.EvaluateFirstNonNull(ci.request, variables)
		if err != nil {
			return nil, fmt.Errorf("request expression evaluation error: %w", err)
		}

		if result != nil {
			return parseInterceptorResult(result)
		}
	}

	return nil, nil
}

// processResponse processes response interceptors and returns first non-null result
func (ci *compiledInterceptor) processResponse(c echo.Context, celEngine *CELEngine, recorder *responseRecorder) (*InterceptorResult, error) {
	// Check condition first
	if ci.condition != "" {
		variables := CreateResponseVariables(c)
		valid, err := celEngine.EvaluateCondition(ci.condition, variables)
		if err != nil {
			return nil, fmt.Errorf("condition evaluation error: %w", err)
		}
		if !valid {
			return nil, nil // Skip this interceptor
		}
	}

	// Process response expressions
	if len(ci.response) > 0 {
		variables := CreateResponseVariables(c)
		// Add response body to variables
		variables["body"] = recorder.body.String()
		variables["response"].(map[string]interface{})["body"] = recorder.body.String()

		result, err := celEngine.EvaluateFirstNonNull(ci.response, variables)
		if err != nil {
			return nil, fmt.Errorf("response expression evaluation error: %w", err)
		}

		if result != nil {
			return parseInterceptorResult(result)
		}
	}

	return nil, nil
}

// parseInterceptorResult parses a CEL result into an InterceptorResult
func parseInterceptorResult(result interface{}) (*InterceptorResult, error) {
	if result == nil {
		return nil, nil
	}

	// Handle different result types
	switch r := result.(type) {
	case map[string]interface{}:
		interceptorResult := &InterceptorResult{}

		// Parse status
		if status, ok := r["status"]; ok {
			if statusInt, err := convertToInt(status); err == nil {
				interceptorResult.Status = statusInt
			}
		}

		// Parse headers
		if headers, ok := r["headers"]; ok {
			if headersMap, ok := headers.(map[string]interface{}); ok {
				interceptorResult.Headers = make(map[string]string)
				for k, v := range headersMap {
					interceptorResult.Headers[k] = fmt.Sprintf("%v", v)
				}
			}
		}

		// Parse body
		if body, ok := r["body"]; ok {
			interceptorResult.Body = fmt.Sprintf("%v", body)
		}

		// Parse HTML
		if html, ok := r["html"]; ok {
			interceptorResult.HTML = fmt.Sprintf("%v", html)
		}

		// Parse JSON
		if jsonData, ok := r["json"]; ok {
			if jsonMap, ok := jsonData.(map[string]interface{}); ok {
				interceptorResult.JSON = jsonMap
			}
		}

		return interceptorResult, nil

	case string:
		// Simple string result becomes body
		return &InterceptorResult{Body: r}, nil

	case int, int64:
		// Simple int result becomes status
		if statusInt, err := convertToInt(r); err == nil {
			return &InterceptorResult{Status: statusInt}, nil
		}
	}

	return nil, fmt.Errorf("unsupported result type: %T", result)
}

// convertToInt converts various number types to int
func convertToInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

// sendInterceptorResponse sends a response based on interceptor result
func sendInterceptorResponse(c echo.Context, result *InterceptorResult) error {
	// Set status code
	status := result.Status
	if status == 0 {
		status = http.StatusOK
	}

	// Set headers
	for key, value := range result.Headers {
		c.Response().Header().Set(key, value)
	}

	// Send response based on content type
	if result.JSON != nil {
		return c.JSON(status, result.JSON)
	} else if result.HTML != "" {
		c.Response().Header().Set("Content-Type", "text/html")
		return c.HTML(status, result.HTML)
	} else if result.Body != "" {
		return c.String(status, result.Body)
	}

	// Empty response
	return c.NoContent(status)
}

// applyResponseTransformation applies response transformation based on interceptor result
func applyResponseTransformation(c echo.Context, result *InterceptorResult, recorder *responseRecorder) error {
	// Override status if specified
	if result.Status != 0 {
		c.Response().Status = result.Status
	}

	// Add/override headers
	for key, value := range result.Headers {
		c.Response().Header().Set(key, value)
	}

	// Override body if specified
	if result.Body != "" {
		return c.String(c.Response().Status, result.Body)
	} else if result.HTML != "" {
		c.Response().Header().Set("Content-Type", "text/html")
		return c.HTML(c.Response().Status, result.HTML)
	} else if result.JSON != nil {
		return c.JSON(c.Response().Status, result.JSON)
	}

	// Use original response
	return writeOriginalResponse(c, recorder)
}

// writeOriginalResponse writes the original response that was captured
func writeOriginalResponse(c echo.Context, recorder *responseRecorder) error {
	// Set status code
	c.Response().Status = recorder.statusCode

	// Write body
	_, err := recorder.body.WriteTo(c.Response().Writer)
	return err
}

// responseRecorder captures response for processing by interceptors
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// Write captures response body
func (r *responseRecorder) Write(data []byte) (int, error) {
	r.body.Write(data)
	return r.ResponseWriter.Write(data)
}

// WriteHeader captures status code
func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Header returns the response headers
func (r *responseRecorder) Header() http.Header {
	return r.ResponseWriter.Header()
}

// ValidateInterceptorConfig validates interceptor configuration
func ValidateInterceptorConfig(config *InterceptorConfig) error {
	if config == nil {
		return fmt.Errorf("interceptor configuration is required")
	}

	if config.Name == "" {
		return fmt.Errorf("interceptor name is required")
	}

	// Validate regex pattern if provided
	if config.Regex != "" {
		_, err := regexp.Compile(config.Regex)
		if err != nil {
			return fmt.Errorf("invalid regex pattern '%s': %w", config.Regex, err)
		}
	}

	// At least one of request or response expressions should be provided
	if len(config.Request) == 0 && len(config.Response) == 0 {
		return fmt.Errorf("interceptor '%s' must have at least one request or response expression", config.Name)
	}

	return nil
}

// LogInterceptorActivity logs interceptor activity for debugging
func LogInterceptorActivity(c echo.Context, interceptorName string, phase string, result interface{}) {
	c.Logger().Debugf("Interceptor '%s' %s: %+v", interceptorName, phase, result)
}
