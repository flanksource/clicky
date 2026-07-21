package formatters

import (
	"fmt"
	"sort"

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
	return f.FormatWithOptions(data, FormatOptions{})
}

func (f *MarkdownFormatter) FormatWithOptions(data interface{}, opts FormatOptions) (string, error) {
	options := api.MarkdownOptions{NoColor: f.NoColor || opts.NoColor}

	// Check if data implements Pretty interface first
	if pretty, ok := data.(api.Pretty); ok {
		text := pretty.Pretty()
		return api.RenderMarkdown(text, options), nil
	}

	// Convert to PrettyData
	prettyData, err := ToPrettyData(data)
	if err != nil {
		return "", fmt.Errorf("failed to convert to PrettyData: %w", err)
	}

	if prettyData == nil || prettyData.Schema == nil {
		return "", nil
	}

	return f.FormatPrettyData(prettyData, opts)
}

// FormatPrettyData formats PrettyData as Markdown
func (f *MarkdownFormatter) FormatPrettyData(data *api.PrettyData, opts FormatOptions) (string, error) {
	if data == nil {
		return "", nil
	}
	options := api.MarkdownOptions{NoColor: f.NoColor || opts.NoColor}
	if ordered, ok := schemaOrderedMarkdown(data); ok {
		return api.RenderMarkdown(ordered, options), nil
	}

	return api.RenderMarkdown(data, options), nil
}

func schemaOrderedMarkdown(data *api.PrettyData) (api.TextList, bool) {
	if data == nil || data.Schema == nil || data.TypedMap == nil {
		return nil, false
	}

	values := *data.TypedMap
	seen := map[string]bool{}
	out := api.TextList{}

	appendField := func(name, label string, value api.TypedValue) {
		if label == "" {
			label = api.PrettifyFieldName(name)
		}
		out = append(out, api.Text{}.Append(label+": ", "text-muted").Add(value.Value()))
		seen[name] = true
	}

	for _, field := range data.Schema.Fields {
		if value, ok := values[field.Name]; ok {
			appendField(field.Name, field.Label, value)
		}
	}

	extra := make([]string, 0, len(values))
	for name := range values {
		if !seen[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	for _, name := range extra {
		appendField(name, "", values[name])
	}

	return out, len(out) > 0
}
