package api

import (
	"testing"
)

func TestText(t *testing.T) {
	fixtures := []struct {
		name     string
		input    Text
		plain    string
		markdown string
		ansi     string
		html     string
	}{
		{
			name:     "Simple text",
			input:    Text{Content: "Hello, world!"},
			plain:    "Hello, world!",
			markdown: "Hello, world!",
			html:     "Hello, world!",
			ansi:     "Hello, world!",
		},
		{
			name:     "Text with children",
			input:    Text{Content: "Hello, ", Children: []Text{{Content: "world!", Style: "font-bold"}}},
			plain:    "Hello, world!",
			markdown: "Hello, **world!**",
			html:     "Hello, <span class=\"font-bold\"><strong>world!</strong></span>",
			ansi:     "Hello, \x1b[1mworld!\x1b[0m",
		},
		{
			name:     "Text Color", 
			input:    Text{Content: "Hello, world!", Style: "text-red-500"},
			plain:    "Hello, world!",
			markdown: "<span style=\"color: #ef4444\">Hello, world!</span>",
			html:     "<span class=\"text-red-500\" style=\"color: #ef4444\">Hello, world!</span>",
			ansi:     "\x1b[38;2;239;68;68mHello, world!\x1b[0m",
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if fixture.input.String() != fixture.plain {
				t.Errorf("Expected plain text %q, got %q", fixture.plain, fixture.input.String())
			}
			if fixture.input.Markdown() != fixture.markdown {
				t.Errorf("Expected markdown %q, got %q", fixture.markdown, fixture.input.Markdown())
			}
			if fixture.input.HTML() != fixture.html {
				t.Errorf("Expected HTML %q, got %q", fixture.html, fixture.input.HTML())
			}
			if fixture.input.ANSI() != fixture.ansi {
				t.Errorf("Expected ANSI %q, got %q", fixture.ansi, fixture.input.ANSI())
			}
		})
	}
}


func TestTailwindStyles(t *testing.T) {
	fixtures := []struct {
		name     string
		input    Text
		plain    string
		markdown string
		ansi     string
		html     string
	}{
		// Font Styles
		{
			name:     "Bold text",
			input:    Text{Content: "Important", Style: "font-bold"},
			plain:    "Important",
			markdown: "**Important**",
			ansi:     "\x1b[1mImportant\x1b[0m",
			html:     "<span class=\"font-bold\"><strong>Important</strong></span>",
		},
		{
			name:     "Italic text",
			input:    Text{Content: "Emphasized", Style: "italic"},
			plain:    "Emphasized",
			markdown: "*Emphasized*",
			ansi:     "\x1b[3mEmphasized\x1b[0m",
			html:     "<span class=\"italic\"><em>Emphasized</em></span>",
		},
		{
			name:     "Bold and italic",
			input:    Text{Content: "Important", Style: "font-bold italic"},
			plain:    "Important",
			markdown: "***Important***",
			ansi:     "\x1b[1;3mImportant\x1b[0m",
			html:     "<span class=\"font-bold italic\"><em><strong>Important</strong></em></span>",
		},
		// Text Decorations
		{
			name:     "Underlined text",
			input:    Text{Content: "Link", Style: "underline"},
			plain:    "Link",
			markdown: "Link", // Markdown doesn't support underline
			ansi:     "\x1b[4mLink\x1b[0m",
			html:     "<span class=\"underline\"><u>Link</u></span>",
		},
		{
			name:     "Strikethrough text",
			input:    Text{Content: "Deleted", Style: "line-through"},
			plain:    "Deleted",
			markdown: "~~Deleted~~",
			ansi:     "\x1b[9mDeleted\x1b[29m",
			html:     "<span class=\"line-through\"><s>Deleted</s></span>",
		},
		// Text Transforms
		{
			name:     "Uppercase transform",
			input:    Text{Content: "hello world", Style: "uppercase"},
			plain:    "HELLO WORLD",
			markdown: "HELLO WORLD",
			ansi:     "HELLO WORLD",
			html:     "<span class=\"uppercase\">HELLO WORLD</span>",
		},
		{
			name:     "Capitalize transform",
			input:    Text{Content: "hello world", Style: "capitalize"},
			plain:    "Hello World",
			markdown: "Hello World",
			ansi:     "Hello World",
			html:     "<span class=\"capitalize\">Hello World</span>",
		},
		// Colors
		{
			name:     "Red text",
			input:    Text{Content: "Error", Style: "text-red-500"},
			plain:    "Error",
			markdown: "<span style=\"color: #ef4444\">Error</span>",
			ansi:     "\x1b[38;2;239;68;68mError\x1b[0m",
			html:     "<span class=\"text-red-500\" style=\"color: #ef4444\">Error</span>",
		},
		{
			name:     "Blue text",
			input:    Text{Content: "Info", Style: "text-blue-700"},
			plain:    "Info",
			markdown: "<span style=\"color: #1d4ed8\">Info</span>",
			ansi:     "\x1b[38;2;29;78;216mInfo\x1b[0m",
			html:     "<span class=\"text-blue-700\" style=\"color: #1d4ed8\">Info</span>",
		},
		// Backgrounds
		{
			name:     "Yellow background",
			input:    Text{Content: "Highlight", Style: "bg-yellow-200"},
			plain:    "Highlight",
			markdown: "<span style=\"background-color: #fef08a\">Highlight</span>",
			ansi:     "\x1b[48;2;254;240;138mHighlight\x1b[0m",
			html:     "<span class=\"bg-yellow-200\" style=\"background-color: #fef08a\">Highlight</span>",
		},
		// Emoji Support
		{
			name:     "Emoji with styles",
			input:    Text{Content: "Success ✅", Style: "text-green-500 font-bold"},
			plain:    "Success ✅",
			markdown: "<span style=\"color: #22c55e\">**Success ✅**</span>",
			ansi:     "\x1b[1;38;2;34;197;94mSuccess ✅\x1b[0m",
			html:     "<span class=\"text-green-500 font-bold\" style=\"color: #22c55e\"><strong>Success ✅</strong></span>",
		},
		{
			name:     "Emoji with uppercase",
			input:    Text{Content: "party 🎉", Style: "uppercase"},
			plain:    "PARTY 🎉",
			markdown: "PARTY 🎉",
			ansi:     "PARTY 🎉",
			html:     "<span class=\"uppercase\">PARTY 🎉</span>",
		},
		// Combined Styles
		{
			name:     "Complex styling",
			input:    Text{Content: "Alert", Style: "uppercase text-red-600 bg-red-100 font-bold underline"},
			plain:    "ALERT",
			markdown: "<span style=\"color: #dc2626; background-color: #fee2e2\">**ALERT**</span>",
			ansi:     "\x1b[1;4;38;2;220;38;38;48;2;254;226;226mALERT\x1b[0m",
			html:     "<span class=\"uppercase text-red-600 bg-red-100 font-bold underline\" style=\"color: #dc2626; background-color: #fee2e2\"><strong><u>ALERT</u></strong></span>",
		},
		// Edge Cases
		{
			name:     "Empty style",
			input:    Text{Content: "Plain text", Style: ""},
			plain:    "Plain text",
			markdown: "Plain text",
			ansi:     "Plain text",
			html:     "Plain text",
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if fixture.input.String() != fixture.plain {
				t.Errorf("Expected plain text %q, got %q", fixture.plain, fixture.input.String())
			}
			if fixture.input.Markdown() != fixture.markdown {
				t.Errorf("Expected markdown %q, got %q", fixture.markdown, fixture.input.Markdown())
			}
			if fixture.input.HTML() != fixture.html {
				t.Errorf("Expected HTML %q, got %q", fixture.html, fixture.input.HTML())
			}
			if fixture.input.ANSI() != fixture.ansi {
				t.Errorf("Expected ANSI %q, got %q", fixture.ansi, fixture.input.ANSI())
			}
		})
	}
}

// Keep a simple benchmark for performance testing
func BenchmarkTailwindStyles(b *testing.B) {
	testText := Text{Content: "Hello World", Style: "uppercase text-blue-700 bg-gray-100 font-bold italic underline"}
	
	b.Run("ANSI", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			testText.ANSI()
		}
	})
	
	b.Run("HTML", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			testText.HTML()
		}
	})
	
	b.Run("Markdown", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			testText.Markdown()
		}
	})
}

// Remove all old test functions - they've been replaced with TestTailwindStyles
