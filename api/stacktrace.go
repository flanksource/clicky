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
	Class           string
	Method          string
	File            string
	Line            int
	Native          bool
	Runtime         bool
	Annotation      string
	SourceLines     []string
	SourceStartLine int
	SourceLanguage  string
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

// ANSI / HTML / Markdown delegate to the assembled Text so a StackTrace is a
// drop-in Textable for any clicky formatter.
func (s StackTrace) ANSI() string     { return s.Render().ANSI() }
func (s StackTrace) HTML() string     { return s.Render().HTML() }
func (s StackTrace) Markdown() string { return s.Render().Markdown() }
func (s StackTrace) Pretty() Text     { return s.Render() }

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
