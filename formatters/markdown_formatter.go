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

	return f.FormatPrettyData(prettyData, FormatOptions{})
}

// FormatPrettyData formats PrettyData as Markdown
func (f *MarkdownFormatter) FormatPrettyData(data *api.PrettyData, opts FormatOptions) (string, error) {
	var sections []string
	var summaryFields []api.PrettyField
	var tableFields []api.PrettyField
	var treeFields []api.PrettyField

	// Separate special format fields from summary fields
	for _, field := range data.Schema.Fields {
		switch field.Format {
		case api.FormatTable:
			tableFields = append(tableFields, field)
		case api.FormatTree:
			treeFields = append(treeFields, field)
		default:
			summaryFields = append(summaryFields, field)
		}
	}

	// Format summary fields as definition list
	if len(summaryFields) > 0 {
		summaryOutput := f.formatSummaryFieldsData(summaryFields, data.Values, opts)
		if summaryOutput != "" {
			sections = append(sections, summaryOutput)
		}
	}

	// Format tables
	for _, field := range tableFields {
		tableData, exists := data.Tables[field.Name]
		if exists && len(tableData) > 0 {
			tableOutput, err := f.formatTableData(tableData, field, opts)
			if err != nil {
				return "", err
			}
			sections = append(sections, tableOutput)
		}
	}

	// Format tree fields
	for _, field := range treeFields {
		if fieldValue, exists := data.Values[field.Name]; exists {
			treeOutput := f.formatTreeData(field, fieldValue, opts)
			if treeOutput != "" {
				sections = append(sections, treeOutput)
			}
		}
	}

	return strings.Join(sections, "\n\n"), nil
}

// formatSummaryFieldsData formats summary fields as Markdown definition list
func (f *MarkdownFormatter) formatSummaryFieldsData(fields []api.PrettyField, values map[string]api.FieldValue, opts FormatOptions) string {
	var result strings.Builder
	depth := opts.Depth()

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

		// Check for nested PrettyData
		if nestedData, ok := fieldValue.Value.(*api.PrettyData); ok {
			// Recursively format with increased depth
			nestedOutput, _ := f.FormatPrettyData(nestedData, opts.IncreaseDepth())

			// Add section heading based on depth
			heading := strings.Repeat("#", depth+2) + " " + fieldName
			result.WriteString(heading + "\n\n" + nestedOutput + "\n\n")
			continue
		}

		// Check if this is an image field
		if f.isImageField(fieldValue, field) {
			imageMarkdown := f.formatImageMarkdown(fieldValue, field)
			if imageMarkdown != "" {
				result.WriteString(fmt.Sprintf("**%s**: %s\n\n", fieldName, imageMarkdown))
				continue
			}
		}

		// Handle maps - ProcessFieldValue already normalizes maps
		if mapVal, ok := fieldValue.Value.(map[string]interface{}); ok {
			mapOutput := f.formatMapMarkdown(mapVal, opts)
			result.WriteString(fmt.Sprintf("**%s**:\n\n%s\n", fieldName, mapOutput))
			continue
		}

		// Handle slices - ProcessFieldValue already normalizes slices
		if sliceVal, ok := fieldValue.Value.([]interface{}); ok {
			sliceOutput := f.formatSliceMarkdown(sliceVal, opts)
			result.WriteString(fmt.Sprintf("**%s**: %s\n\n", fieldName, sliceOutput))
			continue
		}

		// Handle TreeNode fields
		if field.Format == api.FormatTree {
			if treeNode, ok := fieldValue.Value.(api.TreeNode); ok {
				treeOutput := f.formatTreeNode(treeNode, depth)
				result.WriteString(fmt.Sprintf("**%s**:\n\n%s\n", fieldName, treeOutput))
				continue
			}
		}

		// Use Text.Markdown() method for formatted output
		value := ""
		if fieldValue.Text != nil {
			value = fieldValue.Text.Markdown()
		} else {
			value = fmt.Sprintf("%v", fieldValue.Value)
		}
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
func (f *MarkdownFormatter) formatTableData(tableData []api.PrettyDataRow, _ api.PrettyField, opts FormatOptions) (string, error) {
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
						if fieldValue.Text != nil {
							cellContent = fieldValue.Text.Markdown()
						} else {
							cellContent = fmt.Sprintf("%v", fieldValue.Value)
						}
					}
				} else {
					// Use Text.Markdown() for formatted output
					if fieldValue.Text != nil {
						cellContent = fieldValue.Text.Markdown()
					} else {
						cellContent = fmt.Sprintf("%v", fieldValue.Value)
					}
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
func (f *MarkdownFormatter) formatTreeData(field api.PrettyField, fieldValue api.FieldValue, opts FormatOptions) string {
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

	value := ""
	if fieldValue.Text != nil {
		value = fieldValue.Text.Markdown()
	} else {
		value = fmt.Sprintf("%v", fieldValue.Value)
	}
	return fmt.Sprintf("**%s**: %s", fieldName, value)
}

// formatTreeNode recursively formats a tree node as Markdown
func (f *MarkdownFormatter) formatTreeNode(node api.TreeNode, depth int) string {
	if node == nil {
		return ""
	}

	var result strings.Builder

	// Create indentation based on depth
	indent := strings.Repeat("  ", depth)

	if depth == 0 {
		// Root node - use bold
		result.WriteString(fmt.Sprintf("**%s**\n", node.Pretty().Markdown()))
	} else {
		// Child nodes - use bullet points with indentation
		result.WriteString(fmt.Sprintf("%s- %s\n", indent, node.Pretty().Markdown()))
	}

	// Format children recursively
	children := node.GetChildren()
	for _, child := range children {
		childOutput := f.formatTreeNode(child, depth+1)
		result.WriteString(childOutput)
	}

	return result.String()
}

// formatMapMarkdown formats a map with proper indentation based on depth
func (f *MarkdownFormatter) formatMapMarkdown(mapVal map[string]interface{}, opts FormatOptions) string {
	if len(mapVal) == 0 {
		return "{}"
	}

	depth := opts.Depth()
	indent := strings.Repeat("  ", depth)

	// Sort keys for consistent output
	keys := make([]string, 0, len(mapVal))
	for k := range mapVal {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var result strings.Builder
	for _, key := range keys {
		value := mapVal[key]

		// Check if value is nested PrettyData
		if nestedData, ok := value.(*api.PrettyData); ok {
			nestedOutput, _ := f.FormatPrettyData(nestedData, opts.IncreaseDepth())
			result.WriteString(fmt.Sprintf("%s**%s**:\n\n%s\n", indent, key, nestedOutput))
			continue
		}

		// Check if value is nested map
		if nestedMap, ok := value.(map[string]interface{}); ok {
			nestedOutput := f.formatMapMarkdown(nestedMap, opts.IncreaseDepth())
			result.WriteString(fmt.Sprintf("%s**%s**:\n%s\n", indent, key, nestedOutput))
			continue
		}

		// Handle nil values - ProcessFieldValue already normalizes nil pointers to nil
		if value == nil {
			result.WriteString(fmt.Sprintf("%s**%s**: —\n", indent, key))
			continue
		}

		// Handle slices - ProcessFieldValue already normalizes to []interface{}
		if sliceVal, ok := value.([]interface{}); ok {
			sliceOutput := f.formatSliceMarkdown(sliceVal, opts.IncreaseDepth())
			result.WriteString(fmt.Sprintf("%s**%s**: %s\n", indent, key, sliceOutput))
			continue
		}

		// Simple value
		result.WriteString(fmt.Sprintf("%s**%s**: %v\n", indent, key, value))
	}

	return result.String()
}

// formatSliceMarkdown formats a slice based on element types and depth
func (f *MarkdownFormatter) formatSliceMarkdown(arrayVal []interface{}, opts FormatOptions) string {
	if len(arrayVal) == 0 {
		return "[]"
	}

	depth := opts.Depth()
	indent := strings.Repeat("  ", depth)

	// Check if all elements are primitives
	// ProcessFieldValue normalizes structs to maps and dereferences pointers,
	// so we only need to check for maps and slices
	allPrimitives := true
	for _, elem := range arrayVal {
		if elem == nil {
			continue
		}
		_, isMap := elem.(map[string]interface{})
		_, isSlice := elem.([]interface{})
		if isMap || isSlice {
			allPrimitives = false
			break
		}
	}

	// Inline for primitive slices with 5 or fewer elements
	if allPrimitives && len(arrayVal) <= 5 {
		strs := make([]string, len(arrayVal))
		for i, v := range arrayVal {
			if v == nil {
				strs[i] = "—"
			} else {
				strs[i] = fmt.Sprintf("%v", v)
			}
		}
		return "[" + strings.Join(strs, ", ") + "]"
	}

	// Bullet list for complex types or long lists
	var result strings.Builder
	for _, elem := range arrayVal {
		// Check for nested PrettyData
		if nestedData, ok := elem.(*api.PrettyData); ok {
			nestedOutput, _ := f.FormatPrettyData(nestedData, opts.IncreaseDepth())
			result.WriteString(fmt.Sprintf("%s- %s\n", indent, nestedOutput))
			continue
		}

		// Check for nested map
		if nestedMap, ok := elem.(map[string]interface{}); ok {
			mapOutput := f.formatMapMarkdown(nestedMap, opts.IncreaseDepth())
			result.WriteString(fmt.Sprintf("%s- \n%s", indent, mapOutput))
			continue
		}

		// Handle nil
		if elem == nil {
			result.WriteString(fmt.Sprintf("%s- —\n", indent))
			continue
		}

		// Simple value
		result.WriteString(fmt.Sprintf("%s- %v\n", indent, elem))
	}

	return result.String()
}
