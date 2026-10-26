package api

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

const javaSample = `javax.persistence.PersistenceException: deadlock victim
    at com.example.admin.pas.dal.ActivityDal.findNextPendingActivity(ActivityDal.java:241)
    at com.example.admin.pas.bll.ClientBll.getNextPendingActivityExecutorBll(ClientBll.java:305)
    at java.util.concurrent.ThreadPoolExecutor$Worker.run(ThreadPoolExecutor.java:628)
Caused by: com.microsoft.sqlserver.jdbc.SQLServerException: deadlock victim
    ... 65 more`

func TestParseJavaStackTrace_Sample(t *testing.T) {
	s := ParseJavaStackTrace(javaSample)
	if s.ExceptionClass != "javax.persistence.PersistenceException" {
		t.Fatalf("ExceptionClass = %q", s.ExceptionClass)
	}
	if s.Message != "deadlock victim" {
		t.Fatalf("Message = %q", s.Message)
	}
	if got := len(s.Frames); got != 3 {
		t.Fatalf("Frames = %d, want 3", got)
	}
	if s.Frames[0].Class != "com.example.admin.pas.dal.ActivityDal" {
		t.Fatalf("Frames[0].Class = %q", s.Frames[0].Class)
	}
	if s.Frames[0].Line != 241 {
		t.Fatalf("Frames[0].Line = %d", s.Frames[0].Line)
	}
	if !s.Frames[2].Runtime {
		t.Fatalf("expected Frames[2] to be marked runtime")
	}
	if got := len(s.CausedBy); got != 1 {
		t.Fatalf("CausedBy = %d", got)
	}
}

func TestParseJavaStackTrace_NativeAndUnknown(t *testing.T) {
	s := ParseJavaStackTrace(`Foo: bar
    at sun.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
    at sun.reflect.NativeMethodAccessorImpl.invoke(Unknown Source)`)
	if !s.Frames[0].Native {
		t.Fatalf("expected Native=true on first frame")
	}
	if s.Frames[1].File != "" || s.Frames[1].Line != 0 {
		t.Fatalf("Unknown Source should leave file/line empty: %+v", s.Frames[1])
	}
}

func TestStackTrace_RenderIncludesFramesAndSources(t *testing.T) {
	resolver := SourceResolverFunc(func(_ context.Context, frame StackFrame, ctxLines int) ([]string, int, string, bool) {
		if frame.Class != "com.example.admin.pas.dal.ActivityDal" {
			return nil, 0, "", false
		}
		return []string{"a", "b", "c", "d", "e"}, frame.Line - 2, "java", true
	})

	s := ParseJavaStackTrace(javaSample,
		WithSourceResolver(resolver),
		WithStackInclude("com.example.admin."),
		WithStackContext(2),
	)

	rendered := s.Render().ANSI()
	if !strings.Contains(rendered, "ActivityDal.findNextPendingActivity") {
		t.Fatalf("expected ExampleApp frame in output, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "ThreadPoolExecutor") {
		t.Fatalf("excluded JDK frame should not appear, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "241:") {
		t.Fatalf("expected focal line 241 in output, got:\n%s", rendered)
	}
}

func TestStackTrace_NoResolverRendersHeadersOnly(t *testing.T) {
	s := ParseJavaStackTrace(javaSample)
	out := s.Render().ANSI()
	if !strings.Contains(out, "ActivityDal.findNextPendingActivity") {
		t.Fatalf("missing frame header, got:\n%s", out)
	}
	if strings.Contains(out, ">>>") {
		t.Fatalf("no resolver was attached but source highlight present:\n%s", out)
	}
}

func TestStackTrace_RendersExplicitSourceLineNumbersWithoutStartLine(t *testing.T) {
	s := NewStackTrace()
	s.Frames = []StackFrame{{
		Class:             "com.example.App",
		Method:            "run",
		File:              "App.java",
		Line:              42,
		SourceLines:       []string{"before();", "run();", "after();"},
		SourceLineNumbers: []int{41, 42, 43},
		SourceLanguage:    "java",
	}}

	out := s.Render().ANSI()
	if !strings.Contains(out, "42: run();") {
		t.Fatalf("expected explicit source line number in output, got:\n%s", out)
	}
}

// emailTrace has one application frame with a source window whose middle row
// CFR could not map (line number 0) and one runtime frame, so every visual
// role the HTML renderer emits is present.
func emailTrace() StackTrace {
	s := NewStackTrace()
	s.ExceptionClass = "java.lang.IllegalStateException"
	s.Message = "boom"
	s.CausedBy = []string{"java.sql.SQLException: deadlock victim"}
	s.Frames = []StackFrame{
		{
			Class:             "com.example.App",
			Method:            "run",
			File:              "App.java",
			Line:              42,
			SourceLines:       []string{"before();", "synthetic();", "run();"},
			SourceLineNumbers: []int{41, 0, 42},
		},
		{Class: "java.lang.Thread", Method: "run", File: "Thread.java", Line: 750, Runtime: true},
	}
	return s
}

var (
	classTagRegexp  = regexp.MustCompile(`<(span|div)( class="[^"]*")([^>]*)>`)
	cssColourRegexp = regexp.MustCompile(`color: ([^;"]+)|solid ([^;"]+)`)
)

func TestStackTrace_HTMLCarriesInlineStylesOnEveryStyledElement(t *testing.T) {
	out := emailTrace().HTML()
	for _, m := range classTagRegexp.FindAllStringSubmatch(out, -1) {
		if !strings.Contains(m[3], ` style="`) {
			t.Errorf("element%s has classes but no inline style", m[2])
		}
	}
	for _, m := range cssColourRegexp.FindAllStringSubmatch(out, -1) {
		if value := m[1] + m[2]; !strings.HasPrefix(value, "#") && value != "transparent" {
			t.Errorf("invalid CSS colour %q in HTML", m[0])
		}
	}
	for name, want := range map[string]string{
		"method":           `style="color: #1d4ed8">run</span>`,
		"runtime frame":    `color: #64748b`,
		"focal line bg":    `background-color: #fef2f2`,
		"monospace":        `font-family: ui-monospace`,
		"exception colour": `color: #b91c1c`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%s: expected %q in HTML:\n%s", name, want, out)
		}
	}
}

func TestStackTrace_ZeroLineSourceRowHasBlankGutter(t *testing.T) {
	s := emailTrace()
	gutters := regexp.MustCompile(`<span class="stack-source-gutter[^"]*" style="[^"]*">([^<]*)</span>`).
		FindAllStringSubmatch(s.HTML(), -1)
	var got []string
	for _, g := range gutters {
		got = append(got, g[1])
	}
	if strings.Join(got, ",") != "41,,42" {
		t.Fatalf("HTML gutters = %q, want [41 '' 42]", got)
	}
	for format, out := range map[string]string{"ansi": s.Render().String(), "markdown": s.Markdown()} {
		if strings.Contains(out, " 0:") || strings.Contains(out, " 0 ") {
			t.Errorf("%s renders a 0 gutter for an unmapped row:\n%s", format, out)
		}
		if !strings.Contains(out, "synthetic();") {
			t.Errorf("%s dropped the unmapped row:\n%s", format, out)
		}
	}
}

func TestStackTrace_MarkdownIsAFencedJavaTrace(t *testing.T) {
	s := emailTrace()
	s.Message = "bad ``` fence"
	want := "````java\n" +
		"java.lang.IllegalStateException: bad ``` fence\n" +
		"\tat com.example.App.run(App.java:42)\n" +
		"\t      41  before();\n" +
		"\t          synthetic();\n" +
		"\t>>>   42  run();\n" +
		"\tat java.lang.Thread.run(Thread.java:750)\n" +
		"Caused by: java.sql.SQLException: deadlock victim\n" +
		"````\n"
	if got := s.Markdown(); got != want {
		t.Fatalf("Markdown mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
	if got := s.MarkdownWithOptions(MarkdownOptions{}); got != want {
		t.Fatalf("MarkdownWithOptions mismatch\n got:\n%s", got)
	}
}

func TestStackTrace_MaxFramesTruncates(t *testing.T) {
	s := ParseJavaStackTrace(javaSample, WithMaxFrames(1))
	out := s.Render().ANSI()
	if strings.Count(out, "  at ") != 1 {
		t.Fatalf("expected 1 frame after truncation, got:\n%s", out)
	}
}
