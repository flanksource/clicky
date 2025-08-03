package clicky

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

// SchemaFormatter handles schema-based formatting operations
type SchemaFormatter struct {
	Schema        *api.PrettyObject
	Parser        *StructParser
}

// FormatOptions contains options for formatting operations
type FormatOptions struct {
	Format  string
	NoColor bool
	Output  string
	Verbose bool
}

// NewSchemaFormatter creates a new schema formatter with the given schema file
func NewSchemaFormatter(schemaFile string) (*SchemaFormatter, error) {
	parser := NewStructParser()
	schema, err := parser.LoadSchemaFromYAML(schemaFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}

	return &SchemaFormatter{
		Schema:        schema,
		Parser:        parser,
	}, nil
}

// LoadSchemaFromYAML creates a SchemaFormatter from a YAML schema file
func LoadSchemaFromYAML(schemaFile string) (*SchemaFormatter, error) {
	return NewSchemaFormatter(schemaFile)
}

// FormatFile formats a single data file using the schema
func (sf *SchemaFormatter) FormatFile(dataFile string, options FormatOptions) (string, error) {
	// Load and parse data
	data, err := sf.loadDataFile(dataFile)
	if err != nil {
		return "", fmt.Errorf("failed to load data file %s: %w", dataFile, err)
	}

	// Parse data with schema into PrettyData
	prettyData, err := sf.Parser.ParseDataWithSchema(data, sf.Schema)
	if err != nil {
		return "", fmt.Errorf("failed to parse data with schema: %w", err)
	}

	// NoColor option is handled by individual formatters

	// Format output
	return sf.formatWithPrettyData(prettyData, options.Format)
}

// FormatFiles formats multiple data files using the schema
func (sf *SchemaFormatter) FormatFiles(dataFiles []string, options FormatOptions) error {
	for _, dataFile := range dataFiles {

		result, err := sf.FormatFile(dataFile, options)
		if err != nil {
			if options.Verbose {
				fmt.Printf("Error processing %s: %v\n", dataFile, err)
			}
			continue
		}

		// Output result
		if options.Output != "" {
			outputFile := sf.generateOutputFilename(options.Output, dataFile, options.Format)
			if err := sf.writeToFile(outputFile, result); err != nil {
				if options.Verbose {
					fmt.Printf("Failed to write to %s: %v\n", outputFile, err)
				}
			} else if options.Verbose {
				fmt.Printf("Output written to: %s\n", outputFile)
			}
		} else {
			fmt.Println(result)
		}

		if options.Verbose {
			fmt.Println()
		}
	}

	return nil
}

// loadDataFile loads and parses a data file (JSON or YAML)
func (sf *SchemaFormatter) loadDataFile(filename string) (interface{}, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(filename))
	var result interface{}

	switch ext {
	case ".json":
		err = json.Unmarshal(data, &result)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &result)
	default:
		// Try JSON first, then YAML
		if err = json.Unmarshal(data, &result); err != nil {
			err = yaml.Unmarshal(data, &result)
		}
	}

	if err != nil {
		return nil, err
	}

	// Convert map to struct-like representation for compatibility
	if m, ok := result.(map[string]interface{}); ok {
		return sf.convertMapToStruct(m), nil
	}

	return result, nil
}

// convertMapToStruct creates a struct from a map for schema processing
func (sf *SchemaFormatter) convertMapToStruct(data map[string]interface{}) interface{} {
	// For now, we'll work directly with the map in the formatters
	// The schema processing will be updated to handle maps
	return data
}


// formatWithPrettyData formats PrettyData using the specified format
func (sf *SchemaFormatter) formatWithPrettyData(data *api.PrettyData, format string) (string, error) {
	switch strings.ToLower(format) {
	case "json":
		return sf.formatJSONWithPrettyData(data)
	case "yaml":
		return sf.formatYAMLWithPrettyData(data)
	case "csv":
		return sf.formatCSVWithPrettyData(data)
	case "html":
		return sf.formatHTMLWithPrettyData(data)
	case "pdf":
		return sf.formatPDFWithPrettyData(data)
	case "markdown":
		formatter := formatters.NewMarkdownFormatter()
		return formatter.Format(data.Values)
	case "pretty":
		fallthrough
	default:
		return sf.formatPrettyWithPrettyData(data)
	}
}

// formatPrettyWithPrettyData formats PrettyData using pretty formatter
func (sf *SchemaFormatter) formatPrettyWithPrettyData(data *api.PrettyData) (string, error) {
	prettyFormatter := formatters.NewPrettyFormatter()
	return prettyFormatter.Format(data)
}

// formatCSVWithPrettyData formats PrettyData as CSV
func (sf *SchemaFormatter) formatCSVWithPrettyData(data *api.PrettyData) (string, error) {
	csvFormatter := formatters.NewCSVFormatter()

	// Try to find table data first (CSV works best with tabular data)
	for _, tableData := range data.Tables {
		if len(tableData) > 0 {
			// Convert table data to interface{} slice using Formatted() for consistency
			interfaceData := make([]interface{}, len(tableData))
			for i, row := range tableData {
				rowMap := make(map[string]interface{})
				for key, fieldValue := range row {
					// Use Formatted() for consistent string representation like other formatters
					rowMap[key] = fieldValue.Formatted()
				}
				interfaceData[i] = rowMap
			}
			return csvFormatter.Format(interfaceData)
		}
	}

	// Fallback to summary data if no tables found
	summaryMap := make(map[string]interface{})
	for key, fieldValue := range data.Values {
		// Handle nested fields by flattening them
		if len(fieldValue.NestedFields) > 0 {
			// Flatten nested fields into the summary
			for nestedKey, nestedFieldValue := range fieldValue.NestedFields {
				flatKey := fmt.Sprintf("%s.%s", key, nestedKey)
				summaryMap[flatKey] = nestedFieldValue.Formatted()
			}
		} else {
			summaryMap[key] = fieldValue.Formatted()
		}
	}

	// Return single row CSV for summary data
	return csvFormatter.Format([]interface{}{summaryMap})
}

// formatPrettyDataToMap converts PrettyData to a map for JSON/YAML formatting
func (sf *SchemaFormatter) formatPrettyDataToMap(data *api.PrettyData) map[string]interface{} {
	output := make(map[string]interface{})

	// Add all values using Formatted() for consistency with other formatters
	for key, fieldValue := range data.Values {
		if len(fieldValue.NestedFields) > 0 {
			// Handle nested fields recursively
			nestedOutput := make(map[string]interface{})
			for nestedKey, nestedFieldValue := range fieldValue.NestedFields {
				nestedOutput[nestedKey] = sf.formatFieldValueForJSON(nestedFieldValue)
			}
			output[key] = nestedOutput
		} else {
			output[key] = sf.formatFieldValueForJSON(fieldValue)
		}
	}

	// Add all tables using Formatted() for consistency
	for key, tableRows := range data.Tables {
		tableData := make([]map[string]interface{}, len(tableRows))
		for i, row := range tableRows {
			rowData := make(map[string]interface{})
			for fieldName, fieldValue := range row {
				rowData[fieldName] = sf.formatFieldValueForJSON(fieldValue)
			}
			tableData[i] = rowData
		}
		output[key] = tableData
	}

	return output
}

// formatJSONWithPrettyData formats PrettyData as JSON including both values and tables
func (sf *SchemaFormatter) formatJSONWithPrettyData(data *api.PrettyData) (string, error) {
	output := sf.formatPrettyDataToMap(data)
	formatter := formatters.NewJSONFormatter()
	return formatter.Format(output)
}

// formatFieldValueForJSON formats a api.FieldValue for JSON output using Formatted()
func (sf *SchemaFormatter) formatFieldValueForJSON(fieldValue api.FieldValue) interface{} {
	// Always use formatted value if field has a special format (like currency, date, etc.)
	if fieldValue.Field.Format != "" {
		return fieldValue.Formatted()
	}

	// Try to preserve numeric types where possible for fields without special formatting
	switch fieldValue.Field.Type {
	case api.FieldTypeInt:
		if fieldValue.IntValue != nil {
			return *fieldValue.IntValue
		}
	case api.FieldTypeFloat:
		if fieldValue.FloatValue != nil {
			return *fieldValue.FloatValue
		}
	case api.FieldTypeBoolean:
		if fieldValue.BooleanValue != nil {
			return *fieldValue.BooleanValue
		}
	}

	// For all other types, use the formatted string
	return fieldValue.Formatted()
}

// formatYAMLWithPrettyData formats PrettyData as YAML using the same structure as JSON
func (sf *SchemaFormatter) formatYAMLWithPrettyData(data *api.PrettyData) (string, error) {
	output := sf.formatPrettyDataToMap(data)
	formatter := formatters.NewYAMLFormatter()
	return formatter.Format(output)
}

// formatHTMLWithPrettyData formats PrettyData as HTML
func (sf *SchemaFormatter) formatHTMLWithPrettyData(data *api.PrettyData) (string, error) {
	htmlFormatter := formatters.NewHTMLFormatter()
	return htmlFormatter.Format(data)
}

// formatPDFWithPrettyData formats PrettyData as PDF
func (sf *SchemaFormatter) formatPDFWithPrettyData(data *api.PrettyData) (string, error) {
	pdfFormatter := formatters.NewPDFFormatter()
	return pdfFormatter.Format(data)
}


// generateOutputFilename generates output filename based on pattern
func (sf *SchemaFormatter) generateOutputFilename(outputPattern, dataFile, format string) string {
	baseName := strings.TrimSuffix(filepath.Base(dataFile), filepath.Ext(dataFile))

	// If output pattern is a directory, generate filename
	if info, err := os.Stat(outputPattern); err == nil && info.IsDir() {
		return filepath.Join(outputPattern, fmt.Sprintf("%s.%s", baseName, sf.getExtensionForFormat(format)))
	}

	// If output pattern contains placeholders
	if strings.Contains(outputPattern, "{name}") {
		result := strings.ReplaceAll(outputPattern, "{name}", baseName)
		result = strings.ReplaceAll(result, "{format}", format)
		return result
	}

	// Use pattern as-is
	return outputPattern
}

// getExtensionForFormat returns file extension for given format
func (sf *SchemaFormatter) getExtensionForFormat(format string) string {
	switch strings.ToLower(format) {
	case "json":
		return "json"
	case "yaml":
		return "yaml"
	case "csv":
		return "csv"
	case "html":
		return "html"
	case "pdf":
		return "pdf"
	case "markdown":
		return "md"
	default:
		return "txt"
	}
}

// writeToFile writes content to a file
func (sf *SchemaFormatter) writeToFile(filename string, content string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}
