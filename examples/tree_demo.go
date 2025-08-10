package main

import (
	"fmt"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
)

// FileNode represents a file or directory in a tree
type FileNode struct {
	Name     string          `json:"name" pretty:"label,style=text-blue-600"`
	Type     string          `json:"type"`
	Children []*FileNode     `json:"children,omitempty" pretty:"tree"`
	Methods  []MethodInfo   `json:"methods,omitempty" pretty:"compact,render=compact_methods"`
}

// MethodInfo represents a method with complexity
type MethodInfo struct {
	Name       string `json:"name"`
	Line       int    `json:"line"`
	Complexity int    `json:"complexity"`
}

// Implement TreeNode interface
func (f *FileNode) GetLabel() string {
	return f.Name
}

func (f *FileNode) GetChildren() []api.TreeNode {
	nodes := make([]api.TreeNode, len(f.Children))
	for i, child := range f.Children {
		nodes[i] = child
	}
	return nodes
}

func (f *FileNode) GetIcon() string {
	switch f.Type {
	case "dir":
		return "📁"
	case "go":
		return "🐹"
	case "py":
		return "🐍"
	case "js":
		return "📜"
	default:
		return "📄"
	}
}

func (f *FileNode) GetStyle() string {
	if f.Type == "dir" {
		return "text-blue-600 font-bold"
	}
	return "text-green-500"
}

func (f *FileNode) IsLeaf() bool {
	return len(f.Children) == 0
}

func main() {
	// Create a sample project structure
	project := &FileNode{
		Name: "my-project",
		Type: "dir",
		Children: []*FileNode{
			{
				Name: "src",
				Type: "dir",
				Children: []*FileNode{
					{
						Name: "main.go",
						Type: "go",
						Methods: []MethodInfo{
							{Name: "main", Line: 10, Complexity: 3},
							{Name: "init", Line: 25, Complexity: 5},
						},
					},
					{
						Name: "utils.go",
						Type: "go",
						Methods: []MethodInfo{
							{Name: "parseConfig", Line: 15, Complexity: 8},
							{Name: "validateInput", Line: 45, Complexity: 12},
							{Name: "formatOutput", Line: 78, Complexity: 4},
						},
					},
				},
			},
			{
				Name: "tests",
				Type: "dir",
				Children: []*FileNode{
					{
						Name: "main_test.go",
						Type: "go",
						Methods: []MethodInfo{
							{Name: "TestMain", Line: 8, Complexity: 2},
							{Name: "TestConfig", Line: 20, Complexity: 6},
						},
					},
				},
			},
			{
				Name: "README.md",
				Type: "md",
			},
		},
	}

	// Use PrettyParser to render the tree
	parser := clicky.NewPrettyParser()
	
	// Wrap in a struct to use pretty tags
	type ProjectTree struct {
		Root *FileNode `json:"root" pretty:"tree"`
	}
	
	tree := ProjectTree{Root: project}
	output, err := parser.Parse(tree)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Project Structure:")
	fmt.Println(output)

	// Also demonstrate with custom tree options
	fmt.Println("\n--- ASCII Version ---")
	parser.NoColor = true
	asciiTree := struct {
		Root *FileNode `json:"root" pretty:"tree,ascii,no_icons"`
	}{Root: project}
	
	output, err = parser.Parse(asciiTree)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(output)
}