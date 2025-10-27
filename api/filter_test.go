package api

import (
	"testing"
	"time"
)

func TestFilterTableRows(t *testing.T) {
	tests := []struct {
		name           string
		rows           []PrettyDataRow
		filterExpr     string
		expectedCount  int
		expectError    bool
		validateResult func(*testing.T, []PrettyDataRow)
	}{
		{
			name: "filter by string equality",
			rows: []PrettyDataRow{
				{
					"status": FieldValue{Value: "active", Field: PrettyField{Name: "status", Type: "string"}},
					"name":   FieldValue{Value: "item1", Field: PrettyField{Name: "name", Type: "string"}},
				},
				{
					"status": FieldValue{Value: "inactive", Field: PrettyField{Name: "status", Type: "string"}},
					"name":   FieldValue{Value: "item2", Field: PrettyField{Name: "name", Type: "string"}},
				},
				{
					"status": FieldValue{Value: "active", Field: PrettyField{Name: "status", Type: "string"}},
					"name":   FieldValue{Value: "item3", Field: PrettyField{Name: "name", Type: "string"}},
				},
			},
			filterExpr:    "status == 'active'",
			expectedCount: 2,
		},
		{
			name: "filter by numeric comparison",
			rows: []PrettyDataRow{
				{
					"age":  FieldValue{Value: int64(25), Field: PrettyField{Name: "age", Type: "int"}},
					"name": FieldValue{Value: "Alice", Field: PrettyField{Name: "name", Type: "string"}},
				},
				{
					"age":  FieldValue{Value: int64(35), Field: PrettyField{Name: "age", Type: "int"}},
					"name": FieldValue{Value: "Bob", Field: PrettyField{Name: "name", Type: "string"}},
				},
				{
					"age":  FieldValue{Value: int64(45), Field: PrettyField{Name: "age", Type: "int"}},
					"name": FieldValue{Value: "Charlie", Field: PrettyField{Name: "name", Type: "string"}},
				},
			},
			filterExpr:    "age > 30",
			expectedCount: 2,
			validateResult: func(t *testing.T, result []PrettyDataRow) {
				if len(result) != 2 {
					t.Errorf("Expected 2 rows, got %d", len(result))
					return
				}
				// Should contain Bob and Charlie
				names := []string{result[0]["name"].Value.(string), result[1]["name"].Value.(string)}
				if !(contains(names, "Bob") && contains(names, "Charlie")) {
					t.Errorf("Expected Bob and Charlie, got %v", names)
				}
			},
		},
		{
			name: "filter with boolean field",
			rows: []PrettyDataRow{
				{
					"active": FieldValue{Value: true, Field: PrettyField{Name: "active", Type: "boolean"}},
					"name":   FieldValue{Value: "item1", Field: PrettyField{Name: "name", Type: "string"}},
				},
				{
					"active": FieldValue{Value: false, Field: PrettyField{Name: "active", Type: "boolean"}},
					"name":   FieldValue{Value: "item2", Field: PrettyField{Name: "name", Type: "string"}},
				},
			},
			filterExpr:    "active",
			expectedCount: 1,
		},
		{
			name: "filter with AND condition",
			rows: []PrettyDataRow{
				{
					"status": FieldValue{Value: "active", Field: PrettyField{Name: "status", Type: "string"}},
					"age":    FieldValue{Value: int64(25), Field: PrettyField{Name: "age", Type: "int"}},
				},
				{
					"status": FieldValue{Value: "active", Field: PrettyField{Name: "status", Type: "string"}},
					"age":    FieldValue{Value: int64(35), Field: PrettyField{Name: "age", Type: "int"}},
				},
				{
					"status": FieldValue{Value: "inactive", Field: PrettyField{Name: "status", Type: "string"}},
					"age":    FieldValue{Value: int64(35), Field: PrettyField{Name: "age", Type: "int"}},
				},
			},
			filterExpr:    "status == 'active' && age > 30",
			expectedCount: 1,
		},
		{
			name: "filter with OR condition",
			rows: []PrettyDataRow{
				{
					"status":   FieldValue{Value: "pending", Field: PrettyField{Name: "status", Type: "string"}},
					"priority": FieldValue{Value: int64(1), Field: PrettyField{Name: "priority", Type: "int"}},
				},
				{
					"status":   FieldValue{Value: "active", Field: PrettyField{Name: "status", Type: "string"}},
					"priority": FieldValue{Value: int64(5), Field: PrettyField{Name: "priority", Type: "int"}},
				},
				{
					"status":   FieldValue{Value: "inactive", Field: PrettyField{Name: "status", Type: "string"}},
					"priority": FieldValue{Value: int64(10), Field: PrettyField{Name: "priority", Type: "int"}},
				},
			},
			filterExpr:    "status == 'pending' || priority >= 10",
			expectedCount: 2,
		},
		{
			name: "empty filter expression returns all rows",
			rows: []PrettyDataRow{
				{"name": FieldValue{Value: "item1", Field: PrettyField{Name: "name", Type: "string"}}},
				{"name": FieldValue{Value: "item2", Field: PrettyField{Name: "name", Type: "string"}}},
			},
			filterExpr:    "",
			expectedCount: 2,
		},
		{
			name: "invalid CEL expression returns error",
			rows: []PrettyDataRow{
				{"name": FieldValue{Value: "item1", Field: PrettyField{Name: "name", Type: "string"}}},
			},
			filterExpr:  "invalid syntax !!!",
			expectError: true,
		},
		{
			name: "filter with contains function",
			rows: []PrettyDataRow{
				{"name": FieldValue{Value: "hello_world", Field: PrettyField{Name: "name", Type: "string"}}},
				{"name": FieldValue{Value: "goodbye", Field: PrettyField{Name: "name", Type: "string"}}},
				{"name": FieldValue{Value: "world_map", Field: PrettyField{Name: "name", Type: "string"}}},
			},
			filterExpr:    "name.contains('world')",
			expectedCount: 2,
		},
		{
			name: "filter no matches returns empty slice",
			rows: []PrettyDataRow{
				{"status": FieldValue{Value: "active", Field: PrettyField{Name: "status", Type: "string"}}},
				{"status": FieldValue{Value: "pending", Field: PrettyField{Name: "status", Type: "string"}}},
			},
			filterExpr:    "status == 'completed'",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FilterTableRows(tt.rows, tt.filterExpr)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d rows, got %d", tt.expectedCount, len(result))
			}

			if tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

func TestFilterTreeNode(t *testing.T) {
	tests := []struct {
		name           string
		tree           TreeNode
		filterExpr     string
		expectedNodes  int // Total nodes in filtered tree (including parent nodes)
		expectError    bool
		validateResult func(*testing.T, TreeNode)
	}{
		{
			name: "filter leaf nodes by label",
			tree: &SimpleTreeNode{
				Label: "root",
				Children: []TreeNode{
					&SimpleTreeNode{Label: "active_item", Metadata: map[string]interface{}{"status": "active"}},
					&SimpleTreeNode{Label: "pending_item", Metadata: map[string]interface{}{"status": "pending"}},
					&SimpleTreeNode{Label: "active_node", Metadata: map[string]interface{}{"status": "active"}},
				},
			},
			filterExpr:    "label.contains('active')",
			expectedNodes: 3, // root + 2 matching children
			validateResult: func(t *testing.T, result TreeNode) {
				if result == nil {
					t.Fatal("Expected non-nil result")
				}
				children := result.GetChildren()
				// Should have 2 matching children (active_item, active_node)
				if len(children) != 2 {
					t.Errorf("Expected 2 matching children, got %d", len(children))
				}
			},
		},
		{
			name: "filter by metadata field",
			tree: &SimpleTreeNode{
				Label: "root",
				Children: []TreeNode{
					&SimpleTreeNode{Label: "item1", Metadata: map[string]interface{}{"priority": int64(1)}},
					&SimpleTreeNode{Label: "item2", Metadata: map[string]interface{}{"priority": int64(5)}},
					&SimpleTreeNode{Label: "item3", Metadata: map[string]interface{}{"priority": int64(10)}},
				},
			},
			filterExpr:    "priority >= 5",
			expectedNodes: 3, // root + 2 matching children
		},
		{
			name: "filter preserves parent nodes with matching children",
			tree: &SimpleTreeNode{
				Label: "root",
				Children: []TreeNode{
					&SimpleTreeNode{
						Label: "parent1",
						Children: []TreeNode{
							&SimpleTreeNode{Label: "child1", Metadata: map[string]interface{}{"category": "match"}},
							&SimpleTreeNode{Label: "child2", Metadata: map[string]interface{}{"category": "nomatch"}},
						},
					},
					&SimpleTreeNode{
						Label: "parent2",
						Children: []TreeNode{
							&SimpleTreeNode{Label: "child3", Metadata: map[string]interface{}{"category": "nomatch"}},
						},
					},
				},
			},
			filterExpr: "category == 'match'",
			validateResult: func(t *testing.T, result TreeNode) {
				if result == nil {
					t.Fatal("Expected non-nil result")
				}
				children := result.GetChildren()
				if len(children) != 1 {
					t.Errorf("Expected 1 parent node, got %d", len(children))
					return
				}
				// Parent1 should be preserved with only matching child
				parent1Children := children[0].GetChildren()
				if len(parent1Children) != 1 {
					t.Errorf("Expected parent1 to have 1 child, got %d", len(parent1Children))
				}
			},
		},
		{
			name: "empty filter expression returns original tree",
			tree: &SimpleTreeNode{
				Label: "root",
				Children: []TreeNode{
					&SimpleTreeNode{Label: "child1"},
					&SimpleTreeNode{Label: "child2"},
				},
			},
			filterExpr:    "",
			expectedNodes: 3, // root + 2 children
		},
		{
			name: "filter matches root node",
			tree: &SimpleTreeNode{
				Label:    "important_root",
				Metadata: map[string]interface{}{"status": "active"},
				Children: []TreeNode{
					&SimpleTreeNode{Label: "child1", Metadata: map[string]interface{}{"status": "inactive"}},
				},
			},
			filterExpr: "status == 'active'",
			validateResult: func(t *testing.T, result TreeNode) {
				if result == nil {
					t.Fatal("Expected non-nil result")
				}
				// Root matches, so entire tree should be preserved
				children := result.GetChildren()
				if len(children) != 1 {
					t.Errorf("Expected root to have 1 child, got %d", len(children))
				}
			},
		},
		{
			name: "no matches returns nil",
			tree: &SimpleTreeNode{
				Label: "root",
				Children: []TreeNode{
					&SimpleTreeNode{Label: "child1", Metadata: map[string]interface{}{"status": "pending"}},
					&SimpleTreeNode{Label: "child2", Metadata: map[string]interface{}{"status": "pending"}},
				},
			},
			filterExpr:    "status == 'completed'",
			expectedNodes: 0, // No matches
			validateResult: func(t *testing.T, result TreeNode) {
				if result != nil {
					t.Errorf("Expected nil result for no matches, got %v", result)
				}
			},
		},
		{
			name: "invalid CEL expression returns error",
			tree: &SimpleTreeNode{
				Label: "root",
			},
			filterExpr:  "invalid syntax !!!",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FilterTreeNode(tt.tree, tt.filterExpr)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.validateResult != nil {
				tt.validateResult(t, result)
			} else if tt.expectedNodes > 0 {
				// Count total nodes in tree
				count := countTreeNodes(result)
				if count != tt.expectedNodes {
					t.Errorf("Expected %d nodes in filtered tree, got %d", tt.expectedNodes, count)
				}
			} else if tt.expectedNodes == 0 && result != nil {
				t.Errorf("Expected nil result for 0 nodes, got %v", result)
			}
		})
	}
}

func TestRowToCELMap(t *testing.T) {
	tests := []struct {
		name     string
		row      PrettyDataRow
		expected map[string]interface{}
	}{
		{
			name: "string and int fields",
			row: PrettyDataRow{
				"name": FieldValue{Value: "test", Field: PrettyField{Name: "name", Type: "string"}},
				"age":  FieldValue{Value: int64(30), Field: PrettyField{Name: "age", Type: "int"}},
			},
			expected: map[string]interface{}{
				"name": "test",
				"age":  int64(30),
			},
		},
		{
			name: "boolean and float fields",
			row: PrettyDataRow{
				"active": FieldValue{Value: true, Field: PrettyField{Name: "active", Type: "boolean"}},
				"score":  FieldValue{Value: 95.5, Field: PrettyField{Name: "score", Type: "float"}},
			},
			expected: map[string]interface{}{
				"active": true,
				"score":  95.5,
			},
		},
		{
			name: "time field",
			row: PrettyDataRow{
				"created_at": FieldValue{
					Value: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Field: PrettyField{Name: "created_at", Type: "date"},
				},
			},
			expected: map[string]interface{}{
				"created_at": time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rowToCELMap(tt.row)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d fields, got %d", len(tt.expected), len(result))
			}

			for key, expectedVal := range tt.expected {
				actualVal, ok := result[key]
				if !ok {
					t.Errorf("Missing key %q in result", key)
					continue
				}

				// Handle time.Time comparison separately
				if expectedTime, ok := expectedVal.(time.Time); ok {
					actualTime, ok := actualVal.(time.Time)
					if !ok {
						t.Errorf("Expected time.Time for key %q, got %T", key, actualVal)
						continue
					}
					if !expectedTime.Equal(actualTime) {
						t.Errorf("For key %q: expected %v, got %v", key, expectedTime, actualTime)
					}
				} else if actualVal != expectedVal {
					t.Errorf("For key %q: expected %v, got %v", key, expectedVal, actualVal)
				}
			}
		})
	}
}

func TestNodeToCELMap(t *testing.T) {
	tests := []struct {
		name     string
		node     TreeNode
		expected map[string]interface{}
	}{
		{
			name: "simple node with label",
			node: &SimpleTreeNode{
				Label: "test_node",
			},
			expected: map[string]interface{}{
				"label":   "test_node",
				"content": "test_node",
			},
		},
		{
			name: "node with metadata",
			node: &SimpleTreeNode{
				Label: "node",
				Metadata: map[string]interface{}{
					"status":   "active",
					"priority": int64(5),
				},
			},
			expected: map[string]interface{}{
				"label":    "node",
				"content":  "node",
				"status":   "active",
				"priority": int64(5),
			},
		},
		{
			name: "node with style and icon",
			node: &SimpleTreeNode{
				Label: "styled_node",
				Style: "bold",
				Icon:  "check",
			},
			expected: map[string]interface{}{
				"label": "styled_node",
				// content comes from Pretty() which includes icon prefix
				// We just verify label, style, and icon are correct
				"style": "bold",
				"icon":  "check",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nodeToCELMap(tt.node)

			for key, expectedVal := range tt.expected {
				actualVal, ok := result[key]
				if !ok {
					t.Errorf("Missing key %q in result", key)
					continue
				}
				if actualVal != expectedVal {
					t.Errorf("For key %q: expected %v, got %v", key, expectedVal, actualVal)
				}
			}
		})
	}
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func countTreeNodes(node TreeNode) int {
	if node == nil {
		return 0
	}
	count := 1 // Count this node
	for _, child := range node.GetChildren() {
		count += countTreeNodes(child)
	}
	return count
}
