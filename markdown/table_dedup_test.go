package markdown

import "testing"

func TestUniqueColumnName(t *testing.T) {
	seen := map[string]int{}
	got := []string{
		uniqueColumnName("name", seen),
		uniqueColumnName("name", seen),
		uniqueColumnName("name", seen),
		uniqueColumnName("status", seen),
	}
	want := []string{"name", "name_2", "name_3", "status"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("uniqueColumnName #%d = %q, want %q (all: %v)", i, got[i], want[i], got)
		}
	}
}

// findTable returns the first table node in the tree, or false.
func findTable(n Node) (Node, bool) {
	if n.Kind == "table" {
		return n, true
	}
	for _, c := range n.Children {
		if t, ok := findTable(c); ok {
			return t, ok
		}
	}
	return Node{}, false
}

func TestTableClickyDeduplicatesDuplicateHeaderLabels(t *testing.T) {
	doc, err := ParseString("| Name | Name |\n| --- | --- |\n| a | b |\n")
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	table, ok := findTable(doc.Root)
	if !ok {
		t.Fatal("no table node found")
	}

	columns, rows := tableClicky(table)
	if len(columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(columns))
	}
	if columns[0].Name == columns[1].Name {
		t.Fatalf("duplicate header labels produced colliding column names: %q", columns[0].Name)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 data row, got %d", len(rows))
	}
	// Both cells must survive — a key collision would drop one into the other.
	if len(rows[0].Cells) != 2 {
		t.Fatalf("expected 2 distinct cells, got %d: %v", len(rows[0].Cells), rows[0].Cells)
	}
}
