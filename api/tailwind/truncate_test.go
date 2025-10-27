package tailwind

import (
	"testing"
)

func TestTruncateText_NoConstraints(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLines int
		maxWidth int
		mode     string
		expected string
	}{
		{
			name:     "empty mode returns original",
			text:     "some text",
			maxLines: 0,
			maxWidth: 0,
			mode:     "",
			expected: "some text",
		},
		{
			name:     "no constraints with mode returns original",
			text:     "some text",
			maxLines: 0,
			maxWidth: 0,
			mode:     "suffix",
			expected: "some text",
		},
		{
			name:     "empty text",
			text:     "",
			maxLines: 5,
			maxWidth: 100,
			mode:     "suffix",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateText(tt.text, tt.maxLines, tt.maxWidth, tt.mode)
			if result != tt.expected {
				t.Errorf("TruncateText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTruncateText_WidthConstraint(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
		mode     string
		expected string
	}{
		{
			name:     "text under limit",
			text:     "short",
			maxWidth: 10,
			mode:     "suffix",
			expected: "short",
		},
		{
			name:     "text exactly at limit",
			text:     "exactly10c",
			maxWidth: 10,
			mode:     "suffix",
			expected: "exactly10c",
		},
		{
			name:     "text over limit with suffix",
			text:     "This is a very long text that exceeds the limit",
			maxWidth: 20,
			mode:     "suffix",
			expected: "This is a very long …",
		},
		{
			name:     "text over limit with prefix",
			text:     "/very/long/path/to/some/deeply/nested/file.txt",
			maxWidth: 30,
			mode:     "prefix",
			expected: "…to/some/deeply/nested/file.txt",
		},
		{
			name:     "single character",
			text:     "a",
			maxWidth: 1,
			mode:     "suffix",
			expected: "a",
		},
		{
			name:     "truncate to one char",
			text:     "hello",
			maxWidth: 1,
			mode:     "suffix",
			expected: "h…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateText(tt.text, 0, tt.maxWidth, tt.mode)
			if result != tt.expected {
				t.Errorf("TruncateText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTruncateText_MultibyteUTF8(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
		mode     string
		expected string
	}{
		{
			name:     "CJK characters",
			text:     "これは日本語のテキストです",
			maxWidth: 5,
			mode:     "suffix",
			expected: "これは日本…",
		},
		{
			name:     "emoji truncation",
			text:     "Hello 👋 World 🌍 Test 🎉",
			maxWidth: 15,
			mode:     "suffix",
			expected: "Hello 👋 World 🌍…",
		},
		{
			name:     "mixed ASCII and multibyte",
			text:     "Hello こんにちは World",
			maxWidth: 10,
			mode:     "prefix",
			expected: "…んにちは World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateText(tt.text, 0, tt.maxWidth, tt.mode)
			if result != tt.expected {
				t.Errorf("TruncateText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTruncateText_LineConstraint(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLines int
		mode     string
		expected string
	}{
		{
			name:     "single line under limit",
			text:     "single line",
			maxLines: 2,
			mode:     "suffix",
			expected: "single line",
		},
		{
			name:     "multiple lines under limit",
			text:     "Line 1\nLine 2\nLine 3",
			maxLines: 5,
			mode:     "suffix",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "multiple lines exactly at limit",
			text:     "Line 1\nLine 2\nLine 3",
			maxLines: 3,
			mode:     "suffix",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "multiple lines over limit with suffix",
			text:     "Line 1\nLine 2\nLine 3\nLine 4\nLine 5",
			maxLines: 3,
			mode:     "suffix",
			expected: "Line 1\nLine 2\nLine 3…",
		},
		{
			name:     "multiple lines over limit with prefix",
			text:     "Line 1\nLine 2\nLine 3\nLine 4\nLine 5",
			maxLines: 2,
			mode:     "prefix",
			expected: "…Line 1\nLine 2",
		},
		{
			name:     "truncate to one line",
			text:     "Line 1\nLine 2\nLine 3",
			maxLines: 1,
			mode:     "suffix",
			expected: "Line 1…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateText(tt.text, tt.maxLines, 0, tt.mode)
			if result != tt.expected {
				t.Errorf("TruncateText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTruncateText_CombinedConstraints(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLines int
		maxWidth int
		mode     string
		expected string
	}{
		{
			name:     "width exceeded first",
			text:     "This is a very long single line of text",
			maxLines: 5,
			maxWidth: 20,
			mode:     "suffix",
			expected: "This is a very long …",
		},
		{
			name:     "lines exceeded first",
			text:     "L1\nL2\nL3\nL4\nL5",
			maxLines: 2,
			maxWidth: 100,
			mode:     "suffix",
			expected: "L1\nL2…",
		},
		{
			name:     "both constraints not exceeded",
			text:     "Short\ntext",
			maxLines: 3,
			maxWidth: 50,
			mode:     "suffix",
			expected: "Short\ntext",
		},
		{
			name:     "lines truncated then width truncated",
			text:     "Line 1 is very long\nLine 2 is also long\nLine 3\nLine 4",
			maxLines: 2,
			maxWidth: 30,
			mode:     "suffix",
			expected: "Line 1 is very long\nLine 2 is …",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateText(tt.text, tt.maxLines, tt.maxWidth, tt.mode)
			if result != tt.expected {
				t.Errorf("TruncateText() = %q, want %q", result, tt.expected)
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
			style := ParseStyle(tt.styleStr)
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
		styleStr string
		expected string
	}{
		{
			name:     "truncate with width constraint",
			text:     "This is a very long text that needs truncation",
			styleStr: "max-w-[20] truncate-suffix",
			expected: "This is a very long …",
		},
		{
			name:     "truncate with line constraint",
			text:     "Line 1\nLine 2\nLine 3\nLine 4",
			styleStr: "max-lines-[2] truncate-suffix",
			expected: "Line 1\nLine 2…",
		},
		{
			name:     "uppercase then truncate",
			text:     "hello world this is a test",
			styleStr: "uppercase max-w-[15] truncate-suffix",
			expected: "HELLO WORLD THI…",
		},
		{
			name:     "no truncation without mode",
			text:     "This is a very long text",
			styleStr: "max-w-[10]",
			expected: "This is a very long text",
		},
		{
			name:     "no truncation without constraint",
			text:     "Some text here",
			styleStr: "truncate-suffix",
			expected: "Some text here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := ApplyStyle(tt.text, tt.styleStr)
			if result != tt.expected {
				t.Errorf("ApplyStyle() = %q, want %q", result, tt.expected)
			}
		})
	}
}
