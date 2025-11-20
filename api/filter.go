package api

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/flanksource/commons/logger"
	"github.com/flanksource/gomplate/v3"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
)

// boolVal is a simple wrapper for boolean CEL values
type boolVal struct {
	value bool
}

func (b *boolVal) Value() interface{} {
	return b.value
}

func (b *boolVal) Type() ref.Type {
	return nil
}

func (b *boolVal) ConvertToNative(typeDesc reflect.Type) (interface{}, error) {
	return b.value, nil
}

func (b *boolVal) ConvertToType(typeValue ref.Type) ref.Val {
	return b
}

func (b *boolVal) Equal(other ref.Val) ref.Val {
	return b
}

// FilterTableRows filters table rows using a CEL expression.
// Field values are injected directly into the CEL context (no "row." prefix).
// Returns filtered rows or error if CEL expression is invalid.
func FilterTableRows(rows []PrettyDataRow, filterExpr string) ([]PrettyDataRow, error) {
	if filterExpr == "" {
		return rows, nil
	}

	if len(rows) == 0 {
		return rows, nil
	}

	// Get variable declarations from the first row
	variableDecls := getVariableDeclarationsFromRow(rows[0])

	// Create CEL environment with dynamic variables
	env, err := createCELEnvironment(variableDecls)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	// Compile expression once
	ast, issues := env.Compile(filterExpr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile CEL expression '%s': %w", filterExpr, issues.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL program: %w", err)
	}

	filtered := make([]PrettyDataRow, 0, len(rows))
	for i, row := range rows {
		variables := rowToCELMap(row)

		out, _, err := prg.Eval(variables)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate filter expression at row %d: %w", i, err)
		}

		if boolResult, ok := out.Value().(bool); ok && boolResult {
			filtered = append(filtered, row)
		}
	}

	logger.V(4).Infof("Filtered %d rows to %d using expression: %s", len(rows), len(filtered), filterExpr)
	return filtered, nil
}

// FilterTreeNode recursively filters tree nodes using a CEL expression.
// Field values from node content are injected directly into the CEL context.
// Returns filtered tree or error if CEL expression is invalid.
func FilterTreeNode(node TreeNode, filterExpr string) (TreeNode, error) {
	if filterExpr == "" || node == nil {
		return node, nil
	}

	// Collect all unique variable declarations from the entire tree
	variableDecls := collectTreeVariableDeclarations(node)

	// Create CEL environment with dynamic variables
	env, err := createCELEnvironment(variableDecls)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	// Compile expression once
	ast, issues := env.Compile(filterExpr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile CEL expression '%s': %w", filterExpr, issues.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL program: %w", err)
	}

	return filterTreeNodeRecursive(node, prg)
}

// filterTreeNodeRecursive recursively filters tree nodes
func filterTreeNodeRecursive(node TreeNode, prg cel.Program) (TreeNode, error) {
	if node == nil {
		return nil, nil
	}

	// Convert node content to CEL variables
	variables := nodeToCELMap(node)

	// Evaluate filter for this node
	out, _, err := prg.Eval(variables)
	if err != nil {
		// If evaluation fails due to missing attribute, treat as non-match
		// This can happen when a node doesn't have metadata fields that other nodes have
		if strings.Contains(err.Error(), "no such attribute") {
			// Node doesn't have the required field - treat as non-match
			out = &boolVal{value: false}
		} else {
			return nil, fmt.Errorf("failed to evaluate filter expression: %w", err)
		}
	}

	match := false
	if boolResult, ok := out.Value().(bool); ok {
		match = boolResult
	}

	// If node doesn't match, check children
	children := node.GetChildren()
	if !match && len(children) == 0 {
		// Leaf node doesn't match - exclude it
		return nil, nil
	}

	// Process children recursively
	var filteredChildren []TreeNode
	if len(children) > 0 {
		for _, child := range children {
			filteredChild, err := filterTreeNodeRecursive(child, prg)
			if err != nil {
				return nil, err
			}
			if filteredChild != nil {
				filteredChildren = append(filteredChildren, filteredChild)
			}
		}
	}

	// If node doesn't match but has matching children, include it with filtered children
	if !match && len(filteredChildren) > 0 {
		return &ConcreteBranchNode{
			Children: filteredChildren,
		}, nil
	}

	// If node matches, include it with filtered children
	if match {
		if len(filteredChildren) > 0 {
			// Create a new node with filtered children
			simple := TreeNodeToSimple(node)
			simple.Children = filteredChildren
			return simple, nil
		}
		// Leaf node that matches - return as-is
		return node, nil
	}

	// Node doesn't match and has no matching children
	return nil, nil
}

// rowToCELMap converts a PrettyDataRow to a flat map for CEL evaluation.
// Field names become variable names (no "row." prefix).
// Uses Primitive() to extract typed values for accurate CEL comparisons.
func rowToCELMap(row PrettyDataRow) map[string]interface{} {
	result := make(map[string]interface{})
	for key, fieldValue := range row {
		// Use Primitive() to get strongly-typed value
		// This ensures CEL expressions work with proper types:
		// - int64 for integers
		// - float64 for floats
		// - bool for booleans
		// - string for strings
		// - time.Time for dates
		result[key] = fieldValue.String()
	}
	return result
}

// nodeToCELMap converts a TreeNode's Pretty text to CEL variables.
// For SimpleTreeNode, extracts label and metadata fields.
func nodeToCELMap(node TreeNode) map[string]interface{} {
	result := make(map[string]interface{})

	// Get the Pretty() text content
	text := node.Pretty()
	result["label"] = text.Content
	result["content"] = text.Content // Alias for label

	if text.Style != "" {
		result["style"] = text.Style
	}

	// If it's a SimpleTreeNode, extract additional fields
	if simple, ok := node.(*SimpleTreeNode); ok {
		result["label"] = simple.Label

		if simple.Icon != "" {
			result["icon"] = simple.Icon
		}
		if simple.Style != "" {
			result["style"] = simple.Style
		}

		// Include metadata fields at top level for easy access
		for key, value := range simple.Metadata {
			result[key] = value
		}
	}

	return result
}

// createCELEnvironment creates a CEL environment with dynamic variable declarations
func createCELEnvironment(variableDecls []cel.EnvOption) (*cel.Env, error) {
	// Get base gomplate functions
	gomplateFuncs := gomplate.GetCelEnv(make(map[string]any))

	// Combine gomplate functions with variable declarations
	envOptions := append(gomplateFuncs, variableDecls...)

	return cel.NewEnv(envOptions...)
}

// getVariableDeclarationsFromRow creates CEL variable declarations from a row's fields
func getVariableDeclarationsFromRow(row PrettyDataRow) []cel.EnvOption {
	var decls []cel.EnvOption

	for key, fieldValue := range row {
		celType := inferCELTypeFromValue(fieldValue.String())
		decls = append(decls, cel.Variable(key, celType))
	}

	return decls
}

// collectTreeVariableDeclarations collects all unique variable names from an entire tree
func collectTreeVariableDeclarations(node TreeNode) []cel.EnvOption {
	if node == nil {
		return nil
	}

	// Use a map to track unique variable names and their types
	vars := make(map[string]*cel.Type)

	// Recursively collect variables from this node and all children
	collectTreeVariables(node, vars)

	// Convert map to CEL variable declarations
	var decls []cel.EnvOption
	for name, celType := range vars {
		decls = append(decls, cel.Variable(name, celType))
	}

	return decls
}

// collectTreeVariables recursively collects variable names from a tree node and its children
func collectTreeVariables(node TreeNode, vars map[string]*cel.Type) {
	if node == nil {
		return
	}

	// Get variables from this node
	nodeVars := nodeToCELMap(node)
	for key, value := range nodeVars {
		celType := inferCELTypeFromValue(value)
		// If variable already exists, use DynType to allow different types across nodes
		if existingType, exists := vars[key]; exists && existingType != celType {
			vars[key] = cel.DynType
		} else {
			vars[key] = celType
		}
	}

	// Recursively collect from children
	for _, child := range node.GetChildren() {
		collectTreeVariables(child, vars)
	}
}

// inferCELTypeFromValue infers the CEL type from a Go value
func inferCELTypeFromValue(value interface{}) *cel.Type {
	switch value.(type) {
	case string:
		return cel.StringType
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return cel.IntType
	case float32, float64:
		return cel.DoubleType
	case bool:
		return cel.BoolType
	default:
		// Use DynType for unknown types (time.Time, etc.)
		return cel.DynType
	}
}
