package clicky

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/text"
)

type FormatOptions = formatters.FormatOptions

var (
	Formatter   = formatters.NewFormatManager()
	defaultOpts FormatOptions
)

// func TreeOf[T any](nodes ...T) api.TreeNode {
// 	tree := api.SimpleTreeNode{}
// 	for _, n := range nodes {
// 		if t, ok := n.(api.TreeNode); ok {
// 			tree.Children = append(tree.Children, t)
// 		} else {
// 			logger.Errorf("unsupported type for TreeOf: %T", n)
// 		}
// 	}
// 	return &tree
// }

var logWriter = text.LineFilter(os.Stderr, text.RedactSecrets()).(io.StringWriter)

func RedactSecretValues(val ...string) {
	logWriter = text.LineFilter(logWriter.(io.Writer), text.RedactSecrets(val...)).(io.StringWriter)
}

func SQL(format string, args ...any) {
	_, _ = logWriter.WriteString(api.Text{}.Append("SQL", "text-blue-600").Space().
		Add(CodeBlock("sql", fmt.Sprintf(format, args...))).ANSI() + "\n")
}

func Infof(format string, args ...any) {
	_, _ = logWriter.WriteString(api.Text{}.Append("INFO", "text-green-600").Space().Appendf(format, args...).ANSI() + "\n")
}

func Errorf(format string, args ...any) {
	_, _ = logWriter.WriteString(api.Text{}.Append("ERROR", "text-red-600").Space().Appendf(format, args...).ANSI() + "\n")
}

func Warnf(format string, args ...any) {
	_, _ = logWriter.WriteString(api.Text{}.Append("WARN", "text-yellow-600").Space().Appendf(format, args...).ANSI() + "\n")
}

func Debugf(format string, args ...any) {
	_, _ = logWriter.WriteString(api.Text{}.Append("DEBUG", "text-muted").Space().Appendf(format, args...).ANSI() + "\n")
}

func Tracef(format string, args ...any) {
	_, _ = logWriter.WriteString(api.Text{}.Append("TRACE", "text-muted").Space().Appendf(format, args...).ANSI() + "\n")
}

func Format(o any, opts ...FormatOptions) (string, error) {
	return Formatter.FormatWithOptions(formatters.MergeOptions(append([]FormatOptions{defaultOpts}, opts...)...), o)
}

func MustPrint(o any, opts ...FormatOptions) {
	result, err := Format(o, opts...)
	if err != nil {
		panic(err)
	}
	fmt.Println(result)
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

var Human = api.Human
var Class = api.Clz

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

func Collapsed(label string, content api.Textable, styles ...string) api.Collapsed {
	return api.Collapsed{
		Label:   label,
		Content: content,
		Style:   strings.Join(styles, " "),
	}
}

var Map = api.Map
var KeyValue = api.KeyValue
var CodeBlock = api.CodeBlock

func UseFormatter(opts FormatOptions) {
	defaultOpts = opts
}

// RegisterFormatter registers a custom formatter function that can be used
// with Format() by specifying the format name in FormatOptions.
// Custom formatters take precedence over built-in formatters with the same name.
//
// Example:
//
//	clicky.RegisterFormatter("upper", func(data interface{}, opts clicky.FormatOptions) (string, error) {
//	    s := fmt.Sprintf("%v", data)
//	    return strings.ToUpper(s), nil
//	})
//
//	result, _ := clicky.Format(myData, clicky.FormatOptions{Format: "upper"})
func RegisterFormatter(name string, fn func(data interface{}, options FormatOptions) (string, error)) {
	formatters.RegisterFormatter(name, fn)
}

// ListCustomFormatters returns a sorted list of all registered custom formatter names.
func ListCustomFormatters() []string {
	return formatters.ListCustomFormatters()
}
