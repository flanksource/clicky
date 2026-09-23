//go:build wasm

package wasmtest

import (
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
)

type terminalRow struct {
	name string
	note string
}

func (r terminalRow) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		api.Column("name").Label("Name").Build(),
		api.Column("note").Label("Note").Build(),
		api.Column("unused").Label("Unused").Build(),
	}
}

func (r terminalRow) Row() map[string]any {
	return map[string]any{"name": r.name, "note": r.note, "unused": ""}
}

func TestTextTableStringRendersAlignedColumnsWithoutLipgloss(t *testing.T) {
	rendered := api.NewTableFrom([]terminalRow{
		{name: "alpha", note: "pipe | inside"},
		{name: "βeta", note: "multi\nline"},
		{name: "日本", note: "wide"},
	}).String()

	want := strings.Join([]string{
		"Name   Note",
		"─────  ─────────────",
		"alpha  pipe | inside",
		"βeta   multi",
		"       line",
		"日本   wide",
	}, "\n")
	if rendered != want {
		t.Fatalf("String() =\n%q\nwant\n%q", rendered, want)
	}
}

func textNode(label string, children ...api.TextTree) api.TextTree {
	return api.TextTree{Node: api.Text{Content: label}, Children: children}
}

func TestTextTreeStringRendersRoundedConnectorsWithoutLipgloss(t *testing.T) {
	rendered := textNode("root",
		textNode("child-a\nsecond line", textNode("leaf")),
		textNode("child-b"),
	).String()

	want := strings.Join([]string{
		"root",
		"├── child-a",
		"│   second line",
		"│   ╰── leaf",
		"╰── child-b",
	}, "\n")
	if rendered != want {
		t.Fatalf("String() =\n%q\nwant\n%q", rendered, want)
	}
}

func TestEmptyTextTableAndTreeRenderNothing(t *testing.T) {
	if rendered := (api.TextTable{}).String(); rendered != "" {
		t.Fatalf("TextTable{}.String() = %q, want empty", rendered)
	}
	if rendered := (api.TextTree{}).String(); rendered != "" {
		t.Fatalf("TextTree{}.String() = %q, want empty", rendered)
	}
}
