package formatters

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flanksource/clicky/api"
)

// MarkdownFormatter handles Markdown formatting
type MarkdownFormatter struct {
	NoColor bool
}

// NewMarkdownFormatter creates a new Markdown formatter
func NewMarkdownFormatter() *MarkdownFormatter {
	return &MarkdownFormatter{}
}

// Format formats data as Markdown
func (f *MarkdownFormatter) Format(data interface{}) (string, error) {
	// Check if data implements Pretty interface first
	if pretty, ok := data.(api.Pretty); ok {
		text := pretty.Pretty()
		return text.Markdown(), nil
	}

	// Convert to PrettyData
	prettyData, err := ToPrettyData(data)
	if err != nil {
		return "", fmt.Errorf("failed to convert to PrettyData: %w", err)
	}

	if prettyData == nil || prettyData.Schema == nil {
		return "", nil
	}

	return f.FormatPrettyData(prettyData)
}

// FormatPrettyData formats PrettyData as Markdown
func (f *MarkdownFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	var sections []string
	var summaryFields []api.PrettyField
	var tableFields []api.PrettyField
	var treeFields []api.PrettyField

	// Separate special format fields from summary fields
	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatTable {
			tableFields = append(tableFields, field)
		} else if field.Format == api.FormatTree {
			treeFields = append(treeFields, field)
		} else {
			summaryFields = append(summaryFields, field)
		}
	}

	// Format summary fields as definition list
	if len(summaryFields) > 0 {
		summaryOutput := f.formatSummaryFieldsData(summaryFields, data.Values)
		if summaryOutput != "" {
			sections = append(sections, summaryOutput)
		}
	}

	// Format tables
	for _, field := range tableFields {
		tableData, exists := data.Tables[field.Name]
		if exists && len(tableData) > 0 {
			tableOutput, err := f.formatTableData(tableData, field)
			if err != nil {
				return "", err
			}
			sections = append(sections, tableOutput)
		}
	}

	// Format tree fields
	for _, field := range treeFields {
		if fieldValue, exists := data.Values[field.Name]; exists {
			treeOutput := f.formatTreeData(field, fieldValue)
			if treeOutput != "" {
				sections = append(sections, treeOutput)
			}
		}
	}

	return strings.Join(sections, "\n\n"), nil
}

// formatSummaryFieldsData formats summary fields as Markdown definition list
func (f *MarkdownFormatter) formatSummaryFieldsData(fields []api.PrettyField, values map[string]api.FieldValue) string {
	var result strings.Builder

	for _, field := range fields {
		fieldValue, exists := values[field.Name]
		if !exists {
			continue
		}

		// Get field name
		fieldName := field.Name
		if field.Label != "" {
			fieldName = field.Label
		}

		// Check if this is an image field
		if f.isImageField(fieldValue, field) {
			imageMarkdown := f.formatImageMarkdown(fieldValue, field)
			if imageMarkdown != "" {
				result.WriteString(fmt.Sprintf("**%s**: %s\n\n", fieldName, imageMarkdown))
				continue
			}
		}

		// Use FieldValue.Markdown() method for formatted output
		value := fieldValue.Markdown()
		result.WriteString(fmt.Sprintf("**%s**: %s\n\n", fieldName, value))
	}

	return result.String()
}

// isImageField checks if a field value represents an image
func (f *MarkdownFormatter) isImageField(fieldValue api.FieldValue, field api.PrettyField) bool {
	// Check if field has image format hint
	if field.Format == "image" {
		return true
	}

	// Check if the value is a string that looks like an image URL or path
	if strValue, ok := fieldValue.Value.(string); ok {
		return f.isImageURL(strValue)
	}

	return false
}

// isImageURL checks if a string represents an image URL or path
func (f *MarkdownFormatter) isImageURL(s string) bool {
	if s == "" {
		return false
	}

	// Check for data URLs (base64 encoded images)
	if strings.HasPrefix(s, "data:image/") {
		return true
	}

	// Check for common image file extensions
	lower := strings.ToLower(s)
	imageExtensions := []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".svg", ".webp", ".ico", ".tiff", ".tif"}

	// For URLs, extract the path
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		if u, err := url.Parse(s); err == nil {
			path := strings.ToLower(u.Path)
			for _, ext := range imageExtensions {
				if strings.HasSuffix(path, ext) {
					return true
				}
			}
			// Check if the URL contains image-related keywords
			if strings.Contains(path, "/image/") || strings.Contains(path, "/img/") ||
				strings.Contains(path, "/photo/") || strings.Contains(path, "/pic/") ||
				strings.Contains(path, "/avatar/") || strings.Contains(path, "/icon/") ||
				strings.Contains(path, "/logo/") || strings.Contains(path, "/thumb/") ||
				strings.Contains(path, "/screenshot/") {
				return true
			}
		}
	} else {
		// For file paths, check extension
		ext := strings.ToLower(filepath.Ext(s))
		for _, imgExt := range imageExtensions {
			if ext == imgExt {
				return true
			}
		}
	}

	return false
}

// formatImageMarkdown formats an image field value as Markdown image syntax
func (f *MarkdownFormatter) formatImageMarkdown(fieldValue api.FieldValue, field api.PrettyField) string {
	strValue, ok := fieldValue.Value.(string)
	if !ok || strValue == "" {
		return ""
	}

	// Get alt text from field label or name
	altText := field.Label
	if altText == "" {
		altText = field.Name
	}

	// Handle data URLs (truncate for display)
	if strings.HasPrefix(strValue, "data:image/") {
		// For data URLs, we can't really display them inline in markdown
		// but we can indicate it's an embedded image
		return "[Embedded Image]"
	}

	// Return standard Markdown image syntax
	return fmt.Sprintf("![%s](%s)", altText, strValue)
}

// formatTableData formats table data as Markdown table
func (f *MarkdownFormatter) formatTableData(tableData []api.PrettyDataRow, field api.PrettyField) (string, error) {
	if len(tableData) == 0 {
		return "*No data*", nil
	}

	// Get field headers from the first row
	var headers []string
	for key := range tableData[0] {
		headers = append(headers, key)
	}
	sort.Strings(headers) // Consistent ordering

	var result strings.Builder

	// Write table header
	result.WriteString("| ")
	for _, header := range headers {
		result.WriteString(fmt.Sprintf("%s | ", header))
	}
	result.WriteString("\n")

	// Write separator
	result.WriteString("| ")
	for range headers {
		result.WriteString("--- | ")
	}
	result.WriteString("\n")

	// Write data rows
	for _, row := range tableData {
		result.WriteString("| ")
		for _, header := range headers {
			fieldValue, exists := row[header]
			var cellContent string
			if exists {
				// Check if this is an image field
				if f.isImageField(fieldValue, api.PrettyField{Name: header}) {
					imageMarkdown := f.formatImageMarkdown(fieldValue, api.PrettyField{Name: header})
					if imageMarkdown != "" {
						cellContent = imageMarkdown
					} else {
						cellContent = fieldValue.Markdown()
					}
				} else {
					// Use FieldValue.Markdown() for formatted output
					cellContent = fieldValue.Markdown()
				}
				// Escape pipe characters in cell content
				cellContent = strings.ReplaceAll(cellContent, "|", "\\|")
			}
			result.WriteString(fmt.Sprintf("%s | ", cellContent))
		}
		result.WriteString("\n")
	}

	return result.String(), nil
}

// formatTreeData formats tree data as a Markdown tree structure
func (f *MarkdownFormatter) formatTreeData(field api.PrettyField, fieldValue api.FieldValue) string {
	// Check if the value implements TreeNode interface
	if treeNode, ok := fieldValue.Value.(api.TreeNode); ok {
		// Format the tree using TreeNode methods
		return f.formatTreeNode(treeNode, 0)
	}

	// Fallback to regular markdown formatting of the value
	fieldName := field.Name
	if field.Label != "" {
		fieldName = field.Label
	}

	return fmt.Sprintf("**%s**: %s", fieldName, fieldValue.Markdown())
}

// formatTreeNode recursively formats a tree node as Markdown
func (f *MarkdownFormatter) formatTreeNode(node api.TreeNode, depth int) string {
	if node == nil {
		return ""
	}

	var result strings.Builder

	// Create indentation based on depth
	indent := strings.Repeat("  ", depth)

	// Format current node - check if it implements Pretty interface
	var nodeContent string
	if prettyNode, ok := node.(api.Pretty); ok {
		// Use Pretty() method for rich formatting
		text := prettyNode.Pretty()
		nodeContent = text.Markdown()
	} else {
		// Fallback to GetLabel()
		nodeContent = node.GetLabel()
	}

	if depth == 0 {
		// Root node - use bold
		result.WriteString(fmt.Sprintf("**%s**\n", nodeContent))
	} else {
		// Child nodes - use bullet points with indentation
		result.WriteString(fmt.Sprintf("%s- %s\n", indent, nodeContent))
	}

	// Format children recursively
	children := node.GetChildren()
	for _, child := range children {
		childOutput := f.formatTreeNode(child, depth+1)
		result.WriteString(childOutput)
	}

	return result.String()
}
