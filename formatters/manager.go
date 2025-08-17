package formatters

import (
	"fmt"
	"os"
	"strings"

	"github.com/flanksource/clicky/api"
)

type FormatManager struct {
	jsonFormatter     *JSONFormatter
	yamlFormatter     *YAMLFormatter
	csvFormatter      *CSVFormatter
	markdownFormatter *MarkdownFormatter
	htmlFormatter     *HTMLFormatter
	prettyFormatter   *PrettyFormatter
}

// NewFormatManager creates a new format manager with all formatters initialized
func NewFormatManager() *FormatManager {
	return &FormatManager{
		jsonFormatter:     NewJSONFormatter(),
		yamlFormatter:     NewYAMLFormatter(),
		csvFormatter:      NewCSVFormatter(),
		markdownFormatter: NewMarkdownFormatter(),
		htmlFormatter:     NewHTMLFormatter(),
		prettyFormatter:   NewPrettyFormatter(),
	}
}

// ToPrettyData implements api.FormatManager.
func (f FormatManager) ToPrettyData(data interface{}) (*api.PrettyData, error) {
	return ToPrettyData(data)
}

// ToPrettyDataWithFormatHint converts data to PrettyData with a format hint for slices
func (f FormatManager) ToPrettyDataWithFormatHint(data interface{}, formatHint string) (*api.PrettyData, error) {
	return ToPrettyDataWithFormatHint(data, formatHint)
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
	if f.htmlFormatter == nil {
		f.htmlFormatter = NewHTMLFormatter()
	}
	return f.htmlFormatter.Format(data)
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
	case "pretty":
		return f.Pretty(data)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// FormatWithOptions formats data using the specified format options
func (f FormatManager) FormatWithOptions(options FormatOptions, data interface{}) (string, error) {
	// Resolve format from boolean flags
	if err := options.ResolveFormat(); err != nil {
		return "", err
	}

	// Handle format-specific options
	switch strings.ToLower(options.Format) {
	case "json":
		return f.JSON(data)

	case "yaml", "yml":
		return f.YAML(data)

	case "csv":
		return f.CSV(data)

	case "markdown", "md":
		if f.markdownFormatter == nil {
			f.markdownFormatter = NewMarkdownFormatter()
		}
		f.markdownFormatter.NoColor = options.NoColor
		// Convert to PrettyData first to handle pretty tags like tree
		prettyData, err := f.ToPrettyData(data)
		if err != nil {
			// Fallback to direct formatting if PrettyData conversion fails
			return f.markdownFormatter.Format(data)
		}
		return f.markdownFormatter.FormatPrettyData(prettyData)

	case "html":
		return f.HTML(data)

	case "table":
		if f.prettyFormatter == nil {
			f.prettyFormatter = NewPrettyFormatter()
		}
		f.prettyFormatter.NoColor = options.NoColor
		// Force table formatting by setting format hint
		prettyData, err := f.ToPrettyDataWithFormatHint(data, "table")
		if err != nil {
			// Fallback to direct formatting if PrettyData conversion fails
			return f.prettyFormatter.Format(data)
		}
		return f.prettyFormatter.FormatPrettyData(prettyData)

	case "tree":
		if f.prettyFormatter == nil {
			f.prettyFormatter = NewPrettyFormatter()
		}
		f.prettyFormatter.NoColor = options.NoColor
		// Force tree formatting by setting format hint
		prettyData, err := f.ToPrettyDataWithFormatHint(data, "tree")
		if err != nil {
			// Fallback to direct formatting if PrettyData conversion fails
			return f.prettyFormatter.Format(data)
		}
		return f.prettyFormatter.FormatPrettyData(prettyData)

	case "pretty":
		if f.prettyFormatter == nil {
			f.prettyFormatter = NewPrettyFormatter()
		}
		f.prettyFormatter.NoColor = options.NoColor
		// Convert to PrettyData first to handle pretty tags, default slices to table
		prettyData, err := f.ToPrettyDataWithFormatHint(data, "table")
		if err != nil {
			// Fallback to direct formatting if PrettyData conversion fails
			return f.prettyFormatter.Format(data)
		}
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
		if err := os.WriteFile(options.Output, []byte(output), 0644); err != nil {
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

// Excel exports data to Excel format (CSV for now)
func (f FormatManager) Excel(data interface{}, filename string) error {
	// For now, we'll just generate CSV which can be opened in Excel
	// Full Excel support would require a library like excelize
	_, err := f.CSV(data)
	if err != nil {
		return fmt.Errorf("failed to generate Excel-compatible CSV: %w", err)
	}
	// Note: The actual file writing would be handled by the caller
	return nil
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

var DEFAULT_MANAGER api.FormatManager = NewFormatManager()
