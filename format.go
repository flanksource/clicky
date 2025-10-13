package clicky

import (
	"fmt"
	"sort"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
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

var Human = api.Human

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

func mimeTypeToLanguage(mime string) string {
	switch {
	case strings.Contains(mime, "json"):
		return "json"
	case strings.Contains(mime, "xml"):
		return "xml"
	case strings.Contains(mime, "yaml") || strings.Contains(mime, "yml"):
		return "yaml"
	case strings.Contains(mime, "html"):
		return "html"
	case strings.Contains(mime, "text/html"):
		return "html"
	case strings.Contains(mime, "text/plain"):
		return "txt"
	case strings.Contains(mime, "javascript"):
		return "javascript"
	case strings.Contains(mime, "css"):
		return "css"
	case strings.Contains(mime, "csv"):
		return "csv"
	case strings.Contains(mime, "markdown") || strings.Contains(mime, "md"):
		return "markdown"
	case strings.Contains(mime, "sql"):
		return "sql"
	case strings.Contains(mime, "graphql"):
		return "graphql"
	case strings.Contains(mime, "python"):
		return "python"
	case strings.Contains(mime, "java"):
		return "java"
	}
	return ""

}

func KeyValue(key string, value any, styles ...string) api.KeyValuePair {
	style := "compact"
	if len(styles) > 0 {
		style = strings.Join(styles, " ")
	}
	return api.KeyValuePair{
		Key:   key,
		Value: value,
		Style: style,
	}
}

func Map(m map[string]any, styles ...string) api.DescriptionList {
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
func CodeBlock(language, content string, styles ...string) api.Code {
	return api.Code{
		Content:  content,
		Language: mimeTypeToLanguage(language),
		Style:    strings.Join(styles, " "),
	}
}

func UseFormatter(opts FormatOptions) {
	defaultOpts = opts
}
