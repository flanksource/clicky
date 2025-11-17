package tailwind_test

import (
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/tailwind"
)

func TestTruncateText_NoConstraints(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		style    string
		expected string
	}{
		{
			name:     "empty style returns original",
			text:     "some text",
			style:    "",
			expected: "some text",
		},
		{
			name:     "no constraints with mode returns original",
			text:     "some text",
			style:    "truncate-suffix",
			expected: "some text",
		},
		{
			name:     "empty text",
			text:     "",
			style:    "max-lines-[5] max-w-[100] truncate-suffix",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := api.Text{Content: tt.text, Style: tt.style}.String()
			if result != tt.expected {
				t.Errorf("api.Text{Content: %q, Style: %q}.String() = %q, want %q", tt.text, tt.style, result, tt.expected)
			}
		})
	}
}

func TestTruncateText_WidthConstraint(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		style    string
		expected string
	}{
		{
			name:     "text under limit",
			text:     "short",
			style:    "max-w-[10] truncate-suffix",
			expected: "short",
		},
		{
			name:     "text exactly at limit",
			text:     "exactly10c",
			style:    "max-w-[10] truncate-suffix",
			expected: "exactly10c",
		},
		{
			name:     "text over limit with suffix",
			text:     "This is a very long text that exceeds the limit",
			style:    "max-w-[20] truncate-suffix",
			expected: "This is a very long …",
		},
		{
			name:     "text over limit with prefix",
			text:     "/very/long/path/to/some/deeply/nested/file.txt",
			style:    "max-w-[30] truncate-prefix",
			expected: "…to/some/deeply/nested/file.txt",
		},
		{
			name:     "single character",
			text:     "a",
			style:    "max-w-[1] truncate-suffix",
			expected: "a",
		},
		{
			name:     "truncate to one char",
			text:     "hello",
			style:    "max-w-[1] truncate-suffix",
			expected: "h…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := api.Text{Content: tt.text, Style: tt.style}.String()
			if result != tt.expected {
				t.Errorf("api.Text{Content: %q, Style: %q}.String() = %q, want %q", tt.text, tt.style, result, tt.expected)
			}
		})
	}
}

func TestTruncateText_MultibyteUTF8(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		style    string
		expected string
	}{
		{
			name:     "CJK characters",
			text:     "これは日本語のテキストです",
			style:    "max-w-[5] truncate-suffix",
			expected: "これは日本…",
		},
		{
			name:     "emoji truncation",
			text:     "Hello 👋 World 🌍 Test 🎉",
			style:    "max-w-[15] truncate-suffix",
			expected: "Hello 👋 World 🌍…",
		},
		{
			name:     "mixed ASCII and multibyte",
			text:     "Hello こんにちは World",
			style:    "max-w-[10] truncate-prefix",
			expected: "…んにちは World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := api.Text{Content: tt.text, Style: tt.style}.String()
			if result != tt.expected {
				t.Errorf("api.Text{Content: %q, Style: %q}.String() = %q, want %q", tt.text, tt.style, result, tt.expected)
			}
		})
	}
}

func TestTruncateText_LineConstraint(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		style    string
		expected string
	}{
		{
			name:     "single line under limit",
			text:     "single line",
			style:    "max-lines-[2] truncate-suffix",
			expected: "single line",
		},
		{
			name:     "multiple lines under limit",
			text:     "Line 1\nLine 2\nLine 3",
			style:    "max-lines-[5] truncate-suffix",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "multiple lines exactly at limit",
			text:     "Line 1\nLine 2\nLine 3",
			style:    "max-lines-[3] truncate-suffix",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "multiple lines over limit with suffix",
			text:     "Line 1\nLine 2\nLine 3\nLine 4\nLine 5",
			style:    "max-lines-[3] truncate-suffix",
			expected: "Line 1\nLine 2\nLine 3…",
		},
		{
			name:     "multiple lines over limit with prefix",
			text:     "Line 1\nLine 2\nLine 3\nLine 4\nLine 5",
			style:    "max-lines-[2] truncate-prefix",
			expected: "…Line 1\nLine 2",
		},
		{
			name:     "truncate to one line",
			text:     "Line 1\nLine 2\nLine 3",
			style:    "max-lines-[1] truncate-suffix",
			expected: "Line 1…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := api.Text{Content: tt.text, Style: tt.style}.String()
			if result != tt.expected {
				t.Errorf("api.Text{Content: %q, Style: %q}.String() = %q, want %q", tt.text, tt.style, result, tt.expected)
			}
		})
	}
}

func TestTruncateText_CombinedConstraints(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		style    string
		expected string
	}{
		{
			name:     "width exceeded first",
			text:     "This is a very long single line of text",
			style:    "max-lines-[5] max-w-[20] truncate-suffix",
			expected: "This is a very long …",
		},
		{
			name:     "lines exceeded first",
			text:     "L1\nL2\nL3\nL4\nL5",
			style:    "max-lines-[2] max-w-[100] truncate-suffix",
			expected: "L1\nL2…",
		},
		{
			name:     "both constraints not exceeded",
			text:     "Short\ntext",
			style:    "max-lines-[3] max-w-[50] truncate-suffix",
			expected: "Short\ntext",
		},
		{
			name:     "lines truncated then width truncated",
			text:     "Line 1 is very long\nLine 2 is also long\nLine 3\nLine 4",
			style:    "max-lines-[2] max-w-[30] truncate-suffix",
			expected: "Line 1 is very long\nLine 2 is …",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := api.Text{Content: tt.text, Style: tt.style}.String()
			if result != tt.expected {
				t.Errorf("api.Text{Content: %q, Style: %q}.String() = %q, want %q", tt.text, tt.style, result, tt.expected)
			}
		})
	}
}

func TestParseStyle_TruncationClasses(t *testing.T) {
	tests := []struct {
		name        string
		styleStr    string
		expectLines int
		expectWidth int
		expectMode  string
	}{
		{
			name:        "parse max-lines-[3]",
			styleStr:    "max-lines-[3]",
			expectLines: 3,
			expectWidth: 0,
			expectMode:  "",
		},
		{
			name:        "parse max-w-[100]",
			styleStr:    "max-w-[100]",
			expectLines: 0,
			expectWidth: 100,
			expectMode:  "",
		},
		{
			name:        "parse truncate-suffix",
			styleStr:    "truncate-suffix",
			expectLines: 0,
			expectWidth: 0,
			expectMode:  "suffix",
		},
		{
			name:        "parse truncate-prefix",
			styleStr:    "truncate-prefix",
			expectLines: 0,
			expectWidth: 0,
			expectMode:  "prefix",
		},
		{
			name:        "parse combined classes",
			styleStr:    "max-lines-[5] max-w-[50] truncate-suffix",
			expectLines: 5,
			expectWidth: 50,
			expectMode:  "suffix",
		},
		{
			name:        "parse with other classes",
			styleStr:    "text-red-500 bold max-w-[80] truncate-prefix",
			expectLines: 0,
			expectWidth: 80,
			expectMode:  "prefix",
		},
		{
			name:        "last mode wins",
			styleStr:    "truncate-suffix truncate-prefix",
			expectLines: 0,
			expectWidth: 0,
			expectMode:  "prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := tailwind.ParseStyle(tt.styleStr)
			if style.MaxLines != tt.expectLines {
				t.Errorf("MaxLines = %d, want %d", style.MaxLines, tt.expectLines)
			}
			if style.MaxWidth != tt.expectWidth {
				t.Errorf("MaxWidth = %d, want %d", style.MaxWidth, tt.expectWidth)
			}
			if style.TruncateMode != tt.expectMode {
				t.Errorf("TruncateMode = %q, want %q", style.TruncateMode, tt.expectMode)
			}
		})
	}
}

func TestApplyStyle_WithTruncation(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		style    string
		expected string
	}{
		{
			name:     "truncate with width constraint",
			text:     "This is a very long text that needs truncation",
			style:    "max-w-[20] truncate-suffix",
			expected: "This is a very long …",
		},
		{
			name:     "truncate with line constraint",
			text:     "Line 1\nLine 2\nLine 3\nLine 4",
			style:    "max-lines-[2] truncate-suffix",
			expected: "Line 1\nLine 2…",
		},
		{
			name:     "uppercase then truncate",
			text:     "hello world this is a test",
			style:    "uppercase max-w-[15] truncate-suffix",
			expected: "HELLO WORLD THI…",
		},
		{
			name:     "uppercase then truncate",
			text:     "hello world this is a test",
			style:    "uppercase max-w-[15ch] truncate-suffix",
			expected: "HELLO WORLD THI…",
		},
		{
			name:     "uppercase then truncate",
			text:     "hello world this is a test",
			style:    "uppercase max-w-[15ch]",
			expected: "HELLO WORLD THI…",
		},
		{
			name:     "default truncate-suffix with constraint",
			text:     "This is a very long text",
			style:    "max-w-[10]",
			expected: "This is a …",
		},
		{
			name:     "no truncation without constraint",
			text:     "Some text here",
			style:    "truncate-suffix",
			expected: "Some text here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := api.Text{Content: tt.text, Style: tt.style}.String()
			if result != tt.expected {
				t.Errorf("api.Text{Content: %q, Style: %q}.String() = %q, want %q", tt.text, tt.style, result, tt.expected)
			}
		})
	}
}
