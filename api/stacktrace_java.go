package api

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	javaFrameRegexp        = regexp.MustCompile(`^\s*at\s+([\w$.]+)\.([\w$<>]+)\(([^)]+)\)`)
	javaHeaderRegexp       = regexp.MustCompile(`^([\w.$]+(?:Exception|Error|Throwable))(?::\s*(.*))?$`)
	javaContinuationRegexp = regexp.MustCompile(`^\.\.\.\s+\d+\s+more$`)
)

// ParseJavaStackTrace parses a free-form Java exception dump (the body of
// `java.lang.Throwable.printStackTrace()`) into a StackTrace ready to render.
// It tolerates common surrounding noise: EclipseLink "Internal Exception:" /
// "Caused by:" / "... N more" continuation markers, and frames whose
// `(File:Line)` parenthesised location omits the line number, marks the
// method as native, or wraps a JAR descriptor like
// `(SomeClass.java:42) ~[exampleapp-1.0.jar:?]`.
//
// Frames whose class belongs to a JDK / framework package
// (java., javax., jdk., sun., com.sun.) are tagged with Runtime=true so
// renderers can mute them visually. Native methods (`Native Method`) get
// Native=true.
func ParseJavaStackTrace(input string, opts ...StackTraceOption) StackTrace {
	s := NewStackTrace(opts...)
	s.Language = "java"

	if strings.TrimSpace(input) == "" {
		return s
	}

	var headerLines []string
	for _, raw := range strings.Split(input, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		if javaContinuationRegexp.MatchString(trimmed) {
			continue
		}

		if m := javaFrameRegexp.FindStringSubmatch(raw); m != nil {
			s.Frames = append(s.Frames, parseJavaFrame(m))
			continue
		}

		if strings.HasPrefix(trimmed, "Caused by:") {
			s.CausedBy = append(s.CausedBy, strings.TrimSpace(strings.TrimPrefix(trimmed, "Caused by:")))
			continue
		}
		if strings.HasPrefix(trimmed, "Internal Exception:") {
			s.CausedBy = append(s.CausedBy, strings.TrimSpace(strings.TrimPrefix(trimmed, "Internal Exception:")))
			continue
		}

		if s.ExceptionClass == "" {
			if m := javaHeaderRegexp.FindStringSubmatch(trimmed); m != nil {
				s.ExceptionClass = m[1]
				if m[2] != "" {
					s.Message = m[2]
				}
				continue
			}
		}

		headerLines = append(headerLines, trimmed)
	}

	if s.Message == "" && len(headerLines) > 0 {
		s.Message = strings.Join(headerLines, " ")
	}

	for i := range s.Frames {
		s.Frames[i].Runtime = isJavaRuntimeFrame(s.Frames[i].Class)
	}

	s.resolveAndApply()
	return s
}

func parseJavaFrame(m []string) StackFrame {
	f := StackFrame{Class: m[1], Method: m[2]}
	loc := strings.TrimSpace(m[3])
	switch {
	case loc == "Native Method":
		f.Native = true
	case loc == "Unknown Source":
		// no file/line info
	default:
		// strip jar descriptor like " ~[exampleapp.jar:?]"
		if idx := strings.Index(loc, " ~["); idx >= 0 {
			loc = loc[:idx]
		}
		if i := strings.LastIndex(loc, ":"); i >= 0 {
			f.File = loc[:i]
			if n, err := strconv.Atoi(loc[i+1:]); err == nil {
				f.Line = n
			}
		} else {
			f.File = loc
		}
	}
	return f
}

func isJavaRuntimeFrame(class string) bool {
	for _, prefix := range []string{"java.", "javax.", "jdk.", "sun.", "com.sun."} {
		if strings.HasPrefix(class, prefix) {
			return true
		}
	}
	return false
}
