package api

import (
	"context"
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

func TestStackTrace_MaxFramesTruncates(t *testing.T) {
	s := ParseJavaStackTrace(javaSample, WithMaxFrames(1))
	out := s.Render().ANSI()
	if strings.Count(out, "  at ") != 1 {
		t.Fatalf("expected 1 frame after truncation, got:\n%s", out)
	}
}
