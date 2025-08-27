package api

import (
	"strings"
	"testing"
)

func TestTextBuilder(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *TextBuilder
		expected string
	}{
		{
			name: "simple text",
			builder: func() *TextBuilder {
				return NewText("Hello World")
			},
			expected: "Hello World",
		},
		{
			name: "bold text",
			builder: func() *TextBuilder {
				return NewText("Bold Text").Bold()
			},
			expected: "Bold Text", // Actual ANSI formatting depends on implementation
		},
		{
			name: "success text",
			builder: func() *TextBuilder {
				return NewText("SUCCESS").Success()
			},
			expected: "SUCCESS",
		},
		{
			name: "error text with multiple styles",
			builder: func() *TextBuilder {
				return NewText("ERROR").Error().Bold().Uppercase()
			},
			expected: "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.builder().Build()
			if result.Content != tt.expected {
				t.Errorf("expected content %q, got %q", tt.expected, result.Content)
			}

			// Check that styles are applied
			if result.Style == "" && (strings.Contains(tt.name, "bold") || strings.Contains(tt.name, "success") || strings.Contains(tt.name, "error")) {
				t.Error("expected style to be set")
			}
		})
	}
}

func TestStyleBuilder(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *StyleBuilder
		contains []string
	}{
		{
			name: "bold style",
			builder: func() *StyleBuilder {
				return NewStyle().Bold()
			},
			contains: []string{"font-bold"},
		},
		{
			name: "success style",
			builder: func() *StyleBuilder {
				return NewStyle().Success()
			},
			contains: []string{"text-green-600"},
		},
		{
			name: "multiple styles",
			builder: func() *StyleBuilder {
				return NewStyle().Bold().Italic().Error()
			},
			contains: []string{"font-bold", "italic", "text-red-600"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.builder().Build()

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected style to contain %q, got %q", expected, result)
				}
			}
		})
	}
}

func TestPredefinedShortcuts(t *testing.T) {
	tests := []struct {
		name     string
		function func(string) Text
		content  string
	}{
		{"success text", SuccessText, "PASS"},
		{"error text", ErrorText, "FAIL"},
		{"warning text", WarningText, "WARN"},
		{"info text", InfoText, "INFO"},
		{"muted text", MutedText, "SKIP"},
		{"bold text", BoldText, "BOLD"},
		{"italic text", ItalicText, "ITALIC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function(tt.content)
			if result.Content != tt.content {
				t.Errorf("expected content %q, got %q", tt.content, result.Content)
			}
			if result.Style == "" {
				t.Error("expected style to be set")
			}
		})
	}
}

func TestStatusText(t *testing.T) {
	tests := []struct {
		status   string
		content  string
		expected string
	}{
		{"PASS", "Test passed", "Test passed"},
		{"FAIL", "Test failed", "Test failed"},
		{"SUCCESS", "All good", "All good"},
		{"ERROR", "Something broke", "Something broke"},
		{"WARN", "Be careful", "Be careful"},
		{"SKIP", "Skipped test", "Skipped test"},
		{"UNKNOWN", "Unknown status", "Unknown status"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := StatusText(tt.status, tt.content)
			if result.Content != tt.expected {
				t.Errorf("expected content %q, got %q", tt.expected, result.Content)
			}
		})
	}
}

func TestChildBuilder(t *testing.T) {
	parent := NewText("Parent: ")
	child := NewText("Child").Bold()

	result := parent.ChildBuilder(child).Build()

	if result.Content != "Parent: " {
		t.Errorf("expected parent content %q, got %q", "Parent: ", result.Content)
	}

	if len(result.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(result.Children))
	}

	if result.Children[0].Content != "Child" {
		t.Errorf("expected child content %q, got %q", "Child", result.Children[0].Content)
	}
}
