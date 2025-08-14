package formatters

import (
	"fmt"
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
func (f *FormatManager) ToPrettyData(data interface{}) (*api.PrettyData, error) {
	return ToPrettyData(data)
}

// Pretty implements api.FormatManager.
func (f *FormatManager) Pretty(data interface{}) (string, error) {
	if f.prettyFormatter == nil {
		f.prettyFormatter = NewPrettyFormatter()
	}
	return f.prettyFormatter.Format(data)
}

// JSON implements api.FormatManager.
func (f *FormatManager) JSON(data interface{}) (string, error) {
	if f.jsonFormatter == nil {
		f.jsonFormatter = NewJSONFormatter()
	}
	return f.jsonFormatter.Format(data)
}

// YAML implements api.FormatManager.
func (f *FormatManager) YAML(data interface{}) (string, error) {
	if f.yamlFormatter == nil {
		f.yamlFormatter = NewYAMLFormatter()
	}
	return f.yamlFormatter.Format(data)
}

// CSV implements api.FormatManager.
func (f *FormatManager) CSV(data interface{}) (string, error) {
	if f.csvFormatter == nil {
		f.csvFormatter = NewCSVFormatter()
	}
	return f.csvFormatter.Format(data)
}

// Markdown implements api.FormatManager.
func (f *FormatManager) Markdown(data interface{}) (string, error) {
	if f.markdownFormatter == nil {
		f.markdownFormatter = NewMarkdownFormatter()
	}
	return f.markdownFormatter.Format(data)
}

// HTML implements api.FormatManager.
func (f *FormatManager) HTML(data interface{}) (string, error) {
	if f.htmlFormatter == nil {
		f.htmlFormatter = NewHTMLFormatter()
	}
	return f.htmlFormatter.Format(data)
}

// Format implements a generic format method that delegates to specific formatters
func (f *FormatManager) Format(format string, data interface{}) (string, error) {
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

// Excel exports data to Excel format (CSV for now)
func (f *FormatManager) Excel(data interface{}, filename string) error {
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
func (f *FormatManager) Pdf(data interface{}, filename string) error {
	// PDF generation would require a library like gofpdf
	// For now, we'll return an error indicating it's not implemented
	return fmt.Errorf("PDF export is not yet implemented")
}

func (f *FormatManager) ParseSchema(data interface{}) (*api.PrettyObject, error) {
	// This is a no-op for the FormatManager
	return nil, nil
}

var DEFAULT_MANAGER api.FormatManager = NewFormatManager()
