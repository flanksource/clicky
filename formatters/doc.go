/*
Package formatters provides a comprehensive data formatting system with support for multiple output formats,
Tailwind CSS styling, and fluent text composition.

# Quick Start

The formatters package is designed around structs with pretty tags for automatic formatting.
Define your data structures with pretty tags to control how they appear in different output formats:

	import "github.com/flanksource/clicky"

	// Define struct for table rows (individual fields don't need table tag)
	type User struct {
		Name   string `pretty:"label=Full Name"`
		Age    int    `pretty:"color=blue"`        // Uses "Age" as column header
		City   string `pretty:"label=Location,color=gray-500"`
		Status string `pretty:"sort"`              // Uses "Status" as column header
	}

	users := []User{
		{Name: "Alice", Age: 30, City: "New York", Status: "Active"},
		{Name: "Bob", Age: 25, City: "Los Angeles", Status: "Inactive"},
	}

	// When formatting a slice directly, table format is automatically detected
	prettyStr, err := clicky.Format(users, clicky.FormatOptions{Format: "pretty"})

	// Format as JSON
	jsonStr, err := clicky.Format(users, clicky.FormatOptions{Format: "json"})

	// Format as YAML
	yamlStr, err := clicky.Format(users, clicky.FormatOptions{Format: "yaml"})

	// Format as Markdown table
	markdownStr, err := clicky.Format(users, clicky.FormatOptions{Format: "markdown"})

# Table Formatting with Pretty Tags

Tables are automatically detected when formatting struct slices. Individual struct fields
use pretty tags to control column appearance, while the `table` tag is only used on slice fields
to explicitly request table formatting:

	// Define the struct for table rows (no table tags on individual fields)
	type ServerInfo struct {
		Name     string `pretty:"label=Server Name"`
		Status   string `pretty:"color=green"`       // Uses field name "Status" as header
		Uptime   string `pretty:"color=blue"`        // Uses field name "Uptime" as header
		Memory   string `pretty:"label=Memory Usage,color=yellow,sort"`
	}

	servers := []ServerInfo{
		{Name: "web-01", Status: "Running", Uptime: "5d 12h", Memory: "2.1GB"},
		{Name: "db-01", Status: "Running", Uptime: "15d 3h", Memory: "8.4GB"},
	}

	// Format slice directly (automatically detects table format)
	output, err := clicky.Format(servers, clicky.FormatOptions{Format: "pretty"})

	// Or use container struct with explicit table tag on slice field
	type ServerReport struct {
		Servers []ServerInfo `pretty:"label=Server Status,table"`
	}
	report := ServerReport{Servers: servers}
	output, err := clicky.Format(report, clicky.FormatOptions{Format: "pretty"})

# Column Formatting Options

Individual struct fields (table columns) support these pretty tag options:

  - label=HeaderName - Sets the column header (defaults to field name)
  - color=tailwind-class - Applies color styling
  - sort - Makes column sortable
  - width=N - Sets column width
  - format=type - date, currency, percentage, etc.

Note: Column headers automatically default to the struct field name if no `label` is specified.

# Tree Formatting with TreeNode Interface

Tree formatting displays hierarchical data using the TreeNode interface and pretty struct tags.
Implement the api.TreeNode interface for custom tree visualization:

	// FileNode implements api.TreeNode interface
	type FileNode struct {
		Name     string      `json:"name"`
		Path     string      `json:"path"`
		IsDir    bool        `json:"is_dir"`
		Children []*FileNode `json:"children,omitempty"`
	}

	// Implement TreeNode interface methods
	func (f *FileNode) Pretty() api.Text {
		icon := "📄"
		style := "text-gray-600"
		if f.IsDir {
			icon = "📁"
			style = "text-blue-600 font-bold"
		}
		return api.Text{
			Content: fmt.Sprintf("%s %s", icon, f.Name),
			Style:   style,
		}
	}

	func (f *FileNode) GetChildren() []api.TreeNode {
		children := make([]api.TreeNode, len(f.Children))
		for i, child := range f.Children {
			children[i] = child
		}
		return children
	}

	// Use with pretty tag for automatic tree formatting
	type FileSystem struct {
		Root *FileNode `pretty:"tree"`
	}

	fs := FileSystem{
		Root: &FileNode{
			Name:  "project",
			IsDir: true,
			Children: []*FileNode{
				{Name: "main.go", IsDir: false},
				{Name: "utils.go", IsDir: false},
			},
		},
	}

	// Automatically formats as tree
	output, err := clicky.Format(fs, clicky.FormatOptions{Format: "tree"})

# Pretty Tags Reference

The pretty tag system provides comprehensive control over data formatting and styling.
Use these tags on struct fields to control appearance across all output formats:

# Table Tags

	type Product struct {
		ID          int     `pretty:"label=Product ID,width=10"`
		Name        string  `pretty:"color=blue"`                    // Uses "Name" as header
		Price       float64 `pretty:"label=Price,format=currency,sort"`
		InStock     bool    `pretty:"label=Available,color=green"`
		CreatedAt   time.Time `pretty:"format=date"`                 // Uses "CreatedAt" as header
	}

# Table Tag Usage

The `table` tag is used **only on slice/array fields** to explicitly request table formatting:

	type Report struct {
		Users []User `pretty:"label=User List,table"` // table tag on slice field
		Items []Item `pretty:"table"`                 // uses field name "Items" as label
	}

# Column Formatting Options

Individual struct fields (table columns) support these pretty tag options:

  - label=HeaderName - Sets the column header (defaults to field name)
  - color=tailwind-class - Applies color styling
  - sort - Makes column sortable
  - width=N - Sets column width
  - format=type - date, currency, percentage, etc.

Note: Column headers automatically default to the struct field name if no `label` is specified.

# Tree Tags

	type Organization struct {
		Department *Department `pretty:"tree"`
	}

	type Department struct {
		Name     string        `json:"name"`
		Manager  string        `json:"manager"`
		Teams    []*Department `json:"teams,omitempty"`
	}

Tree tag options:

  - tree - Marks field for tree formatting
  - icon=custom-icon - Sets custom icon for nodes

# Style Integration Tags

	type StatusReport struct {
		Message string `pretty:"color:green,font:bold"`
		Level   string `pretty:"color:yellow"`
		Time    string `pretty:"color:gray-500,italic"`
	}

Style tag options:

  - color=tailwind-class - Text colors (red, green, blue, etc.)
  - font=weight - bold, semibold, medium, normal
  - style=modifier - italic, underline, line-through
  - opacity=level - 25, 50, 75, 100

# Supported Format Types

All struct-based data works seamlessly across multiple output formats:

  - "pretty": Human-readable tables and formatting (default)
  - "json": JSON formatted output with proper indentation
  - "yaml"/"yml": YAML formatted output
  - "csv": Comma-separated values format for tabular data
  - "markdown"/"md": Markdown tables with styling
  - "html": HTML tables with Tailwind CSS classes
  - "html-pdf": HTML-to-PDF conversion for reports
  - "excel"/"xlsx": Excel spreadsheet format
  - "tree": Hierarchical tree view for nested data

# Format Options

Control output behavior with FormatOptions:

	options := clicky.FormatOptions{
		Format:  "markdown",
		NoColor: false,
		Output:  "report.md",
	}

	// Write to file
	err := clicky.FormatToFile(users, options, "report.md")

	// Or get as string
	result, err := clicky.Format(users, options)

# Tailwind CSS Support

The formatters package provides comprehensive Tailwind CSS integration through pretty tags and the api.Text system.
Tailwind utility classes are automatically parsed and applied to output:

	type StyledContent struct {
		Title   string `pretty:"color=blue-600,font=bold,text=lg"`
		Status  string `pretty:"color=green-500"`
		Footer  string `pretty:"color=gray-500,style=italic"`
	}

Supported Tailwind utilities:

# Colors
  - text-{color}-{shade} (e.g., text-blue-600, text-red-500)
  - bg-{color}-{shade} (e.g., bg-gray-100, bg-green-50)

# Typography
  - font-bold, font-semibold, font-medium, font-normal
  - italic, not-italic
  - underline, line-through
  - text-xs, text-sm, text-base, text-lg, text-xl, text-2xl, etc.

# Spacing
  - p-{value} (padding all sides)
  - px-{value}, py-{value} (horizontal/vertical padding)
  - pt-{value}, pr-{value}, pb-{value}, pl-{value} (individual sides)

# Opacity
  - opacity-25, opacity-50, opacity-75, opacity-100

# Fluent api.Text Interface

For dynamic styling, use the api.Text type with fluent interface:

	text := api.Text{Content: "Hello"}
		.Styles("font-bold", "text-blue-600")
		.Text(" World", "text-green-500")
		.Append("!", "font-semibold")
		.WrapSpace()
		.Indent(4)

Methods include:
  - Styles(classes...): Add Tailwind CSS classes
  - Text(content, styles...): Add styled child text
  - Append(content, styles...): Append styled content
  - Wrap(prefix, suffix): Wrap content with strings
  - WrapSpace(): Add spaces before and after
  - Indent(spaces): Indent content and children
  - Printf(format, args...): Add formatted content

# Integration Examples

Struct-based formatting integrates seamlessly with Go applications:

	// Web API responses
	type APIResponse struct {
		Status  string      `json:"status" pretty:"color=green"`     // Uses "Status" as header
		Message string      `json:"message"`                         // Uses "Message" as header
		Data    interface{} `json:"data" pretty:"label=Response Data"`
	}

	response := APIResponse{
		Status:  "success",
		Message: "Users retrieved",
		Data:    users,
	}

	// CLI output with format flag
	var format = flag.String("format", "pretty", "Output format")
	var noColor = flag.Bool("no-color", false, "Disable colors")
	var output = flag.String("output", "", "Output file")

	options := clicky.FormatOptions{
		Format:  *format,
		NoColor: *noColor,
	}

	if *output != "" {
		err := clicky.FormatToFile(response, options, *output)
	} else {
		result, err := clicky.Format(response, options)
		fmt.Println(result)
	}

# Advanced Usage

For complex scenarios where struct tags aren't sufficient or when working with dynamic data,
the formatters package provides programmatic construction and unstructured data handling.

# Unstructured Data Handling

When working with dynamic data from APIs or user input, use map[string]interface{}:

	// Dynamic data from external API
	data := map[string]interface{}{
		"name":   "Alice",
		"age":    30,
		"city":   "New York",
		"active": true,
	}

	// Format dynamic data
	jsonStr, err := clicky.Format(data, clicky.FormatOptions{Format: "json"})
	yamlStr, err := clicky.Format(data, clicky.FormatOptions{Format: "yaml"})
	prettyStr, err := clicky.Format(data, clicky.FormatOptions{Format: "pretty"})

	// Handle arrays of mixed data
	mixedData := []map[string]interface{}{
		{"type": "user", "name": "Alice", "role": "admin"},
		{"type": "user", "name": "Bob", "role": "user"},
	}

	tableOutput, err := clicky.Format(mixedData, clicky.FormatOptions{Format: "pretty"})

# Advanced Programmatic Construction

For maximum control over formatting, construct tables and trees programmatically:

# Advanced Table Construction

	prettyData := &api.PrettyData{
		Tables: map[string][]map[string]*api.FieldValue{
			"users": {
				{
					"name": &api.FieldValue{Value: "Alice", Style: "font-bold"},
					"age":  &api.FieldValue{Value: 30, Style: "text-blue-600"},
					"city": &api.FieldValue{Value: "New York", Style: "text-gray-500"},
				},
			},
		},
		Schema: &api.PrettyObject{
			Table: &api.TableConfig{
				Headers: map[string]string{
					"name": "Full Name",
					"age":  "Age",
					"city": "Location",
				},
				SortBy: []string{"name"},
			},
		},
	}

	output, err := clicky.Format(prettyData, clicky.FormatOptions{Format: "pretty"})

# Advanced Tree Construction

For complex trees, use the built-in api.SimpleTreeNode:

	treeData := &api.SimpleTreeNode{
		Label: "Application",
		Icon:  "📁",
		Style: "text-blue-600 font-bold",
		Children: []api.TreeNode{
			&api.SimpleTreeNode{
				Label: "Frontend",
				Icon:  "📁",
				Style: "text-blue-500",
				Children: []api.TreeNode{
					&api.SimpleTreeNode{
						Label: "components",
						Icon:  "📂",
						Style: "text-gray-600",
					},
					&api.SimpleTreeNode{
						Label: "App.tsx",
						Icon:  "📜",
						Style: "text-green-500",
					},
				},
			},
			&api.SimpleTreeNode{
				Label: "Backend",
				Icon:  "📁",
				Style: "text-blue-500",
				Children: []api.TreeNode{
					&api.SimpleTreeNode{
						Label: "main.go",
						Icon:  "🐹",
						Style: "text-green-600",
					},
					&api.SimpleTreeNode{
						Label: "handlers.go",
						Icon:  "🐹",
						Style: "text-green-600",
					},
				},
			},
		},
	}

	output, err := clicky.Format(treeData, clicky.FormatOptions{Format: "tree"})

# Tree Options

Customize tree rendering with TreeOptions:

	options := &api.TreeOptions{
		ShowIcons:    true,
		IndentSize:   4,
		UseUnicode:   true,
		MaxDepth:     3,
		Compact:      false,
	}

	formatter := formatters.NewTreeFormatter(api.DefaultTheme(), false, options)
	output := formatter.FormatTreeFromRoot(treeData)

The formatters package is designed to be the central formatting solution for all data output needs,
providing consistent styling, multiple format support, and seamless integration with existing systems.
*/
package formatters
