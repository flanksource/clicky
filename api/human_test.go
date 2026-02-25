package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
)

func TestInferValueType_UUID(t *testing.T) {
	RegisterTestingT(t)

	Expect(InferValueType(uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"))).To(Equal(FieldTypeString))
	Expect(InferValueType(uuid.Nil)).To(Equal(FieldTypeString))

	// Regular byte arrays should still be FieldTypeArray
	Expect(InferValueType([3]byte{1, 2, 3})).To(Equal(FieldTypeArray))
	Expect(InferValueType([]int{1, 2, 3})).To(Equal(FieldTypeArray))
}

func TestPrettyFieldParse_UUID(t *testing.T) {
	RegisterTestingT(t)

	field := PrettyField{Name: "id", Type: FieldTypeString}
	u := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	val, err := field.Parse(u)
	Expect(err).To(BeNil())
	Expect(val.ArrayValue).To(BeNil())
	Expect(val.StringValue).ToNot(BeNil())
	Expect(*val.StringValue).To(Equal("550e8400-e29b-41d4-a716-446655440000"))
	Expect(val.Text).ToNot(BeNil())
	Expect(val.Text.String()).To(Equal("550e8400-e29b-41d4-a716-446655440000"))
}

func TestHuman(t *testing.T) {
	RegisterTestingT(t)
	tests := []struct {
		input    any
		expected string
	}{
		{input: "Hello World", expected: "Hello World"},
		{input: 12345, expected: "12.3K"},
		{input: 123345633, expected: "123M"},
		{input: 67.89, expected: "67.89"},
		{input: fmt.Sprintf("(%v in, %v out)", Human(5403200), Human(9003200)), expected: "(5.4M in, 9M out)"},
		{input: time.Date(2023, 10, 5, 14, 30, 0, 0, time.UTC), expected: "2023-10-05 14:30:00"},
		{input: time.Date(2023, 10, 5, 0, 0, 0, 0, time.UTC), expected: "2023-10-05"},

		{input: Text{Content: "Preformatted Text"}, expected: "Preformatted Text"},
		{input: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"), expected: "550e8400-e29b-41d4-a716-446655440000"},
		{input: uuid.Nil, expected: ""},
		{input: (*uuid.UUID)(nil), expected: ""},
		{input: func() *uuid.UUID { u := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"); return &u }(), expected: "550e8400-e29b-41d4-a716-446655440000"},
	}

	for _, test := range tests {
		result := Human(test.input)
		Expect(result.Content).To(Equal(test.expected))
	}
}
