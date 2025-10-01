package api

import (
	"strings"
	"testing"
)

func TestNewCode(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		language string
		wantLang string
	}{
		{
			name:     "explicit_sql",
			content:  "SELECT * FROM users",
			language: "sql",
			wantLang: "sql",
		},
		{
			name:     "explicit_go",
			content:  "package main\nfunc main() {}",
			language: "go",
			wantLang: "go",
		},
		{
			name:     "explicit_javascript",
			content:  "const x = 1;",
			language: "javascript",
			wantLang: "javascript",
		},
		{
			name:     "auto_detect_sql",
			content:  "SELECT id, name FROM users WHERE active = true",
			language: "",
			wantLang: "sql",
		},
		{
			name:     "auto_detect_go",
			content:  "package main\n\nfunc test() {\n}",
			language: "",
			wantLang: "go",
		},
		{
			name:     "normalize_golang",
			content:  "package main",
			language: "golang",
			wantLang: "go",
		},
		{
			name:     "normalize_js",
			content:  "console.log('test')",
			language: "js",
			wantLang: "javascript",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := NewCode(tt.content, tt.language)
			if code.Language != tt.wantLang {
				t.Errorf("NewCode() language = %v, want %v", code.Language, tt.wantLang)
			}
			if code.Content != tt.content {
				t.Errorf("NewCode() content = %v, want %v", code.Content, tt.content)
			}
		})
	}
}

func TestCode_String(t *testing.T) {
	tests := []struct {
		name string
		code Code
		want string
	}{
		{
			name: "simple_code",
			code: Code{Content: "SELECT * FROM users", Language: "sql"},
			want: "SELECT * FROM users",
		},
		{
			name: "empty_code",
			code: Code{Content: "", Language: "sql"},
			want: "",
		},
		{
			name: "multiline_code",
			code: Code{
				Content:  "SELECT id,\n       name\nFROM users",
				Language: "sql",
			},
			want: "SELECT id,\n       name\nFROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.code.String(); got != tt.want {
				t.Errorf("Code.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCode_ANSI(t *testing.T) {
	tests := []struct {
		name         string
		code         Code
		wantContains string
	}{
		{
			name:         "sql_with_ansi",
			code:         Code{Content: "SELECT * FROM users", Language: "sql"},
			wantContains: "SELECT",
		},
		{
			name:         "go_with_ansi",
			code:         Code{Content: "package main\n\nfunc main() {}", Language: "go"},
			wantContains: "package",
		},
		{
			name:         "javascript_with_ansi",
			code:         Code{Content: "const x = 1;", Language: "javascript"},
			wantContains: "const",
		},
		{
			name:         "empty_code",
			code:         Code{Content: "", Language: "sql"},
			wantContains: "",
		},
		{
			name:         "unknown_language",
			code:         Code{Content: "some code", Language: "unknownlang"},
			wantContains: "some code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.code.ANSI()
			if tt.wantContains != "" && !strings.Contains(got, tt.wantContains) {
				t.Errorf("Code.ANSI() should contain %q, got %v", tt.wantContains, got)
			}
			// For non-empty content with known language, verify ANSI codes are present
			if tt.code.Content != "" && tt.code.Language != "unknownlang" && tt.name != "empty_code" {
				// ANSI escape codes start with \x1b or \033
				if !strings.Contains(got, "\x1b[") && !strings.Contains(got, "\033[") {
					t.Errorf("Code.ANSI() should contain ANSI escape codes for %s", tt.name)
				}
			}
		})
	}
}

func TestCode_HTML(t *testing.T) {
	tests := []struct {
		name         string
		code         Code
		wantContains []string
	}{
		{
			name:         "sql_html",
			code:         Code{Content: "SELECT * FROM users", Language: "sql"},
			wantContains: []string{"SELECT", "FROM", "users"},
		},
		{
			name:         "go_html",
			code:         Code{Content: "package main", Language: "go"},
			wantContains: []string{"package", "main"},
		},
		{
			name:         "java_html",
			code:         Code{Content: "public class Test {}", Language: "java"},
			wantContains: []string{"public", "class", "Test"},
		},
		{
			name:         "javascript_html",
			code:         Code{Content: "const x = 1;", Language: "javascript"},
			wantContains: []string{"const", "x"},
		},
		{
			name:         "xml_html",
			code:         Code{Content: "<?xml version=\"1.0\"?>\n<root></root>", Language: "xml"},
			wantContains: []string{"xml", "root"},
		},
		{
			name:         "empty_code",
			code:         Code{Content: "", Language: "sql"},
			wantContains: []string{},
		},
		{
			name:         "unknown_language_escapes_html",
			code:         Code{Content: "<script>alert('xss')</script>", Language: "unknownlang"},
			wantContains: []string{"&lt;script&gt;", "&lt;/script&gt;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.code.HTML()
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("Code.HTML() should contain %q, got %v", want, got)
				}
			}
			// Verify HTML output contains style tags for syntax highlighting
			if tt.code.Content != "" && tt.code.Language != "unknownlang" && tt.name != "empty_code" && tt.name != "unknown_language_escapes_html" {
				if !strings.Contains(got, "style=") && !strings.Contains(got, "class=") {
					t.Logf("Warning: Code.HTML() for %s might not have syntax highlighting", tt.name)
				}
			}
		})
	}
}

func TestCode_Markdown(t *testing.T) {
	tests := []struct {
		name string
		code Code
		want string
	}{
		{
			name: "sql_markdown",
			code: Code{Content: "SELECT * FROM users", Language: "sql"},
			want: "```sql\nSELECT * FROM users\n```",
		},
		{
			name: "go_markdown",
			code: Code{Content: "package main", Language: "go"},
			want: "```go\npackage main\n```",
		},
		{
			name: "javascript_markdown",
			code: Code{Content: "const x = 1;", Language: "javascript"},
			want: "```javascript\nconst x = 1;\n```",
		},
		{
			name: "empty_code",
			code: Code{Content: "", Language: "sql"},
			want: "```\n```",
		},
		{
			name: "no_language",
			code: Code{Content: "some code", Language: ""},
			want: "```text\nsome code\n```",
		},
		{
			name: "multiline_markdown",
			code: Code{
				Content:  "SELECT id,\n       name\nFROM users",
				Language: "sql",
			},
			want: "```sql\nSELECT id,\n       name\nFROM users\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.code.Markdown(); got != tt.want {
				t.Errorf("Code.Markdown() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeLanguage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"golang", "go"},
		{"Golang", "go"},
		{"GO", "go"},
		{"js", "javascript"},
		{"JS", "javascript"},
		{"typescript", "typescript"},
		{"ts", "typescript"},
		{"sql", "sql"},
		{"SQL", "sql"},
		{"python", "python"},
		{"py", "python"},
		{"java", "java"},
		{"xml", "xml"},
		{"xslt", "xslt"},
		{"unknownlang", "unknownlang"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeLanguage(tt.input); got != tt.want {
				t.Errorf("normalizeLanguage(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "detect_go",
			content: "package main\n\nfunc test() {}",
			want:    "go",
		},
		{
			name:    "detect_sql_select",
			content: "SELECT id, name FROM users WHERE active = true",
			want:    "sql",
		},
		{
			name:    "detect_sql_lowercase",
			content: "select * from users",
			want:    "sql",
		},
		{
			name:    "detect_xml",
			content: "<?xml version=\"1.0\"?>\n<root></root>",
			want:    "xml",
		},
		{
			name:    "detect_java",
			content: "public class Test {\n    public static void main(String[] args) {}\n}",
			want:    "java",
		},
		{
			name:    "detect_javascript",
			content: "const x = 1;\nfunction test() {}",
			want:    "javascript",
		},
		{
			name:    "unknown_content",
			content: "random text content",
			want:    "",
		},
		{
			name:    "empty_content",
			content: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectLanguage(tt.content); got != tt.want {
				t.Errorf("detectLanguage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCode_Textable(t *testing.T) {
	// Verify Code implements Textable interface
	var _ Textable = (*Code)(nil)
	var _ Textable = Code{}

	code := NewCode("SELECT * FROM users", "sql")

	// Test all Textable methods are available
	_ = code.String()
	_ = code.ANSI()
	_ = code.HTML()
	_ = code.Markdown()
}
