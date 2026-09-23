//go:build wasm

package wasmtest

import (
	"testing"

	"github.com/flanksource/clicky/api"
)

type markdownRow struct {
	name string
	note any
}

func (r markdownRow) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		api.Column("name").Label("display_name").Build(),
		api.Column("note").Label("Note").MaxWidth(13).Build(),
	}
}

func (r markdownRow) Row() map[string]any {
	return map[string]any{"name": r.name, "note": r.note}
}

func TestTextTableMarkdownRendersUnpaddedGFMWithEscapedWidthLimitedCells(t *testing.T) {
	rendered := api.NewTableFrom([]markdownRow{
		{name: "alpha", note: "pipe | inside"},
		{name: "βeta", note: "multi\nline"},
		{name: "gamma", note: "abcdefghijklmno"},
	}).Markdown()

	want := "\n" +
		"| DISPLAY NAME | NOTE |\n" +
		"| --- | --- |\n" +
		"| alpha | pipe \\| inside |\n" +
		"| βeta | multi<br>line |\n" +
		"| gamma | abcdefghijkl… |\n"
	if rendered != want {
		t.Fatalf("Markdown() =\n%q\nwant\n%q", rendered, want)
	}
}

func TestTextTableMarkdownWithoutHeadersIsEmpty(t *testing.T) {
	if rendered := (api.TextTable{}).Markdown(); rendered != "" {
		t.Fatalf("Markdown() = %q, want empty", rendered)
	}
}
