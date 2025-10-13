package api

import (
	"strings"
	"testing"
)

func TestKeyValuePair_String(t *testing.T) {
	tests := []struct {
		name     string
		kv       KeyValuePair
		expected string
	}{
		{
			name:     "basic key-value",
			kv:       KeyValuePair{Key: "Status", Value: "Active"},
			expected: "Status: Active",
		},
		{
			name:     "numeric value",
			kv:       KeyValuePair{Key: "Count", Value: 42},
			expected: "Count: 42",
		},
		{
			name:     "empty value",
			kv:       KeyValuePair{Key: "Empty", Value: ""},
			expected: "Empty: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.kv.String()
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestKeyValuePair_ANSI(t *testing.T) {
	tests := []struct {
		name  string
		kv    KeyValuePair
		check func(string) bool
	}{
		{
			name: "basic key-value",
			kv:   KeyValuePair{Key: "Status", Value: "Active"},
			check: func(s string) bool {
				// Should contain the key and value, key should be muted (faint)
				return strings.Contains(s, "Status:") && strings.Contains(s, "Active")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.kv.ANSI()
			if !tt.check(result) {
				t.Errorf("ANSI() = %q, check failed", result)
			}
		})
	}
}

func TestKeyValuePair_HTML_Compact(t *testing.T) {
	tests := []struct {
		name     string
		kv       KeyValuePair
		expected []string // Substrings that must be present
	}{
		{
			name: "compact style (default)",
			kv:   KeyValuePair{Key: "Status", Value: "Active", Style: "compact"},
			expected: []string{
				"<div class=\"inline-flex gap-1\">",
				"<dt class=\"text-gray-500 font-medium\">Status:</dt>",
				"<dd class=\"text-gray-900\">Active</dd>",
				"</div>",
			},
		},
		{
			name: "compact style with special characters",
			kv:   KeyValuePair{Key: "Test<Key>", Value: "Value&Data", Style: "compact"},
			expected: []string{
				"Test&lt;Key&gt;",
				"Value&amp;Data",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.kv.HTML()
			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("HTML() missing expected substring %q in result: %q", expected, result)
				}
			}
		})
	}
}

func TestKeyValuePair_HTML_Badge(t *testing.T) {
	tests := []struct {
		name     string
		kv       KeyValuePair
		expected []string // Substrings that must be present
	}{
		{
			name: "badge style",
			kv:   KeyValuePair{Key: "Status", Value: "Active", Style: "badge"},
			expected: []string{
				"<span class=\"inline-flex items-center gap-1 px-3 py-1 rounded-full bg-gray-100\">",
				"<dt class=\"text-xs font-medium text-gray-600\">Status:</dt>",
				"<dd class=\"text-xs font-semibold text-gray-900\">Active</dd>",
				"</span>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.kv.HTML()
			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("HTML() missing expected substring %q in result: %q", expected, result)
				}
			}
		})
	}
}

func TestKeyValuePair_Markdown(t *testing.T) {
	tests := []struct {
		name     string
		kv       KeyValuePair
		expected string
	}{
		{
			name:     "basic key-value",
			kv:       KeyValuePair{Key: "Status", Value: "Active"},
			expected: "**Status**: Active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.kv.Markdown()
			if result != tt.expected {
				t.Errorf("Markdown() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDescriptionList_String(t *testing.T) {
	tests := []struct {
		name     string
		dl       DescriptionList
		expected string
	}{
		{
			name: "multiple items",
			dl: DescriptionList{
				Items: []KeyValuePair{
					{Key: "Name", Value: "John"},
					{Key: "Age", Value: 30},
				},
			},
			expected: "Name: John, Age: 30",
		},
		{
			name:     "empty list",
			dl:       DescriptionList{Items: []KeyValuePair{}},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dl.String()
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDescriptionList_HTML_Compact(t *testing.T) {
	tests := []struct {
		name     string
		dl       DescriptionList
		expected []string
	}{
		{
			name: "compact style (default)",
			dl: DescriptionList{
				Items: []KeyValuePair{
					{Key: "Status", Value: "Active"},
					{Key: "Count", Value: 42},
				},
				Style: "compact",
			},
			expected: []string{
				"<dl class=\"inline-flex flex-wrap gap-x-4 gap-y-1\">",
				"<dt class=\"text-gray-500 font-medium\">Status:</dt>",
				"<dd class=\"text-gray-900\">Active</dd>",
				"<dt class=\"text-gray-500 font-medium\">Count:</dt>",
				"<dd class=\"text-gray-900\">42</dd>",
				"</dl>",
			},
		},
		{
			name:     "empty list",
			dl:       DescriptionList{Items: []KeyValuePair{}, Style: "compact"},
			expected: []string{`<span class="text-gray-400">{}</span>`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dl.HTML()
			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("HTML() missing expected substring %q in result: %q", expected, result)
				}
			}
		})
	}
}

func TestDescriptionList_HTML_Badge(t *testing.T) {
	tests := []struct {
		name     string
		dl       DescriptionList
		expected []string
	}{
		{
			name: "badge style",
			dl: DescriptionList{
				Items: []KeyValuePair{
					{Key: "Status", Value: "Active"},
					{Key: "Count", Value: 42},
				},
				Style: "badge",
			},
			expected: []string{
				"<div class=\"inline-flex flex-wrap gap-2\">",
				"<span class=\"inline-flex items-center gap-1 px-3 py-1 rounded-full bg-gray-100\">",
				"<dt class=\"text-xs font-medium text-gray-600\">Status:</dt>",
				"<dd class=\"text-xs font-semibold text-gray-900\">Active</dd>",
				"</span>",
				"<dt class=\"text-xs font-medium text-gray-600\">Count:</dt>",
				"<dd class=\"text-xs font-semibold text-gray-900\">42</dd>",
				"</div>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dl.HTML()
			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("HTML() missing expected substring %q in result: %q", expected, result)
				}
			}
		})
	}
}

func TestDescriptionList_Markdown(t *testing.T) {
	tests := []struct {
		name     string
		dl       DescriptionList
		expected string
	}{
		{
			name: "multiple items",
			dl: DescriptionList{
				Items: []KeyValuePair{
					{Key: "Name", Value: "John"},
					{Key: "Age", Value: 30},
				},
			},
			expected: "**Name**: John, **Age**: 30",
		},
		{
			name:     "empty list",
			dl:       DescriptionList{Items: []KeyValuePair{}},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dl.Markdown()
			if result != tt.expected {
				t.Errorf("Markdown() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHTMLEscaping(t *testing.T) {
	tests := []struct {
		name     string
		kv       KeyValuePair
		expected []string
	}{
		{
			name: "HTML special characters",
			kv:   KeyValuePair{Key: "<script>alert('test')</script>", Value: "Value & <Data>", Style: "compact"},
			expected: []string{
				"&lt;script&gt;alert(&#39;test&#39;)&lt;/script&gt;",
				"Value &amp; &lt;Data&gt;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.kv.HTML()
			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("HTML() missing expected escaped substring %q in result: %q", expected, result)
				}
			}
		})
	}
}
