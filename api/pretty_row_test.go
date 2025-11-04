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
			Expect(nameField.Value).To(Equal("Interface Test"))
			Expect(nameField.Text).ToNot(BeNil())
			nameText, ok := nameField.Text.(*Text)
			Expect(ok).To(BeTrue())
			Expect(nameText.Content).To(Equal("Interface Test"))
			Expect(nameText.Style).To(Equal("font-bold"))

			ginkgo.By("verifying Count field has correct style")
			countField, exists := row["Count"]
			Expect(exists).To(BeTrue())
			Expect(countField.Value).To(Equal("7"))
			Expect(countField.Text).ToNot(BeNil())
			countText, ok := countField.Text.(*Text)
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
			Expect(nameField.Value).To(Equal("Regular Struct"))
		})
	})
})
