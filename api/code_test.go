package api

import "testing"

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

// TestCodeBlock_NormalizesPlainLanguageNames pins the regression where
// CodeBlock("typescript", ...) produced unwrapped escaped text because the
// underlying mimeTypeToLanguage helper only recognised MIME types and
// silently returned "" for plain language identifiers like "typescript",
// "go", "sql", causing Code.HTML() to fall through to html.EscapeString
// without the surrounding <pre><code> chroma wrapper.
func TestCodeBlock_NormalizesPlainLanguageNames(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"typescript", "typescript"},
		{"ts", "typescript"},
		{"go", "go"},
		{"golang", "go"},
		{"javascript", "javascript"},
		{"js", "javascript"},
		{"sql", "sql"},
		{"yaml", "yaml"},
		{"json", "json"},
		// MIME types still resolve correctly.
		{"application/json", "json"},
		{"text/x-typescript", "typescript"},
		{"application/javascript", "javascript"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := CodeBlock(tc.input, "x").Language
			if got != tc.want {
				t.Errorf("CodeBlock(%q).Language = %q, want %q", tc.input, got, tc.want)
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
