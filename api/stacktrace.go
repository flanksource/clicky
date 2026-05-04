package api

import (
	"context"
	"fmt"
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
	if s.ExceptionClass != "" || s.Message != "" {
		header := s.ExceptionClass
		if s.Message != "" {
			if header != "" {
				header += ": "
			}
			header += s.Message
		}
		out = out.Add(Text{Content: header, Style: "font-bold text-red-600"})
	}
	for _, cause := range s.CausedBy {
		out = out.Add(Text{Content: "\n"})
		out = out.Add(Text{Content: "Caused by: " + cause, Style: "text-orange-600"})
	}

	rendered := 0
	for _, f := range s.Frames {
		if !s.frameKept(f) {
			continue
		}
		if s.options.maxFrames > 0 && rendered >= s.options.maxFrames {
			break
		}
		out = out.Add(Text{Content: "\n"})
		out = out.Add(renderFrameHeader(f))
		out = appendSourceLines(out, f, s.options)
		rendered++
	}
	return out
}

// String renders ANSI for fmt %s usage and convenient terminal printing.
func (s StackTrace) String() string {
	return s.Render().ANSI()
}

// ANSI / Pretty / Markdown delegate to the assembled Text so a StackTrace is a
// drop-in Textable for any clicky formatter. HTML uses a dedicated structured
// renderer (renderHTML) so frames and source context get semantic blocks and
// monospaced layout instead of a single whitespace-collapsed line.
func (s StackTrace) ANSI() string     { return s.Render().ANSI() }
func (s StackTrace) HTML() string     { return s.renderHTML() }
func (s StackTrace) Markdown() string { return s.Render().Markdown() }
func (s StackTrace) Pretty() Text     { return s.Render() }

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
// Tailwind classes are emitted directly (the Mission Control HTML formatter
// already loads Tailwind), so no inline styles are needed.
func (s StackTrace) renderHTML() string {
	var b strings.Builder
	// Per-element margin (rather than parent space-y-*) so header-only frames
	// can stack tightly while exception/cause/source-bearing cards keep gap.
	b.WriteString(`<div class="stack-trace font-mono text-xs leading-tight">`)

	if s.ExceptionClass != "" || s.Message != "" {
		header := s.ExceptionClass
		if s.Message != "" {
			if header != "" {
				header += ": "
			}
			header += s.Message
		}
		fmt.Fprintf(&b, `<div class="stack-exception font-bold text-red-700 bg-red-50 border border-red-200 rounded px-2 mb-1">%s</div>`, htmlEscapeString(header))
	}

	for _, cause := range s.CausedBy {
		fmt.Fprintf(&b, `<div class="stack-cause text-orange-700 bg-orange-50 border-l-2 border-orange-300 px-2 py-0.5 mb-1"><span class="font-semibold">Caused by:</span> %s</div>`, htmlEscapeString(cause))
	}

	rendered := 0
	for _, f := range s.Frames {
		if !s.frameKept(f) {
			continue
		}
		if s.options.maxFrames > 0 && rendered >= s.options.maxFrames {
			break
		}
		s.renderHTMLFrame(&b, f)
		rendered++
	}

	b.WriteString(`</div>`)
	return b.String()
}

// renderHTMLFrame emits one frame. Frames with source context render as a
// bordered card so the source block sits inside its own boundary; frames
// without source render as a flat row so a stack of header-only frames reads
// as a single coalesced list rather than a column of empty cards.
func (s StackTrace) renderHTMLFrame(b *strings.Builder, f StackFrame) {
	hasSource := len(f.SourceLines) > 0 && f.SourceStartLine > 0

	textClass := "text-slate-800"
	if f.Runtime {
		textClass = "text-slate-500"
	}

	if hasSource {
		wrapperClass := "stack-frame border border-slate-200 rounded overflow-hidden mb-1"
		if f.Runtime {
			wrapperClass = "stack-frame border border-slate-100 rounded overflow-hidden opacity-75 mb-1"
		}
		fmt.Fprintf(b, `<div class="%s">`, wrapperClass)
		fmt.Fprintf(b, `<div class="stack-frame-header %s flex flex-wrap items-baseline gap-x-1 px-2 py-0.5 bg-slate-50 border-b border-slate-200">`, textClass)
	} else {
		flatClass := "stack-frame-row flex flex-wrap items-baseline gap-x-1 px-2"
		if f.Runtime {
			flatClass += " opacity-75"
		}
		fmt.Fprintf(b, `<div class="%s %s">`, flatClass, textClass)
	}

	fmt.Fprintf(b, `<span class="font-semibold">%s.<span class="text-blue-700">%s</span></span>`,
		htmlEscapeString(f.Class), htmlEscapeString(f.Method))

	if f.File != "" {
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		fmt.Fprintf(b, `<span class="text-slate-500">(%s)</span>`, htmlEscapeString(loc))
	}
	if f.Native {
		b.WriteString(`<span class="italic text-slate-500">[native]</span>`)
	}
	if f.Annotation != "" {
		fmt.Fprintf(b, `<span class="italic text-slate-500">— %s</span>`, htmlEscapeString(f.Annotation))
	}

	if hasSource {
		b.WriteString(`</div>`)
		s.renderHTMLSource(b, f)
		b.WriteString(`</div>`)
	} else {
		b.WriteString(`</div>`)
	}
}

func (s StackTrace) renderHTMLSource(b *strings.Builder, f StackFrame) {
	b.WriteString(`<div class="stack-source bg-slate-50">`)
	for idx, src := range f.SourceLines {
		line := f.SourceStartLine + idx
		if idx < len(f.SourceLineNumbers) && f.SourceLineNumbers[idx] > 0 {
			line = f.SourceLineNumbers[idx]
		}
		// Every row carries a 2px left border so the focal row's red marker
		// doesn't shift the gutter/code columns relative to surrounding rows.
		// Non-focal rows use a transparent border to reserve the same width.
		rowClass := "flex border-l-2 border-transparent"
		gutterClass := "stack-source-gutter w-10 flex-none px-1 text-right text-slate-400 select-none border-r border-slate-200"
		codeClass := "stack-source-code px-2 whitespace-pre overflow-x-auto flex-1"
		if s.options.highlightLine && line == f.Line {
			rowClass = "flex border-l-2 border-red-400 bg-red-50"
			gutterClass = "stack-source-gutter w-10 flex-none px-1 text-right text-red-600 font-bold select-none border-r border-red-200"
			codeClass = "stack-source-code px-2 whitespace-pre overflow-x-auto flex-1 font-bold text-red-700"
		}
		fmt.Fprintf(b, `<div class="%s"><span class="%s">%d</span><span class="%s">%s</span></div>`,
			rowClass, gutterClass, line, codeClass, renderHTMLSourceContent(src, f.SourceLanguage))
	}
	b.WriteString(`</div>`)
}

// renderHTMLSourceContent returns syntax-highlighted HTML for a single source
// line when a language is known; otherwise the line is HTML-escaped. Each
// chroma-tokenised <span> is preserved so multi-line highlighting stays
// consistent across rows.
func renderHTMLSourceContent(line, language string) string {
	if language == "" {
		return htmlEscapeString(line)
	}
	highlighted := strings.TrimRight(NewCode(line, language).HTML(), "\n")
	// chroma's HTML formatter wraps output in `<pre class="chroma">…</pre>`
	// (sometimes with a `<code>` inside); strip those so the line sits inline.
	highlighted = strings.TrimPrefix(highlighted, `<pre class="chroma">`)
	highlighted = strings.TrimPrefix(highlighted, `<code>`)
	highlighted = strings.TrimSuffix(highlighted, `</pre>`)
	highlighted = strings.TrimSuffix(highlighted, `</code>`)
	if strings.TrimSpace(highlighted) == "" {
		return htmlEscapeString(line)
	}
	return highlighted
}

func renderFrameHeader(f StackFrame) Text {
	style := "font-semibold"
	if f.Runtime {
		style = "text-muted-foreground"
	}
	header := Text{Content: "  at " + f.Class + "." + f.Method, Style: style}
	if f.File != "" {
		suffix := "(" + f.File
		if f.Line > 0 {
			suffix += fmt.Sprintf(":%d", f.Line)
		}
		suffix += ")"
		header = header.Add(Text{Content: " " + suffix, Style: "text-muted-foreground"})
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
	if len(f.SourceLines) == 0 || f.SourceStartLine <= 0 {
		return out
	}
	for idx, src := range f.SourceLines {
		line := f.SourceStartLine + idx
		if idx < len(f.SourceLineNumbers) && f.SourceLineNumbers[idx] > 0 {
			line = f.SourceLineNumbers[idx]
		}
		out = out.Add(Text{Content: "\n"})
		marker := "    "
		style := "text-muted-foreground"
		if opts.highlightLine && line == f.Line {
			marker = ">>> "
			style = "font-bold text-red-600"
		}
		out = out.Add(Text{Content: fmt.Sprintf("%s%4d: %s", marker, line, src), Style: style})
	}
	return out
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
