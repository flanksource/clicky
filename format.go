package clicky

import (
	"fmt"
	"io"
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

// logWriter is the destination for clicky.Infof / Errorf / Warnf / SQL etc.
// task.GatedStderr buffers writes while the interactive renderer owns the
// TTY (so log lines cannot land mid-frame) and flushes them right after
// the renderer lets go. Off-renderer it forwards straight to os.Stderr.
var logWriter = text.LineFilter(task.GatedStderr(), text.RedactSecrets()).(io.StringWriter)

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

	// Terminate the output. Without this the shell prompt lands on the last
	// rendered line, and line-oriented consumers of a redirect drop it.
	if result != "" && !strings.HasSuffix(result, "\n") {
		result += "\n"
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

func List(items ...api.Textable) api.List {
	return api.List{Items: items}
}

func TextList(items ...api.Textable) api.TextList {
	return api.TextList(items)
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

func Heading(level int, content api.Textable) api.Heading {
	return api.Heading{Level: level, Content: content}
}

func Header(level int, content api.Textable) api.Heading {
	return Heading(level, content)
}

func Blockquote(content api.Textable) api.Blockquote {
	return api.Blockquote{Content: content}
}

func FootnoteRef(id string) api.FootnoteRef {
	return api.FootnoteRef{ID: id}
}

func Footnote(id string, content api.Textable) api.Footnote {
	return api.Footnote{ID: id, Content: content}
}

func Footnotes(notes ...api.Footnote) api.Footnotes {
	return api.Footnotes{Items: notes}
}

// WithKey wraps a Textable with a data key. The result renders identically to
// value in every format but serializes to JSON as {key: value}. Callers reading
// the key for their own serialization can type-assert to api.Keyed.
func WithKey(key string, value api.Textable) api.Keyed {
	return api.Keyed{Key: key, Value: value}
}

func Table(headers ...string) api.TextTable {
	table := api.TextTable{}
	for _, header := range headers {
		table.Headers = append(table.Headers, Text(header, "font-bold"))
		table.FieldNames = append(table.FieldNames, header)
	}
	return table
}

func Tree(node api.Textable, children ...api.TextTree) api.TextTree {
	return api.TextTree{
		Node:     node,
		Children: children,
	}
}

func Collapsed(label string, content api.Textable, styles ...string) api.Collapsed {
	return api.Collapsed{
		Label:   label,
		Content: content,
		Style:   strings.Join(styles, " "),
	}
}

func Button(label, href string, options ...func(*api.Button)) api.Button {
	button := api.Button{Label: label, Href: href}
	for _, option := range options {
		if option != nil {
			option(&button)
		}
	}
	return button
}

func ButtonID(id string) func(*api.Button) {
	return func(button *api.Button) {
		button.ID = id
	}
}

func ButtonPayload(payload string) func(*api.Button) {
	return func(button *api.Button) {
		button.Payload = payload
	}
}

func ButtonVariant(variant string) func(*api.Button) {
	return func(button *api.Button) {
		button.Variant = variant
	}
}

func ButtonGroup(buttons ...api.Button) api.ButtonGroup {
	return api.ButtonGroup{Buttons: buttons}
}

func LabelBadge(label, value string, options ...func(*api.LabelBadge)) api.LabelBadge {
	badge := api.LabelBadge{Label: label, Value: value}
	for _, option := range options {
		if option != nil {
			option(&badge)
		}
	}
	return badge
}

func LabelBadgeColor(color string) func(*api.LabelBadge) {
	return func(badge *api.LabelBadge) {
		badge.Color = color
	}
}

func LabelBadgeTextColor(color string) func(*api.LabelBadge) {
	return func(badge *api.LabelBadge) {
		badge.TextColor = color
	}
}

func LabelBadgeShape(shape string) func(*api.LabelBadge) {
	return func(badge *api.LabelBadge) {
		badge.Shape = shape
	}
}

func LabelBadgeIcon(icon string) func(*api.LabelBadge) {
	return func(badge *api.LabelBadge) {
		badge.Icon = icon
	}
}

func Admonition(severity api.Severity, title, body api.Textable) api.Admonition {
	return api.Admonition{
		Severity: severity,
		Title:    title,
		Body:     body,
	}
}

func Diff(before, after, fromLabel, toLabel string) api.Diff {
	return api.NewDiff(before, after, fromLabel, toLabel)
}

func Comment(text string) api.Comment {
	return api.Comment(text)
}

func HTMLElement(tag, content string, attrs ...map[string]string) api.HtmlElement {
	var attributes map[string]string
	if len(attrs) > 0 {
		attributes = attrs[0]
	}
	return api.HtmlElement{
		Tag:        tag,
		Attributes: attributes,
		Content:    content,
		Fallback:   Text(content),
	}
}

func ClickyText(text api.Textable) formatters.ClickyText {
	return formatters.ClickyText{Textable: text}
}

var KeyValue = api.KeyValue
var CodeBlock = api.CodeBlock
var Badge = api.Badge

// StackTrace parses a free-form runtime stack-trace string and returns a
// styled, render-ready trace. The default parser is language-agnostic and
// auto-detects Java traces; pass clicky.StackTraceJava (or another
// language-specific parser) when the language is known up front.
//
// Pass options like clicky.WithSourceResolver(r), clicky.WithStackContext(5),
// or clicky.WithStackInclude("com.example.admin.") to attach inline source
// context and filter frames.
func StackTrace(input string, opts ...api.StackTraceOption) api.StackTrace {
	return api.ParseJavaStackTrace(input, opts...)
}

// StackTraceJava is the explicit Java parser. Equivalent to StackTrace today;
// kept distinct so future non-Java parsers (Python, .NET) can plug in without
// breaking callers that have explicitly opted into Java semantics.
var StackTraceJava = api.ParseJavaStackTrace

// SourceResolver is the extension point a StackTrace consults to populate
// each frame with surrounding source lines. See api.SourceResolver for the
// interface contract.
type SourceResolver = api.SourceResolver

// SourceResolverFunc adapts a plain function to SourceResolver.
type SourceResolverFunc = api.SourceResolverFunc

var (
	WithSourceResolver        = api.WithSourceResolver
	WithSourceResolverContext = api.WithSourceResolverContext
	WithStackInclude          = api.WithStackInclude
	WithStackExclude          = api.WithStackExclude
	WithStackContext          = api.WithStackContext
	WithMaxStackFrames        = api.WithMaxFrames
)
var LinkTargetDialog = api.LinkTargetDialog
var LinkTargetHover = api.LinkTargetHover
var LinkTargetExpand = api.LinkTargetExpand
var LinkTargetClicky = api.LinkTargetClicky
var LinkTargetSelf = api.LinkTargetSelf
var LinkTargetWindow = api.LinkTargetWindow
var LinkTargetTab = api.LinkTargetTab

func Link(href string) api.Link {
	return api.NewLink(href)
}

func LinkCommand(command string) api.LinkCommand {
	return api.NewLinkCommand(command)
}

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
