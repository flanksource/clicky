// Package filter applies filter expressions to api table rows, tree nodes and
// map rows through the configured Adapter.
package filter

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/commons/logger"
)

// TableRows filters table rows using an expression.
// Field values are injected directly into the expression context (no "row." prefix).
func TableRows(rows []api.PrettyDataRow, filterExpr string) ([]api.PrettyDataRow, error) {
	if filterExpr == "" || len(rows) == 0 {
		return rows, nil
	}

	predicate, err := Compile(filterExpr, rowVariables(rows[0]))
	if err != nil {
		return nil, err
	}

	filtered := make([]api.PrettyDataRow, 0, len(rows))
	for i, row := range rows {
		match, err := predicate.Match(rowVariables(row))
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate filter expression at row %d: %w", i+1, err)
		}
		if match {
			filtered = append(filtered, row)
		}
	}

	logger.V(4).Infof("Filtered %d rows to %d using expression: %s", len(rows), len(filtered), filterExpr)
	return filtered, nil
}

// TreeNode recursively filters tree nodes using an expression.
// Field values from node content are injected directly into the expression context.
func TreeNode(node api.TreeNode, filterExpr string) (api.TreeNode, error) {
	if filterExpr == "" || node == nil {
		return node, nil
	}

	sample := map[string]any{}
	collectTreeVariables(node, sample)
	predicate, err := Compile(filterExpr, sample)
	if err != nil {
		return nil, err
	}

	return filterTreeNodeRecursive(node, predicate)
}

func filterTreeNodeRecursive(node api.TreeNode, predicate Predicate) (api.TreeNode, error) {
	if node == nil {
		return nil, nil
	}

	match, err := predicate.Match(nodeVariables(node))
	// A node without metadata fields that other nodes have is a non-match.
	if err != nil && !errors.Is(err, ErrMissingVariable) {
		return nil, fmt.Errorf("failed to evaluate filter expression: %w", err)
	}

	children := node.GetChildren()
	if !match && len(children) == 0 {
		return nil, nil
	}

	var filteredChildren []api.TreeNode
	for _, child := range children {
		filteredChild, err := filterTreeNodeRecursive(child, predicate)
		if err != nil {
			return nil, err
		}
		if filteredChild != nil {
			filteredChildren = append(filteredChildren, filteredChild)
		}
	}

	if !match && len(filteredChildren) > 0 {
		return &api.ConcreteBranchNode{
			Children: filteredChildren,
		}, nil
	}

	if match {
		if len(filteredChildren) > 0 {
			simple := api.TreeNodeToSimple(node)
			simple.Children = filteredChildren
			return simple, nil
		}
		return node, nil
	}

	return nil, nil
}

// rowVariables converts an api.PrettyDataRow to a flat variable map.
// Field names become variable names (no "row." prefix); "row" holds every
// field, so keys that aren't identifiers stay addressable as row["Total Amount"].
// FilterValue keeps typed values (int64, float64, bool, time.Time) for accurate comparisons.
func rowVariables(row api.PrettyDataRow) map[string]any {
	result := make(map[string]any, len(row)+1)
	for key, fieldValue := range row {
		if fieldValue.FilterValue != nil {
			result[key] = fieldValue.FilterValue
		} else {
			result[key] = fieldValue.String()
		}
	}
	rowValues := make(map[string]any, len(result))
	for key, value := range result {
		rowValues[key] = value
	}
	result["row"] = rowValues
	return result
}

// nodeVariables converts an api.TreeNode's Pretty text to variables.
// For SimpleTreeNode, extracts label and metadata fields.
func nodeVariables(node api.TreeNode) map[string]any {
	result := make(map[string]any)

	text := node.Pretty()
	result["label"] = text.Content
	result["content"] = text.Content

	if text.Style != "" {
		result["style"] = text.Style
	}

	if simple, ok := node.(*api.SimpleTreeNode); ok {
		result["label"] = simple.Label

		if simple.Icon != "" {
			result["icon"] = simple.Icon
		}
		if simple.Style != "" {
			result["style"] = simple.Style
		}

		for key, value := range simple.Metadata {
			result[key] = value
		}
	}

	return result
}

// collectTreeVariables merges the variables of every node into sample.
// A variable whose Go type differs across nodes is stored as nil (untyped).
func collectTreeVariables(node api.TreeNode, sample map[string]any) {
	if node == nil {
		return
	}

	for key, value := range nodeVariables(node) {
		if existing, exists := sample[key]; exists && reflect.TypeOf(existing) != reflect.TypeOf(value) {
			sample[key] = nil
		} else if !exists {
			sample[key] = value
		}
	}

	for _, child := range node.GetChildren() {
		collectTreeVariables(child, sample)
	}
}
