package filter

import (
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.XDescribe("TableRows", func() {
	tests := []struct {
		name           string
		rows           []api.PrettyDataRow
		filterExpr     string
		expectedCount  int
		expectError    bool
		validateResult func([]api.PrettyDataRow)
	}{
		{
			name: "filter by string equality",
			rows: []api.PrettyDataRow{
				{
					"status": api.NewTypedValue("active"),
					"name":   api.NewTypedValue("item1"),
				},
				{
					"status": api.NewTypedValue("inactive"),
					"name":   api.NewTypedValue("item2"),
				},
				{
					"status": api.NewTypedValue("active"),
					"name":   api.NewTypedValue("item3"),
				},
			},
			filterExpr:    "status == 'active'",
			expectedCount: 2,
		},
		{
			name: "filter by numeric comparison",
			rows: []api.PrettyDataRow{
				{
					"age":  api.NewTypedValue(int64(25)),
					"name": api.NewTypedValue("Alice"),
				},
				{
					"age":  api.NewTypedValue(int64(35)),
					"name": api.NewTypedValue("Bob"),
				},
				{
					"age":  api.NewTypedValue(int64(45)),
					"name": api.NewTypedValue("Charlie"),
				},
			},
			filterExpr:    "age > 30",
			expectedCount: 2,
			validateResult: func(result []api.PrettyDataRow) {
				Expect(result).To(HaveLen(2))
				names := []string{result[0]["name"].String(), result[1]["name"].String()}
				Expect(names).To(ContainElement("Bob"))
				Expect(names).To(ContainElement("Charlie"))
			},
		},
		{
			name: "filter with boolean field",
			rows: []api.PrettyDataRow{
				{
					"active": api.NewTypedValue(true),
					"name":   api.NewTypedValue("item1"),
				},
				{
					"active": api.NewTypedValue(false),
					"name":   api.NewTypedValue("item2"),
				},
			},
			filterExpr:    "active",
			expectedCount: 1,
		},
		{
			name: "filter with AND condition",
			rows: []api.PrettyDataRow{
				{
					"status": api.NewTypedValue("active"),
					"age":    api.NewTypedValue(int64(25)),
				},
				{
					"status": api.NewTypedValue("active"),
					"age":    api.NewTypedValue(int64(35)),
				},
				{
					"status": api.NewTypedValue("inactive"),
					"age":    api.NewTypedValue(int64(35)),
				},
			},
			filterExpr:    "status == 'active' && age > 30",
			expectedCount: 1,
		},
		{
			name: "filter with OR condition",
			rows: []api.PrettyDataRow{
				{
					"status":   api.NewTypedValue("pending"),
					"priority": api.NewTypedValue(int64(1)),
				},
				{
					"status":   api.NewTypedValue("active"),
					"priority": api.NewTypedValue(int64(5)),
				},
				{
					"status":   api.NewTypedValue("inactive"),
					"priority": api.NewTypedValue(int64(10)),
				},
			},
			filterExpr:    "status == 'pending' || priority >= 10",
			expectedCount: 2,
		},
		{
			name: "empty filter expression returns all rows",
			rows: []api.PrettyDataRow{
				{"name": api.NewTypedValue("item1")},
				{"name": api.NewTypedValue("item2")},
			},
			filterExpr:    "",
			expectedCount: 2,
		},
		{
			name: "invalid CEL expression returns error",
			rows: []api.PrettyDataRow{
				{"name": api.NewTypedValue("item1")},
			},
			filterExpr:  "invalid syntax !!!",
			expectError: true,
		},
		{
			name: "filter with contains function",
			rows: []api.PrettyDataRow{
				{"name": api.NewTypedValue("hello_world")},
				{"name": api.NewTypedValue("goodbye")},
				{"name": api.NewTypedValue("world_map")},
			},
			filterExpr:    "name.contains('world')",
			expectedCount: 2,
		},
		{
			name: "filter no matches returns empty slice",
			rows: []api.PrettyDataRow{
				{"status": api.NewTypedValue("active")},
				{"status": api.NewTypedValue("pending")},
			},
			filterExpr:    "status == 'completed'",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		ginkgo.It(tt.name, func() {
			result, err := TableRows(tt.rows, tt.filterExpr)

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

var _ = ginkgo.Describe("TreeNode", func() {
	tests := []struct {
		name           string
		tree           api.TreeNode
		filterExpr     string
		expectedNodes  int
		expectError    bool
		validateResult func(api.TreeNode)
	}{
		{
			name: "filter leaf nodes by label",
			tree: &api.SimpleTreeNode{
				Label: "root",
				Children: []api.TreeNode{
					&api.SimpleTreeNode{Label: "active_item", Metadata: map[string]interface{}{"status": "active"}},
					&api.SimpleTreeNode{Label: "pending_item", Metadata: map[string]interface{}{"status": "pending"}},
					&api.SimpleTreeNode{Label: "active_node", Metadata: map[string]interface{}{"status": "active"}},
				},
			},
			filterExpr:    "label.contains('active')",
			expectedNodes: 3,
			validateResult: func(result api.TreeNode) {
				Expect(result).ToNot(BeNil())
				children := result.GetChildren()
				Expect(children).To(HaveLen(2))
			},
		},
		{
			name: "filter by metadata field",
			tree: &api.SimpleTreeNode{
				Label: "root",
				Children: []api.TreeNode{
					&api.SimpleTreeNode{Label: "item1", Metadata: map[string]interface{}{"priority": int64(1)}},
					&api.SimpleTreeNode{Label: "item2", Metadata: map[string]interface{}{"priority": int64(5)}},
					&api.SimpleTreeNode{Label: "item3", Metadata: map[string]interface{}{"priority": int64(10)}},
				},
			},
			filterExpr:    "priority >= 5",
			expectedNodes: 3,
		},
		{
			name: "filter preserves parent nodes with matching children",
			tree: &api.SimpleTreeNode{
				Label: "root",
				Children: []api.TreeNode{
					&api.SimpleTreeNode{
						Label: "parent1",
						Children: []api.TreeNode{
							&api.SimpleTreeNode{Label: "child1", Metadata: map[string]interface{}{"category": "match"}},
							&api.SimpleTreeNode{Label: "child2", Metadata: map[string]interface{}{"category": "nomatch"}},
						},
					},
					&api.SimpleTreeNode{
						Label: "parent2",
						Children: []api.TreeNode{
							&api.SimpleTreeNode{Label: "child3", Metadata: map[string]interface{}{"category": "nomatch"}},
						},
					},
				},
			},
			filterExpr: "category == 'match'",
			validateResult: func(result api.TreeNode) {
				Expect(result).ToNot(BeNil())
				children := result.GetChildren()
				Expect(children).To(HaveLen(1))

				parent1Children := children[0].GetChildren()
				Expect(parent1Children).To(HaveLen(1))
			},
		},
		{
			name: "empty filter expression returns original tree",
			tree: &api.SimpleTreeNode{
				Label: "root",
				Children: []api.TreeNode{
					&api.SimpleTreeNode{Label: "child1"},
					&api.SimpleTreeNode{Label: "child2"},
				},
			},
			filterExpr:    "",
			expectedNodes: 3,
		},
		{
			name: "filter matches root node",
			tree: &api.SimpleTreeNode{
				Label:    "important_root",
				Metadata: map[string]interface{}{"status": "active"},
				Children: []api.TreeNode{
					&api.SimpleTreeNode{Label: "child1", Metadata: map[string]interface{}{"status": "inactive"}},
				},
			},
			filterExpr: "status == 'active'",
			validateResult: func(result api.TreeNode) {
				Expect(result).ToNot(BeNil())
				children := result.GetChildren()
				Expect(children).To(HaveLen(1))
			},
		},
		{
			name: "no matches returns nil",
			tree: &api.SimpleTreeNode{
				Label: "root",
				Children: []api.TreeNode{
					&api.SimpleTreeNode{Label: "child1", Metadata: map[string]interface{}{"status": "pending"}},
					&api.SimpleTreeNode{Label: "child2", Metadata: map[string]interface{}{"status": "pending"}},
				},
			},
			filterExpr:    "status == 'completed'",
			expectedNodes: 0,
			validateResult: func(result api.TreeNode) {
				Expect(result).To(BeNil())
			},
		},
		{
			name: "invalid CEL expression returns error",
			tree: &api.SimpleTreeNode{
				Label: "root",
			},
			filterExpr:  "invalid syntax !!!",
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		ginkgo.It(tt.name, func() {
			result, err := TreeNode(tt.tree, tt.filterExpr)

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
		row      api.PrettyDataRow
		expected map[string]interface{}
	}{
		{
			name: "string and int fields",
			row: api.PrettyDataRow{
				"name": api.NewTypedValue("test"),
				"age":  api.NewTypedValue(int64(30)),
			},
			expected: map[string]interface{}{
				"name": "test",
				"age":  int64(30),
			},
		},
		{
			name: "boolean and float fields",
			row: api.PrettyDataRow{
				"active": api.NewTypedValue(true),
				"score":  api.NewTypedValue(95.5),
			},
			expected: map[string]interface{}{
				"active": true,
				"score":  95.5,
			},
		},
		{
			name: "time field",
			row: api.PrettyDataRow{
				"created_at": api.NewTypedValue(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)),
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
		node     api.TreeNode
		expected map[string]interface{}
	}{
		{
			name: "simple node with label",
			node: &api.SimpleTreeNode{
				Label: "test_node",
			},
			expected: map[string]interface{}{
				"label":   "test_node",
				"content": "test_node",
			},
		},
		{
			name: "node with metadata",
			node: &api.SimpleTreeNode{
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
			node: &api.SimpleTreeNode{
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

func countTreeNodes(node api.TreeNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.GetChildren() {
		count += countTreeNodes(child)
	}
	return count
}
