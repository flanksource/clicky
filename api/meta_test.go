package api

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

// testTableProvider implements TableProvider for testing reflection
type testTableProvider struct {
	ID   int
	Name string
}

func (testTableProvider) Columns() []ColumnDef {
	return []ColumnDef{
		Column("id").Build(),
		Column("name").Build(),
	}
}

func (t testTableProvider) Row() map[string]any {
	return map[string]any{"id": t.ID, "name": t.Name}
}

// testTableMixin implements TableMixin for testing reflection
type testTableMixin struct {
	Field1 string
	Field2 int
}

func (t testTableMixin) TableHeaders() TextList {
	return TextList{Text{Content: "Field1"}, Text{Content: "Field2"}}
}

func (t testTableMixin) TableCells() TableRow {
	return TableRow{
		"Field1": TypedValue{Textable: Text{Content: t.Field1}},
		"Field2": TypedValue{Textable: Human(t.Field2)},
	}
}

// testTreeNode implements TreeNode for testing reflection
type testTreeNode struct {
	Label    string
	Children []testTreeNode
}

func (t testTreeNode) Pretty() Text {
	return Text{Content: t.Label}
}

func (t testTreeNode) GetChildren() []TreeNode {
	result := make([]TreeNode, len(t.Children))
	for i, c := range t.Children {
		result[i] = c
	}
	return result
}

// testTreeMixin implements TreeMixin for testing reflection
type testTreeMixin struct {
	Value string
}

func (t testTreeMixin) Tree() TreeNode {
	return testTreeNode{Label: t.Value}
}

// testPretty implements Pretty for testing reflection
type testPretty struct {
	Content string
}

func (t testPretty) Pretty() Text {
	return Text{Content: t.Content}
}

// testTextable implements Textable for testing reflection
type testTextable struct {
	Value string
}

func (t testTextable) String() string   { return t.Value }
func (t testTextable) ANSI() string     { return t.Value }
func (t testTextable) HTML() string     { return t.Value }
func (t testTextable) Markdown() string { return t.Value }

// testPrettyLink implements BOTH Pretty and Textable, with Pretty() returning
// a Link. This mirrors models.EntityLink: TryTypedValue must honor Pretty()
// (yielding the Link) rather than treating the value as a bare Textable that
// would serialize as a struct/map node.
type testPrettyLink struct {
	Href  string
	Label string
}

func (t testPrettyLink) Pretty() Text {
	return Text{}.Add(Link{Href: t.Href, Content: Text{Content: t.Label}})
}
func (t testPrettyLink) String() string   { return t.Label }
func (t testPrettyLink) ANSI() string     { return t.Label }
func (t testPrettyLink) HTML() string     { return t.Pretty().HTML() }
func (t testPrettyLink) Markdown() string { return t.Pretty().Markdown() }

var _ = Describe("PrettyData", func() {
	Describe("IsEmpty", func() {
		It("returns true for nil PrettyData", func() {
			var pd *PrettyData
			Expect(pd.IsEmpty()).To(BeTrue())
		})

		It("returns true when all fields are nil", func() {
			pd := &PrettyData{}
			Expect(pd.IsEmpty()).To(BeTrue())
		})

		It("returns false when Schema is set", func() {
			pd := &PrettyData{Schema: &PrettyObject{}}
			Expect(pd.IsEmpty()).To(BeFalse())
		})

		It("returns false when Table is set", func() {
			table := TextTable{}
			pd := &PrettyData{TypedValue: TypedValue{Table: &table}}
			Expect(pd.IsEmpty()).To(BeFalse())
		})

		It("returns false when Tree is set", func() {
			tree := TextTree{}
			pd := &PrettyData{TypedValue: TypedValue{Tree: &tree}}
			Expect(pd.IsEmpty()).To(BeFalse())
		})

		It("returns false when TypedMap is set", func() {
			tm := TypedMap{}
			pd := &PrettyData{TypedValue: TypedValue{TypedMap: &tm}}
			Expect(pd.IsEmpty()).To(BeFalse())
		})
	})
})

var _ = Describe("TryTypedValue", func() {
	Describe("reflection-based slice detection", func() {
		It("converts []ConcreteTableProvider to TextTable", func() {
			items := []testTableProvider{
				{ID: 1, Name: "Alice"},
				{ID: 2, Name: "Bob"},
			}
			result := TryTypedValue(items)

			Expect(result).NotTo(BeNil())
			Expect(result.Table).NotTo(BeNil())
			Expect(result.Table.Rows).To(HaveLen(2))
			Expect(result.Table.Rows[0]["id"].String()).To(Equal("1"))
			Expect(result.Table.Rows[0]["name"].String()).To(Equal("Alice"))
		})

		It("converts []ConcreteTableMixin to TextTable", func() {
			items := []testTableMixin{
				{Field1: "a", Field2: 1},
				{Field1: "b", Field2: 2},
			}
			result := TryTypedValue(items)

			Expect(result).NotTo(BeNil())
			Expect(result.Table).NotTo(BeNil())
			Expect(result.Table.Headers).To(HaveLen(2))
			Expect(result.Table.Rows).To(HaveLen(2))
		})

		It("converts []ConcreteTreeNode to TextTree", func() {
			items := []testTreeNode{
				{Label: "root1", Children: []testTreeNode{{Label: "child1"}}},
				{Label: "root2"},
			}
			result := TryTypedValue(items)

			Expect(result).NotTo(BeNil())
			Expect(result.Tree).NotTo(BeNil())
			Expect(result.Tree.Children).To(HaveLen(2))
		})

		It("converts []ConcreteTreeMixin to TextTree", func() {
			items := []testTreeMixin{
				{Value: "node1"},
				{Value: "node2"},
			}
			result := TryTypedValue(items)

			Expect(result).NotTo(BeNil())
			Expect(result.Tree).NotTo(BeNil())
			Expect(result.Tree.Children).To(HaveLen(2))
		})

		It("converts []ConcretePretty to TextList", func() {
			items := []testPretty{
				{Content: "item1"},
				{Content: "item2"},
			}
			result := TryTypedValue(items)

			Expect(result).NotTo(BeNil())
			Expect(result.Slice).NotTo(BeNil())
			Expect(*result.Slice).To(HaveLen(2))
			Expect((*result.Slice)[0].String()).To(Equal("item1"))
		})

		It("converts []ConcreteTextable to TextList", func() {
			items := []testTextable{
				{Value: "text1"},
				{Value: "text2"},
			}
			result := TryTypedValue(items)

			Expect(result).NotTo(BeNil())
			Expect(result.Slice).NotTo(BeNil())
			Expect(*result.Slice).To(HaveLen(2))
			Expect((*result.Slice)[0].String()).To(Equal("text1"))
		})

		It("emits a header-only table for an empty TableProvider slice", func() {
			var items []testTableProvider
			result := TryTypedValue(items)
			Expect(result).NotTo(BeNil())
			Expect(result.Table).NotTo(BeNil())
			Expect(result.Table.Headers).To(HaveLen(2))
			Expect(result.Table.FieldNames).To(Equal([]string{"id", "name"}))
			Expect(result.Table.Rows).To(BeEmpty())
		})

		It("returns nil for []string (non-matching)", func() {
			items := []string{"a", "b", "c"}
			result := TryTypedValue(items)
			Expect(result).To(BeNil())
		})

		It("returns nil for []int (non-matching)", func() {
			items := []int{1, 2, 3}
			result := TryTypedValue(items)
			Expect(result).To(BeNil())
		})
	})

	Describe("direct type assertions", func() {
		It("handles TextTable directly", func() {
			table := TextTable{Headers: TextList{Text{Content: "H1"}}}
			result := TryTypedValue(table)

			Expect(result).NotTo(BeNil())
			Expect(result.Table).NotTo(BeNil())
		})

		It("handles TextTree directly", func() {
			tree := TextTree{Node: Text{Content: "root"}}
			result := TryTypedValue(tree)

			Expect(result).NotTo(BeNil())
			Expect(result.Tree).NotTo(BeNil())
		})

		It("handles TreeNode interface", func() {
			node := testTreeNode{Label: "single"}
			result := TryTypedValue(node)

			Expect(result).NotTo(BeNil())
			Expect(result.Tree).NotTo(BeNil())
		})
	})
})

var _ = Describe("TextTable marshaling", func() {
	var table TextTable

	BeforeEach(func() {
		table = TextTable{
			Headers: TextList{Text{Content: "id"}, Text{Content: "name"}},
			Rows: []TableRow{
				{"id": TypedValue{Textable: Text{Content: "101"}}, "name": TypedValue{Textable: Text{Content: "Alice"}}},
				{"id": TypedValue{Textable: Text{Content: "102"}}, "name": TypedValue{Textable: Text{Content: "Bob"}}},
			},
		}
	})

	Describe("MarshalJSON", func() {
		It("serializes as array of row objects", func() {
			data, err := json.Marshal(table)
			Expect(err).NotTo(HaveOccurred())

			var result []map[string]string
			err = json.Unmarshal(data, &result)
			Expect(err).NotTo(HaveOccurred())

			Expect(result).To(Equal([]map[string]string{
				{"id": "101", "name": "Alice"},
				{"id": "102", "name": "Bob"},
			}))
		})

		It("handles empty table", func() {
			emptyTable := TextTable{}
			data, err := json.Marshal(emptyTable)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("[]"))
		})
	})

	Describe("MarshalYAML", func() {
		It("serializes as array of row objects", func() {
			data, err := yaml.Marshal(table)
			Expect(err).NotTo(HaveOccurred())

			var result []map[string]string
			err = yaml.Unmarshal(data, &result)
			Expect(err).NotTo(HaveOccurred())

			Expect(result).To(Equal([]map[string]string{
				{"id": "101", "name": "Alice"},
				{"id": "102", "name": "Bob"},
			}))
		})

		It("handles empty table", func() {
			emptyTable := TextTable{}
			data, err := yaml.Marshal(emptyTable)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("[]\n"))
		})
	})

	Describe("Pretty precedence over Textable", func() {
		It("uses Pretty() for a value implementing both Pretty and Textable", func() {
			// EntityLink-style value: Pretty() returns a Link. The result must
			// carry the rendered Link, not the bare struct, so it serializes as
			// a link node rather than a {href,label} field map.
			result := TryTypedValue(testPrettyLink{Href: "/entity/client/abc", Label: "GL Scheme G0796016"})

			Expect(result).NotTo(BeNil())
			Expect(result.Textable).NotTo(BeNil())
			html := result.Textable.HTML()
			Expect(html).To(ContainSubstring(`href="/entity/client/abc"`))
			Expect(html).To(ContainSubstring("GL Scheme G0796016"))
		})
	})
})
