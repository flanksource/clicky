package api

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky/internal/markdownutil"
)

// renderMarkdown renders the trace as one fenced code block in the shape
// Throwable.printStackTrace() prints: the exception header, one
// `\tat class.method(file:line)` line per kept frame followed by its source
// window, then the `Caused by:` lines. The fence is longer than any backtick
// run in the content so a message containing ``` cannot close it early.
func (s StackTrace) renderMarkdown() string {
	var body strings.Builder
	if header := s.exceptionHeader(); header != "" {
		body.WriteString(header + "\n")
	}
	for _, f := range s.keptFrames() {
		body.WriteString(markdownFrameLine(f) + "\n")
		s.writeMarkdownSource(&body, f)
	}
	for _, cause := range s.CausedBy {
		body.WriteString("Caused by: " + cause + "\n")
	}
	language := s.Language
	if language == "" {
		language = "java"
	}
	fence := markdownutil.Fence(body.String())
	return fence + language + "\n" + body.String() + fence + "\n"
}

func markdownFrameLine(f StackFrame) string {
	line := "\tat " + f.Class + "." + f.Method
	switch loc := frameLocation(f); {
	case loc != "":
		line += "(" + loc + ")"
	case f.Native:
		line += "(Native Method)"
	default:
		line += "(Unknown Source)"
	}
	if f.Annotation != "" {
		line += " — " + f.Annotation
	}
	return line
}

func (s StackTrace) writeMarkdownSource(b *strings.Builder, f StackFrame) {
	if !frameHasSource(f) {
		return
	}
	for idx, src := range f.SourceLines {
		line := frameSourceLineNumber(f, idx)
		marker := "    "
		if s.isFocalLine(f, line) {
			marker = ">>> "
		}
		fmt.Fprintf(b, "\t%s%4s  %s\n", marker, sourceGutter(line), src)
	}
}
