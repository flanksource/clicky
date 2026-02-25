package formatters

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type testStruct struct {
	Name string
}

var _ = Describe("unwrapElement", func() {
	It("returns concrete value from a plain struct", func() {
		v := reflect.ValueOf(testStruct{Name: "hello"})
		result, err := unwrapElement(v)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Kind()).To(Equal(reflect.Struct))
		Expect(result.FieldByName("Name").String()).To(Equal("hello"))
	})

	It("dereferences a single pointer", func() {
		s := &testStruct{Name: "ptr"}
		v := reflect.ValueOf(s)
		result, err := unwrapElement(v)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Kind()).To(Equal(reflect.Struct))
		Expect(result.FieldByName("Name").String()).To(Equal("ptr"))
	})

	It("dereferences multiple pointer layers", func() {
		s := &testStruct{Name: "deep"}
		p := &s
		v := reflect.ValueOf(p)
		result, err := unwrapElement(v)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Kind()).To(Equal(reflect.Struct))
		Expect(result.FieldByName("Name").String()).To(Equal("deep"))
	})

	It("unwraps an interface value", func() {
		var iface interface{} = testStruct{Name: "iface"}
		v := reflect.ValueOf(&iface).Elem()
		result, err := unwrapElement(v)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Kind()).To(Equal(reflect.Struct))
	})

	It("returns error for nil pointer", func() {
		var s *testStruct
		v := reflect.ValueOf(s)
		_, err := unwrapElement(v)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("nil pointer"))
	})

	It("returns error for pointer to nil pointer", func() {
		var s *testStruct
		p := &s
		v := reflect.ValueOf(p)
		_, err := unwrapElement(v)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("nil pointer"))
	})

	It("handles a map value directly", func() {
		m := map[string]string{"key": "val"}
		v := reflect.ValueOf(m)
		result, err := unwrapElement(v)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Kind()).To(Equal(reflect.Map))
	})
})
