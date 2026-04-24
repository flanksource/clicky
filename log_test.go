package clicky

import (
	"bytes"
	"strings"
	"testing"

	"github.com/flanksource/commons/logger"
)

func TestPrintln_TargetsLoggerOutput(t *testing.T) {
	original := logger.GetOutput()
	t.Cleanup(func() { logger.SetOutput(original) })

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	Println("hello", "world")

	if got := buf.String(); !strings.Contains(got, "hello world") {
		t.Fatalf("Println did not route through logger output: got %q", got)
	}
}

func TestPrintf_TargetsLoggerOutput(t *testing.T) {
	original := logger.GetOutput()
	t.Cleanup(func() { logger.SetOutput(original) })

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	Printf("n=%d\n", 42)

	if got := buf.String(); got != "n=42\n" {
		t.Fatalf("Printf unexpected output: got %q want %q", got, "n=42\n")
	}
}

func TestFprintln_UsesGivenWriter(t *testing.T) {
	original := logger.GetOutput()
	t.Cleanup(func() { logger.SetOutput(original) })

	var logBuf bytes.Buffer
	logger.SetOutput(&logBuf)

	var targetBuf bytes.Buffer
	Fprintln(&targetBuf, "to", "target")

	if logBuf.Len() != 0 {
		t.Fatalf("Fprintln leaked into logger output: %q", logBuf.String())
	}
	if got := targetBuf.String(); got != "to target\n" {
		t.Fatalf("Fprintln wrote %q want %q", got, "to target\n")
	}
}
