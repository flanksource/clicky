package rpc

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/flanksource/clicky/entity"
)

// FindOperationByID resolves the stable operationId emitted in OpenAPI.
func (e *CommandExecutor) FindOperationByID(id string) *RPCOperation {
	for i := range e.service.Operations {
		operation := &e.service.Operations[i]
		if operationID(operation) == id {
			return operation
		}
	}
	return nil
}

// ExecuteOperation validates JSON object values against the generated
// operation schema before invoking the same executor used by HTTP requests.
func (e *CommandExecutor) ExecuteOperation(
	ctx context.Context,
	id string,
	values map[string]any,
) (any, *ExecutionResponse, error) {
	operation := e.FindOperationByID(id)
	if operation == nil {
		return nil, nil, fmt.Errorf("operation not found: %s", id)
	}
	request, err := executionRequestFromValues(ctx, operation, values)
	if err != nil {
		return nil, nil, err
	}
	if err := e.ValidateParameters(request, operation); err != nil {
		return nil, nil, err
	}
	return e.ExecuteCommand(operation, request)
}

func operationID(operation *RPCOperation) string {
	return strings.ReplaceAll(operation.Name, " ", "_")
}

func executionRequestFromValues(
	ctx context.Context,
	operation *RPCOperation,
	values map[string]any,
) (*ExecutionRequest, error) {
	request := &ExecutionRequest{
		Flags:   map[string]string{},
		Context: entityScheduleContext(ctx),
	}
	parameters := make(map[string]RPCParameter, len(operation.Parameters))
	for _, parameter := range operation.Parameters {
		parameters[parameter.Name] = parameter
	}
	for name, value := range values {
		parameter, ok := parameters[name]
		if !ok {
			return nil, fmt.Errorf("operation %s: unknown argument %q", operationID(operation), name)
		}
		if err := validateOperationValue(parameter, value); err != nil {
			return nil, fmt.Errorf("operation %s: invalid type for argument %q: %w", operationID(operation), name, err)
		}
		if name == "args" {
			request.Args = operationArgs(value)
			continue
		}
		request.Flags[name] = convertValueToString(value)
	}
	return request, nil
}

func entityScheduleContext(ctx context.Context) context.Context {
	if ctx == nil {
		panic("rpc: operation execution context is required")
	}
	return entity.ContextWithOperationSurface(ctx, "schedule")
}

func validateOperationValue(parameter RPCParameter, value any) error {
	if value == nil {
		return fmt.Errorf("value must not be null")
	}
	switch parameter.Type {
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case "integer":
		number, ok := value.(float64)
		if !ok {
			if _, ok := value.(int); ok {
				return nil
			}
			return fmt.Errorf("expected integer, got %T", value)
		}
		if math.Trunc(number) != number {
			return fmt.Errorf("expected integer, got %v", value)
		}
	case "number":
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		default:
			return fmt.Errorf("expected number, got %T", value)
		}
	case "array":
		switch value.(type) {
		case []any, []string:
		default:
			return fmt.Errorf("expected array, got %T", value)
		}
	case "string", "":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	}
	return nil
}

func operationArgs(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		args := make([]string, len(typed))
		for i := range typed {
			args[i] = convertValueToString(typed[i])
		}
		return args
	default:
		return []string{convertValueToString(value)}
	}
}
