package api

import (
	"fmt"
	"reflect"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestStruct implements PrettyRow interface for testing
type TestStruct struct {
	Name   string
	Count  int
	Status string
}

// PrettyRow implements the PrettyRow interface
func (t TestStruct) PrettyRow(opts interface{}) map[string]Text {
	result := make(map[string]Text)

	result["Name"] = Text{Content: t.Name, Style: "font-bold"}

	countStyle := "text-blue-600"
	if opts != nil {
		if s := fmt.Sprintf("%+v", opts); s != "" && fmt.Sprintf("%v", opts) != "<nil>" {
			if noColorOpts, ok := opts.(struct{ NoColor bool }); ok && noColorOpts.NoColor {
				countStyle = ""
			}
		}
	}
	result["Count"] = Text{Content: fmt.Sprintf("%d", t.Count), Style: countStyle}

	statusStyle := "text-green-600"
	if t.Status == "error" {
		statusStyle = "text-red-600"
	}
	if opts != nil {
		if noColorOpts, ok := opts.(struct{ NoColor bool }); ok && noColorOpts.NoColor {
			statusStyle = ""
		}
	}
	result["Status"] = Text{Content: t.Status, Style: statusStyle}

	return result
}

var _ = ginkgo.Describe("PrettyRow Interface", func() {
	ginkgo.Context("when using basic PrettyRow functionality", func() {
		ginkgo.It("should return properly formatted row with styles", func() {
			testStruct := TestStruct{
				Name:   "Test Item",
				Count:  5,
				Status: "success",
			}

			prettyRow := testStruct.PrettyRow(nil)
			Expect(prettyRow).To(HaveLen(3))
			Expect(prettyRow["Name"].Content).To(Equal("Test Item"))
			Expect(prettyRow["Name"].Style).To(Equal("font-bold"))
			Expect(prettyRow["Count"].Content).To(Equal("5"))
			Expect(prettyRow["Count"].Style).To(Equal("text-blue-600"))
			Expect(prettyRow["Status"].Content).To(Equal("success"))
			Expect(prettyRow["Status"].Style).To(Equal("text-green-600"))
		})
	})

	ginkgo.Context("when using PrettyRow with NoColor option", func() {
		ginkgo.It("should disable styles when NoColor is true", func() {
			testStruct := TestStruct{
				Name:   "Test Item",
				Count:  3,
				Status: "error",
			}

			opts := struct{ NoColor bool }{NoColor: true}
			prettyRow := testStruct.PrettyRow(opts)

			Expect(prettyRow["Count"].Style).To(Equal(""))
			Expect(prettyRow["Status"].Style).To(Equal(""))
			Expect(prettyRow["Name"].Style).To(Equal("font-bold"))
		})
	})
})

var _ = ginkgo.Describe("StructToRowWithOptions", func() {
	ginkgo.Context("when struct implements PrettyRow interface", func() {
		ginkgo.It("should use the interface implementation", func() {
			parser := NewStructParser()
			testStruct := TestStruct{
				Name:   "Interface Test",
				Count:  7,
				Status: "active",
			}

			val := reflect.ValueOf(testStruct)
			opts := struct{ NoColor bool }{NoColor: false}

			row, err := parser.StructToRowWithOptions(val, opts)
			Expect(err).ToNot(HaveOccurred())
			Expect(row).ToNot(BeNil())
			Expect(row).To(HaveLen(3))

			ginkgo.By("verifying Name field uses custom implementation")
			nameField, exists := row["Name"]
			Expect(exists).To(BeTrue())
			Expect(nameField.String()).To(Equal("Interface Test"))
			Expect(nameField.Textable).ToNot(BeNil())
			Expect(fmt.Sprintf("%T", nameField.Textable)).To(Equal("api.Text"))

			nameText, _ := nameField.Textable.(Text)
			Expect(nameText.Content).To(Equal("Interface Test"))
			Expect(nameText.Style).To(Equal("font-bold"))

			ginkgo.By("verifying Count field has correct style")
			countField, exists := row["Count"]
			Expect(exists).To(BeTrue())
			Expect(countField.String()).To(Equal("7"))
			Expect(countField.Textable).ToNot(BeNil())
			countText, ok := countField.Textable.(Text)
			Expect(ok).To(BeTrue())
			Expect(countText.Style).To(Equal("text-blue-600"))
		})
	})

	ginkgo.Context("when struct does not implement PrettyRow interface", func() {
		ginkgo.It("should fall back to reflection-based approach", func() {
			parser := NewStructParser()

			regularStruct := struct {
				Name  string
				Value int
			}{
				Name:  "Regular Struct",
				Value: 42,
			}

			val := reflect.ValueOf(regularStruct)
			opts := struct{ NoColor bool }{NoColor: false}

			row, err := parser.StructToRowWithOptions(val, opts)
			Expect(err).ToNot(HaveOccurred())
			Expect(row).ToNot(BeNil())

			nameField, exists := row["Name"]
			Expect(exists).To(BeTrue())
			Expect(nameField.Value().String()).To(Equal("Regular Struct"))
		})
	})
})

var _ = ginkgo.Describe("ExtractOrderValue", func() {
	ginkgo.Context("when extracting order values from style strings", func() {
		ginkgo.It("should return 0 for empty style", func() {
			orderValue := ExtractOrderValue("")
			Expect(orderValue).To(Equal(0))
		})

		ginkgo.It("should return 0 for style without order class", func() {
			orderValue := ExtractOrderValue("text-blue-600 font-bold")
			Expect(orderValue).To(Equal(0))
		})

		ginkgo.It("should extract order-1", func() {
			orderValue := ExtractOrderValue("text-blue-600 order-1")
			Expect(orderValue).To(Equal(1))
		})

		ginkgo.It("should extract order-5 from complex style", func() {
			orderValue := ExtractOrderValue("font-bold text-red-600 order-5 underline")
			Expect(orderValue).To(Equal(5))
		})

		ginkgo.It("should extract order-12", func() {
			orderValue := ExtractOrderValue("order-12")
			Expect(orderValue).To(Equal(12))
		})

		ginkgo.It("should handle order at the beginning", func() {
			orderValue := ExtractOrderValue("order-3 text-green-600")
			Expect(orderValue).To(Equal(3))
		})
	})
})

// OrderedTestStruct implements PrettyRow with order-X styles for testing column ordering
type OrderedTestStruct struct {
	FirstName string
	LastName  string
	Age       int
	Email     string
}

// PrettyRow implements PrettyRow interface with explicit column ordering
func (o OrderedTestStruct) PrettyRow(opts interface{}) map[string]Text {
	return map[string]Text{
		"Email":     {Content: o.Email, Style: "order-1"},     // Should appear second (order-1)
		"FirstName": {Content: o.FirstName, Style: "order-2"}, // Should appear third (order-2)
		"Age":       {Content: fmt.Sprintf("%d", o.Age)},      // Should appear first (no order = 0)
		"LastName":  {Content: o.LastName, Style: "order-1"},  // Should appear second (order-1, same as Email)
	}
}
