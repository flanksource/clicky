package formatters

import (
	"strings"
	"testing"

	"github.com/flanksource/clicky/api/icons"
	. "github.com/flanksource/clicky/formatters"
)

// TestHTMLFormatter_IconifyScriptIncluded verifies that the Iconify script is included in HTML output
func TestHTMLFormatter_IconifyScriptIncluded(t *testing.T) {
	formatter := NewHTMLFormatter()
	formatter.IncludeCSS = true

	type TestData struct {
		Name string `json:"name"`
	}

	data := TestData{Name: "Test"}
	html, err := formatter.Format(data, FormatOptions{})
	if err != nil {
		t.Fatalf("Failed to format HTML: %v", err)
	}

	// Check that Iconify script is present
	if !strings.Contains(html, "https://code.iconify.design/iconify-icon/2.0.0/iconify-icon.min.js") {
		t.Error("Iconify script not found in HTML output")
	}
}

// TestHTMLFormatter_IconInText verifies that Icon.HTML() renders properly
func TestHTMLFormatter_IconInText(t *testing.T) {
	// Test Icon.HTML() method directly
	icon := icons.Success
	html := icon.HTML()

	// Icon.HTML() includes class="text-lg" and the icon's style (e.g., "text-green-500" for Success)
	expected := `<iconify-icon icon="ion:checkmark" class="text-lg text-green-500"></iconify-icon>`
	if html != expected {
		t.Errorf("Expected Icon.HTML() to return %q, got %q", expected, html)
	}
}

// TestHTMLFormatter_IconWithoutIconify verifies Unicode fallback when Iconify field is empty
func TestHTMLFormatter_IconWithoutIconify(t *testing.T) {
	icon := icons.Icon{
		Unicode: "✓",
		Iconify: "",
	}
	html := icon.HTML()

	expected := "✓"
	if html != expected {
		t.Errorf("Expected Icon.HTML() to return %q when no Iconify field, got %q", expected, html)
	}
}

// TestHTMLFormatter_IconInPrettyText verifies icons work when embedded in content
func TestHTMLFormatter_IconInPrettyText(t *testing.T) {
	formatter := NewHTMLFormatter()
	formatter.IncludeCSS = false

	// Create a struct with icon content
	type StatusData struct {
		Status string `json:"status"`
	}

	// Embed icon HTML directly in the content
	data := StatusData{
		Status: icons.Success.HTML() + " Success",
	}

	html, err := formatter.Format(data, FormatOptions{})
	if err != nil {
		t.Fatalf("Failed to format HTML: %v", err)
	}

	// Check that icon HTML is present (may be escaped)
	// Since it's embedded as a string, it might be HTML-escaped
	hasIconify := strings.Contains(html, `iconify`) || strings.Contains(html, `icon=`)
	if !hasIconify {
		t.Logf("HTML output: %s", html)
		t.Error("Icon HTML not found in formatted output (this is expected if HTML is escaped)")
	}
}

// TestHTMLFormatter_MultipleIcons verifies multiple different icons render correctly
func TestHTMLFormatter_MultipleIcons(t *testing.T) {
	testCases := []struct {
		name     string
		icon     icons.Icon
		dataIcon string
		unicode  string
	}{
		{
			name:     "Success icon",
			icon:     icons.Success,
			dataIcon: "ion:checkmark",
			unicode:  "✓",
		},
		{
			name:     "Error icon",
			icon:     icons.Error,
			dataIcon: "ion:close",
			unicode:  "✗",
		},
		{
			name:     "Warning icon",
			icon:     icons.Warning,
			dataIcon: "ion:warning",
			unicode:  "!",
		},
		{
			name:     "Info icon",
			icon:     icons.Info,
			dataIcon: "ion:information-circle",
			unicode:  "•",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			html := tc.icon.HTML()

			// Check for iconify-icon element
			if !strings.Contains(html, `<iconify-icon`) {
				t.Errorf("Expected iconify-icon element in HTML output")
			}

			// Check for icon attribute
			if !strings.Contains(html, `icon="`+tc.dataIcon+`"`) {
				t.Errorf("Expected icon=%q in HTML output", tc.dataIcon)
			}
		})
	}
}

// TestHTMLFormatter_IconInContent verifies icon HTML is preserved in output
func TestHTMLFormatter_IconInContent(t *testing.T) {
	// This test verifies that when Icon.HTML() output is used,
	// the resulting HTML contains the iconify markup

	iconHTML := icons.Success.HTML()

	// Verify the icon HTML contains the required attributes
	if !strings.Contains(iconHTML, `<iconify-icon`) {
		t.Error("Icon HTML missing iconify-icon element")
	}
	// Success = Check which uses ion:checkmark
	if !strings.Contains(iconHTML, `icon="ion:checkmark"`) {
		t.Error("Icon HTML missing icon attribute")
	}
}
