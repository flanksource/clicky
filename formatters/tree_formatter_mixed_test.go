package formatters

import (
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
)

// TestFormatMixedTreeNodes tests formatting with mixed concrete TreeNode types
func TestFormatMixedTreeNodes(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []any
		contains []string
	}{
		{
			name: "mixed SimpleTreeNode types",
			nodes: []any{
				&api.SimpleTreeNode{Label: "Node1"},
				&api.SimpleTreeNode{Label: "Node2"},
				&api.SimpleTreeNode{Label: "Node3"},
			},
			contains: []string{"Node1", "Node2", "Node3"},
		},
		{
			name: "mixed SimpleTreeNode and CompactListNode",
			nodes: []any{
				&api.SimpleTreeNode{Label: "Simple"},
				&api.CompactListNode{Label: "Compact", Items: []string{"item1", "item2"}},
			},
			contains: []string{"Simple", "Compact"},
		},
		{
			name: "variadic TreeNode arguments",
			nodes: []any{
				&api.SimpleTreeNode{Label: "First"},
				&api.SimpleTreeNode{Label: "Second"},
			},
			contains: []string{"First", "Second"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewTreeFormatter(api.DefaultTheme(), true, nil)
			result, err := formatter.Format(tt.nodes...)

			if err != nil {
				t.Fatalf("Format() error = %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Format() result should contain %q, got:\n%s", expected, result)
				}
			}
		})
	}
}

// TestToSliceWithMixedTypes tests the ToSlice helper with mixed TreeNode types
func TestToSliceWithMixedTypes(t *testing.T) {
	tests := []struct {
		name      string
		input     []any
		wantCount int
		wantOk    bool
	}{
		{
			name: "mixed TreeNode types",
			input: []any{
				&api.SimpleTreeNode{Label: "Simple"},
				&api.CompactListNode{Label: "Compact"},
			},
			wantCount: 2,
			wantOk:    true,
		},
		{
			name: "slice of mixed TreeNode types",
			input: []any{
				[]any{
					&api.SimpleTreeNode{Label: "Node1"},
					&api.SimpleTreeNode{Label: "Node2"},
				},
			},
			wantCount: 2,
			wantOk:    true,
		},
		{
			name: "nested slices of TreeNodes",
			input: []any{
				[]any{&api.SimpleTreeNode{Label: "A"}},
				[]any{&api.SimpleTreeNode{Label: "B"}},
			},
			wantCount: 2,
			wantOk:    true,
		},
		{
			name: "empty slice",
			input: []any{},
			wantCount: 0,
			wantOk:    false,
		},
		{
			name: "non-TreeNode types",
			input: []any{"string", 42},
			wantCount: 0,
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ToSlice[api.TreeNode](tt.input...)

			if ok != tt.wantOk {
				t.Errorf("ToSlice() ok = %v, want %v", ok, tt.wantOk)
			}

			if len(result) != tt.wantCount {
				t.Errorf("ToSlice() got %d nodes, want %d", len(result), tt.wantCount)
			}
		})
	}
}
