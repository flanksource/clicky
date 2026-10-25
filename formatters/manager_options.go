package formatters

import "github.com/flanksource/clicky/api"

type ManagerOptions struct {
	NoColor bool
}

// NewFormatManager configures terminal behavior before initializing renderers.
func NewFormatManager(options ...ManagerOptions) *FormatManager {
	if len(options) > 1 {
		panic("format manager accepts one options value")
	}
	var noColor bool
	if len(options) == 1 {
		noColor = options[0].NoColor
	}
	var pretty *PrettyFormatter
	theme := api.NoTTYTheme()
	if noColor {
		pretty = NewPrettyFormatterWithTheme(theme)
		pretty.NoColor = true
	} else {
		pretty = NewPrettyFormatter()
		theme = api.DefaultTheme()
	}
	return &FormatManager{
		noColor:       noColor,
		jsonFormatter: NewJSONFormatter(), yamlFormatter: NewYAMLFormatter(),
		csvFormatter: NewCSVFormatter(), markdownFormatter: NewMarkdownFormatter(),
		prettyFormatter: pretty, treeFormatter: NewTreeFormatter(theme, noColor, nil),
		slackFormatter: NewSlackFormatter(), toonFormatter: NewTOONFormatter(),
	}
}
