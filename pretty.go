package clicky

import (
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

// PrettyParser is a wrapper around formatters.PrettyFormatter for backwards compatibility
type PrettyParser struct {
	*formatters.PrettyFormatter
}

// NewPrettyParser creates a new parser with adaptive theme
func NewPrettyParser() *PrettyParser {
	return &PrettyParser{
		PrettyFormatter: formatters.NewPrettyFormatter(),
	}
}

// NewPrettyParserWithTheme creates a new parser with a specific theme
func NewPrettyParserWithTheme(theme api.Theme) *PrettyParser {
	return &PrettyParser{
		PrettyFormatter: formatters.NewPrettyFormatterWithTheme(theme),
	}
}
