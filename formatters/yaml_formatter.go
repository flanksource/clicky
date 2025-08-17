package formatters

import (
	"gopkg.in/yaml.v3"

	"github.com/flanksource/clicky/api"
)

// YAMLFormatter handles YAML formatting
type YAMLFormatter struct{}

// NewYAMLFormatter creates a new YAML formatter
func NewYAMLFormatter() *YAMLFormatter {
	return &YAMLFormatter{}
}

// Format formats data as YAML
func (f *YAMLFormatter) Format(data interface{}) (string, error) {
	if data == nil {
		return "null", nil
	}
	// Convert to PrettyData and use FormatPrettyData
	prettyData, err := ToPrettyData(data)
	if err != nil || prettyData == nil || prettyData.Original == nil {
		// Fallback to direct YAML serialization
		return "", err
	}
	return f.FormatPrettyData(prettyData)

}

// FormatPrettyData formats PrettyData as YAML using the original data if available
func (f *YAMLFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	if b, err := yaml.Marshal(data.Original); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}
