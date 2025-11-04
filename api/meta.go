// DO NOT EDIT THE STRUCTS in this file as they are public contracts used throughout the application.
// Any changes to these structs may have wide-ranging effects.

package api

import (
	"fmt"
	"sort"
	"strings"

	"github.com/samber/lo"
)

// PrettyData contains structured data processed through schema-driven formatting.
// It separates regular field values from tabular and tree data, maintaining
// the original data for serialization while providing formatted access.
type PrettyData struct {
	TypedValue

	Schema *PrettyObject
	// Values stores regular field values (non-table, non-tree)
	Values map[string]FieldValue
	// Tables stores tabular data by field name
	// TODO: This should be unified with TypedValue.Table in the refactoring
	Tables map[string][]PrettyDataRow
	// Original stores the original data interface for JSON/YAML marshaling
	Original interface{}
}

// PrettyDataRow represents a single row in a table
type PrettyDataRow map[string]FieldValue

// GetValue returns a field value by name from the Values map
func (pd *PrettyData) GetValue(name string) (FieldValue, bool) {
	if pd.Values == nil {
		return FieldValue{}, false
	}
	v, ok := pd.Values[name]
	return v, ok
}

// GetTable returns a table by name from the Tables map
func (pd *PrettyData) GetTable(name string) ([]PrettyDataRow, bool) {
	if pd.Tables == nil {
		return nil, false
	}
	t, ok := pd.Tables[name]
	return t, ok
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
	TableCells() TextList
}

type TableRowMixin2 interface {
	PrettyRow(opt any) map[string]Text
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

func NewTableFromRows[T TableRowMixin2](o []T) TextTable {
	table := TextTable{}

	if len(o) == 0 {
		return table
	}

	// Use the first row to determine headers
	firstRow := o[0].PrettyRow(nil)

	headers := lo.Keys(firstRow)
	sort.StringSlice(headers).Sort()
	for _, key := range headers {
		table.Headers = append(table.Headers, Text{Content: key})
	}

	for _, v := range o {
		rowMap := v.PrettyRow(nil)
		row := TextList{}
		for _, header := range headers {
			if cell, exists := rowMap[header]; exists {
				row = append(row, cell)
			} else {
				row = append(row, Text{}) // Empty cell
			}
		}
		table.Rows = append(table.Rows, row)
	}

	return table
}

type TextTable struct {
	Headers     TextList
	Rows        []TextList
	Interactive bool
}

type TextTree struct {
	Node     Textable
	Children []TextTree
	depth    int
}

func (tt TextTree) String() string {
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

func (tt TextTree) HTML() string {
	return tt.String()
}

func (tt TextTree) ANSI() string {
	return tt.String()
}

func (tt TextTree) Markdown() string {
	return tt.String()
}

type PrettyFieldData struct {
	Label Text
	Value Textable
}

var all = []Textable{
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

type TypedValue struct {
	Textable   Textable
	Slice      *TextList
	Map        *TextMap
	TypedMap   *TypedMap
	TypedList  *TypedList
	Table      *TextTable
	Tree       *TextTree
	IsCircular bool
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
		t = append(t, Text{}.Append(k+": ", "text-muted").Add(v))
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
