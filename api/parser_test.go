package api

import (
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type emptyImpl struct{ empty bool }

func (e emptyImpl) IsEmpty() bool { return e.empty }

type nilImpl struct{ isNil bool }

func (n nilImpl) IsNil() bool { return n.isNil }

type zeroImpl struct{ zero bool }

func (z zeroImpl) IsZero() bool { return z.zero }

type stringerStruct struct{ val string }

func (s stringerStruct) String() string { return fmt.Sprintf("custom:%s", s.val) }

type structWithZeroFields struct {
	Name      string
	CreatedAt time.Time
	UpdatedAt *time.Time
	Nested    zeroImpl
}

var _ = Describe("IsEmptyValue", func() {
	It("returns true for nil", func() {
		Expect(IsEmpty(nil)).To(BeTrue())
	})

	It("returns true for invalid reflect.Value", func() {
		Expect(IsEmpty(reflect.Value{})).To(BeTrue())
	})

	It("returns true for zero time.Time via IsZero()", func() {
		Expect(IsEmpty(time.Time{})).To(BeTrue())
	})

	It("returns false for non-zero time.Time", func() {
		Expect(IsEmpty(time.Now())).To(BeFalse())
	})

	It("returns true when IsEmpty() returns true", func() {
		Expect(IsEmpty(emptyImpl{empty: true})).To(BeTrue())
	})

	It("returns false when IsEmpty() returns false", func() {
		Expect(IsEmpty(emptyImpl{empty: false})).To(BeFalse())
	})

	It("returns true when IsNil() returns true", func() {
		Expect(IsEmpty(nilImpl{isNil: true})).To(BeTrue())
	})

	It("returns false when IsNil() returns false", func() {
		Expect(IsEmpty(nilImpl{isNil: false})).To(BeFalse())
	})

	It("returns true when IsZero() returns true", func() {
		Expect(IsEmpty(zeroImpl{zero: true})).To(BeTrue())
	})

	It("still handles primitives correctly", func() {
		Expect(IsEmpty("")).To(BeTrue())
		Expect(IsEmpty("hello")).To(BeFalse())
		Expect(IsEmpty([]int{})).To(BeTrue())
		Expect(IsEmpty([]int{1})).To(BeFalse())
	})

	It("returns true for nil pointer", func() {
		var p *string
		Expect(IsEmpty(p)).To(BeTrue())
	})

	It("recurses into interface values", func() {
		var iface interface{} = ""
		v := reflect.ValueOf(&iface).Elem()
		Expect(IsEmpty(v)).To(BeTrue())

		iface = "hello"
		v = reflect.ValueOf(&iface).Elem()
		Expect(IsEmpty(v)).To(BeFalse())
	})

	It("returns true for nil interface", func() {
		var iface interface{}
		v := reflect.ValueOf(&iface).Elem()
		Expect(IsEmpty(v)).To(BeTrue())
	})

	It("does not panic on nil *time.Time", func() {
		var t *time.Time
		Expect(IsEmpty(t)).To(BeTrue())
	})

	It("does not panic on nil *string", func() {
		var s *string
		Expect(IsEmpty(s)).To(BeTrue())
	})

	It("returns true for zero-value struct", func() {
		Expect(IsEmpty(structWithZeroFields{})).To(BeTrue())
	})

	It("returns false for non-zero struct", func() {
		Expect(IsEmpty(structWithZeroFields{Name: "x"})).To(BeFalse())
	})

	It("returns true for zero-value int array", func() {
		Expect(IsEmpty([3]int{})).To(BeTrue())
	})

	It("returns false for non-zero int array", func() {
		Expect(IsEmpty([3]int{0, 1, 0})).To(BeFalse())
	})

	It("returns true for pointer to zero-value string", func() {
		s := ""
		Expect(IsEmpty(&s)).To(BeTrue())
	})

	It("returns false for pointer to non-zero string", func() {
		s := "hello"
		Expect(IsEmpty(&s)).To(BeFalse())
	})

	It("returns true for zero int", func() {
		Expect(IsEmpty(0)).To(BeTrue())
	})

	It("returns false for non-zero int", func() {
		Expect(IsEmpty(42)).To(BeFalse())
	})
})

var _ = Describe("processFieldValueWithVisited", func() {
	var parser *StructParser

	BeforeEach(func() {
		parser = &StructParser{}
	})

	It("returns empty TypedValue for zero time.Time", func() {
		result := parser.ProcessFieldValue(reflect.ValueOf(time.Time{}))
		Expect(result.Textable).To(BeNil())
		Expect(result.Table).To(BeNil())
	})

	It("returns Textable for non-zero time.Time via fmt.Stringer", func() {
		result := parser.ProcessFieldValue(reflect.ValueOf(time.Now()))
		Expect(result.Textable).ToNot(BeNil())
	})

	It("returns empty TypedValue for struct with IsEmpty() true", func() {
		result := parser.ProcessFieldValue(reflect.ValueOf(emptyImpl{empty: true}))
		Expect(result.Textable).To(BeNil())
		Expect(result.Table).To(BeNil())
	})

	It("returns empty TypedValue for struct with IsNil() true", func() {
		result := parser.ProcessFieldValue(reflect.ValueOf(nilImpl{isNil: true}))
		Expect(result.Textable).To(BeNil())
	})

	It("returns empty TypedValue for struct with IsZero() true", func() {
		result := parser.ProcessFieldValue(reflect.ValueOf(zeroImpl{zero: true}))
		Expect(result.Textable).To(BeNil())
	})

	It("uses fmt.Stringer for structs implementing String()", func() {
		result := parser.ProcessFieldValue(reflect.ValueOf(stringerStruct{val: "hello"}))
		Expect(result.Textable).ToNot(BeNil())
		Expect(result.Textable.String()).To(Equal("custom:hello"))
	})

	It("returns true for zero byte array in isEmptyValue", func() {
		var zeroArr [16]byte
		Expect(IsEmpty(reflect.ValueOf(zeroArr))).To(BeTrue())
	})

	It("returns false for non-zero byte array in isEmptyValue", func() {
		arr := [16]byte{1}
		Expect(IsEmpty(reflect.ValueOf(arr))).To(BeFalse())
	})

	It("returns empty TypedValue for zero uuid.UUID", func() {
		result := parser.ProcessFieldValue(reflect.ValueOf(uuid.UUID{}))
		Expect(result.Textable).To(BeNil())
	})

	It("returns Textable for non-zero uuid.UUID", func() {
		result := parser.ProcessFieldValue(reflect.ValueOf(uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")))
		Expect(result.Textable).ToNot(BeNil())
		Expect(result.Textable.String()).To(Equal("550e8400-e29b-41d4-a716-446655440000"))
	})

	It("returns empty TypedValue for zero time.Duration", func() {
		result := parser.ProcessFieldValue(reflect.ValueOf(time.Duration(0)))
		Expect(result.Textable).To(BeNil())
	})

	It("returns Textable for non-zero time.Duration", func() {
		result := parser.ProcessFieldValue(reflect.ValueOf(5 * time.Second))
		Expect(result.Textable).ToNot(BeNil())
		Expect(result.Textable.String()).ToNot(BeEmpty())
	})
})

var _ = Describe("IsEmptyValue - UUID and Duration", func() {
	It("returns true for zero uuid.UUID", func() {
		Expect(IsEmpty(uuid.UUID{})).To(BeTrue())
	})

	It("returns false for non-zero uuid.UUID", func() {
		Expect(IsEmpty(uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"))).To(BeFalse())
	})

	It("returns true for zero time.Duration", func() {
		Expect(IsEmpty(time.Duration(0))).To(BeTrue())
	})

	It("returns false for non-zero time.Duration", func() {
		Expect(IsEmpty(5 * time.Second)).To(BeFalse())
	})
})
