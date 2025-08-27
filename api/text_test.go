package api

import (
	"strings"
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

func TestResolveStyles(t *testing.T) {
	tests := []struct {
		name     string
		styles   []string
		expected Class
	}{
		{
			name:   "single text color",
			styles: []string{"text-red-500"},
			expected: Class{
				Foreground: &Color{Hex: "#ef4444"},
				Font:       &Font{},
			},
		},
		{
			name:   "single background color",
			styles: []string{"bg-blue-500"},
			expected: Class{
				Background: &Color{Hex: "#3b82f6"},
				Font:       &Font{},
			},
		},
		{
			name:   "both colors",
			styles: []string{"text-white bg-black"},
			expected: Class{
				Foreground: &Color{Hex: "#ffffff"},
				Background: &Color{Hex: "#000000"},
				Font:       &Font{},
			},
		},
		{
			name:   "font properties",
			styles: []string{"font-bold italic underline line-through"},
			expected: Class{
				Font: &Font{
					Bold:          true,
					Italic:        true,
					Underline:     true,
					Strikethrough: true,
				},
			},
		},
		{
			name:   "font size",
			styles: []string{"text-lg"},
			expected: Class{
				Font: &Font{
					Size: 1.125, // 18px
				},
			},
		},
		{
			name:   "multiple font sizes (last wins)",
			styles: []string{"text-sm", "text-xl"},
			expected: Class{
				Font: &Font{
					Size: 1.25, // 20px
				},
			},
		},
		{
			name:   "padding all sides",
			styles: []string{"p-4"},
			expected: Class{
				Font: &Font{},
				Padding: &Padding{
					Top:    1,
					Right:  1,
					Bottom: 1,
					Left:   1,
				},
			},
		},
		{
			name:   "padding horizontal",
			styles: []string{"px-8"},
			expected: Class{
				Font: &Font{},
				Padding: &Padding{
					Right: 2,
					Left:  2,
				},
			},
		},
		{
			name:   "padding vertical",
			styles: []string{"py-2"},
			expected: Class{
				Font: &Font{},
				Padding: &Padding{
					Top:    0.5,
					Bottom: 0.5,
				},
			},
		},
		{
			name:   "individual padding sides",
			styles: []string{"pt-1 pr-2 pb-3 pl-4"},
			expected: Class{
				Font: &Font{},
				Padding: &Padding{
					Top:    0.25,
					Right:  0.5,
					Bottom: 0.75,
					Left:   1,
				},
			},
		},
		{
			name:   "padding override",
			styles: []string{"p-4 px-2"},
			expected: Class{
				Font: &Font{},
				Padding: &Padding{
					Top:    1,
					Right:  0.5, // overridden by px-2
					Bottom: 1,
					Left:   0.5, // overridden by px-2
				},
			},
		},
		{
			name:   "complex combination",
			styles: []string{"text-green-600 bg-yellow-100 font-bold text-xl p-4 px-6"},
			expected: Class{
				Foreground: &Color{Hex: "#16a34a"},
				Background: &Color{Hex: "#fef9c3"},
				Font: &Font{
					Bold: true,
					Size: 1.25, // 20px
				},
				Padding: &Padding{
					Top:    1,
					Right:  1.5, // overridden by px-6
					Bottom: 1,
					Left:   1.5, // overridden by px-6
				},
			},
		},
		{
			name:   "opacity classes",
			styles: []string{"opacity-50"},
			expected: Class{
				Font: &Font{
					Faint: true,
				},
			},
		},
		{
			name:   "multiple style strings",
			styles: []string{"text-red-500 font-bold", "bg-blue-500 italic", "p-4"},
			expected: Class{
				Foreground: &Color{Hex: "#ef4444"},
				Background: &Color{Hex: "#3b82f6"},
				Font: &Font{
					Bold:   true,
					Italic: true,
				},
				Padding: &Padding{
					Top:    1,
					Right:  1,
					Bottom: 1,
					Left:   1,
				},
			},
		},
		{
			name:   "resetting font properties",
			styles: []string{"font-bold italic", "font-normal not-italic"},
			expected: Class{
				Font: &Font{
					Bold:   false,
					Italic: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveStyles(tt.styles...)

			// Check foreground color
			if tt.expected.Foreground != nil {
				if result.Foreground == nil {
					t.Errorf("Expected foreground color %v, got nil", tt.expected.Foreground)
				} else if result.Foreground.Hex != tt.expected.Foreground.Hex {
					t.Errorf("Expected foreground hex %s, got %s", tt.expected.Foreground.Hex, result.Foreground.Hex)
				}
			} else if result.Foreground != nil {
				t.Errorf("Expected nil foreground, got %v", result.Foreground)
			}

			// Check background color
			if tt.expected.Background != nil {
				if result.Background == nil {
					t.Errorf("Expected background color %v, got nil", tt.expected.Background)
				} else if result.Background.Hex != tt.expected.Background.Hex {
					t.Errorf("Expected background hex %s, got %s", tt.expected.Background.Hex, result.Background.Hex)
				}
			} else if result.Background != nil {
				t.Errorf("Expected nil background, got %v", result.Background)
			}

			// Check font properties
			if tt.expected.Font != nil && result.Font != nil {
				if result.Font.Bold != tt.expected.Font.Bold {
					t.Errorf("Expected bold %v, got %v", tt.expected.Font.Bold, result.Font.Bold)
				}
				if result.Font.Italic != tt.expected.Font.Italic {
					t.Errorf("Expected italic %v, got %v", tt.expected.Font.Italic, result.Font.Italic)
				}
				if result.Font.Underline != tt.expected.Font.Underline {
					t.Errorf("Expected underline %v, got %v", tt.expected.Font.Underline, result.Font.Underline)
				}
				if result.Font.Strikethrough != tt.expected.Font.Strikethrough {
					t.Errorf("Expected strikethrough %v, got %v", tt.expected.Font.Strikethrough, result.Font.Strikethrough)
				}
				if result.Font.Faint != tt.expected.Font.Faint {
					t.Errorf("Expected faint %v, got %v", tt.expected.Font.Faint, result.Font.Faint)
				}
				if result.Font.Size != tt.expected.Font.Size {
					t.Errorf("Expected size %v, got %v", tt.expected.Font.Size, result.Font.Size)
				}
			}

			// Check padding
			if tt.expected.Padding != nil {
				if result.Padding == nil {
					t.Errorf("Expected padding %v, got nil", tt.expected.Padding)
				} else {
					if result.Padding.Top != tt.expected.Padding.Top {
						t.Errorf("Expected padding top %v, got %v", tt.expected.Padding.Top, result.Padding.Top)
					}
					if result.Padding.Right != tt.expected.Padding.Right {
						t.Errorf("Expected padding right %v, got %v", tt.expected.Padding.Right, result.Padding.Right)
					}
					if result.Padding.Bottom != tt.expected.Padding.Bottom {
						t.Errorf("Expected padding bottom %v, got %v", tt.expected.Padding.Bottom, result.Padding.Bottom)
					}
					if result.Padding.Left != tt.expected.Padding.Left {
						t.Errorf("Expected padding left %v, got %v", tt.expected.Padding.Left, result.Padding.Left)
					}
				}
			} else if result.Padding != nil {
				t.Errorf("Expected nil padding, got %v", result.Padding)
			}
		})
	}
}

func TestTextWithClass(t *testing.T) {
	tests := []struct {
		name     string
		text     Text
		contains []string // strings that should be in the output
	}{
		{
			name: "text with Class colors ANSI",
			text: Text{
				Content: "Hello",
				Class: Class{
					Foreground: &Color{Hex: "#ff0000"},
					Font:       &Font{Bold: true},
				},
			},
			contains: []string{"Hello", "\x1b[1"}, // Bold ANSI code (partial match since it's combined with color)
		},
		{
			name: "text with Class bold markdown",
			text: Text{
				Content: "Hello",
				Class: Class{
					Font: &Font{Bold: true},
				},
			},
			contains: []string{"**Hello**"},
		},
		{
			name: "text with Class italic markdown",
			text: Text{
				Content: "Hello",
				Class: Class{
					Font: &Font{Italic: true},
				},
			},
			contains: []string{"*Hello*"},
		},
		{
			name: "text with Class HTML",
			text: Text{
				Content: "Hello",
				Class: Class{
					Foreground: &Color{Hex: "#ff0000"},
					Font:       &Font{Bold: true},
				},
			},
			contains: []string{"<strong>", "Hello", "</strong>", "color: #ff0000"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" ANSI", func(t *testing.T) {
			result := tt.text.ANSI()
			if strings.Contains(tt.name, "ANSI") {
				for _, expected := range tt.contains {
					if !strings.Contains(result, expected) {
						t.Errorf("Expected ANSI output to contain %q, got %q", expected, result)
					}
				}
			}
		})

		t.Run(tt.name+" Markdown", func(t *testing.T) {
			result := tt.text.Markdown()
			if strings.Contains(tt.name, "markdown") {
				for _, expected := range tt.contains {
					if !strings.Contains(result, expected) {
						t.Errorf("Expected Markdown output to contain %q, got %q", expected, result)
					}
				}
			}
		})

		t.Run(tt.name+" HTML", func(t *testing.T) {
			result := tt.text.HTML()
			if strings.Contains(tt.name, "HTML") {
				for _, expected := range tt.contains {
					if !strings.Contains(result, expected) {
						t.Errorf("Expected HTML output to contain %q, got %q", expected, result)
					}
				}
			}
		})
	}
}

func TestClassPrecedence(t *testing.T) {
	// Test that Class takes precedence over Style string
	text := Text{
		Content: "Hello",
		Class: Class{
			Foreground: &Color{Hex: "#ff0000"},
			Font:       &Font{Bold: true},
		},
		Style: "text-blue-500 italic", // This should be ignored
	}

	// In ANSI output, should use Class (red + bold) not Style (blue + italic)
	ansi := text.ANSI()
	if !strings.Contains(ansi, "\x1b[1") { // Bold from Class (partial match since combined with color)
		t.Error("Expected ANSI to contain bold escape code")
	}
	if strings.Contains(ansi, "\x1b[3") { // Should NOT have italic from Style
		t.Error("Expected ANSI to not contain italic escape code")
	}

	// In Markdown output, should use Class
	markdown := text.Markdown()
	if !strings.Contains(markdown, "**Hello**") { // Bold from Class
		t.Error("Expected Markdown to contain bold markers")
	}
	// Check that it's not italic - but be careful since **Hello** contains *Hello*
	// The output should be <span style="color: #ff0000">**Hello**</span>
	// If it were italic, it would be ***Hello*** or *Hello*
	if strings.Contains(markdown, "***Hello***") || (strings.Contains(markdown, "*Hello*") && !strings.Contains(markdown, "**Hello**")) {
		t.Error("Expected Markdown to not contain italic markers")
	}
	if !strings.Contains(markdown, "color: #ff0000") { // Red from Class
		t.Error("Expected Markdown to contain red color")
	}
	if strings.Contains(markdown, "color: #3b82") { // Not blue from Style
		t.Error("Expected Markdown to not contain blue color")
	}
}
