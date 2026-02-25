package formatters

import (
	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type sampleStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

var _ = Describe("ToPrettyDataWithOptions", func() {
	It("returns empty PrettyData for nil input", func() {
		result, err := ToPrettyDataWithOptions(nil, FormatOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Schema.Fields).To(BeEmpty())
	})

	It("returns empty PrettyData for nil pointer", func() {
		var p *sampleStruct
		result, err := ToPrettyDataWithOptions(p, FormatOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Schema.Fields).To(BeEmpty())
		Expect(result.Original).To(Equal(p))
	})

	It("handles a struct value", func() {
		s := sampleStruct{Name: "test", Age: 25}
		result, err := ToPrettyDataWithOptions(s, FormatOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
	})

	It("handles a pointer to struct", func() {
		s := &sampleStruct{Name: "ptr", Age: 30}
		result, err := ToPrettyDataWithOptions(s, FormatOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
	})
})

var _ = Describe("ToPrettyData", func() {
	It("returns empty PrettyData for nil input", func() {
		result, err := ToPrettyData(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Schema.Fields).To(BeEmpty())
	})

	It("returns empty PrettyData for nil pointer", func() {
		var p *sampleStruct
		result, err := ToPrettyData(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Schema.Fields).To(BeEmpty())
	})
})

var _ = Describe("InferValueType consistency", func() {
	It("uses InferValueType for map field types in ToPrettyDataWithOptions", func() {
		data := map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"name":   "test",
					"active": true,
					"count":  42,
					"rate":   3.14,
				},
			},
		}
		result, err := ToPrettyDataWithOptions(data, FormatOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Schema).NotTo(BeNil())

		var itemsField *api.PrettyField
		for i := range result.Schema.Fields {
			if result.Schema.Fields[i].Name == "items" {
				itemsField = &result.Schema.Fields[i]
				break
			}
		}
		Expect(itemsField).NotTo(BeNil())

		fieldTypes := map[string]string{}
		for _, col := range itemsField.TableOptions.Columns {
			fieldTypes[col.Name] = col.Type
		}
		Expect(fieldTypes["active"]).To(Equal(api.FieldTypeBoolean))
		Expect(fieldTypes["count"]).To(Equal(api.FieldTypeInt))
		Expect(fieldTypes["rate"]).To(Equal(api.FieldTypeFloat))
		Expect(fieldTypes["name"]).To(Equal(api.FieldTypeString))
	})
})
