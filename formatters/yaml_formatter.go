package formatters

import (
	"gopkg.in/yaml.v3"
)

// YAMLFormatter handles YAML formatting
type YAMLFormatter struct{}

// NewYAMLFormatter creates a new YAML formatter
func NewYAMLFormatter() *YAMLFormatter {
	return &YAMLFormatter{}
}

// Format formats data as YAML
func (f *YAMLFormatter) Format(data interface{}) (string, error) {
	if b, err := yaml.Marshal(data); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}
