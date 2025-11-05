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

func (f FormatManager) FormatWithOptions(options FormatOptions, data ...any) (string, error) {

	if len(data) == 0 {
		return "", fmt.Errorf("no data provided for formatting")
	}
	// Resolve format from boolean flags first to check for custom formatters
	format := options.ResolveFormat()
	logger.V(4).Infof("FormatWithOptions called with %d data items, format=%s", len(data), format)

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

	// Extract single element from variadic data
	var d any
	if len(data) == 1 {
		d = data[0]
	} else {
		d = data
	}
	logger.V(4).Infof("Extracted data type: %T", d)

	// Handle format-specific options
	switch strings.ToLower(format) {
	case "json":

		return f.JSON(d)

	case "yaml", "yml":
		return f.YAML(d)

	case "csv":
		return f.CSV(d)

	case "markdown", "md":
		if f.markdownFormatter == nil {
			f.markdownFormatter = NewMarkdownFormatter()
		}
		f.markdownFormatter.NoColor = options.NoColor
		// Convert to PrettyData first to handle pretty tags like tree
		prettyData, err := f.ToPrettyDataWithOptions(d, options)
		if err != nil {
			// Fallback to direct formatting if PrettyData conversion fails
			return f.markdownFormatter.Format(d)
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
		prettyData, err := ToPrettyDataWithOptions(d, FormatOptions{Format: "table"})
		if err != nil {
			// Fallback to direct formatting if PrettyData conversion fails
			return f.prettyFormatter.Format(d)
		}
		return f.prettyFormatter.FormatPrettyData(prettyData)

	case "tree":
		if f.treeFormatter == nil {
			f.treeFormatter = NewTreeFormatter(api.DefaultTheme(), options.NoColor, nil)
		}
		return f.treeFormatter.Format(d)

	case "excel", "xlsx":
		if f.excelFormatter == nil {
			f.excelFormatter = NewExcelFormatter()
		}
		return f.excelFormatter.Format(d)

	case "pretty":
		// Convert to PrettyData first
		prettyData, err := ToPrettyDataWithOptions(d, options)
		if err != nil {
			// Fallback to direct formatting if PrettyData conversion fails
			if f.prettyFormatter == nil {
				f.prettyFormatter = NewPrettyFormatter()
			}
			f.prettyFormatter.NoColor = options.NoColor
			return f.prettyFormatter.Format(d)
		}

		// Use pretty formatter
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
		return f.prettyFormatter.Format(d)
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

		return f.jsonFormatter.FormatValue(prettyData.Original)
	case "yaml", "yml":
		return f.yamlFormatter.FormatValue(prettyData.Original)
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
			return "", fmt.Errorf("%s formatter not registered, registing using 'import _ github.com/flanksource/clicky/formatters/html'", options.Format)
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

var DEFAULT_MANAGER api.FormatManager = NewFormatManager()
