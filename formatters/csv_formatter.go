package formatters

import (
	"encoding/csv"
	"fmt"
	"sort"
	"strings"
	
	"github.com/flanksource/clicky/api"
)

// CSVFormatter handles CSV formatting
type CSVFormatter struct {
	Separator rune
}

// NewCSVFormatter creates a new CSV formatter
func NewCSVFormatter() *CSVFormatter {
	return &CSVFormatter{
		Separator: ',',
	}
}

// Format formats data as CSV
func (f *CSVFormatter) Format(data interface{}) (string, error) {
	// Check if data implements Pretty interface first
	if pretty, ok := data.(api.Pretty); ok {
		text := pretty.Pretty()
		return text.String(), nil // Use plain text for CSV
	}

	// Convert to PrettyData (handles both structs and slices)
	prettyData, err := ToPrettyData(data)
	if err != nil {
		return "", fmt.Errorf("failed to convert to PrettyData: %w", err)
	}
	
	if prettyData == nil || prettyData.Schema == nil {
		return "", nil
	}

	return f.FormatPrettyData(prettyData)
}


// FormatPrettyData formats PrettyData as CSV, flattening all fields
func (f *CSVFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	if data == nil || data.Schema == nil {
		return "", nil
	}

	var output strings.Builder
	writer := csv.NewWriter(&output)
	writer.Comma = f.Separator

	// Check if this is primarily table data (from a slice)
	// If there's exactly one table field and no regular fields, format as table
	var tableField *api.PrettyField
	var treeField *api.PrettyField
	var nonTableFields []api.PrettyField
	
	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatTable {
			tableField = &field
		} else if field.Format == api.FormatTree {
			treeField = &field
		} else {
			nonTableFields = append(nonTableFields, field)
		}
	}

	// If we have tree data as the primary field, flatten it to CSV
	if treeField != nil && len(nonTableFields) == 0 && tableField == nil {
		if fieldValue, exists := data.Values[treeField.Name]; exists {
			// Check if the value implements TreeNode interface
			if treeNode, ok := fieldValue.Value.(api.TreeNode); ok {
				// Flatten tree to CSV rows
				rows := f.flattenTree(treeNode, 0)
				
				// Write headers
				headers := []string{"Level", "Name", "Details"}
				if err := writer.Write(headers); err != nil {
					return "", err
				}
				
				// Write rows
				for _, row := range rows {
					if err := writer.Write(row); err != nil {
						return "", err
					}
				}
			} else {
				// Fall back to regular formatting if not a tree
				headers := []string{treeField.Name}
				values := []string{fieldValue.Plain()}
				if err := writer.Write(headers); err != nil {
					return "", err
				}
				if err := writer.Write(values); err != nil {
					return "", err
				}
			}
		}
	} else if tableField != nil && len(nonTableFields) == 0 {
		// If we have table data and it's the primary data, format it as CSV rows
		if tableData, exists := data.Tables[tableField.Name]; exists && len(tableData) > 0 {
			// Get headers from the first row
			var headers []string
			for key := range tableData[0] {
				headers = append(headers, key)
			}
			
			// Sort headers for consistent output
			sort.Strings(headers)
			
			// Write headers
			if err := writer.Write(headers); err != nil {
				return "", err
			}
			
			// Write data rows
			for _, row := range tableData {
				var values []string
				for _, header := range headers {
					if fieldValue, exists := row[header]; exists {
						values = append(values, fieldValue.Plain())
					} else {
						values = append(values, "")
					}
				}
				if err := writer.Write(values); err != nil {
					return "", err
				}
			}
		}
	} else {
		// Format as single row with headers (original behavior for structs)
		var headers []string
		var values []string

		// Process regular fields (non-table, non-tree)
		for _, field := range data.Schema.Fields {
			if field.Format == api.FormatTable || field.Format == api.FormatTree {
				continue
			}
			
			if fieldValue, exists := data.Values[field.Name]; exists {
				headers = append(headers, field.Name)
				values = append(values, fieldValue.Plain())
			}
		}

		// Write headers and values if we have any
		if len(headers) > 0 {
			if err := writer.Write(headers); err != nil {
				return "", err
			}
			
			if err := writer.Write(values); err != nil {
				return "", err
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return output.String(), nil
}

// flattenTree recursively flattens a tree node into CSV rows
func (f *CSVFormatter) flattenTree(node api.TreeNode, depth int) [][]string {
	if node == nil {
		return nil
	}
	
	var rows [][]string
	
	// Format current node
	var nodeContent string
	if prettyNode, ok := node.(api.Pretty); ok {
		// Use Pretty() method for rich formatting, but get plain text for CSV
		text := prettyNode.Pretty()
		nodeContent = text.String()
	} else {
		// Fallback to GetLabel()
		nodeContent = node.GetLabel()
	}
	
	// Create indentation based on depth
	indent := strings.Repeat("  ", depth)
	
	// Add current node as a row
	row := []string{
		fmt.Sprintf("%d", depth),           // Level
		fmt.Sprintf("%s%s", indent, nodeContent), // Name with indentation
		"",                                  // Details (could be extended)
	}
	rows = append(rows, row)
	
	// Process children recursively
	children := node.GetChildren()
	for _, child := range children {
		childRows := f.flattenTree(child, depth+1)
		rows = append(rows, childRows...)
	}
	
	return rows
}
