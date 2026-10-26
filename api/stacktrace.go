package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// StackFrame is one parsed entry of a runtime stack trace. Only a subset of
// fields is populated for any given language; renderers tolerate empty values.
//
// Class+Method together form a stable, language-agnostic identifier that
// SourceResolver implementations may use as a lookup key (e.g., for Java
// `decompile` output) when no usable File path is available.
type StackFrame struct {
	Class             string
	Method            string
	File              string
	Line              int
	Native            bool
	Runtime           bool
	Annotation        string
	SourceLines       []string
	SourceLineNumbers []int
	SourceStartLine   int
	SourceLanguage    string
}

// StackTrace is a parsed runtime stack trace. It carries the high-level
// exception/cause metadata that most languages emit (Java, Python, .NET) plus
// the ordered frame list. Construct one via clicky.StackTrace(input, opts...)
// or one of the language-specific parsers (clicky.StackTraceJava).
type StackTrace struct {
	ExceptionClass string
	Message        string
	CausedBy       []string
	Frames         []StackFrame
	Language       string

	options stackTraceOptions
}

// SourceResolver supplies source lines for a frame so the rendered stack
// trace can show inline code context. Implementations may consult a local
// checkout, decompiled JARs, a remote service, or anything else; failures
// must be non-fatal — return ("", 0, nil, false) when no source is
// available.
//
// Returning startLine + len(lines)-1 must cover frame.Line so renderers can
// highlight the focal line. ctxLines is the number of lines requested above
// and below the focal line; the resolver may return less.
type SourceResolver interface {
	Resolve(ctx context.Context, frame StackFrame, ctxLines int) (lines []string, startLine int, lang string, ok bool)
}

// SourceResolverFunc adapts a plain function to SourceResolver, mirroring
// http.HandlerFunc so callers don't have to declare a struct.
type SourceResolverFunc func(ctx context.Context, frame StackFrame, ctxLines int) (lines []string, startLine int, lang string, ok bool)

func (f SourceResolverFunc) Resolve(ctx context.Context, frame StackFrame, ctxLines int) ([]string, int, string, bool) {
	return f(ctx, frame, ctxLines)
}

// StackTraceOption mutates a StackTrace's render configuration. Use the
// WithXxx helpers; the option type is exported only so callers can build
// their own option sets without importing private state.
type StackTraceOption func(*stackTraceOptions)

type stackTraceOptions struct {
	include       []string
	exclude       []string
	contextLines  int
	resolver      SourceResolver
	resolverCtx   context.Context
	collapseEmpty bool
	highlightLine bool
	maxFrames     int
}

func defaultStackTraceOptions() stackTraceOptions {
	return stackTraceOptions{
		contextLines:  3,
		resolverCtx:   context.Background(),
		collapseEmpty: true,
		highlightLine: true,
	}
}

// WithStackInclude restricts decoration (frame display + source resolution) to
// classes whose fully-qualified name starts with one of the given prefixes.
// An empty include set means "match any".
func WithStackInclude(prefixes ...string) StackTraceOption {
	return func(o *stackTraceOptions) { o.include = append(o.include, prefixes...) }
}

// WithStackExclude drops frames whose class starts with any of the prefixes
// from the rendered output. Exclude beats include.
func WithStackExclude(prefixes ...string) StackTraceOption {
	return func(o *stackTraceOptions) { o.exclude = append(o.exclude, prefixes...) }
}

// WithStackContext sets the number of source lines rendered before and after
// each frame's reported line. Default 3. Must be >= 0.
func WithStackContext(n int) StackTraceOption {
	return func(o *stackTraceOptions) {
		if n >= 0 {
			o.contextLines = n
		}
	}
}

// WithSourceResolver attaches a SourceResolver that supplies inline source
// lines. Without one, frames render headers only.
func WithSourceResolver(r SourceResolver) StackTraceOption {
	return func(o *stackTraceOptions) { o.resolver = r }
}

// WithSourceResolverContext supplies the context.Context that resolver calls
// run under. Defaults to context.Background().
func WithSourceResolverContext(ctx context.Context) StackTraceOption {
	return func(o *stackTraceOptions) {
		if ctx != nil {
			o.resolverCtx = ctx
		}
	}
}

// WithMaxFrames truncates the rendered stack to the top N kept frames so very
// long traces don't dominate the output. Zero (default) keeps everything.
func WithMaxFrames(n int) StackTraceOption {
	return func(o *stackTraceOptions) {
		if n > 0 {
			o.maxFrames = n
		}
	}
}

// resolveAndApply runs the SourceResolver against each kept frame and copies
// the result onto the frame's SourceLines/SourceStartLine/SourceLanguage
// fields so renderers don't need to know about the resolver.
func (s *StackTrace) resolveAndApply() {
	if s.options.resolver == nil {
		return
	}
	for i := range s.Frames {
		f := &s.Frames[i]
		if !s.frameKept(*f) {
			continue
		}
		lines, start, lang, ok := s.options.resolver.Resolve(s.options.resolverCtx, *f, s.options.contextLines)
		if !ok || len(lines) == 0 {
			continue
		}
		f.SourceLines = lines
		f.SourceStartLine = start
		if lang != "" {
			f.SourceLanguage = lang
		} else if s.Language != "" {
			f.SourceLanguage = s.Language
		}
	}
}

func (s StackTrace) frameKept(f StackFrame) bool {
	for _, p := range s.options.exclude {
		if p != "" && strings.HasPrefix(f.Class, p) {
			return false
		}
	}
	if len(s.options.include) == 0 {
		return true
	}
	for _, p := range s.options.include {
		if p != "" && strings.HasPrefix(f.Class, p) {
			return true
		}
	}
	return false
}

// Render builds the styled api.Text for this trace. ANSI/HTML/Markdown all
// share the same Text tree — terminal renderers honor `Style` ANSI mappings,
// HTML preserves arbitrary Tailwind classes, and Markdown round-trips inline
// styled spans.
func (s StackTrace) Render() Text {
	out := Text{}
	if header := s.exceptionHeader(); header != "" {
		out = out.Add(Text{Content: header, Style: "font-bold text-red-600"})
	}
	for _, cause := range s.CausedBy {
		out = out.Add(Text{Content: "\n"})
		out = out.Add(Text{Content: "Caused by: " + cause, Style: "text-orange-600"})
	}
	for _, f := range s.keptFrames() {
		out = out.Add(Text{Content: "\n"})
		out = out.Add(renderFrameHeader(f))
		out = appendSourceLines(out, f, s.options)
	}
	return out
}

// String renders ANSI for fmt %s usage and convenient terminal printing.
func (s StackTrace) String() string {
	return s.Render().ANSI()
}

// ANSI / Pretty delegate to the assembled Text so a StackTrace is a drop-in
// Textable for any clicky formatter. HTML uses a dedicated structured renderer
// (renderHTML) so frames and source context get semantic blocks and monospaced
// layout instead of a single whitespace-collapsed line. Markdown renders a
// fenced code block (renderMarkdown) so the trace survives paste verbatim.
func (s StackTrace) ANSI() string     { return s.Render().ANSI() }
func (s StackTrace) HTML() string     { return s.renderHTML() }
func (s StackTrace) Markdown() string { return s.renderMarkdown() }

// MarkdownWithOptions ignores the dialect: a fenced block reads the same in
// every Markdown flavour clicky targets.
func (s StackTrace) MarkdownWithOptions(MarkdownOptions) string { return s.renderMarkdown() }
func (s StackTrace) Pretty() Text                               { return s.Render() }

// keptFrames returns the frames the include/exclude and max-frames options
// keep, in order.
func (s StackTrace) keptFrames() []StackFrame {
	var kept []StackFrame
	for _, f := range s.Frames {
		if !s.frameKept(f) {
			continue
		}
		if s.options.maxFrames > 0 && len(kept) >= s.options.maxFrames {
			break
		}
		kept = append(kept, f)
	}
	return kept
}

func (s StackTrace) exceptionHeader() string {
	header := s.ExceptionClass
	if s.Message != "" {
		if header != "" {
			header += ": "
		}
		header += s.Message
	}
	return header
}

// renderHTML produces a structured HTML block for the stack trace:
//
//   - Exception header in a red banner.
//   - One <div class="stack-frame"> per kept frame, with the frame header
//     (class.method + file:line) and an optional source-context <pre> below.
//   - Source-context lines render as a single <pre class="stack-source"> with
//     a left-gutter line number column and the focal line marked via
//     `font-bold text-red-700 bg-red-50` instead of an ASCII ">>>" prefix.
//   - When the frame's SourceLanguage is known, source lines pass through
//     Code.HTML() for syntax highlighting; otherwise they render as escaped
//     plain text.
//
// Every element carries its Tailwind classes (the Mission Control HTML
// formatter loads Tailwind) and the equivalent inline style (htmlOpenTag), so
// the trace keeps its colours when copied as rich text into email or docs,
// which strip <style> blocks and classes.
func (s StackTrace) renderHTML() string {
	var b strings.Builder
	// Per-element margin (rather than parent space-y-*) so header-only frames
	// can stack tightly while exception/cause/source-bearing cards keep gap.
	b.WriteString(htmlOpenTag("div", "stack-trace font-mono text-xs leading-tight"))

	if header := s.exceptionHeader(); header != "" {
		b.WriteString(htmlOpenTag("div", "stack-exception font-bold text-red-700 bg-red-50 border border-red-200 rounded px-2 mb-1"))
		b.WriteString(htmlEscapeString(header))
		b.WriteString(`</div>`)
	}

	for _, cause := range s.CausedBy {
		b.WriteString(htmlOpenTag("div", "stack-cause text-orange-700 bg-orange-50 border-l-2 border-orange-300 px-2 py-0.5 mb-1"))
		fmt.Fprintf(&b, `%sCaused by:</span> %s</div>`, htmlOpenTag("span", "font-semibold"), htmlEscapeString(cause))
	}

	for _, f := range s.keptFrames() {
		s.renderHTMLFrame(&b, f)
	}

	b.WriteString(`</div>`)
	return b.String()
}

// renderHTMLFrame emits one frame. Frames with source context render as a
// bordered card so the source block sits inside its own boundary; frames
// without source render as a flat row so a stack of header-only frames reads
// as a single coalesced list rather than a column of empty cards.
func (s StackTrace) renderHTMLFrame(b *strings.Builder, f StackFrame) {
	hasSource := frameHasSource(f)

	// Application frames are dark and semibold; runtime (JDK/framework)
	// frames are muted grey at reduced opacity.
	textClass := "text-slate-800"
	if f.Runtime {
		textClass = "text-slate-500"
	}

	if hasSource {
		wrapperClass := "stack-frame border border-slate-200 rounded overflow-hidden mb-1"
		if f.Runtime {
			wrapperClass = "stack-frame border border-slate-100 rounded overflow-hidden opacity-75 mb-1"
		}
		b.WriteString(htmlOpenTag("div", wrapperClass))
		b.WriteString(htmlOpenTag("div", "stack-frame-header "+textClass+" flex flex-wrap items-baseline gap-x-1 px-2 py-0.5 bg-slate-50 border-b border-slate-200"))
	} else {
		flatClass := "stack-frame-row flex flex-wrap items-baseline gap-x-1 px-2"
		if f.Runtime {
			flatClass += " opacity-75"
		}
		b.WriteString(htmlOpenTag("div", flatClass+" "+textClass))
	}

	fmt.Fprintf(b, `%sat</span> `, htmlOpenTag("span", "stack-frame-at text-slate-500"))
	fmt.Fprintf(b, `%s%s.%s%s</span></span>`, htmlOpenTag("span", "font-semibold "+textClass),
		htmlEscapeString(f.Class), htmlOpenTag("span", "text-blue-700"), htmlEscapeString(f.Method))

	if loc := frameLocation(f); loc != "" {
		fmt.Fprintf(b, `%s(%s)</span>`, htmlOpenTag("span", "text-slate-500"), htmlEscapeString(loc))
	}
	if f.Native {
		b.WriteString(htmlOpenTag("span", "italic text-slate-500") + `[native]</span>`)
	}
	if f.Annotation != "" {
		fmt.Fprintf(b, `%s— %s</span>`, htmlOpenTag("span", "italic text-slate-500"), htmlEscapeString(f.Annotation))
	}

	b.WriteString(`</div>`)
	if hasSource {
		s.renderHTMLSource(b, f)
		b.WriteString(`</div>`)
	}
}

func (s StackTrace) renderHTMLSource(b *strings.Builder, f StackFrame) {
	b.WriteString(htmlOpenTag("div", "stack-source bg-slate-50"))
	for idx, src := range f.SourceLines {
		line := frameSourceLineNumber(f, idx)
		// Every row carries a 2px left border so the focal row's red marker
		// doesn't shift the gutter/code columns relative to surrounding rows.
		// Non-focal rows use a transparent border to reserve the same width.
		rowClass := "flex border-l-2 border-transparent"
		gutterClass := "stack-source-gutter w-10 flex-none px-1 text-right text-slate-400 select-none border-r border-slate-200"
		codeClass := "stack-source-code font-mono px-2 whitespace-pre overflow-x-auto flex-1"
		if s.isFocalLine(f, line) {
			rowClass = "flex border-l-2 border-red-400 bg-red-50"
			gutterClass = "stack-source-gutter w-10 flex-none px-1 text-right text-red-600 font-bold select-none border-r border-red-200"
			codeClass = "stack-source-code font-mono px-2 whitespace-pre overflow-x-auto flex-1 font-bold text-red-700"
		}
		fmt.Fprintf(b, `%s%s%s</span>%s%s</span></div>`, htmlOpenTag("div", rowClass),
			htmlOpenTag("span", gutterClass), sourceGutter(line),
			htmlOpenTag("span", codeClass), renderHTMLSourceContent(src, f.SourceLanguage))
	}
	b.WriteString(`</div>`)
}

func (s StackTrace) isFocalLine(f StackFrame, line int) bool {
	return s.options.highlightLine && line > 0 && line == f.Line
}

// sourceGutter is a source row's line-number label: blank when the line is
// unknown (0), e.g. a decompiled row CFR could not map back to the class file.
func sourceGutter(line int) string {
	if line <= 0 {
		return ""
	}
	return strconv.Itoa(line)
}

// frameLocation is the `File:Line` text of a frame ("" without a file).
func frameLocation(f StackFrame) string {
	if f.File == "" {
		return ""
	}
	if f.Line > 0 {
		return fmt.Sprintf("%s:%d", f.File, f.Line)
	}
	return f.File
}

// renderHTMLSourceContent returns the inline HTML for a single source line:
// syntax-highlighted when a language is known (plain-escaped on wasm, see
// highlightCodeLineHTML), HTML-escaped otherwise.
func renderHTMLSourceContent(line, language string) string {
	if language == "" {
		return htmlEscapeString(line)
	}
	return highlightCodeLineHTML(line, language)
}

func renderFrameHeader(f StackFrame) Text {
	style := "font-semibold"
	if f.Runtime {
		style = "text-muted-foreground"
	}
	header := Text{Content: "  at " + f.Class + "." + f.Method, Style: style}
	if loc := frameLocation(f); loc != "" {
		header = header.Add(Text{Content: " (" + loc + ")", Style: "text-muted-foreground"})
	}
	if f.Native {
		header = header.Add(Text{Content: " [native]", Style: "italic text-muted-foreground"})
	}
	if f.Annotation != "" {
		header = header.Add(Text{Content: " — " + f.Annotation, Style: "italic text-muted-foreground"})
	}
	return header
}

func appendSourceLines(out Text, f StackFrame, opts stackTraceOptions) Text {
	if !frameHasSource(f) {
		return out
	}
	for idx, src := range f.SourceLines {
		line := frameSourceLineNumber(f, idx)
		out = out.Add(Text{Content: "\n"})
		marker := "    "
		style := "text-muted-foreground"
		if opts.highlightLine && line > 0 && line == f.Line {
			marker = ">>> "
			style = "font-bold text-red-600"
		}
		gutter := fmt.Sprintf("%4s: ", sourceGutter(line))
		if line <= 0 {
			gutter = "      "
		}
		out = out.Add(Text{Content: marker + gutter + src, Style: style})
	}
	return out
}

func frameHasSource(f StackFrame) bool {
	if len(f.SourceLines) == 0 {
		return false
	}
	if f.SourceStartLine > 0 {
		return true
	}
	for _, line := range f.SourceLineNumbers {
		if line > 0 {
			return true
		}
	}
	return false
}

// frameSourceLineNumber is the original line of source row idx, or 0 when it
// is unknown. Explicit SourceLineNumbers win (a 0 entry stays unknown rather
// than falling back to the row's window position); otherwise rows count up
// from SourceStartLine.
func frameSourceLineNumber(f StackFrame, idx int) int {
	if len(f.SourceLineNumbers) > 0 {
		if idx < len(f.SourceLineNumbers) && f.SourceLineNumbers[idx] > 0 {
			return f.SourceLineNumbers[idx]
		}
		return 0
	}
	if f.SourceStartLine > 0 {
		return f.SourceStartLine + idx
	}
	return 0
}

// NewStackTrace returns an empty trace with options applied. Language-specific
// parsers populate the fields after construction.
func NewStackTrace(opts ...StackTraceOption) StackTrace {
	s := StackTrace{options: defaultStackTraceOptions()}
	for _, opt := range opts {
		opt(&s.options)
	}
	return s
}
