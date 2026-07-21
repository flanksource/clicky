package api

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type cappedWidthTableRow struct{}

func (cappedWidthTableRow) Columns() []ColumnDef {
	return []ColumnDef{
		Column("agent").Label("Agent").MaxWidth(12).Build(),
		Column("session").Label("Session").MaxWidth(8).Build(),
		Column("title").Label("Title").Build(),
	}
}

type shrinkingCappedWidthTableRow struct{}

func (shrinkingCappedWidthTableRow) Columns() []ColumnDef {
	return []ColumnDef{
		Column("prompt").Label("Prompt").MaxWidth(30).Build(),
		Column("usage").Label("Usage").Build(),
	}
}

func (shrinkingCappedWidthTableRow) Row() map[string]any {
	return map[string]any{
		"prompt": "A capped column must yield space when the terminal is narrow",
		"usage":  "$1.25",
	}
}

// uncappedProseTableRow is the shape a caller uses to say "let this column run
// to whatever the terminal allows, then truncate" -- no MaxWidth at all.
type uncappedProseTableRow struct{}

func (uncappedProseTableRow) Columns() []ColumnDef {
	return []ColumnDef{
		Column("prose").Label("Prose").Style("max-lines-[1] truncate-suffix").Build(),
		Column("usage").Label("Usage").Build(),
	}
}

// uncappedProse is deliberately longer than any width a caller would hard-code,
// so a spec can tell "ran to the terminal edge" apart from "hit a cap".
const uncappedProse = "An uncapped column runs all the way to the terminal edge and is then truncated there, rather than at some width chosen in advance"

func (uncappedProseTableRow) Row() map[string]any {
	return map[string]any{
		"prose": uncappedProse,
		"usage": "$1.25",
	}
}

func (cappedWidthTableRow) Row() map[string]any {
	return map[string]any{
		"agent":   "codex-agent",
		"session": "019f5c3c",
		"title":   "Title uses all terminal space left after capped columns",
	}
}

var _ = Describe("WithoutEmptyColumns", func() {
	It("removes columns where every row is empty", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "Name"}, Text{Content: "Age"}, Text{Content: "Notes"}},
			FieldNames: []string{"name", "age", "notes"},
		}
		table.Rows = []TableRow{
			{"name": TypedValue{Textable: Text{Content: "Alice"}}, "age": TypedValue{Textable: Text{Content: "30"}}, "notes": TypedValue{Textable: Text{}}},
			{"name": TypedValue{Textable: Text{Content: "Bob"}}, "age": TypedValue{Textable: Text{Content: "25"}}, "notes": TypedValue{Textable: Text{}}},
		}

		filtered := table.WithoutEmptyColumns()
		Expect(filtered.Headers).To(HaveLen(2))
		Expect(filtered.Headers[0].String()).To(Equal("Name"))
		Expect(filtered.Headers[1].String()).To(Equal("Age"))
		Expect(filtered.FieldNames).To(Equal([]string{"name", "age"}))
		Expect(filtered.Rows).To(HaveLen(2))
		Expect(filtered.Rows[0]).To(HaveKey("name"))
		Expect(filtered.Rows[0]).To(HaveKey("age"))
		Expect(filtered.Rows[0]).NotTo(HaveKey("notes"))
	})

	It("keeps all columns when none are fully empty", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "A"}, Text{Content: "B"}},
			FieldNames: []string{"a", "b"},
		}
		table.Rows = []TableRow{
			{"a": TypedValue{Textable: Text{Content: "1"}}, "b": TypedValue{Textable: Text{Content: "2"}}},
		}

		filtered := table.WithoutEmptyColumns()
		Expect(filtered.Headers).To(HaveLen(2))
	})

	It("keeps column if at least one row has a value", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "X"}, Text{Content: "Y"}},
			FieldNames: []string{"x", "y"},
		}
		table.Rows = []TableRow{
			{"x": TypedValue{Textable: Text{Content: "val"}}, "y": TypedValue{Textable: Text{}}},
			{"x": TypedValue{Textable: Text{}}, "y": TypedValue{Textable: Text{Content: "val"}}},
		}

		filtered := table.WithoutEmptyColumns()
		Expect(filtered.Headers).To(HaveLen(2))
	})

	It("returns same table when no rows exist", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "A"}},
			FieldNames: []string{"a"},
		}
		filtered := table.WithoutEmptyColumns()
		Expect(filtered.Headers).To(HaveLen(1))
	})

	It("treats whitespace-only values as empty", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "Col"}},
			FieldNames: []string{"col"},
		}
		table.Rows = []TableRow{
			{"col": TypedValue{Textable: Text{Content: "  "}}},
		}

		filtered := table.WithoutEmptyColumns()
		Expect(filtered.Headers).To(BeEmpty())
	})

	It("does not panic when all columns are filtered out during render", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "Col"}},
			FieldNames: []string{"col"},
		}
		table.Rows = []TableRow{
			{"col": TypedValue{Textable: Text{Content: "  "}}},
		}

		Expect(func() { _ = table.String() }).NotTo(Panic())
		Expect(table.String()).To(BeEmpty())
		Expect(func() { table.ANSI() }).NotTo(Panic())
	})
})

var _ = Describe("TextTable Markdown pipe escaping", func() {
	It("escapes a literal pipe in a cell so the GFM table stays intact", func() {
		t := TextTable{
			Headers:    TextList{Text{Content: "A"}, Text{Content: "B"}},
			FieldNames: []string{"a", "b"},
			Rows: []TableRow{
				{"a": TypedValue{Textable: Text{Content: "x|y"}}, "b": TypedValue{Textable: Text{Content: "ok"}}},
			},
		}
		Expect(t.Markdown()).To(ContainSubstring(`x\|y`))
		Expect(t.Markdown()).NotTo(ContainSubstring(`| x|y |`))
	})
})

// headerCells returns the rune width of each cell in the rendered header row
// identified by label, which is how these specs observe the allocated widths.
func headerCells(rendered, label string) []int {
	for _, line := range strings.Split(rendered, "\n") {
		if !strings.Contains(line, "│"+label) {
			continue
		}
		var widths []int
		for _, cell := range strings.Split(strings.Trim(line, "│"), "│") {
			widths = append(widths, len([]rune(cell)))
		}
		return widths
	}
	return nil
}

var _ = Describe("TextTable terminal column widths", func() {
	It("sizes columns to their content rather than filling the terminal", func() {
		previousWidth := terminalWidth.Swap(120)
		DeferCleanup(func() { terminalWidth.Store(previousWidth) })

		widths := headerCells(NewTableFrom([]cappedWidthTableRow{{}}).String(), "Agent")
		Expect(widths).To(HaveLen(3))
		// Widest value per column: "codex-agent", "019f5c3c", and the title.
		Expect(widths[0]).To(Equal(len("codex-agent")))
		Expect(widths[1]).To(Equal(len("019f5c3c")))
		Expect(widths[2]).To(Equal(len("Title uses all terminal space left after capped columns")))
	})

	It("takes the deficit from the widest column and leaves narrow ones alone", func() {
		previousWidth := terminalWidth.Swap(30)
		DeferCleanup(func() { terminalWidth.Store(previousWidth) })

		widths := headerCells(NewTableFrom([]shrinkingCappedWidthTableRow{{}}).String(), "Prompt")
		Expect(widths).To(HaveLen(2))
		// 30 columns less 3 borders leaves 27. Usage sits under the ceiling and
		// keeps its content width, so Prompt gives up the whole deficit.
		Expect(widths[1]).To(Equal(len("$1.25")))
		Expect(widths[0] + widths[1]).To(Equal(27))
	})

	It("lets a column with no declared MaxWidth run to the terminal edge", func() {
		previousWidth := terminalWidth.Swap(200)
		DeferCleanup(func() { terminalWidth.Store(previousWidth) })

		widths := headerCells(NewTableFrom([]uncappedProseTableRow{{}}).String(), "Prose")
		Expect(widths).To(HaveLen(2))
		Expect(widths[0]).To(Equal(len(uncappedProse)))
		Expect(len(uncappedProse)).To(BeNumerically(">", 100))
	})

	It("shrinks a column with no declared MaxWidth just the same", func() {
		previousWidth := terminalWidth.Swap(30)
		DeferCleanup(func() { terminalWidth.Store(previousWidth) })

		widths := headerCells(NewTableFrom([]uncappedProseTableRow{{}}).String(), "Prose")
		Expect(widths).To(HaveLen(2))
		Expect(widths[1]).To(Equal(len("$1.25")))
		Expect(widths[0] + widths[1]).To(Equal(27))
	})

	It("keeps every column readable when they all must shrink", func() {
		previousWidth := terminalWidth.Swap(16)
		DeferCleanup(func() { terminalWidth.Store(previousWidth) })

		widths := headerCells(NewTableFrom([]shrinkingCappedWidthTableRow{{}}).String(), "Prompt")
		Expect(widths).To(HaveLen(2))
		for _, width := range widths {
			Expect(width).To(BeNumerically(">=", columnMinWidth))
		}
	})

	It("truncates an over-long cell instead of wrapping it onto a second line", func() {
		previousWidth := terminalWidth.Swap(30)
		DeferCleanup(func() { terminalWidth.Store(previousWidth) })

		rendered := NewTableFrom([]shrinkingCappedWidthTableRow{{}}).String()
		Expect(rendered).To(ContainSubstring("$1.25"))
		// Border top, header, separator, one row, border bottom.
		Expect(strings.Split(strings.TrimRight(rendered, "\n"), "\n")).To(HaveLen(5))
	})

	It("preserves explicit lines within a cell while automatic wrapping is disabled", func() {
		previousWidth := terminalWidth.Swap(30)
		DeferCleanup(func() { terminalWidth.Store(previousWidth) })

		table := TextTable{
			Headers:    TextList{Text{Content: "Name"}, Text{Content: "Count"}},
			FieldNames: []string{"name", "count"},
			Rows: []TableRow{{
				"name":  TypedValue{Textable: Text{Content: "Item1\nItem2"}},
				"count": TypedValue{Textable: Text{Content: "10\n20"}},
			}},
		}

		rendered := table.String()
		Expect(rendered).To(ContainSubstring("Item1"))
		Expect(rendered).To(ContainSubstring("Item2"))
		Expect(rendered).To(ContainSubstring("10"))
		Expect(rendered).To(ContainSubstring("20"))
	})
})
