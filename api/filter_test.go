package api

import (
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.XDescribe("FilterTableRows", func() {
	tests := []struct {
		name           string
		rows           []PrettyDataRow
		filterExpr     string
		expectedCount  int
		expectError    bool
		validateResult func([]PrettyDataRow)
	}{
		{
			name: "filter by string equality",
			rows: []PrettyDataRow{
				{
					"status": NewTypedValue("active"),
					"name":   NewTypedValue("item1"),
				},
				{
					"status": NewTypedValue("inactive"),
					"name":   NewTypedValue("item2"),
				},
				{
					"status": NewTypedValue("active"),
					"name":   NewTypedValue("item3"),
				},
			},
			filterExpr:    "status == 'active'",
			expectedCount: 2,
		},
		{
			name: "filter by numeric comparison",
			rows: []PrettyDataRow{
				{
					"age":  NewTypedValue(int64(25)),
					"name": NewTypedValue("Alice"),
				},
				{
					"age":  NewTypedValue(int64(35)),
					"name": NewTypedValue("Bob"),
				},
				{
					"age":  NewTypedValue(int64(45)),
					"name": NewTypedValue("Charlie"),
				},
			},
			filterExpr:    "age > 30",
			expectedCount: 2,
			validateResult: func(result []PrettyDataRow) {
				Expect(result).To(HaveLen(2))
				names := []string{result[0]["name"].String(), result[1]["name"].String()}
				Expect(names).To(ContainElement("Bob"))
				Expect(names).To(ContainElement("Charlie"))
			},
		},
		{
			name: "filter with boolean field",
			rows: []PrettyDataRow{
				{
					"active": NewTypedValue(true),
					"name":   NewTypedValue("item1"),
				},
				{
					"active": NewTypedValue(false),
					"name":   NewTypedValue("item2"),
				},
			},
			filterExpr:    "active",
			expectedCount: 1,
		},
		{
			name: "filter with AND condition",
			rows: []PrettyDataRow{
				{
					"status": NewTypedValue("active"),
					"age":    NewTypedValue(int64(25)),
				},
				{
					"status": NewTypedValue("active"),
					"age":    NewTypedValue(int64(35)),
				},
				{
					"status": NewTypedValue("inactive"),
					"age":    NewTypedValue(int64(35)),
				},
			},
			filterExpr:    "status == 'active' && age > 30",
			expectedCount: 1,
		},
		{
			name: "filter with OR condition",
			rows: []PrettyDataRow{
				{
					"status":   NewTypedValue("pending"),
					"priority": NewTypedValue(int64(1)),
				},
				{
					"status":   NewTypedValue("active"),
					"priority": NewTypedValue(int64(5)),
				},
				{
					"status":   NewTypedValue("inactive"),
					"priority": NewTypedValue(int64(10)),
				},
			},
			filterExpr:    "status == 'pending' || priority >= 10",
			expectedCount: 2,
		},
		{
			name: "empty filter expression returns all rows",
			rows: []PrettyDataRow{
				{"name": NewTypedValue("item1")},
				{"name": NewTypedValue("item2")},
			},
			filterExpr:    "",
			expectedCount: 2,
		},
		{
			name: "invalid CEL expression returns error",
			rows: []PrettyDataRow{
				{"name": NewTypedValue("item1")},
			},
			filterExpr:  "invalid syntax !!!",
			expectError: true,
		},
		{
			name: "filter with contains function",
			rows: []PrettyDataRow{
				{"name": NewTypedValue("hello_world")},
				{"name": NewTypedValue("goodbye")},
				{"name": NewTypedValue("world_map")},
			},
			filterExpr:    "name.contains('world')",
			expectedCount: 2,
		},
		{
			name: "filter no matches returns empty slice",
			rows: []PrettyDataRow{
				{"status": NewTypedValue("active")},
				{"status": NewTypedValue("pending")},
			},
			filterExpr:    "status == 'completed'",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		ginkgo.It(tt.name, func() {
			result, err := FilterTableRows(tt.rows, tt.filterExpr)

			if tt.expectError {
				Expect(err).To(HaveOccurred())
				return
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(tt.expectedCount))

			if tt.validateResult != nil {
				tt.validateResult(result)
			}
		})
	}
})

var _ = ginkgo.Describe("FilterTreeNode", func() {
	tests := []struct {
		name           string
		tree           TreeNode
		filterExpr     string
		expectedNodes  int
		expectError    bool
		validateResult func(TreeNode)
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
			expectedNodes: 3,
			validateResult: func(result TreeNode) {
				Expect(result).ToNot(BeNil())
				children := result.GetChildren()
				Expect(children).To(HaveLen(2))
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
			expectedNodes: 3,
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
			validateResult: func(result TreeNode) {
				Expect(result).ToNot(BeNil())
				children := result.GetChildren()
				Expect(children).To(HaveLen(1))

				parent1Children := children[0].GetChildren()
				Expect(parent1Children).To(HaveLen(1))
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
			expectedNodes: 3,
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
			validateResult: func(result TreeNode) {
				Expect(result).ToNot(BeNil())
				children := result.GetChildren()
				Expect(children).To(HaveLen(1))
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
			expectedNodes: 0,
			validateResult: func(result TreeNode) {
				Expect(result).To(BeNil())
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
		tt := tt
		ginkgo.It(tt.name, func() {
			result, err := FilterTreeNode(tt.tree, tt.filterExpr)

			if tt.expectError {
				Expect(err).To(HaveOccurred())
				return
			}

			Expect(err).ToNot(HaveOccurred())

			if tt.validateResult != nil {
				tt.validateResult(result)
			} else if tt.expectedNodes > 0 {
				count := countTreeNodes(result)
				Expect(count).To(Equal(tt.expectedNodes))
			} else if tt.expectedNodes == 0 {
				Expect(result).To(BeNil())
			}
		})
	}
})

var _ = ginkgo.XDescribe("rowToCELMap", func() {
	tests := []struct {
		name     string
		row      PrettyDataRow
		expected map[string]interface{}
	}{
		{
			name: "string and int fields",
			row: PrettyDataRow{
				"name": NewTypedValue("test"),
				"age":  NewTypedValue(int64(30)),
			},
			expected: map[string]interface{}{
				"name": "test",
				"age":  int64(30),
			},
		},
		{
			name: "boolean and float fields",
			row: PrettyDataRow{
				"active": NewTypedValue(true),
				"score":  NewTypedValue(95.5),
			},
			expected: map[string]interface{}{
				"active": true,
				"score":  95.5,
			},
		},
		{
			name: "time field",
			row: PrettyDataRow{
				"created_at": NewTypedValue(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)),
			},
			expected: map[string]interface{}{
				"created_at": time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		ginkgo.It(tt.name, func() {
			result := rowToCELMap(tt.row)

			Expect(result).To(HaveLen(len(tt.expected)))

			for key, expectedVal := range tt.expected {
				actualVal, ok := result[key]
				Expect(ok).To(BeTrue(), "Missing key %q in result", key)

				if expectedTime, ok := expectedVal.(time.Time); ok {
					actualTime, ok := actualVal.(time.Time)
					Expect(ok).To(BeTrue(), "Expected time.Time for key %q, got %T", key, actualVal)
					Expect(actualTime.Equal(expectedTime)).To(BeTrue(), "For key %q: expected %v, got %v", key, expectedTime, actualTime)
				} else {
					Expect(actualVal).To(Equal(expectedVal), "For key %q", key)
				}
			}
		})
	}
})

var _ = ginkgo.Describe("nodeToCELMap", func() {
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
				"style": "bold",
				"icon":  "check",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		ginkgo.It(tt.name, func() {
			result := nodeToCELMap(tt.node)

			for key, expectedVal := range tt.expected {
				actualVal, ok := result[key]
				Expect(ok).To(BeTrue(), "Missing key %q in result", key)
				Expect(actualVal).To(Equal(expectedVal), "For key %q", key)
			}
		})
	}
})

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
	count := 1
	for _, child := range node.GetChildren() {
		count += countTreeNodes(child)
	}
	return count
}
