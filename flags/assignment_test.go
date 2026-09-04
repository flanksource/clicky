package flags

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	"github.com/xuri/excelize/v2"
)

// allowFileRead is the opt-in a field carries via clicky:"cli-file-read"; the
// loader does nothing without it.
var allowFileRead = FileReadPolicy{Enabled: true}

func TestSplitColumnSelector(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantPath   string
		wantColumn string
	}{
		{"no colon", "data.csv", "data.csv", ""},
		{"csv with column", "data.csv:PolicyNumber", "data.csv", "PolicyNumber"},
		{"xlsx with column", "data.xlsx:ID", "data.xlsx", "ID"},
		{"xls with column", "legacy.xls:Name", "legacy.xls", "Name"},
		{"txt is not split", "list.txt:ignored", "list.txt:ignored", ""},
		{"windows drive letter path", "C:/data/list.txt", "C:/data/list.txt", ""},
		{"windows drive letter csv", "C:/data/policies.csv:PolicyNumber", "C:/data/policies.csv", "PolicyNumber"},
		{"https url is not split", "https://example.com/data.txt", "https://example.com/data.txt", ""},
		{"empty column after colon", "data.csv:", "data.csv:", ""},
		{"uppercase extension", "DATA.CSV:Name", "DATA.CSV", "Name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, column := splitColumnSelector(tc.in)
			if path != tc.wantPath || column != tc.wantColumn {
				t.Errorf("splitColumnSelector(%q) = (%q, %q); want (%q, %q)",
					tc.in, path, column, tc.wantPath, tc.wantColumn)
			}
		})
	}
}

func TestIndexOfHeader(t *testing.T) {
	headers := []string{"PolicyNumber", "Amount", " Status "}
	cases := []struct {
		column string
		want   int
	}{
		{"PolicyNumber", 0},
		{"policynumber", 0}, // case-insensitive
		{"Amount", 1},
		{"Status", 2}, // trimmed match
		{"Missing", -1},
		{"", -1},
	}
	for _, tc := range cases {
		if got := indexOfHeader(headers, tc.column); got != tc.want {
			t.Errorf("indexOfHeader(%q) = %d; want %d", tc.column, got, tc.want)
		}
	}
}

func TestLoadLinesFromFileOrURL_PlainText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "list.txt")
	content := "# header comment\nP001\n\nP002\n# another comment\nP003\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	lines, err := loadLinesFromFileOrURL("@"+path, allowFileRead)
	if err != nil {
		t.Fatalf("loadLinesFromFileOrURL: %v", err)
	}
	want := []string{"P001", "P002", "P003"}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("loadLinesFromFileOrURL(@plain.txt) = %v; want %v", lines, want)
	}
}

func TestLoadLinesFromFileOrURL_LiteralValuePassthrough(t *testing.T) {
	// A non-@ value should be returned as a single-element slice so the
	// caller's len(val)==1 branch still fills the slice correctly.
	lines, err := loadLinesFromFileOrURL("P12345", allowFileRead)
	if err != nil {
		t.Fatalf("loadLinesFromFileOrURL: %v", err)
	}
	if !reflect.DeepEqual(lines, []string{"P12345"}) {
		t.Errorf("expected single literal, got %v", lines)
	}
}

func TestLoadLinesFromFileOrURL_CSVColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policies.csv")
	content := "PolicyNumber,Amount,Status\nP001,100,Active\nP002,200,Active\n,300,Active\nP003,,Closed\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	// Named column
	lines, err := loadLinesFromFileOrURL("@"+path+":PolicyNumber", allowFileRead)
	if err != nil {
		t.Fatalf("loadLinesFromFileOrURL: %v", err)
	}
	// The blank PolicyNumber row is skipped; the row whose Amount is blank
	// still contributes P003 because we're reading the PolicyNumber column.
	want := []string{"P001", "P002", "P003"}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("CSV column read = %v; want %v", lines, want)
	}

	// Case-insensitive column match
	lines2, err := loadLinesFromFileOrURL("@"+path+":policynumber", allowFileRead)
	if err != nil {
		t.Fatalf("case-insensitive: %v", err)
	}
	if !reflect.DeepEqual(lines2, want) {
		t.Errorf("case-insensitive CSV column read = %v; want %v", lines2, want)
	}
}

func TestLoadLinesFromFileOrURL_CSVMissingColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.csv")
	content := "A,B\n1,2\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadLinesFromFileOrURL("@"+path+":NotPresent", allowFileRead)
	if err == nil {
		t.Fatal("expected error for missing column")
	}
}

func TestLoadLinesFromFileOrURL_XLSXColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policies.xlsx")

	f := excelize.NewFile()
	// Default sheet is "Sheet1"
	headers := []any{"PolicyNumber", "Amount"}
	rows := [][]any{
		{"P001", 100},
		{"P002", 200},
		{"", 300}, // blank policy number — must be skipped
		{"P003", 400},
	}
	if err := f.SetSheetRow("Sheet1", "A1", &headers); err != nil {
		t.Fatalf("write headers: %v", err)
	}
	for i, row := range rows {
		cell := "A" + itoa(i+2)
		r := row
		if err := f.SetSheetRow("Sheet1", cell, &r); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save xlsx: %v", err)
	}
	_ = f.Close()

	lines, err := loadLinesFromFileOrURL("@"+path+":PolicyNumber", allowFileRead)
	if err != nil {
		t.Fatalf("loadLinesFromFileOrURL: %v", err)
	}
	want := []string{"P001", "P002", "P003"}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("XLSX column read = %v; want %v", lines, want)
	}
}

// itoa is an alloc-free integer-to-string for small values used only by the
// test to build Excel cell references.
func itoa(i int) string {
	if i < 10 {
		return string('0' + byte(i))
	}
	return string('0'+byte(i/10)) + string('0'+byte(i%10))
}

// cliPathOpts mirrors the repomap `deps`/`images` path field: a positional arg
// with a non-empty default. The documented precedence is flag → args → default,
// but a non-empty default used to clobber the positional arg.
type cliPathOpts struct {
	Path string `flag:"path" args:"true" default:"."`
}

// bindPathField builds the FlagValue for the Path field exactly as the cobra
// command builder does, then optionally simulates an explicitly-set flag by
// writing through the bound pointer (what pflag does when --path is passed).
func bindPathField(t *testing.T, explicitFlag string) (reflect.Value, *FlagValue) {
	t.Helper()
	fields, err := ParseStructFields(reflect.TypeOf(cliPathOpts{}))
	if err != nil {
		t.Fatalf("ParseStructFields: %v", err)
	}
	cmd := &cobra.Command{Use: "x"}
	var fv *FlagValue
	for _, info := range fields {
		if bound := BindFlag(cmd, info); info.IsArgs {
			fv = bound
		}
	}
	if fv == nil {
		t.Fatal("no args field bound")
	}
	if explicitFlag != "" {
		*fv.StringPtr = explicitFlag
	}
	var opts cliPathOpts
	return reflect.ValueOf(&opts).Elem(), fv
}

func TestAssignFieldValue_PositionalArgOverridesDefault(t *testing.T) {
	structVal, fv := bindPathField(t, "")
	if err := AssignFieldValue(structVal, fv, []string{"/some/path"}, false); err != nil {
		t.Fatalf("AssignFieldValue: %v", err)
	}
	if got := structVal.Interface().(cliPathOpts).Path; got != "/some/path" {
		t.Fatalf("Path = %q; want /some/path (positional arg must beat the default)", got)
	}
}

func TestAssignFieldValue_DefaultWhenNoArg(t *testing.T) {
	structVal, fv := bindPathField(t, "")
	if err := AssignFieldValue(structVal, fv, nil, false); err != nil {
		t.Fatalf("AssignFieldValue: %v", err)
	}
	if got := structVal.Interface().(cliPathOpts).Path; got != "." {
		t.Fatalf("Path = %q; want . (default applies when no positional arg)", got)
	}
}

func TestAssignFieldValue_ExplicitFlagBeatsArg(t *testing.T) {
	structVal, fv := bindPathField(t, "/from/flag")
	if err := AssignFieldValue(structVal, fv, []string{"/from/arg"}, false); err != nil {
		t.Fatalf("AssignFieldValue: %v", err)
	}
	if got := structVal.Interface().(cliPathOpts).Path; got != "/from/flag" {
		t.Fatalf("Path = %q; want /from/flag (explicit flag must beat the positional arg)", got)
	}
}
