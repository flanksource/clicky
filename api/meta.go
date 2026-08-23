// DO NOT EDIT THE STRUCTS in this file as they are public contracts used throughout the application.
// Any changes to these structs may have wide-ranging effects.

package api

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	lipglosstree "github.com/charmbracelet/lipgloss/tree"
	"github.com/charmbracelet/x/ansi"
	"github.com/samber/lo"
)

// normalizeTreeLabel drops blank lines from a tree-node label and trims
// trailing whitespace per line. lipgloss prefixes every physical line of a
// multi-line node label with the tree continuation gutter, so a blank line
// (including one that is only ANSI escape codes) renders as a bare "│"-gutter
// line. A node label is one tree row's content, not a paragraph — blank-line
// spacing that reads well in flat output is noise inside the tree. Operates on
// already-rendered text (ANSI or plain): a line counts as blank when its
// ANSI-stripped, space-trimmed form is empty.
func normalizeTreeLabel(s string) string {
	return normalizeTreeLabelWidth(s, GetTerminalWidth())
}

func normalizeTreeLabelWidth(s string, width int) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(ansi.Strip(line)) == "" {
			continue
		}
		out = append(out, ansi.Truncate(strings.TrimRight(line, " \t"), width, "…"))
	}
	return strings.Join(out, "\n")
}

func trimTreePadding(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}

// Interface types for reflection-based slice checking
var (
	tableProviderType  = reflect.TypeOf((*TableProvider)(nil)).Elem()
	tableMixinType     = reflect.TypeOf((*TableMixin)(nil)).Elem()
	tableRowMixin2Type = reflect.TypeOf((*TableRowMixin2)(nil)).Elem()
	treeNodeType       = reflect.TypeOf((*TreeNode)(nil)).Elem()
	treeMixinType      = reflect.TypeOf((*TreeMixin)(nil)).Elem()
	prettyRowType      = reflect.TypeOf((*PrettyRow)(nil)).Elem()
	prettyType         = reflect.TypeOf((*Pretty)(nil)).Elem()
	textableType       = reflect.TypeOf((*Textable)(nil)).Elem()
)

// PrettyData contains structured data processed through schema-driven formatting.
// It separates regular field values from tabular and tree data, maintaining
// the original data for serialization while providing formatted access.
type PrettyData struct {
	TypedValue

	Schema *PrettyObject

	Original interface{}
}

// IsEmpty returns true if the PrettyData has no meaningful content
func (pd *PrettyData) IsEmpty() bool {
	if pd == nil {
		return true
	}
	return pd.Schema == nil &&
		pd.Textable == nil &&
		pd.Slice == nil &&
		pd.Map == nil &&
		pd.TypedMap == nil &&
		pd.TypedList == nil &&
		pd.Table == nil &&
		pd.Tree == nil
}

// GetValue retrieves a typed value by field name from the TypedMap
func (pd *PrettyData) GetValue(fieldName string) (TypedValue, bool) {
	if pd.TypedMap == nil {
		return TypedValue{}, false
	}
	value, exists := (*pd.TypedMap)[fieldName]
	return value, exists
}

// GetTable returns the table data if it exists
func (pd *PrettyData) GetTable(tableName string) (*TextTable, bool) {
	if pd.Table != nil {
		return pd.Table, true
	}
	if pd.TypedMap != nil {
		if tableName != "" {
			if value, exists := (*pd.TypedMap)[tableName]; exists && value.Table != nil {
				return value.Table, true
			}
		}
		for _, value := range *pd.TypedMap {
			if value.Table != nil {
				return value.Table, true
			}
		}
	}
	return nil, false
}

// TreeNode defines the interface for hierarchical tree structures.
// Implementations provide formatted content and child relationships for tree rendering.
type TreeNode interface {
	Pretty() Text
	GetChildren() []TreeNode
}

// TreeMixin allows types to provide tree representation without being TreeNodes themselves.
// This is useful for data types that need tree formatting but aren't primarily tree structures.
type TreeMixin interface {
	Tree() TreeNode
}

// PrettyNode extends TreeNode with rich text formatting capabilities.
type PrettyNode interface {
	Pretty() Text
}

type TableMixin interface {
	TableHeaderMixin
	TableRowMixin
}

type TableHeaderMixin interface {
	TableHeaders() TextList
}

type TableRowMixin interface {
	TableCells() TableRow
}

type PrettyDataRow map[string]TypedValue

type TableRowMixin2 interface {
	PrettyRow(opt any) PrettyDataRow
}

func NewTree[T TreeNode](nodes ...T) TextTree {
	tree := TextTree{}
	for _, n := range nodes {
		child := TextTree{
			Node: n.Pretty(),
		}
		for _, c := range n.GetChildren() {
			child.Children = append(child.Children, NewTree(c))
		}

		tree.Children = append(tree.Children, child)
	}
	if len(tree.Children) == 1 {
		return tree.Children[0]
	}
	return tree
}

func NewTable[T TableMixin](o []T) TextTable {
	table := TextTable{}

	if len(o) == 0 {
		return table
	}
	table.Headers = o[0].TableHeaders()

	for _, v := range o {

		table.Rows = append(table.Rows, v.TableCells())
	}

	return table
}

func NewTableFromMixin[T TableRowMixin2](o []T) TextTable {
	table := TextTable{}

	if len(o) == 0 {
		return table
	}
	var rows []PrettyDataRow
	for _, v := range o {
		rows = append(rows, v.PrettyRow(nil))
	}
	return NewTableFromRows(rows)

}

func NewTableFromRows(o []PrettyDataRow) TextTable {
	table := TextTable{}
	if len(o) == 0 {
		return table
	}

	// Use the first row to determine headers
	firstRow := o[0]

	headers := lo.Keys(firstRow)
	sort.StringSlice(headers).Sort()
	for _, key := range headers {
		table.Headers = append(table.Headers, Text{Content: key})
	}

	for _, rowMap := range o {
		row := TableRow{}
		for _, header := range headers {
			if cell, exists := rowMap[header]; exists {
				row[header] = cell
			} else {
				row[header] = TypedValue{} // Empty cell
			}
		}
		table.Rows = append(table.Rows, row)
	}

	return table
}

type TableRow map[string]TypedValue

type TextTable struct {
	Headers     TextList
	FieldNames  []string // Maps header index to field name for row lookups
	Columns     []PrettyField
	Rows        []TableRow
	RowDetail   []Textable // Per-row detail content shown on expand (nil entry = no detail)
	Interactive bool
}

func (tt TextTable) AsString(row TableRow) []string {
	var result []string
	for i, header := range tt.Headers {
		// Use FieldNames for row lookup if available (rows are keyed by field names, not labels)
		key := header.String()
		if i < len(tt.FieldNames) && tt.FieldNames[i] != "" {
			key = tt.FieldNames[i]
		}
		if cell, exists := row[key]; exists {
			result = append(result, cell.String())
		} else {
			result = append(result, "")
		}
	}
	return result
}

// MarshalJSON serializes TextTable as an array of row objects
func (tt TextTable) MarshalJSON() ([]byte, error) {
	rows := make([]map[string]any, len(tt.Rows))
	for i, row := range tt.Rows {
		rowMap := make(map[string]any)
		for key, value := range row {
			rowMap[key] = value.String()
		}
		rows[i] = rowMap
	}
	return json.Marshal(rows)
}

// MarshalYAML serializes TextTable as an array of row objects
func (tt TextTable) MarshalYAML() (interface{}, error) {
	rows := make([]map[string]any, len(tt.Rows))
	for i, row := range tt.Rows {
		rowMap := make(map[string]any)
		for key, value := range row {
			rowMap[key] = value.String()
		}
		rows[i] = rowMap
	}
	return rows, nil
}

type TextTree struct {
	Node     Textable
	Children []TextTree
	depth    int
}

func (tt TextTree) Visit(visitor VisitorFunc) bool {
	if !visitor(NewTypedValue(tt.Node)) {
		return false
	}
	for _, child := range tt.Children {
		if !child.Visit(visitor) {
			return false
		}
	}
	return true
}

// buildLipglossTree converts a TextTree to a lipgloss tree
func (tt TextTree) buildLipglossTree(withColors bool, depth int) *lipglosstree.Tree {
	// Build the node label
	var nodeLabel string
	if tt.Node != nil {
		if withColors {
			nodeLabel = tt.Node.ANSI()
		} else {
			nodeLabel = tt.Node.String()
		}
		width := max(1, GetTerminalWidth()-depth*4-2)
		nodeLabel = normalizeTreeLabelWidth(nodeLabel, width)
	}

	// If we have no node and only one child, return the child tree directly
	if nodeLabel == "" && len(tt.Children) == 1 {
		return tt.Children[0].buildLipglossTree(withColors, depth)
	}

	// If we have no node and multiple children, we need to create a wrapper
	// Create the tree with root (use empty string if no node)
	t := lipglosstree.New().Root(nodeLabel)

	// Add children
	for _, child := range tt.Children {
		childTree := child.buildLipglossTree(withColors, depth+1)
		if childTree != nil {
			t = t.Child(childTree)
		}
	}

	return t
}

func (tt TextTree) String() string {
	if tt.Node == nil && len(tt.Children) == 0 {
		return ""
	}

	t := tt.buildLipglossTree(false, 0)
	if t == nil {
		return ""
	}

	// Use rounded enumerator
	t = t.Enumerator(lipglosstree.RoundedEnumerator)

	return trimTreePadding(t.String())
}

func (tt TextTree) HTML() string {
	// Render as interactive HTML tree with Alpine.js
	return RenderTreeHTML(&tt, true)
}

// StaticHTML renders the tree as static HTML without JavaScript.
// This is suitable for PDF output where JavaScript may not execute.
func (tt TextTree) StaticHTML() string {
	return RenderTreeHTML(&tt, false)
}

func (tt TextTree) ANSI() string {
	if tt.Node == nil && len(tt.Children) == 0 {
		return ""
	}

	t := tt.buildLipglossTree(true, 0)
	if t == nil {
		return ""
	}

	// Use rounded enumerator
	t = t.Enumerator(lipglosstree.RoundedEnumerator)

	return trimTreePadding(t.String())
}

func (tt TextTree) Markdown() string {
	// Keep simple indentation for Markdown
	var n = ""
	if tt.Node != nil {
		n = strings.Repeat("  ", tt.depth) + tt.Node.String()
	}
	for _, child := range tt.Children {
		child.depth = tt.depth + 1
		n += "\n" + child.String()
	}
	return n
}

type PrettyFieldData struct {
	Label Text
	Value Textable
}

// Compile-time check that these types satisfy Textable.
var _ = []Textable{
	Text{},
	TextList{},
	TextMap{},
	TypedValue{},
	TypedMap{},
	TypedList{},
	TextTable{},
	TextTree{},
}

type TextMap map[string]Textable

// FieldMeta contains metadata about a field for rendering purposes
type FieldMeta struct {
	Name         string
	CompactItems bool
	Short        bool
	Format       string
}

type TypedValue struct {
	Textable Textable
	// FilterValue preserves a filterable table cell's raw scalar independently
	// from its formatted Textable representation.
	FilterValue any
	Slice       *TextList
	Map         *TextMap
	TypedMap    *TypedMap
	TypedList   *TypedList
	Table       *TextTable
	Tree        *TextTree
	IsCircular  bool
	FieldMeta   *FieldMeta // Metadata for rendering hints
}

type VisitorFunc func(TypedValue) bool

func (tv TypedValue) FirstTable() *TextTable {
	if tv.Table != nil {
		return tv.Table
	}
	var table *TextTable
	tv.Visit(func(t TypedValue) bool {
		if t.Table != nil {
			table = t.Table
			return false
		}
		return true
	})
	return table
}

func (tv TypedValue) Visit(visitor VisitorFunc) bool {
	if !visitor(tv) {
		return false
	}
	if tv.Slice != nil {
		for _, item := range *tv.Slice {
			if !NewTypedValue(item).Visit(visitor) {
				return false
			}
		}
	}
	if tv.Map != nil {
		for _, item := range *tv.Map {
			if !NewTypedValue(item).Visit(visitor) {
				return false
			}
		}
	}
	if tv.TypedMap != nil {
		for _, item := range *tv.TypedMap {
			if !item.Visit(visitor) {
				return false
			}
		}
	}
	if tv.TypedList != nil {
		for _, item := range *tv.TypedList {
			if !item.Visit(visitor) {
				return false
			}
		}
	}
	if tv.Table != nil {
		for _, row := range tv.Table.Rows {
			for _, item := range row {
				if !item.Visit(visitor) {
					return false
				}
			}
		}
	}
	if tv.Tree != nil {
		if tv.Tree.Node != nil {
			tv.Tree.Node.(TypedValue).Visit(visitor)
		}
		for _, child := range tv.Tree.Children {
			if !child.Visit(visitor) {
				return false
			}
		}
	}
	return true
}

func implementsOrDeref(t reflect.Type, iface reflect.Type) bool {
	if t.Implements(iface) {
		return true
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
		if t.Implements(iface) {
			return true
		}
	}
	return false
}

func TryTypedValue(o any) *TypedValue {
	if o == nil {
		return nil
	}
	if rv := reflect.ValueOf(o); (rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface) && rv.IsNil() {
		return nil
	}
	switch v := o.(type) {
	case TypedValue:
		return &v
	case *TypedValue:
		return v
	case *PrettyData:
		return &TypedValue{Textable: v}
	case TextTable:
		return &TypedValue{Table: &v}
	case TextTree:
		return &TypedValue{Tree: &v}
	case TextList:
		return &TypedValue{Slice: &v}
	case TextMap:
		return &TypedValue{Map: &v}
	case TypedMap:
		return &TypedValue{TypedMap: &v}
	case TypedList:
		return &TypedValue{TypedList: &v}
	case TreeNode:
		return &TypedValue{Tree: lo.ToPtr(NewTree(v))}
	case TreeMixin:
		return &TypedValue{Tree: lo.ToPtr(NewTree(v.Tree()))}
	// Pretty must be checked before Textable: a type implementing both (e.g.
	// an EntityLink whose Pretty() returns a Link) controls its own rendered
	// node via Pretty(), so honor that rather than treating the bare value as
	// Textable and serializing it as a struct/map. This mirrors the slice path
	// below, which already checks Pretty before Textable.
	case Pretty:
		return &TypedValue{Textable: v.Pretty()}
	case Textable:
		return &TypedValue{Textable: v}
	case []TableMixin:
		return &TypedValue{Table: lo.ToPtr(NewTable(v))}
	case []TableRowMixin2:
		return &TypedValue{Table: lo.ToPtr(NewTableFromMixin(v))}
	case []PrettyDataRow:
		return &TypedValue{Table: lo.ToPtr(NewTableFromRows(v))}
	}

	// Use reflection to check slices of interface implementations
	val := reflect.ValueOf(o)
	if val.Kind() == reflect.Slice {
		elemType := val.Type().Elem()

		// Handle empty slices - emit the table schema (headers, no rows) when the
		// element type is a table so empty result sets still render the table
		// chrome instead of collapsing to a "No data" placeholder. A zero value
		// of the concrete element type sources the columns.
		if val.Len() == 0 {
			if elemType.Implements(tableProviderType) {
				// An interface element type has no concrete zero value to read
				// Columns() from: render an empty table without a schema
				// instead of panicking on a nil-interface assertion.
				if elemType.Kind() == reflect.Interface {
					return &TypedValue{Table: lo.ToPtr(NewEmptyTable(nil))}
				}
				zero := zeroTableProvider(elemType)
				columns := MustMergeSortableColumns(elemType, zero.Columns())
				return &TypedValue{Table: lo.ToPtr(NewEmptyTable(columns))}
			}
			return nil
		}

		// Check TableProvider first (most specific table interface)
		if elemType.Implements(tableProviderType) {
			items := make([]TableProvider, val.Len())
			for i := 0; i < val.Len(); i++ {
				items[i] = val.Index(i).Interface().(TableProvider)
			}
			rowType := elemType
			if elemType.Kind() == reflect.Interface {
				// Merge sort tags from the concrete first item, not the interface.
				rowType = reflect.TypeOf(items[0])
			}
			return &TypedValue{Table: lo.ToPtr(newTableFromProviders(items, rowType))}
		}

		// Check TableMixin
		if elemType.Implements(tableMixinType) {
			items := make([]TableMixin, val.Len())
			for i := 0; i < val.Len(); i++ {
				items[i] = val.Index(i).Interface().(TableMixin)
			}
			return &TypedValue{Table: lo.ToPtr(NewTable(items))}
		}

		// Check TableRowMixin2
		if elemType.Implements(tableRowMixin2Type) {
			items := make([]TableRowMixin2, val.Len())
			for i := 0; i < val.Len(); i++ {
				items[i] = val.Index(i).Interface().(TableRowMixin2)
			}
			return &TypedValue{Table: lo.ToPtr(NewTableFromMixin(items))}
		}

		// Check TreeNode
		if elemType.Implements(treeNodeType) {
			items := make([]TreeNode, val.Len())
			for i := 0; i < val.Len(); i++ {
				items[i] = val.Index(i).Interface().(TreeNode)
			}
			return &TypedValue{Tree: lo.ToPtr(NewTree(items...))}
		}

		// Check TreeMixin
		if elemType.Implements(treeMixinType) {
			nodes := make([]TreeNode, val.Len())
			for i := 0; i < val.Len(); i++ {
				nodes[i] = val.Index(i).Interface().(TreeMixin).Tree()
			}
			return &TypedValue{Tree: lo.ToPtr(NewTree(nodes...))}
		}

		// If elements implement PrettyRow, return nil to let the slice→table
		// conversion in convertSliceToPrettyDataWithOptions handle it properly
		if implementsOrDeref(elemType, prettyRowType) {
			return nil
		}

		// Check Pretty
		if elemType.Implements(prettyType) {
			list := make(TextList, val.Len())
			for i := 0; i < val.Len(); i++ {
				list[i] = val.Index(i).Interface().(Pretty).Pretty()
			}
			return &TypedValue{Slice: &list}
		}

		// Check Textable (last - most general)
		if elemType.Implements(textableType) {
			list := make(TextList, val.Len())
			for i := 0; i < val.Len(); i++ {
				list[i] = val.Index(i).Interface().(Textable)
			}
			return &TypedValue{Slice: &list}
		}
	}

	return nil
}

func newTableFromProviders(items []TableProvider, rowType reflect.Type) TextTable {
	columns := MustMergeSortableColumns(rowType, items[0].Columns())
	table := NewEmptyTable(columns)
	var hasDetail bool
	for _, item := range items {
		rowData := item.Row()
		row := TableRow{}
		for _, col := range columns {
			val, exists := rowData[col.Name]
			if !exists {
				// A hidden column with no value contributes nothing; a visible one
				// still needs a placeholder cell so the row stays column-aligned.
				if !col.Hidden {
					row[col.Name] = TypedValue{Textable: Text{}.Styles(col.Style)}
				}
				continue
			}
			// Hidden columns ride along as row metadata (row identity such as
			// _id, and raw values backing client-side filters). They are absent
			// from table.Columns, so they never render as a visible cell — and
			// display styling therefore only applies to visible columns.
			text := Text{}.Add(ColumnTextable(col, val))
			if !col.Hidden {
				text = text.Styles(col.Style)
				if col.MaxWidth > 0 {
					text = text.Styles(fmt.Sprintf("max-w-[%dch]", col.MaxWidth), "truncate")
				}
			}
			cell := TypedValue{Textable: text}
			// A filterable cell keeps its raw scalar so filtering compares
			// against the value, not its rendered representation.
			if col.FilterKey != "" {
				cell.FilterValue = val
			}
			row[col.Name] = cell
		}
		table.Rows = append(table.Rows, row)
		if detail, ok := item.(DetailProvider); ok {
			content := detail.RowDetail()
			table.RowDetail = append(table.RowDetail, content)
			hasDetail = hasDetail || content != nil
		}
	}
	if !hasDetail {
		table.RowDetail = nil
	}
	return table
}

// zeroTableProvider returns a usable zero TableProvider for an element type so
// its Columns() can be read from an empty slice. Pointer element types get a
// non-nil instance to avoid a nil-receiver panic in value-receiver Columns().
func zeroTableProvider(elemType reflect.Type) TableProvider {
	if elemType.Kind() == reflect.Ptr {
		return reflect.New(elemType.Elem()).Interface().(TableProvider)
	}
	return reflect.New(elemType).Elem().Interface().(TableProvider)
}

func NewTypedValue(o any) TypedValue {
	if v := TryTypedValue(o); v != nil {
		return *v
	}
	return TypedValue{Textable: Text{Content: fmt.Sprintf("%v", o)}}
}

func (tv TypedValue) Value() Textable {
	if tv.Textable != nil {
		return tv.Textable
	}
	if tv.Slice != nil {
		return *tv.Slice
	}
	if tv.Map != nil {
		return *tv.Map
	}
	if tv.TypedMap != nil {
		return *tv.TypedMap
	}
	if tv.TypedList != nil {
		return *tv.TypedList
	}
	if tv.Table != nil {
		return *tv.Table
	}
	if tv.Tree != nil {
		return *tv.Tree
	}
	return Text{}
}

func (tv TypedValue) String() string {
	return tv.Value().String()
}

func (tv TypedValue) HTML() string {
	return tv.Value().HTML()
}
func (tv TypedValue) ANSI() string {
	return tv.Value().ANSI()
}

func (tv TypedValue) Markdown() string {
	return tv.Value().Markdown()
}

func (tv TypedValue) MarkdownSlack() string {
	return markdownTextable(tv.Value(), true)
}

type TypedMap map[string]TypedValue
type TypedList []TypedValue

// NewTypedList creates a new TypedList from a variadic list of TypedValues
func NewTypedList(items ...TypedValue) TypedList {
	return TypedList(items)
}

// NewTypedMap creates a new TypedMap from key-value pairs
func NewTypedMap(pairs map[string]TypedValue) TypedMap {
	return TypedMap(pairs)
}

func (tl TypedList) Value() Textable {
	list := TextList{}
	for _, item := range tl {
		list = append(list, item.Value())
	}
	return list
}

func (tl TypedList) String() string {
	return tl.Value().String()
}

func (tl TypedList) HTML() string {
	return tl.Value().HTML()
}
func (tl TypedList) ANSI() string {
	return tl.Value().ANSI()
}

func (tl TypedList) Markdown() string {
	return tl.Value().Markdown()
}

func (tl TypedList) MarkdownSlack() string {
	return markdownTextable(tl.Value(), true)
}

func (tm TypedMap) Value() Textable {
	textMap := TextMap{}
	for key, val := range tm {
		textMap[key] = val.Value()
	}
	return textMap
}

func (tm TypedMap) String() string {
	return tm.Value().String()
}

func (tm TypedMap) HTML() string {
	return tm.Value().HTML()
}
func (tm TypedMap) ANSI() string {
	return tm.Value().ANSI()
}

func (tm TypedMap) Markdown() string {
	return tm.Value().Markdown()
}

func (tm TypedMap) MarkdownSlack() string {
	return markdownTextable(tm.Value(), true)
}

func (tm TextMap) String() string {
	result := "{"
	first := true
	for k, v := range tm {
		if !first {
			result += ", "
		}
		result += fmt.Sprintf("%s: %s", k, v.String())
		first = false
	}
	result += "}"
	return result
}

func (tm TextMap) Value() Textable {
	t := TextList{}
	for k, v := range tm {
		t = append(t, Text{}.Append(PrettifyFieldName(k)+": ", "text-muted").Add(v))
	}
	return t
}

func (tm TextMap) HTML() string {
	return tm.Value().HTML()
}

func (tm TextMap) ANSI() string {
	return tm.Value().ANSI()
}

func (tm TextMap) Markdown() string {
	return tm.Value().Markdown()
}

func (tm TextMap) MarkdownSlack() string {
	return markdownTextable(tm.Value(), true)
}
