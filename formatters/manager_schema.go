package formatters

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/flanksource/clicky/api"
)

// FormatWithSchemaContext handles schema-aware formatting while forwarding ctx
// to registered format callbacks.
func (f *FormatManager) FormatWithSchemaContext(ctx any, prettyData *api.PrettyData, options FormatOptions) (string, error) {
	options.NoColor = options.NoColor || f.noColor
	before := any(prettyData)
	before = f.applyBeforeFormatCallbacks(ctx, options, before)
	if transformed, ok := before.(*api.PrettyData); ok && transformed != nil {
		prettyData = transformed
	}

	// Handle different output formats for schema-aware data
	format := strings.ToLower(options.Format)
	var output string
	var err error
	switch format {
	case "llm":
		output, err = formatLLM(prettyData, options)
	case "json":
		output, err = f.jsonFormatter.FormatValue(prettyData.Original)
	case "yaml", "yml":
		output, err = f.yamlFormatter.FormatValue(prettyData.Original)
	case "csv":
		if f.csvFormatter == nil {
			f.csvFormatter = NewCSVFormatter()
		}
		output, err = f.csvFormatter.FormatPrettyData(prettyData)
	case "markdown", "md":
		if f.markdownFormatter == nil {
			f.markdownFormatter = NewMarkdownFormatter()
		}
		output, err = f.markdownFormatter.FormatPrettyData(prettyData, options)
	case "slack":
		if f.slackFormatter == nil {
			f.slackFormatter = NewSlackFormatter()
		}
		output, err = f.slackFormatter.FormatPrettyData(prettyData, options)
	case "html", "html-static", "html-pdf", "pdf":
		formatter, ok := GetCustomFormatter(options.Format)
		if !ok {
			return "", fmt.Errorf("%s formatter not registered, register using 'import _ github.com/flanksource/clicky/formatters/html'", options.Format)
		}
		output, err = formatter(prettyData, options)
	default:
		// Default to pretty format
		if f.prettyFormatter == nil {
			f.prettyFormatter = NewPrettyFormatter()
		}
		f.prettyFormatter.NoColor = options.NoColor
		output, err = f.prettyFormatter.FormatPrettyData(prettyData)
	}

	if err != nil {
		return "", err
	}
	output = f.applyAfterFormatCallbacks(ctx, options, before, output)
	if options.NoColor {
		output = ansi.Strip(output)
	}
	return output, nil
}
