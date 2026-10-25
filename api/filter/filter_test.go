package filter

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// recordingAdapter matches when the variable named by the expression is true,
// and records the sample it was compiled with.
type recordingAdapter struct {
	sample map[string]any
}

func (a *recordingAdapter) Validate(string) error { return nil }

func (a *recordingAdapter) Compile(expression string, sample map[string]any) (Predicate, error) {
	a.sample = sample
	return variablePredicate(expression), nil
}

type variablePredicate string

func (p variablePredicate) Match(variables map[string]any) (bool, error) {
	value, ok := variables[string(p)]
	if !ok {
		return false, fmt.Errorf("%s: %w", p, ErrMissingVariable)
	}
	match, ok := value.(bool)
	if !ok {
		return false, errors.New("not a bool")
	}
	return match, nil
}

var _ = ginkgo.Describe("Adapter", func() {
	ginkgo.AfterEach(func() {
		SetAdapter(nil)
	})

	ginkgo.Context("when no adapter is configured", func() {
		ginkgo.BeforeEach(func() {
			SetAdapter(nil)
		})

		ginkgo.It("TableRows returns ErrNoAdapter", func() {
			_, err := TableRows([]api.PrettyDataRow{{"name": api.NewTypedValue("a")}}, "name == 'a'")
			Expect(err).To(MatchError(ErrNoAdapter))
		})

		ginkgo.It("TreeNode returns ErrNoAdapter", func() {
			_, err := TreeNode(&api.SimpleTreeNode{Label: "root"}, "label == 'root'")
			Expect(err).To(MatchError(ErrNoAdapter))
		})

		ginkgo.It("MapRows returns ErrNoAdapter", func() {
			_, err := MapRows([]map[string]any{{"Amount": int64(1)}}, "Amount > 0")
			Expect(err).To(MatchError(ErrNoAdapter))
		})

		ginkgo.It("Validate returns ErrNoAdapter", func() {
			Expect(Validate("x")).To(MatchError(ErrNoAdapter))
		})

		ginkgo.It("an empty expression needs no adapter", func() {
			rows := []api.PrettyDataRow{{"name": api.NewTypedValue("a")}}
			Expect(TableRows(rows, "")).To(Equal(rows))
			Expect(Validate("")).To(Succeed())
		})
	})

	ginkgo.It("compiles table filters against the first row's typed values", func() {
		adapter := &recordingAdapter{}
		SetAdapter(adapter)
		typed := func(value any) api.TypedValue {
			cell := api.NewTypedValue(value)
			cell.FilterValue = value
			return cell
		}
		rows := []api.PrettyDataRow{
			{"keep": typed(true), "Total Amount": typed(int64(3))},
			{"keep": typed(false), "Total Amount": typed(int64(4))},
		}

		filtered, err := TableRows(rows, "keep")

		Expect(err).NotTo(HaveOccurred())
		Expect(filtered).To(Equal(rows[:1]))
		Expect(adapter.sample).To(Equal(map[string]any{
			"keep":         true,
			"Total Amount": int64(3),
			"row":          map[string]any{"keep": true, "Total Amount": int64(3)},
		}))
	})

	ginkgo.It("untypes tree variables whose type differs across nodes and treats missing ones as non-matches", func() {
		adapter := &recordingAdapter{}
		SetAdapter(adapter)
		tree := &api.SimpleTreeNode{
			Label: "root",
			Children: []api.TreeNode{
				&api.SimpleTreeNode{Label: "a", Metadata: map[string]any{"keep": true, "size": int64(1)}},
				&api.SimpleTreeNode{Label: "b", Metadata: map[string]any{"size": "large"}},
			},
		}

		filtered, err := TreeNode(tree, "keep")

		Expect(err).NotTo(HaveOccurred())
		Expect(filtered.GetChildren()).To(HaveLen(1))
		Expect(adapter.sample).To(HaveKeyWithValue("size", BeNil()))
		Expect(adapter.sample).To(HaveKeyWithValue("keep", true))
	})
})

// Pending since before the adapter split: api.NewTypedValue leaves FilterValue
// nil, so typed fields are mapped as strings.
var _ = ginkgo.XDescribe("rowVariables", func() {
	tests := []struct {
		name     string
		row      api.PrettyDataRow
		expected map[string]interface{}
	}{
		{
			name: "string and int fields",
			row: api.PrettyDataRow{
				"name": api.NewTypedValue("test"),
				"age":  api.NewTypedValue(int64(30)),
			},
			expected: map[string]interface{}{
				"name": "test",
				"age":  int64(30),
			},
		},
		{
			name: "boolean and float fields",
			row: api.PrettyDataRow{
				"active": api.NewTypedValue(true),
				"score":  api.NewTypedValue(95.5),
			},
			expected: map[string]interface{}{
				"active": true,
				"score":  95.5,
			},
		},
		{
			name: "time field",
			row: api.PrettyDataRow{
				"created_at": api.NewTypedValue(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)),
			},
			expected: map[string]interface{}{
				"created_at": time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		ginkgo.It(tt.name, func() {
			result := rowVariables(tt.row)

			Expect(result).To(HaveLen(len(tt.expected) + 1))
			Expect(result).To(HaveKey("row"))

			for key, expectedVal := range tt.expected {
				actualVal, ok := result[key]
				Expect(ok).To(BeTrue(), "Missing key %q in result", key)

				if expectedTime, ok := expectedVal.(time.Time); ok {
					actualTime, ok := actualVal.(time.Time)
					Expect(ok).To(BeTrue(), "Expected time.Time for key %q, got %T", key, actualVal)
					Expect(actualTime.Equal(expectedTime)).To(BeTrue(), "For key %q: expected %v, got %v", key, expectedTime, actualTime)
				} else {
					Expect(actualVal).To(Equal(expectedVal), "For key %q", key)
				}
			}
		})
	}
})

var _ = ginkgo.Describe("nodeVariables", func() {
	tests := []struct {
		name     string
		node     api.TreeNode
		expected map[string]interface{}
	}{
		{
			name: "simple node with label",
			node: &api.SimpleTreeNode{
				Label: "test_node",
			},
			expected: map[string]interface{}{
				"label":   "test_node",
				"content": "test_node",
			},
		},
		{
			name: "node with metadata",
			node: &api.SimpleTreeNode{
				Label: "node",
				Metadata: map[string]interface{}{
					"status":   "active",
					"priority": int64(5),
				},
			},
			expected: map[string]interface{}{
				"label":    "node",
				"content":  "node",
				"status":   "active",
				"priority": int64(5),
			},
		},
		{
			name: "node with style and icon",
			node: &api.SimpleTreeNode{
				Label: "styled_node",
				Style: "bold",
				Icon:  "check",
			},
			expected: map[string]interface{}{
				"label": "styled_node",
				"style": "bold",
				"icon":  "check",
			},
		},
	}

	for _, tt := range tests {
		ginkgo.It(tt.name, func() {
			result := nodeVariables(tt.node)

			for key, expectedVal := range tt.expected {
				actualVal, ok := result[key]
				Expect(ok).To(BeTrue(), "Missing key %q in result", key)
				Expect(actualVal).To(Equal(expectedVal), "For key %q", key)
			}
		})
	}
})

func TestFilter(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Filter Suite")
}
