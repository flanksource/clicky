package clicky

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/task"
	"github.com/flanksource/clicky/text"
	"github.com/flanksource/commons/logger"
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
	if logger.IsDebugEnabled() {
		_, _ = logWriter.WriteString(api.Text{}.Append("DEBUG", "text-muted").Space().Appendf(format, args...).ANSI() + "\n")
	}
}

func Tracef(format string, args ...any) {
	if logger.IsTraceEnabled() {
		_, _ = logWriter.WriteString(api.Text{}.Append("TRACE", "text-muted").Space().Appendf(format, args...).ANSI() + "\n")
	}
}

func Format(o any, opts ...FormatOptions) (string, error) {
	return Formatter.FormatWithOptions(formatters.MergeOptions(append([]FormatOptions{defaultOpts}, opts...)...), o)
}

func MustPrint(o any, opts ...FormatOptions) {
	_ = task.Wait()
	result, err := Format(o, opts...)
	if err != nil {
		panic(err)
	}

	fmt.Print(result)
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

// PrintAndWriteSinks renders o for each sink in opts.Sinks.
//
// Stdout sinks (File == "") go through MustPrint, preserving existing
// pretty/stdout behaviour. File sinks call Formatter.FormatToFile, which
// renders in the sink's format and writes to the given path.
//
// Per-sink errors are logged via Errorf but do NOT abort other sinks, so a
// broken HTML template cannot eat the JSON artifact that CI depends on. If
// opts.Sinks is empty the function falls back to MustPrint(o, opts) for
// back-compat with call sites that haven't run ParseFormatSpec.
func PrintAndWriteSinks(o any, opts FormatOptions) {
	_ = task.Wait()
	if len(opts.Sinks) == 0 {
		MustPrint(o, opts)
		return
	}
	for _, sink := range opts.Sinks {
		sinkOpts := opts
		sinkOpts.Sinks = nil
		sinkOpts.Format = sink.Format
		sinkOpts.JSON, sinkOpts.YAML, sinkOpts.CSV = false, false, false
		sinkOpts.HTML, sinkOpts.Markdown, sinkOpts.Pretty = false, false, false
		sinkOpts.PDF, sinkOpts.Slack = false, false
		if sink.File == "" {
			sinkOpts.Output = ""
			MustPrint(o, sinkOpts)
			continue
		}
		sinkOpts.Output = sink.File
		if err := Formatter.FormatToFile(sinkOpts, o); err != nil {
			Errorf("failed to write %s output to %s: %v", sink.Format, sink.File, err)
		}
	}
}

var Human = api.Human
var Class = api.Clz

func CompactList[T any](items []T) api.Textable {
	if len(items) == 0 {
		return Text("[]", "text-muted")
	}
	list := api.List{
		MaxInline: 3,
	}
	for _, item := range items {
		list.Items = append(list.Items, Human(item))
	}
	return list
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

func Collapsed(label string, content api.Textable, styles ...string) api.Collapsed {
	return api.Collapsed{
		Label:   label,
		Content: content,
		Style:   strings.Join(styles, " "),
	}
}

var KeyValue = api.KeyValue
var CodeBlock = api.CodeBlock
var Badge = api.Badge

func Map[T any](m map[string]T, styles ...string) api.DescriptionList {
	style := "compact"
	if len(styles) > 0 {
		style = strings.Join(styles, " ")
	}

	// Sort keys for consistent ordering
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	items := make([]api.KeyValuePair, 0, len(m))
	for _, k := range keys {
		if fmt.Sprintf("%v", m[k]) == "" || fmt.Sprintf("%v", m[k]) == "<nil>" {
			continue
		}
		items = append(items, api.KeyValuePair{
			Key:   k,
			Value: m[k],
			Style: style,
		})
	}

	return api.DescriptionList{
		Items: items,
		Style: style,
	}
}

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
