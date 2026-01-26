package formatters

import "fmt"

// TeamsFormatter formats data into Microsoft Teams Adaptive Card JSON.
type TeamsFormatter struct{}

func NewTeamsFormatter() *TeamsFormatter {
	return &TeamsFormatter{}
}

func (f *TeamsFormatter) Format(in interface{}, options FormatOptions) (string, error) {
	return "", fmt.Errorf("teams formatter not implemented")
}
