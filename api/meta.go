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
	"github.com/samber/lo"
)

// Interface types for reflection-based slice checking
var (
	tableProviderType  = reflect.TypeOf((*TableProvider)(nil)).Elem()
	tableMixinType     = reflect.TypeOf((*TableMixin)(nil)).Elem()
	tableRowMixin2Type = reflect.TypeOf((*TableRowMixin2)(nil)).Elem()
	treeNodeType       = reflect.TypeOf((*TreeNode)(nil)).Elem()
	treeMixinType      = reflect.TypeOf((*TreeMixin)(nil)).Elem()
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
	return pd.Schema == nil && pd.Table == nil && pd.Tree == nil && pd.TypedMap == nil
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
func (tt TextTree) buildLipglossTree(withColors bool) *lipglosstree.Tree {
	// Build the node label
	var nodeLabel string
	if tt.Node != nil {
		if withColors {
			nodeLabel = tt.Node.ANSI()
		} else {
			nodeLabel = tt.Node.String()
		}
	}

	// If we have no node and only one child, return the child tree directly
	if nodeLabel == "" && len(tt.Children) == 1 {
		return tt.Children[0].buildLipglossTree(withColors)
	}

	// If we have no node and multiple children, we need to create a wrapper
	// Create the tree with root (use empty string if no node)
	t := lipglosstree.New().Root(nodeLabel)

	// Add children
	for _, child := range tt.Children {
		childTree := child.buildLipglossTree(withColors)
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

	t := tt.buildLipglossTree(false)
	if t == nil {
		return ""
	}

	// Use rounded enumerator
	t = t.Enumerator(lipglosstree.RoundedEnumerator)

	return t.String()
}

func (tt TextTree) HTML() string {
	// Render as interactive HTML tree with Alpine.js
	return RenderTreeHTML(&tt, true)
}

func (tt TextTree) ANSI() string {
	if tt.Node == nil && len(tt.Children) == 0 {
		return ""
	}

	t := tt.buildLipglossTree(true)
	if t == nil {
		return ""
	}

	// Use rounded enumerator
	t = t.Enumerator(lipglosstree.RoundedEnumerator)

	return t.String()
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
	Format       string
}

type TypedValue struct {
	Textable   Textable
	Slice      *TextList
	Map        *TextMap
	TypedMap   *TypedMap
	TypedList  *TypedList
	Table      *TextTable
	Tree       *TextTree
	IsCircular bool
	FieldMeta  *FieldMeta // Metadata for rendering hints
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

func TryTypedValue(o any) *TypedValue {
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
	case Textable:
		return &TypedValue{Textable: v}
	case TreeNode:
		return &TypedValue{Tree: lo.ToPtr(NewTree(v))}
	case TreeMixin:
		return &TypedValue{Tree: lo.ToPtr(NewTree(v.Tree()))}
	case Pretty:
		return &TypedValue{Textable: v.Pretty()}
	case []TableMixin:
		return &TypedValue{Table: lo.ToPtr(NewTable(v))}
	case []TableRowMixin2:
		return &TypedValue{Table: lo.ToPtr(NewTableFromMixin(v))}
	case []PrettyDataRow:
		return &TypedValue{Table: lo.ToPtr(NewTableFromRows(v))}
	}

	// Use reflection to check slices of interface implementations
	val := reflect.ValueOf(o)
	if val.Kind() == reflect.Slice && val.Len() > 0 {
		elemType := val.Type().Elem()

		// Check TableProvider first (most specific table interface)
		if elemType.Implements(tableProviderType) {
			items := make([]TableProvider, val.Len())
			for i := 0; i < val.Len(); i++ {
				items[i] = val.Index(i).Interface().(TableProvider)
			}
			return &TypedValue{Table: lo.ToPtr(NewTableFrom(items))}
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
