package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRootInputFormatAutoMarkdownToClickyJSON(t *testing.T) {
	out, _, err := executeRoot(t,
		[]string{"--input-format", "auto", "--format", "clicky-json"},
		"# Report\n\n- [x] Done\n")
	if err != nil {
		t.Fatalf("execute root: %v", err)
	}

	var payload struct {
		Version int `json:"version"`
		Node    struct {
			Kind     string `json:"kind"`
			Children []struct {
				Kind  string `json:"kind"`
				Level int    `json:"level,omitempty"`
			} `json:"children"`
		} `json:"node"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal clicky-json: %v\n%s", err, out)
	}
	if payload.Version != 1 || payload.Node.Kind != "document" {
		t.Fatalf("unexpected clicky-json envelope: %#v", payload)
	}
	if len(payload.Node.Children) != 2 || payload.Node.Children[0].Kind != "heading" || payload.Node.Children[0].Level != 1 {
		t.Fatalf("unexpected parsed markdown children: %#v", payload.Node.Children)
	}
	assertNoInputParserProvenance(t, out)
}

func TestRootInputFormatDefaultsToAutoForStdin(t *testing.T) {
	out, _, err := executeRoot(t,
		[]string{"--format", "clicky-json"},
		"# Report\n\n- [x] Done\n")
	if err != nil {
		t.Fatalf("execute root: %v", err)
	}

	var payload struct {
		Version int `json:"version"`
		Node    struct {
			Kind     string `json:"kind"`
			Children []struct {
				Kind  string `json:"kind"`
				Level int    `json:"level,omitempty"`
			} `json:"children"`
		} `json:"node"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal clicky-json: %v\n%s", err, out)
	}
	if payload.Version != 1 || payload.Node.Kind != "document" {
		t.Fatalf("unexpected clicky-json envelope: %#v", payload)
	}
	if len(payload.Node.Children) != 2 || payload.Node.Children[0].Kind != "heading" || payload.Node.Children[0].Level != 1 {
		t.Fatalf("unexpected parsed markdown children: %#v", payload.Node.Children)
	}
	assertNoInputParserProvenance(t, out)
}

func TestRootInputFormatAutoJSONToMarkdown(t *testing.T) {
	out, _, err := executeRoot(t,
		[]string{"--input-format", "auto", "--format", "markdown"},
		`[{"name":"api","status":"ok"}]`)
	if err != nil {
		t.Fatalf("execute root: %v", err)
	}
	if !strings.Contains(out, "api") || !strings.Contains(out, "ok") {
		t.Fatalf("markdown output missing JSON values:\n%s", out)
	}
}

func TestRootInputFormatOutputDirectoryUsesSourceName(t *testing.T) {
	tempDir := t.TempDir()
	out, _, err := executeRoot(t,
		[]string{"--input-format", "auto", "--format", "clicky-json", "--output", tempDir, kitchenSinkExamplePath(t)},
		"")
	if err != nil {
		t.Fatalf("execute root: %v", err)
	}
	if out != "" {
		t.Fatalf("expected no stdout when --output is set, got %q", out)
	}

	outputPath := filepath.Join(tempDir, "kitchen-sink.clicky.json")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	var payload struct {
		Version int `json:"version"`
		Node    struct {
			Kind string `json:"kind"`
		} `json:"node"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal output file: %v\n%s", err, string(data))
	}
	if payload.Version != 1 || payload.Node.Kind != "document" {
		t.Fatalf("unexpected output payload: %#v", payload)
	}
	assertNoInputParserProvenance(t, string(data))
}

func executeRoot(t *testing.T, args []string, stdin string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand()
	cmd.SetArgs(args)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func kitchenSinkExamplePath(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate command test file")
	}
	return filepath.Join(filepath.Dir(currentFile), "..", "..", "examples", "kitchen-sink.md")
}

func assertNoInputParserProvenance(t *testing.T, out string) {
	t.Helper()
	for _, field := range []string{"sourceMarkdown", "lineStart", "lineEnd"} {
		if strings.Contains(out, field) {
			t.Fatalf("clicky-json leaked parser provenance field %q:\n%s", field, out)
		}
	}
}
