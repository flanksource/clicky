package mcp

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderClientListEscapesMarkdownCells(t *testing.T) {
	records := []clientListRecord{{
		Name: "demo", Transport: "stdio", Endpoint: "echo a|b\nnext", ToolCount: 1, Cache: "fresh",
		Tools: []CachedTool{{Name: "read|file", Description: "first line\r\nsecond | line"}},
	}}
	var output bytes.Buffer
	if err := renderClientList(&output, records, "markdown", true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`| demo | stdio | echo a\|b<br>next | 1 | fresh |`,
		`| ↳ read\|file | | first line<br>second \| line | | |`,
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("markdown output missing %q:\n%s", want, output.String())
		}
	}
}
