package formatters

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/flanksource/clicky/api"
)

// SchemaFormatter handles schema-based formatting operations
type SchemaFormatter struct {
	Schema *api.PrettyObject
	Parser *api.StructParser
}

// NewSchemaFormatter creates a new schema formatter with the given schema file
func NewSchemaFormatter(schemaFile string) (*SchemaFormatter, error) {
	parser := api.NewStructParser()
	schema, err := parser.LoadSchemaFromYAML(schemaFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}

	return &SchemaFormatter{
		Schema: schema,
		Parser: parser,
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

	// Apply global filter from FormatOptions to all table fields in schema
	schema := sf.applyFilterToSchema(sf.Schema, options.Filter)

	// Parse data with schema into PrettyData
	prettyData, err := sf.Parser.ParseDataWithSchema(data, schema)
	if err != nil {
		return "", fmt.Errorf("failed to parse data with schema: %w", err)
	}

	// Format output
	return sf.formatWithPrettyData(prettyData, options)
}

// FormatFiles formats multiple data files using the schema
func (sf *SchemaFormatter) FormatFiles(dataFiles []string, options FormatOptions) error {
	// Dump schema to stderr if requested
	if options.DumpSchema {
		schemaYAML, err := yaml.Marshal(sf.Schema)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling schema: %v\n", err)
		} else {
			fmt.Fprintln(os.Stderr, "=== Schema Dump ===")
			fmt.Fprintln(os.Stderr, string(schemaYAML))
			fmt.Fprintln(os.Stderr, "==================")
		}
	}

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
func (sf *SchemaFormatter) formatWithPrettyData(data *api.PrettyData, options FormatOptions) (string, error) {
	// Convert PrettyData to the appropriate format for the FormatManager
	output := sf.formatPrettyDataToMap(data)

	// For JSON/YAML/CSV, use direct formatting to avoid the struct requirement
	switch strings.ToLower(options.Format) {
	case "json":
		jsonFormatter := NewJSONFormatter()
		b, err := json.MarshalIndent(output, "", jsonFormatter.Indent)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case "yaml", "yml":
		b, err := yaml.Marshal(output)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case "csv":
		csvFormatter := NewCSVFormatter()
		// Use the original PrettyData directly for CSV formatting
		return csvFormatter.FormatPrettyData(data)
	case "html", "html-pdf":
		formatter, ok := GetCustomFormatter(options.Format)
		if !ok {
			return "", fmt.Errorf("%s formatter not registered, registing using 'import _ github.com/flanksource/clicky/formatters/http'", options.Format)
		}
		return formatter(data, options)
	default:
		// For other formats, delegate to the format manager
		manager := NewFormatManager()
		return manager.Format(options.Format, output)
	}
}

// convertTypedValueToInterface recursively converts TypedValue to appropriate Go types
// for JSON/YAML serialization, preserving nested structures
func (sf *SchemaFormatter) convertTypedValueToInterface(tv api.TypedValue) interface{} {
	// Handle nested maps - preserve as map[string]interface{}
	if tv.TypedMap != nil {
		result := make(map[string]interface{})
		for key, value := range *tv.TypedMap {
			result[key] = sf.convertTypedValueToInterface(value)
		}
		return result
	}

	// Handle nested lists - preserve as []interface{}
	if tv.TypedList != nil {
		result := make([]interface{}, len(*tv.TypedList))
		for i, value := range *tv.TypedList {
			result[i] = sf.convertTypedValueToInterface(value)
		}
		return result
	}

	// For primitives (dates, numbers, strings, etc.), use String() to get formatted value
	return tv.String()
}

// formatPrettyDataToMap converts PrettyData to a map for JSON/YAML formatting
func (sf *SchemaFormatter) formatPrettyDataToMap(data *api.PrettyData) map[string]interface{} {
	output := make(map[string]interface{})

	// Add all values using recursive conversion to preserve nested structures
	if data.TypedMap != nil {
		for key, typedValue := range *data.TypedMap {
			output[key] = sf.convertTypedValueToInterface(typedValue)
		}
	}

	// Add table data if present
	if data.Table != nil {
		tableData := make([]map[string]interface{}, len(data.Table.Rows))
		for i, row := range data.Table.Rows {
			rowData := make(map[string]interface{})
			for fieldName, typedValue := range row {
				rowData[fieldName] = sf.convertTypedValueToInterface(typedValue)
			}
			tableData[i] = rowData
		}
		output["table"] = tableData
	}

	return output
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
	case "html-pdf":
		return "pdf"
	case "markdown":
		return "md"
	default:
		return "txt"
	}
}

// writeToFile writes content to a file
func (sf *SchemaFormatter) writeToFile(filename, content string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
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

// applyFilterToSchema creates a copy of the schema with filter applied to all table fields
func (sf *SchemaFormatter) applyFilterToSchema(schema *api.PrettyObject, filter string) *api.PrettyObject {
	if schema == nil || filter == "" {
		return schema
	}

	// Create a deep copy of the schema
	schemaCopy := &api.PrettyObject{
		Fields: make([]api.PrettyField, len(schema.Fields)),
	}

	// Copy all fields and inject filter into table fields
	for i, field := range schema.Fields {
		fieldCopy := field

		// Initialize FormatOptions if nil
		if fieldCopy.FormatOptions == nil {
			fieldCopy.FormatOptions = make(map[string]string)
		}

		// Apply filter to table fields (only if not already set in schema)
		if fieldCopy.Format == api.FormatTable && fieldCopy.FormatOptions["filter"] == "" {
			fieldCopy.FormatOptions["filter"] = filter
		}

		// Apply filter to tree fields (only if not already set in schema)
		if fieldCopy.Format == api.FormatTree && fieldCopy.FormatOptions["filter"] == "" {
			fieldCopy.FormatOptions["filter"] = filter
		}

		schemaCopy.Fields[i] = fieldCopy
	}

	return schemaCopy
}
