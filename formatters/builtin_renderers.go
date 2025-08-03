package formatters

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/clicky/api"
)

func init() {
	// Register common render functions
	api.RegisterRenderFunc("ast_node", RenderASTNode)
	api.RegisterRenderFunc("file_tree", RenderFileTree)
	api.RegisterRenderFunc("compact_methods", RenderCompactMethods)
	api.RegisterRenderFunc("complexity_colored", RenderComplexityColored)
	api.RegisterRenderFunc("line_number", RenderLineNumber)
	api.RegisterRenderFunc("icon_label", RenderIconLabel)
}

// RenderASTNode renders an AST node in compact format
func RenderASTNode(value interface{}, field api.PrettyField, theme api.Theme) string {
	// Expected format: map with name, line, and complexity fields
	switch v := value.(type) {
	case map[string]interface{}:
		name := fmt.Sprintf("%v", v["name"])
		line := 0
		if l, ok := v["line"].(int); ok {
			line = l
		}
		complexity := 0
		if c, ok := v["complexity"].(int); ok {
			complexity = c
		}
		
		result := fmt.Sprintf("%s:%d", name, line)
		if complexity > 0 {
			complexityStr := fmt.Sprintf("(c:%d)", complexity)
			// Color based on complexity
			style := lipgloss.NewStyle()
			if complexity > 10 {
				style = style.Foreground(theme.Error)
			} else if complexity > 5 {
				style = style.Foreground(theme.Warning)
			} else {
				style = style.Foreground(theme.Success)
			}
			result += style.Render(complexityStr)
		}
		return result
	case string:
		return v
	default:
		return fmt.Sprintf("%v", value)
	}
}

// RenderFileTree renders a file tree node with appropriate icons
func RenderFileTree(value interface{}, field api.PrettyField, theme api.Theme) string {
	switch v := value.(type) {
	case api.TreeNode:
		formatter := NewTreeFormatter(theme, false, field.TreeOptions)
		return formatter.FormatTreeFromRoot(v)
	case map[string]interface{}:
		path := fmt.Sprintf("%v", v["path"])
		isDir := false
		if d, ok := v["isDir"].(bool); ok {
			isDir = d
		}
		
		icon := "📄"
		if isDir {
			icon = "📁"
		}
		
		// Special icons for specific file types
		if strings.HasSuffix(path, ".go") {
			icon = "🐹"
		} else if strings.HasSuffix(path, ".py") {
			icon = "🐍"
		} else if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".ts") {
			icon = "📜"
		} else if strings.HasSuffix(path, ".md") {
			icon = "📝"
		}
		
		style := lipgloss.NewStyle().Foreground(theme.Info)
		return fmt.Sprintf("%s %s", icon, style.Render(path))
	default:
		return fmt.Sprintf("%v", value)
	}
}

// RenderCompactMethods renders a list of methods in compact format
func RenderCompactMethods(value interface{}, field api.PrettyField, theme api.Theme) string {
	switch v := value.(type) {
	case []interface{}:
		var items []string
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				name := fmt.Sprintf("%v", m["name"])
				line := 0
				if l, ok := m["line"].(int); ok {
					line = l
				}
				complexity := 0
				if c, ok := m["complexity"].(int); ok {
					complexity = c
				}
				
				itemStr := fmt.Sprintf("%s:%d", name, line)
				if complexity > 0 {
					itemStr += fmt.Sprintf("(c:%d)", complexity)
				}
				items = append(items, itemStr)
			} else {
				items = append(items, fmt.Sprintf("%v", item))
			}
		}
		
		// Join with comma and wrap if needed
		formatter := NewTreeFormatter(theme, false, nil)
		return formatter.FormatCompactList(items, ", ")
	case []string:
		formatter := NewTreeFormatter(theme, false, nil)
		return formatter.FormatCompactList(v, ", ")
	default:
		return fmt.Sprintf("%v", value)
	}
}

// RenderComplexityColored renders a complexity value with color coding
func RenderComplexityColored(value interface{}, field api.PrettyField, theme api.Theme) string {
	var complexity int
	switch v := value.(type) {
	case int:
		complexity = v
	case int64:
		complexity = int(v)
	case float64:
		complexity = int(v)
	default:
		return fmt.Sprintf("%v", value)
	}
	
	style := lipgloss.NewStyle()
	if complexity > 10 {
		style = style.Foreground(theme.Error).Bold(true)
	} else if complexity > 5 {
		style = style.Foreground(theme.Warning)
	} else if complexity > 0 {
		style = style.Foreground(theme.Success)
	} else {
		style = style.Foreground(theme.Muted)
	}
	
	return style.Render(fmt.Sprintf("%d", complexity))
}

// RenderLineNumber renders a line number with formatting
func RenderLineNumber(value interface{}, field api.PrettyField, theme api.Theme) string {
	lineStr := fmt.Sprintf("%v", value)
	style := lipgloss.NewStyle().Foreground(theme.Muted)
	return style.Render(fmt.Sprintf("L%s", lineStr))
}

// RenderIconLabel renders a value with an icon prefix
func RenderIconLabel(value interface{}, field api.PrettyField, theme api.Theme) string {
	text := fmt.Sprintf("%v", value)
	
	// Get icon from field metadata
	icon := field.FormatOptions["icon"]
	if icon == "" {
		// Auto-detect icon based on content
		lower := strings.ToLower(text)
		switch {
		case strings.Contains(lower, "error"):
			icon = "❌"
		case strings.Contains(lower, "warning"):
			icon = "⚠️"
		case strings.Contains(lower, "success"):
			icon = "✅"
		case strings.Contains(lower, "info"):
			icon = "ℹ️"
		case strings.Contains(lower, "class"):
			icon = "🏗️"
		case strings.Contains(lower, "function") || strings.Contains(lower, "method"):
			icon = "⚡"
		case strings.Contains(lower, "variable") || strings.Contains(lower, "field"):
			icon = "📊"
		default:
			icon = "•"
		}
	}
	
	// Apply style if specified
	if field.Style != "" {
		text = applyStyle(text, field.Style, theme)
	}
	
	return fmt.Sprintf("%s %s", icon, text)
}

// applyStyle applies a style string to text
func applyStyle(text string, styleStr string, theme api.Theme) string {
	style := lipgloss.NewStyle()
	
	// Parse style string (simplified)
	styles := strings.Fields(styleStr)
	for _, s := range styles {
		switch {
		case strings.HasPrefix(s, "text-blue"):
			style = style.Foreground(theme.Info)
		case strings.HasPrefix(s, "text-green"):
			style = style.Foreground(theme.Success)
		case strings.HasPrefix(s, "text-red"):
			style = style.Foreground(theme.Error)
		case strings.HasPrefix(s, "text-yellow"):
			style = style.Foreground(theme.Warning)
		case strings.HasPrefix(s, "text-muted"):
			style = style.Foreground(theme.Muted)
		case s == "bold":
			style = style.Bold(true)
		case s == "italic":
			style = style.Italic(true)
		case s == "underline":
			style = style.Underline(true)
		}
	}
	
	return style.Render(text)
}