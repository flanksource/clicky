package formatters

import (
	"github.com/flanksource/clicky/api"
)

// PrettyFormatter handles formatting of structs with pretty tags
type PrettyFormatter struct {
	Theme   api.Theme
	NoColor bool
	parser  *api.StructParser
}

// NewPrettyFormatter creates a new formatter with adaptive theme
func NewPrettyFormatter() *PrettyFormatter {
	return &PrettyFormatter{
		Theme:  api.AutoTheme(),
		parser: api.NewStructParser(),
	}
}

// NewPrettyFormatterWithTheme creates a new formatter with a specific theme
func NewPrettyFormatterWithTheme(theme api.Theme) *PrettyFormatter {
	return &PrettyFormatter{
		Theme:  theme,
		parser: api.NewStructParser(),
	}
}

func (p *PrettyFormatter) Parse(data interface{}) (*api.PrettyData, error) {
	schema, err := p.parser.Parse(data)
	if err != nil {
		return nil, err
	}
	return &api.PrettyData{
		Schema:   schema,
		Original: data,
	}, nil
}

// Format formats data and returns formatted output
func (p *PrettyFormatter) Format(data interface{}) (string, error) {
	// Check if this is already parsed PrettyData
	if prettyData, ok := data.(*api.PrettyData); ok {
		return p.FormatPrettyData(prettyData)
	}
	schema, err := p.parser.Parse(data)
	if err != nil {
		return "", err
	}
	return p.FormatPrettyData(&api.PrettyData{
		Schema:   schema,
		Original: data,
	})
}

// FormatPrettyData formats PrettyData structure
func (p *PrettyFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	if data == nil {
		return "", nil
	}

	return data.ANSI(), nil
}
