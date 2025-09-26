package middleware

import (
	"fmt"
	"reflect"
	"time"

	"github.com/flanksource/gomplate/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/labstack/echo/v4"
)

// CELEngine provides CEL expression evaluation with custom functions for middleware processing
type CELEngine struct {
	env *cel.Env
}

// NewCELEngine creates a new CEL engine with custom functions for middleware processing
func NewCELEngine() (*CELEngine, error) {
	// Get base environment from flanksource/gomplate with CEL extensions
	gomplateFuncs := gomplate.GetCelEnv(make(map[string]any))

	// Create CEL environment with gomplate functions and custom middleware functions
	env, err := cel.NewEnv(
		append(gomplateFuncs,
			// Request/Response variables
			cel.Variable("request", cel.DynType),
			cel.Variable("response", cel.DynType),
			cel.Variable("context", cel.DynType),
			cel.Variable("headers", cel.MapType(cel.StringType, cel.StringType)),
			cel.Variable("body", cel.StringType),
			cel.Variable("user", cel.StringType),
			cel.Variable("password", cel.StringType),
			cel.Variable("token", cel.DynType),
			cel.Variable("claims", cel.MapType(cel.StringType, cel.DynType)),

			// Custom functions for middleware processing
			cel.Function("now",
				cel.Overload("now", []*cel.Type{}, cel.IntType,
					cel.FunctionBinding(func(values ...ref.Val) ref.Val {
						return types.Int(time.Now().Unix())
					}))),

			cel.Function("setHeader",
				cel.Overload("setHeader", []*cel.Type{cel.StringType, cel.StringType}, cel.MapType(cel.StringType, cel.DynType),
					cel.FunctionBinding(func(values ...ref.Val) ref.Val {
						if len(values) != 2 {
							return types.NewErr("setHeader requires 2 arguments")
						}
						keyStr, _ := values[0].ConvertToNative(reflect.TypeOf(""))
						valueStr, _ := values[1].ConvertToNative(reflect.TypeOf(""))
						return types.NewStringStringMap(types.DefaultTypeAdapter, map[string]string{
							"headers": fmt.Sprintf(`{"%s": "%s"}`, keyStr, valueStr),
						})
					}))),

			cel.Function("addHeader",
				cel.Overload("addHeader", []*cel.Type{cel.StringType, cel.StringType}, cel.MapType(cel.StringType, cel.DynType),
					cel.FunctionBinding(func(values ...ref.Val) ref.Val {
						if len(values) != 2 {
							return types.NewErr("addHeader requires 2 arguments")
						}
						keyStr, _ := values[0].ConvertToNative(reflect.TypeOf(""))
						valueStr, _ := values[1].ConvertToNative(reflect.TypeOf(""))
						return types.NewStringStringMap(types.DefaultTypeAdapter, map[string]string{
							"add_headers": fmt.Sprintf(`{"%s": "%s"}`, keyStr, valueStr),
						})
					}))),

			cel.Function("removeHeader",
				cel.Overload("removeHeader", []*cel.Type{cel.StringType}, cel.MapType(cel.StringType, cel.DynType),
					cel.FunctionBinding(func(values ...ref.Val) ref.Val {
						if len(values) != 1 {
							return types.NewErr("removeHeader requires 1 argument")
						}
						keyStr, _ := values[0].ConvertToNative(reflect.TypeOf(""))
						return types.NewStringStringMap(types.DefaultTypeAdapter, map[string]string{
							"remove_headers": fmt.Sprintf(`["%s"]`, keyStr),
						})
					}))),

			cel.Function("returnStatus",
				cel.Overload("returnStatus_body", []*cel.Type{cel.IntType, cel.StringType}, cel.MapType(cel.StringType, cel.DynType),
					cel.FunctionBinding(func(values ...ref.Val) ref.Val {
						if len(values) != 2 {
							return types.NewErr("returnStatus requires 2 arguments")
						}
						statusInt, _ := values[0].ConvertToNative(reflect.TypeOf(0))
						bodyStr, _ := values[1].ConvertToNative(reflect.TypeOf(""))
						return types.NewStringStringMap(types.DefaultTypeAdapter, map[string]string{
							"status": fmt.Sprintf("%d", statusInt),
							"body":   bodyStr.(string),
						})
					}))),

			cel.Function("returnJSON",
				cel.Overload("returnJSON", []*cel.Type{cel.IntType, cel.MapType(cel.StringType, cel.DynType)}, cel.MapType(cel.StringType, cel.DynType),
					cel.FunctionBinding(func(values ...ref.Val) ref.Val {
						if len(values) != 2 {
							return types.NewErr("returnJSON requires 2 arguments")
						}
						statusInt, _ := values[0].ConvertToNative(reflect.TypeOf(0))
						dataMap, _ := values[1].ConvertToNative(reflect.TypeOf(map[string]interface{}{}))
						return types.NewStringStringMap(types.DefaultTypeAdapter, map[string]string{
							"status": fmt.Sprintf("%d", statusInt),
							"json":   fmt.Sprintf("%v", dataMap),
						})
					}))),

			cel.Function("returnHTML",
				cel.Overload("returnHTML", []*cel.Type{cel.IntType, cel.StringType}, cel.MapType(cel.StringType, cel.DynType),
					cel.FunctionBinding(func(values ...ref.Val) ref.Val {
						if len(values) != 2 {
							return types.NewErr("returnHTML requires 2 arguments")
						}
						statusInt, _ := values[0].ConvertToNative(reflect.TypeOf(0))
						htmlStr, _ := values[1].ConvertToNative(reflect.TypeOf(""))
						return types.NewStringStringMap(types.DefaultTypeAdapter, map[string]string{
							"status": fmt.Sprintf("%d", statusInt),
							"html":   htmlStr.(string),
						})
					}))),

			cel.Function("hasRole",
				cel.Overload("hasRole", []*cel.Type{cel.StringType}, cel.BoolType,
					cel.FunctionBinding(func(values ...ref.Val) ref.Val {
						// Placeholder - in real implementation, this would check user roles from context
						return types.Bool(false)
					}))),

			cel.Function("getUser",
				cel.Overload("getUser", []*cel.Type{}, cel.StringType,
					cel.FunctionBinding(func(values ...ref.Val) ref.Val {
						// Placeholder - in real implementation, this would get user from context
						return types.String("")
					}))),

			cel.Function("checkPermission",
				cel.Overload("checkPermission", []*cel.Type{cel.StringType}, cel.BoolType,
					cel.FunctionBinding(func(values ...ref.Val) ref.Val {
						// Placeholder - in real implementation, this would check permissions
						return types.Bool(false)
					}))),

			cel.Function("validateJWT",
				cel.Overload("validateJWT", []*cel.Type{cel.StringType}, cel.BoolType,
					cel.FunctionBinding(func(values ...ref.Val) ref.Val {
						// Placeholder - in real implementation, this would validate JWT
						return types.Bool(false)
					}))),

			cel.Function("jsonToXML",
				cel.Overload("jsonToXML", []*cel.Type{cel.StringType}, cel.StringType,
					cel.FunctionBinding(func(values ...ref.Val) ref.Val {
						if len(values) != 1 {
							return types.NewErr("jsonToXML requires 1 argument")
						}
						// Placeholder - in real implementation, this would convert JSON to XML
						jsonStr, _ := values[0].ConvertToNative(reflect.TypeOf(""))
						return types.String(fmt.Sprintf("<xml>%s</xml>", jsonStr))
					}))),
		)...,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return &CELEngine{env: env}, nil
}

// EvaluateCondition evaluates a CEL expression with provided variables and returns a boolean result
func (ce *CELEngine) EvaluateCondition(expression string, variables map[string]interface{}) (bool, error) {
	if expression == "" {
		return true, nil // Empty condition is always true
	}

	result, err := ce.Evaluate(expression, variables)
	if err != nil {
		return false, err
	}

	if boolResult, ok := result.(bool); ok {
		return boolResult, nil
	}

	return false, fmt.Errorf("condition expression must return boolean, got %T", result)
}

// Evaluate evaluates a CEL expression with provided variables and returns the result
func (ce *CELEngine) Evaluate(expression string, variables map[string]interface{}) (interface{}, error) {
	// Parse the expression
	ast, issues := ce.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile CEL expression '%s': %w", expression, issues.Err())
	}

	// Create program
	prg, err := ce.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL program: %w", err)
	}

	// Evaluate with variables
	out, _, err := prg.Eval(variables)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate CEL expression '%s': %w", expression, err)
	}

	// Return the CEL value directly - we'll handle conversion at usage time
	return out.Value(), nil
}

// EvaluateFirstNonNull evaluates an array of CEL expressions and returns the first non-null result
func (ce *CELEngine) EvaluateFirstNonNull(expressions []string, variables map[string]interface{}) (interface{}, error) {
	for i, expr := range expressions {
		result, err := ce.Evaluate(expr, variables)
		if err != nil {
			return nil, fmt.Errorf("error evaluating expression %d ('%s'): %w", i, expr, err)
		}

		// Return first non-null result
		if result != nil {
			return result, nil
		}
	}

	return nil, nil // All expressions returned null
}

// CreateRequestVariables creates a variable map for request processing CEL expressions
func CreateRequestVariables(c echo.Context) map[string]interface{} {
	req := c.Request()

	// Create headers map
	headers := make(map[string]string)
	for name, values := range req.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	return map[string]interface{}{
		"request": map[string]interface{}{
			"method": req.Method,
			"path":   req.URL.Path,
			"query":  req.URL.RawQuery,
			"header": headers,
		},
		"context": c,
		"headers": headers,
		"body":    "", // Body would be read separately if needed
	}
}

// CreateResponseVariables creates a variable map for response processing CEL expressions
func CreateResponseVariables(c echo.Context) map[string]interface{} {
	req := c.Request()
	resp := c.Response()

	// Create request headers map
	reqHeaders := make(map[string]string)
	for name, values := range req.Header {
		if len(values) > 0 {
			reqHeaders[name] = values[0]
		}
	}

	// Create response headers map
	respHeaders := make(map[string]string)
	for name, values := range resp.Header() {
		if len(values) > 0 {
			respHeaders[name] = values[0]
		}
	}

	return map[string]interface{}{
		"request": map[string]interface{}{
			"method": req.Method,
			"path":   req.URL.Path,
			"query":  req.URL.RawQuery,
			"header": reqHeaders,
		},
		"response": map[string]interface{}{
			"status": resp.Status,
			"header": respHeaders,
		},
		"context": c,
		"headers": respHeaders,
		"body":    "", // Response body would be captured separately if needed
	}
}

// CreateAuthVariables creates a variable map for authentication CEL expressions
func CreateAuthVariables(c echo.Context, user, password string) map[string]interface{} {
	vars := CreateRequestVariables(c)
	vars["user"] = user
	vars["password"] = password
	return vars
}

// CreateJWTVariables creates a variable map for JWT CEL expressions
func CreateJWTVariables(c echo.Context, token *jwt.Token, claims jwt.MapClaims) map[string]interface{} {
	vars := CreateRequestVariables(c)
	vars["token"] = token
	vars["claims"] = claims
	return vars
}