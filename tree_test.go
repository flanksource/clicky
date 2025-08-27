package clicky

import (
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

func TestTreeRendering(t *testing.T) {
	// Create a simple tree structure
	root := &api.SimpleTreeNode{
		Label: "Project",
		Icon:  "📁",
		Style: "text-blue-600 font-bold",
		Children: []api.TreeNode{
			&api.SimpleTreeNode{
				Label: "src",
				Icon:  "📁",
				Style: "text-blue-500",
				Children: []api.TreeNode{
					&api.SimpleTreeNode{
						Label: "main.go",
						Icon:  "🐹",
						Style: "text-green-500",
					},
					&api.SimpleTreeNode{
						Label: "utils.go",
						Icon:  "🐹",
						Style: "text-green-500",
					},
				},
			},
			&api.SimpleTreeNode{
				Label: "README.md",
				Icon:  "📝",
				Style: "text-gray-500",
			},
		},
	}

	// Test tree formatting
	formatter := formatters.NewTreeFormatter(api.DefaultTheme(), true, api.DefaultTreeOptions())
	output := formatter.FormatTreeFromRoot(root)

	// Check that output contains expected elements
	if !strings.Contains(output, "Project") {
		t.Error("Tree output should contain 'Project'")
	}
	if !strings.Contains(output, "src") {
		t.Error("Tree output should contain 'src'")
	}
	if !strings.Contains(output, "main.go") {
		t.Error("Tree output should contain 'main.go'")
	}
	if !strings.Contains(output, "├──") || !strings.Contains(output, "└──") {
		t.Error("Tree output should contain tree characters")
	}
	if !strings.Contains(output, "📁") {
		t.Error("Tree output should contain folder icons")
	}

	t.Logf("Tree output:\n%s", output)
}

func TestCompactListNode(t *testing.T) {
	// Create a tree with compact list nodes
	root := &api.SimpleTreeNode{
		Label: "Calculator",
		Icon:  "🏗️",
		Children: []api.TreeNode{
			&api.CompactListNode{
				Label: "Methods",
				Icon:  "⚡",
				Items: []string{"add:10", "multiply:25", "divide:40"},
			},
			&api.CompactListNode{
				Label: "Fields",
				Icon:  "📊",
				Items: []string{"value:5", "result:8"},
			},
		},
	}

	opts := api.DefaultTreeOptions()
	opts.Compact = true
	formatter := formatters.NewTreeFormatter(api.DefaultTheme(), true, opts)
	output := formatter.FormatTreeFromRoot(root)

	// Check compact list formatting
	if !strings.Contains(output, "add:10") {
		t.Error("Output should contain method 'add:10'")
	}
	if !strings.Contains(output, "multiply:25, divide:40") || !strings.Contains(output, "multiply:25") {
		t.Error("Output should contain methods in compact format")
	}

	t.Logf("Compact tree output:\n%s", output)
}

func TestASCIITreeOptions(t *testing.T) {
	// Create a simple tree
	root := &api.SimpleTreeNode{
		Label: "Root",
		Children: []api.TreeNode{
			&api.SimpleTreeNode{Label: "Child1"},
			&api.SimpleTreeNode{Label: "Child2"},
		},
	}

	// Test with ASCII options
	formatter := formatters.NewTreeFormatter(api.DefaultTheme(), true, api.ASCIITreeOptions())
	output := formatter.FormatTreeFromRoot(root)

	// Check for ASCII characters instead of Unicode
	if strings.Contains(output, "├──") || strings.Contains(output, "└──") {
		t.Error("ASCII mode should not contain Unicode box characters")
	}
	if !strings.Contains(output, "+--") || !strings.Contains(output, "`--") {
		t.Error("ASCII mode should contain ASCII tree characters")
	}

	t.Logf("ASCII tree output:\n%s", output)
}

func TestCustomRenderFunction(t *testing.T) {
	// Register a test render function
	api.RegisterRenderFunc("test_render", func(value interface{}, field api.PrettyField, theme api.Theme) string {
		return "CUSTOM:" + formatters.RenderComplexityColored(value, field, theme)
	})

	// Test struct with custom render function
	type TestStruct struct {
		Complexity int `json:"complexity" pretty:"int,render=test_render"`
	}

	parser := NewPrettyParser()
	parser.NoColor = true // Disable colors for testing

	test := TestStruct{Complexity: 8}
	output, err := parser.Parse(test)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if !strings.Contains(output, "CUSTOM:") {
		t.Error("Output should use custom render function")
	}

	t.Logf("Custom render output: %s", output)
}

func TestTreeWithPrettyTags(t *testing.T) {
	// Test struct with tree pretty tag
	type FileTree struct {
		Root api.TreeNode `json:"root" pretty:"tree,indent=4,no_icons"`
	}

	tree := FileTree{
		Root: &api.SimpleTreeNode{
			Label: "project",
			Children: []api.TreeNode{
				&api.SimpleTreeNode{Label: "src"},
				&api.SimpleTreeNode{Label: "test"},
			},
		},
	}

	parser := NewPrettyParser()
	parser.NoColor = true

	output, err := parser.Parse(tree)
	if err != nil {
		t.Fatalf("Failed to parse tree: %v", err)
	}

	// Should render as tree
	if !strings.Contains(output, "project") {
		t.Error("Tree should contain root label")
	}
	if !strings.Contains(output, "src") && !strings.Contains(output, "test") {
		t.Error("Tree should contain child nodes")
	}

	t.Logf("Tree with tags output:\n%s", output)
}

func TestBuiltinRenderers(t *testing.T) {
	tests := []struct {
		name     string
		renderer string
		value    interface{}
		contains string
	}{
		{
			name:     "AST Node",
			renderer: "ast_node",
			value: map[string]interface{}{
				"name":       "calculate",
				"line":       42,
				"complexity": 7,
			},
			contains: "calculate:42",
		},
		{
			name:     "Compact Methods",
			renderer: "compact_methods",
			value: []interface{}{
				map[string]interface{}{"name": "add", "line": 10},
				map[string]interface{}{"name": "sub", "line": 20},
			},
			contains: "add:10, sub:20",
		},
		{
			name:     "Line Number",
			renderer: "line_number",
			value:    123,
			contains: "L123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, exists := api.RenderFuncRegistry[tt.renderer]
			if !exists {
				t.Fatalf("Renderer %s not found", tt.renderer)
			}

			output := fn(tt.value, api.PrettyField{}, api.DefaultTheme())
			if !strings.Contains(output, tt.contains) {
				t.Errorf("Expected output to contain '%s', got: %s", tt.contains, output)
			}
		})
	}
}

func TestTreeMaxDepth(t *testing.T) {
	// Create a deep tree
	child3 := &api.SimpleTreeNode{Label: "Level3"}
	child2 := &api.SimpleTreeNode{Label: "Level2", Children: []api.TreeNode{child3}}
	child1 := &api.SimpleTreeNode{Label: "Level1", Children: []api.TreeNode{child2}}
	root := &api.SimpleTreeNode{Label: "Root", Children: []api.TreeNode{child1}}

	// Test with max depth = 2
	opts := api.DefaultTreeOptions()
	opts.MaxDepth = 2
	formatter := formatters.NewTreeFormatter(api.DefaultTheme(), true, opts)
	output := formatter.FormatTreeFromRoot(root)

	// Should contain up to Level2 but not Level3
	if !strings.Contains(output, "Level1") {
		t.Error("Should contain Level1")
	}
	if !strings.Contains(output, "Level2") {
		t.Error("Should contain Level2")
	}
	if strings.Contains(output, "Level3") {
		t.Error("Should not contain Level3 (exceeds max depth)")
	}

	t.Logf("Max depth output:\n%s", output)
}
