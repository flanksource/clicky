package api

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

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
		{input: time.Date(2023, 10, 5, 14, 30, 0, 0, time.UTC), expected: "2023-10-05T14:30:00Z"},
		{input: time.Date(2023, 10, 5, 0, 0, 0, 0, time.UTC), expected: "2023-10-05"},

		{input: Text{Content: "Preformatted Text"}, expected: "Preformatted Text"},
	}

	for _, test := range tests {
		result := Human(test.input)
		Expect(result.Content).To(Equal(test.expected))
	}
}
