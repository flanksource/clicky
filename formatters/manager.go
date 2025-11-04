package formatters

import (
	"fmt"
	"os"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/commons/logger"
)

type FormatManager struct {
	jsonFormatter     *JSONFormatter
	yamlFormatter     *YAMLFormatter
	csvFormatter      *CSVFormatter
	markdownFormatter *MarkdownFormatter
	prettyFormatter   *PrettyFormatter
	treeFormatter     *TreeFormatter
	excelFormatter    *ExcelFormatter
}

// NewFormatManager creates a new format manager with all formatters initialized
func NewFormatManager() *FormatManager {
	return &FormatManager{
		jsonFormatter:     NewJSONFormatter(),
		yamlFormatter:     NewYAMLFormatter(),
		csvFormatter:      NewCSVFormatter(),
		markdownFormatter: NewMarkdownFormatter(),
		prettyFormatter:   NewPrettyFormatter(),
		treeFormatter:     NewTreeFormatter(api.DefaultTheme(), false, nil),
		excelFormatter:    NewExcelFormatter(),
	}
}

// ToPrettyData implements api.FormatManager.
func (f FormatManager) ToPrettyData(data interface{}) (*api.PrettyData, error) {
	return ToPrettyData(data)
}

// ToPrettyDataWithOptions converts data to PrettyData using format options
func (f FormatManager) ToPrettyDataWithOptions(data interface{}, opts FormatOptions) (*api.PrettyData, error) {
	return ToPrettyDataWithOptions(data, opts)
}

// Pretty implements api.FormatManager.
func (f FormatManager) Pretty(data interface{}) (string, error) {
	if f.prettyFormatter == nil {
		f.prettyFormatter = NewPrettyFormatter()
	}
	return f.prettyFormatter.Format(data)
}

// JSON implements api.FormatManager.
func (f FormatManager) JSON(data interface{}) (string, error) {
	if f.jsonFormatter == nil {
		f.jsonFormatter = NewJSONFormatter()
	}
	return f.jsonFormatter.Format(data)
}

// YAML implements api.FormatManager.
func (f FormatManager) YAML(data interface{}) (string, error) {
	if f.yamlFormatter == nil {
		f.yamlFormatter = NewYAMLFormatter()
	}
	return f.yamlFormatter.Format(data)
}

// CSV implements api.FormatManager.
func (f FormatManager) CSV(data interface{}) (string, error) {
	if f.csvFormatter == nil {
		f.csvFormatter = NewCSVFormatter()
	}
	return f.csvFormatter.Format(data)
}

// Markdown implements api.FormatManager.
func (f FormatManager) Markdown(data interface{}) (string, error) {
	if f.markdownFormatter == nil {
		f.markdownFormatter = NewMarkdownFormatter()
	}
	return f.markdownFormatter.Format(data)
}

// HTML implements api.FormatManager.
func (f FormatManager) HTML(data interface{}) (string, error) {
	if formatter, ok := GetCustomFormatter("html"); ok {
		return "", fmt.Errorf("html formatter not registered, registing using 'import _ github.com/flanksource/clicky/formatters/http'")
	} else {
		return formatter(data, FormatOptions{})
	}
}

func (f FormatManager) HTMLPDF(data interface{}) (string, error) {
	if formatter, ok := GetCustomFormatter("html-pdf"); ok {
		return "", fmt.Errorf("html-pdf formatter not registered, registing using 'import _ github.com/flanksource/clicky/formatters/http'")
	} else {
		return formatter(data, FormatOptions{})
	}

}

// Tree formats data as a tree structure
func (f FormatManager) Tree(data interface{}) (string, error) {
	if f.treeFormatter == nil {
		f.treeFormatter = NewTreeFormatter(api.DefaultTheme(), false, nil)
	}
	return f.treeFormatter.Format(data)
}

// Excel formats data as Excel file (requires file output)
func (f FormatManager) Excel(data interface{}) (string, error) {
	if f.excelFormatter == nil {
		f.excelFormatter = NewExcelFormatter()
	}
	return f.excelFormatter.Format(data)
}

// ExcelToFile formats data as Excel file and saves to specified path
func (f FormatManager) ExcelToFile(data interface{}, filename string) error {
	if f.excelFormatter == nil {
		f.excelFormatter = NewExcelFormatter()
	}
	return f.excelFormatter.FormatToFile(data, filename)
}

// Format implements a generic format method that delegates to specific formatters
func (f FormatManager) Format(format string, data interface{}) (string, error) {
	switch format {
	case "json":
		return f.JSON(data)
	case "yaml", "yml":
		return f.YAML(data)
	case "csv":
		return f.CSV(data)
	case "markdown", "md":
		return f.Markdown(data)
	case "html":
		return f.HTML(data)
	case "html-pdf":
		return f.HTMLPDF(data)
	case "excel", "xlsx":
		return f.Excel(data)
	case "pretty":
		return f.Pretty(data)
	case "tree":
		return f.Tree(data)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// FormatWithOptions formats data using the specified format options
// convertPrettyDataToSimple converts filtered PrettyData back to simple data structures
// Returns either a slice of maps (for table data) or a map (for struct data with tables)
func convertPrettyDataToSimple(prettyData *api.PrettyData) interface{} {
	// If there are no tables, just return original
	if len(prettyData.Tables) == 0 {
		return prettyData.Original
	}

	// Check if this was originally a slice (single table named "table")
	if table, ok := prettyData.Tables["table"]; ok && len(prettyData.Tables) == 1 {
		// This was a slice - convert table rows back to []map[string]interface{}
		var result []map[string]interface{}
		for _, row := range table {
			simpleRow := make(map[string]interface{})
			for fieldName, fieldValue := range row {
				simpleRow[fieldName] = fieldValue.Primitive()
			}
			result = append(result, simpleRow)
		}
		return result
	}

	// This was a struct with multiple fields - reconstruct with filtered tables
	result := make(map[string]interface{})

	// Add scalar values from the schema
	if prettyData.Schema != nil {
		for _, field := range prettyData.Schema.Fields {
			if field.Format != api.FormatTable {
				// Add scalar field value with lowercase field name (matching JSON convention)
				if fieldValue, ok := prettyData.Values[field.Name]; ok {
					// Use lowercase field name to match JSON tags
					fieldKey := strings.ToLower(field.Name[:1]) + field.Name[1:]
					result[fieldKey] = fieldValue.Primitive()
				}
			}
		}
	}

	// Add filtered table data as arrays
	for tableName, rows := range prettyData.Tables {
		tableArray := make([]map[string]interface{}, 0, len(rows))
		for _, row := range rows {
			simpleRow := make(map[string]interface{})
			for fieldName, fieldValue := range row {
				simpleRow[fieldName] = fieldValue.Primitive()
			}
			tableArray = append(tableArray, simpleRow)
		}
		// Use lowercase table name to match JSON tags
		tableKey := strings.ToLower(tableName[:1]) + tableName[1:]
		result[tableKey] = tableArray
	}

	return result
}

func (f FormatManager) FormatWithOptions(options FormatOptions, data ...any) (string, error) {

	if len(data) == 0 {
		return "", fmt.Errorf("no data provided for formatting")
	}
	// Resolve format from boolean flags first to check for custom formatters
	format := options.ResolveFormat()

	// Check for custom formatters BEFORE the string shortcut
	// This allows custom formatters to process strings
	if customFn, exists := GetCustomFormatter(format); exists {
		return customFn(data, options)
	}

	if len(data) == 1 {
		if s, ok := data[0].(string); ok {
			return s, nil
		}
	}

	// Handle display structure overrides (additive flags)
	// Tree flag: For text formats, use tree visual; for structured formats, pass tree data through
	// Table flag: Convert to table structure before applying format
	if options.Tree {
		return f.treeFormatter.Format(data...)
	} else if options.Table {
		logger.V(4).Infof("Applying table structure transformation before %s formatting", format)
		// Convert data to table structure first, then apply the format
		// For text-based formats, apply table formatting directly
		if format == "pretty" || format == "table" || format == "" {
			if f.prettyFormatter == nil {
				f.prettyFormatter = NewPrettyFormatter()
			}
			f.prettyFormatter.NoColor = options.NoColor
			prettyData, err := ToPrettyDataWithOptions(data, FormatOptions{Format: "table"})
			if err != nil {
				return f.prettyFormatter.Format(data)
			}
			return f.prettyFormatter.FormatPrettyData(prettyData)
		}
		// For other formats, convert data to table structure and pass through
		prettyData, err := ToPrettyDataWithOptions(data, FormatOptions{Format: "table"})
		if err == nil {
			data = []any{prettyData}
		} else {
			return "", fmt.Errorf("failed to convert data to table structure: %w", err)
		}
	}

	// If schema is provided, delegate to external handler
	// (the calling code should handle ParseDataWithSchema and call FormatWithSchema directly)

	// Handle format-specific options
	switch strings.ToLower(format) {
	case "json":
		// Apply filter if provided by converting to PrettyData first
		if options.Filter != "" && len(data) == 1 {
			prettyData, err := ToPrettyDataWithOptions(data[0], options)
			if err != nil {
				return "", fmt.Errorf("failed to apply filter: %w", err)
			}
			// Convert filtered PrettyData back to simple data
			filteredData := convertPrettyDataToSimple(prettyData)
			return f.JSON([]any{filteredData})
		}
		return f.JSON(data)

	case "yaml", "yml":
		// Apply filter if provided by converting to PrettyData first
		if options.Filter != "" && len(data) == 1 {
			prettyData, err := ToPrettyDataWithOptions(data[0], options)
			if err != nil {
				return "", fmt.Errorf("failed to apply filter: %w", err)
			}
			// Convert filtered PrettyData back to simple data
			filteredData := convertPrettyDataToSimple(prettyData)
			return f.YAML([]any{filteredData})
		}
		return f.YAML(data)

	case "csv":
		// Apply filter if provided by converting to PrettyData first
		if options.Filter != "" && len(data) == 1 {
			prettyData, err := ToPrettyDataWithOptions(data[0], options)
			if err != nil {
				return "", fmt.Errorf("failed to apply filter: %w", err)
			}
			// Convert filtered PrettyData back to simple data
			filteredData := convertPrettyDataToSimple(prettyData)
			return f.CSV([]any{filteredData})
		}
		return f.CSV(data)

	case "markdown", "md":
		if f.markdownFormatter == nil {
			f.markdownFormatter = NewMarkdownFormatter()
		}
		f.markdownFormatter.NoColor = options.NoColor
		// Convert to PrettyData first to handle pretty tags like tree
		prettyData, err := f.ToPrettyDataWithOptions(data, options)
		if err != nil {
			// Fallback to direct formatting if PrettyData conversion fails
			return f.markdownFormatter.Format(data)
		}
		return f.markdownFormatter.FormatPrettyData(prettyData, options)

	case "html", "html-pdf":
		if formatter, ok := GetCustomFormatter(format); ok {
			return "", fmt.Errorf("html formatter not registered, registing using 'import _ github.com/flanksource/clicky/formatters/http'")
		} else {
			return formatter(data, options)
		}
	case "table":
		if f.prettyFormatter == nil {
			f.prettyFormatter = NewPrettyFormatter()
		}
		f.prettyFormatter.NoColor = options.NoColor
		// Force table formatting by setting format option
		prettyData, err := ToPrettyDataWithOptions(data, FormatOptions{Format: "table"})
		if err != nil {
			// Fallback to direct formatting if PrettyData conversion fails
			return f.prettyFormatter.Format(data)
		}
		return f.prettyFormatter.FormatPrettyData(prettyData)

	case "tree":
		if f.treeFormatter == nil {
			f.treeFormatter = NewTreeFormatter(api.DefaultTheme(), options.NoColor, nil)
		}
		return f.treeFormatter.Format(data)

	case "excel", "xlsx":
		if f.excelFormatter == nil {
			f.excelFormatter = NewExcelFormatter()
		}
		return f.excelFormatter.Format(data)

	case "pretty":
		var d any
		if len(data) == 1 {
			d = data[0]
		} else {
			d = data
		}
		// Convert to PrettyData first to detect structure (tree vs table)
		prettyData, err := ToPrettyDataWithOptions(d, options)
		if err != nil {
			// Fallback to direct formatting if PrettyData conversion fails
			if f.prettyFormatter == nil {
				f.prettyFormatter = NewPrettyFormatter()
			}
			f.prettyFormatter.NoColor = options.NoColor
			return f.prettyFormatter.Format(d)
		}

		// Check if data has tree fields - if so, use tree formatter
		hasTreeField := false
		if prettyData != nil && prettyData.Schema != nil {
			for _, field := range prettyData.Schema.Fields {
				if field.Format == api.FormatTree {
					hasTreeField = true
					break
				}
			}
		}

		if hasTreeField {
			// Use tree formatter for tree-structured data
			if f.treeFormatter == nil {
				f.treeFormatter = NewTreeFormatter(api.DefaultTheme(), options.NoColor, nil)
			}
			return f.treeFormatter.FormatPrettyData(prettyData)
		}

		// Otherwise use pretty formatter with table structure
		if f.prettyFormatter == nil {
			f.prettyFormatter = NewPrettyFormatter()
		}
		f.prettyFormatter.NoColor = options.NoColor
		return f.prettyFormatter.FormatPrettyData(prettyData)

	default:
		// Default to pretty format
		if f.prettyFormatter == nil {
			f.prettyFormatter = NewPrettyFormatter()
		}
		f.prettyFormatter.NoColor = options.NoColor
		return f.prettyFormatter.Format(data)
	}
}

// FormatToFile formats data and writes to a file if output is specified
func (f FormatManager) FormatToFile(options FormatOptions, data interface{}) error {
	// Format the data
	output, err := f.FormatWithOptions(options, data)
	if err != nil {
		return fmt.Errorf("failed to format data: %w", err)
	}

	// Write to file or stdout
	if options.Output != "" {
		// Write to file
		if err := os.WriteFile(options.Output, []byte(output), 0o644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		if options.Verbose {
			fmt.Fprintf(os.Stderr, "Output written to: %s\n", options.Output)
		}
	} else {
		// Write to stdout
		fmt.Print(output)
		// Add newline if pretty format doesn't end with one
		if options.Format == "pretty" && !strings.HasSuffix(output, "\n") {
			fmt.Println()
		}
	}

	return nil
}

// ExcelExport exports data to Excel format using filename
func (f FormatManager) ExcelExport(data interface{}, filename string) error {
	if f.excelFormatter == nil {
		f.excelFormatter = NewExcelFormatter()
	}
	return f.excelFormatter.FormatToFile(data, filename)
}

// Pdf exports data to PDF format
func (f FormatManager) Pdf(data interface{}, filename string) error {
	// PDF generation would require a library like gofpdf
	// For now, we'll return an error indicating it's not implemented
	return fmt.Errorf("PDF export is not yet implemented")
}

func (f FormatManager) ParseSchema(data interface{}) (*api.PrettyObject, error) {
	// This is a no-op for the FormatManager
	d, err := f.ToPrettyData(data)
	if err != nil {
		return nil, err
	}
	return d.Schema, nil
}

// FormatWithSchema handles schema-aware formatting using provided PrettyData
func (f FormatManager) FormatWithSchema(prettyData *api.PrettyData, options FormatOptions) (string, error) {
	// Handle different output formats for schema-aware data
	switch strings.ToLower(options.Format) {
	case "json":
		// Convert PrettyData back to map for JSON output
		output := f.prettyDataToMap(prettyData)
		if f.jsonFormatter == nil {
			f.jsonFormatter = NewJSONFormatter()
		}
		// Use FormatValue directly to avoid ToPrettyData conversion
		return f.jsonFormatter.FormatValue(output)
	case "yaml", "yml":
		// Convert PrettyData back to map for YAML output
		output := f.prettyDataToMap(prettyData)
		if f.yamlFormatter == nil {
			f.yamlFormatter = NewYAMLFormatter()
		}
		return f.yamlFormatter.FormatValue(output)
	case "csv":
		if f.csvFormatter == nil {
			f.csvFormatter = NewCSVFormatter()
		}
		return f.csvFormatter.FormatPrettyData(prettyData)
	case "markdown", "md":
		if f.markdownFormatter == nil {
			f.markdownFormatter = NewMarkdownFormatter()
		}
		f.markdownFormatter.NoColor = options.NoColor
		return f.markdownFormatter.FormatPrettyData(prettyData, options)
	case "html", "html-pdf":
		formatter, ok := GetCustomFormatter(options.Format)
		if !ok {
			return "", fmt.Errorf("%s formatter not registered, registing using 'import _ github.com/flanksource/clicky/formatters/http'", options.Format)
		}
		return formatter(prettyData, options)
	default:
		// Default to pretty format
		if f.prettyFormatter == nil {
			f.prettyFormatter = NewPrettyFormatter()
		}
		f.prettyFormatter.NoColor = options.NoColor
		return f.prettyFormatter.FormatPrettyData(prettyData)
	}
}

// prettyDataToMap converts PrettyData back to a map for JSON/YAML formatting
func (f FormatManager) prettyDataToMap(data *api.PrettyData) map[string]interface{} {
	output := make(map[string]interface{})

	// Add regular field values
	for name, fieldValue := range data.Values {
		output[name] = fieldValue.Value
	}

	// Add table data as arrays
	for name, rows := range data.Tables {
		tableData := make([]map[string]interface{}, len(rows))
		for i, row := range rows {
			rowData := make(map[string]interface{})
			for k, v := range row {
				rowData[k] = v.Value
			}
			tableData[i] = rowData
		}
		output[name] = tableData
	}

	return output
}

var DEFAULT_MANAGER api.FormatManager = NewFormatManager()
