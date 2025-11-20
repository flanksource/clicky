package formatters

import (
	"fmt"

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

	return data.Markdown(), nil
}
