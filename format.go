package clicky

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

type FormatOptions = formatters.FormatOptions

var Formatter = formatters.NewFormatManager()
var defaultOpts FormatOptions

func Format(o any, opts ...FormatOptions) (string, error) {
	return Formatter.FormatWithOptions(formatters.MergeOptions(append([]FormatOptions{defaultOpts}, opts...)...), o)
}
func MustFormat(o any, opts ...FormatOptions) string {
	result, _ := Formatter.FormatWithOptions(formatters.MergeOptions(append([]FormatOptions{defaultOpts}, opts...)...), o)
	return result
}

func FormatToFile(o any, opts FormatOptions, file string) error {
	opts.Output = file
	_opts := formatters.MergeOptions(append([]FormatOptions{defaultOpts}, opts)...)
	return Formatter.FormatToFile(_opts, o)
}

func Text(content string, tailwindClasses ...string) api.Text {
	return api.Text{
		Content: content,
		Style:   strings.Join(tailwindClasses, " "),
	}
}

func Textf(content string, args ...any) api.Text {
	return api.Text{
		Content: fmt.Sprintf(content, args...),
	}
}

func UseFormatter(opts FormatOptions) {
	defaultOpts = opts

}
