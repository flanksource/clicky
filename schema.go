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
	Schema *api.PrettyObject
	Parser *StructParser
}

// NewSchemaFormatter creates a new schema formatter with the given schema file
func NewSchemaFormatter(schemaFile string) (*SchemaFormatter, error) {
	parser := NewStructParser()
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
func (sf *SchemaFormatter) FormatFile(dataFile string, options formatters.FormatOptions) (string, error) {
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

	// Format output
	return sf.formatWithPrettyData(prettyData, options)
}

// FormatFiles formats multiple data files using the schema
func (sf *SchemaFormatter) FormatFiles(dataFiles []string, options formatters.FormatOptions) error {
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
func (sf *SchemaFormatter) formatWithPrettyData(data *api.PrettyData, options formatters.FormatOptions) (string, error) {
	// Convert PrettyData to the appropriate format for the FormatManager
	output := sf.formatPrettyDataToMap(data)
	
	// For JSON/YAML/CSV, use direct formatting to avoid the struct requirement
	switch strings.ToLower(options.Format) {
	case "json":
		jsonFormatter := formatters.NewJSONFormatter()
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
		csvFormatter := formatters.NewCSVFormatter()
		// Use the original PrettyData directly for CSV formatting
		return csvFormatter.FormatPrettyData(data)
	default:
		// For other formats, delegate to the format manager
		manager := formatters.NewFormatManager()
		return manager.Format(options.Format, output)
	}
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
				nestedOutput[nestedKey] = nestedFieldValue.Formatted()
			}
			output[key] = nestedOutput
		} else {
			output[key] = fieldValue.Formatted()
		}
	}

	// Add all tables using Formatted() for consistency
	for key, tableRows := range data.Tables {
		tableData := make([]map[string]interface{}, len(tableRows))
		for i, row := range tableRows {
			rowData := make(map[string]interface{})
			for fieldName, fieldValue := range row {
				rowData[fieldName] = fieldValue.Formatted()
			}
			tableData[i] = rowData
		}
		output[key] = tableData
	}

	return output
}







// prettifyFieldName converts field names to human-readable format
func (sf *SchemaFormatter) prettifyFieldName(name string) string {
	// Convert snake_case to Title Case
	parts := strings.Split(name, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
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
