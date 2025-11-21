package formatters

import (
	"testing"

	. "github.com/flanksource/clicky/formatters"
)

// Test struct with yaml tags
type PersonWithYAMLTags struct {
	Name string `json:"name" yaml:"name"`
	Age  int    `json:"age" yaml:"age"`
	City string `json:"city" yaml:"city"`
}

// Test struct without yaml tags (only json tags)
type PersonWithoutYAMLTags struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
	City string `json:"city"`
}

// Test struct with nested yaml tags
type CompanyWithYAMLTags struct {
	Name string             `json:"name" yaml:"name"`
	CEO  PersonWithYAMLTags `json:"ceo" yaml:"ceo"`
}

// Test struct with nested without yaml tags
type CompanyWithoutYAMLTags struct {
	Name string                `json:"name"`
	CEO  PersonWithoutYAMLTags `json:"ceo"`
}

func TestYAMLFormatter_WithYAMLTags(t *testing.T) {
	formatter := NewYAMLFormatter()
	person := PersonWithYAMLTags{
		Name: "John Doe",
		Age:  30,
		City: "New York",
	}

	result, err := formatter.FormatValue(person)
	if err != nil {
		t.Fatalf("Failed to format YAML: %v", err)
	}

	expected := `name: John Doe
age: 30
city: New York
`
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestYAMLFormatter_WithoutYAMLTags(t *testing.T) {
	formatter := NewYAMLFormatter()
	person := PersonWithoutYAMLTags{
		Name: "Jane Doe",
		Age:  25,
		City: "Los Angeles",
	}

	result, err := formatter.FormatValue(person)
	if err != nil {
		t.Fatalf("Failed to format YAML: %v", err)
	}

	// When no yaml tags, it should use JSON->YAML conversion
	// which preserves json tag names
	expected := `age: 25
city: Los Angeles
name: Jane Doe
`
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestYAMLFormatter_NestedWithYAMLTags(t *testing.T) {
	formatter := NewYAMLFormatter()
	company := CompanyWithYAMLTags{
		Name: "Tech Corp",
		CEO: PersonWithYAMLTags{
			Name: "Alice Smith",
			Age:  45,
			City: "San Francisco",
		},
	}

	result, err := formatter.FormatValue(company)
	if err != nil {
		t.Fatalf("Failed to format YAML: %v", err)
	}

	expected := `name: Tech Corp
ceo:
    name: Alice Smith
    age: 45
    city: San Francisco
`
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestYAMLFormatter_NestedWithoutYAMLTags(t *testing.T) {
	formatter := NewYAMLFormatter()
	company := CompanyWithoutYAMLTags{
		Name: "Tech Corp",
		CEO: PersonWithoutYAMLTags{
			Name: "Bob Johnson",
			Age:  40,
			City: "Seattle",
		},
	}

	result, err := formatter.FormatValue(company)
	if err != nil {
		t.Fatalf("Failed to format YAML: %v", err)
	}

	// When no yaml tags, JSON field names should be used
	expected := `ceo:
    age: 40
    city: Seattle
    name: Bob Johnson
name: Tech Corp
`
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestYAMLFormatter_SliceWithYAMLTags(t *testing.T) {
	formatter := NewYAMLFormatter()
	people := []PersonWithYAMLTags{
		{Name: "Alice", Age: 30, City: "NYC"},
		{Name: "Bob", Age: 25, City: "LA"},
	}

	result, err := formatter.FormatValue(people)
	if err != nil {
		t.Fatalf("Failed to format YAML: %v", err)
	}

	expected := `- name: Alice
  age: 30
  city: NYC
- name: Bob
  age: 25
  city: LA
`
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestYAMLFormatter_SliceWithoutYAMLTags(t *testing.T) {
	formatter := NewYAMLFormatter()
	people := []PersonWithoutYAMLTags{
		{Name: "Charlie", Age: 35, City: "Boston"},
		{Name: "Diana", Age: 28, City: "Austin"},
	}

	result, err := formatter.FormatValue(people)
	if err != nil {
		t.Fatalf("Failed to format YAML: %v", err)
	}

	// JSON field ordering
	expected := `- age: 35
  city: Boston
  name: Charlie
- age: 28
  city: Austin
  name: Diana
`
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestYAMLFormatter_Nil(t *testing.T) {
	formatter := NewYAMLFormatter()
	result, err := formatter.FormatValue(nil)
	if err != nil {
		t.Fatalf("Failed to format nil: %v", err)
	}

	expected := "null"
	if result != expected {
		t.Errorf("Expected: %s, Got: %s", expected, result)
	}
}

func TestHasYAMLTags_Struct(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{
			name:     "struct with yaml tags",
			input:    PersonWithYAMLTags{},
			expected: true,
		},
		{
			name:     "struct without yaml tags",
			input:    PersonWithoutYAMLTags{},
			expected: false,
		},
		{
			name:     "nested struct with yaml tags",
			input:    CompanyWithYAMLTags{},
			expected: true,
		},
		{
			name:     "nested struct without yaml tags",
			input:    CompanyWithoutYAMLTags{},
			expected: false,
		},
		{
			name:     "slice with yaml tags",
			input:    []PersonWithYAMLTags{},
			expected: true,
		},
		{
			name:     "slice without yaml tags",
			input:    []PersonWithoutYAMLTags{},
			expected: false,
		},
		{
			name:     "nil",
			input:    nil,
			expected: false,
		},
		{
			name:     "map",
			input:    map[string]string{"key": "value"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasYAMLTags(tt.input)
			if result != tt.expected {
				t.Errorf("hasYAMLTags(%v) = %v, expected %v", tt.name, result, tt.expected)
			}
		})
	}
}
