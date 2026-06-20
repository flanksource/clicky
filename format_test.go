package clicky

import (
	"strings"
	"testing"
)

// TestFormatMap pins down the public Format() entry point's behavior for
// map[string]any input across the three formats that already work.
//
// Format() routes through FormatWithOptions, which tries
// ToPrettyDataWithOptions (which handles maps) before falling back to
// the broken ToPrettyData path. This test guards against a future
// refactor that breaks that working path.
func TestFormatMap(t *testing.T) {
	data := map[string]any{
		"name":  "clicky",
		"count": 42,
		"nested": map[string]any{
			"key": "value",
		},
	}

	cases := []struct {
		name      string
		format    string
		contains  []string
	}{
		{
			name:     "markdown",
			format:   "markdown",
			contains: []string{"name", "count", "nested", "key", "value", "clicky"},
		},
		{
			name:     "json",
			format:   "json",
			contains: []string{`"name"`, `"clicky"`},
		},
		{
			name:     "yaml",
			format:   "yaml",
			contains: []string{"name:", "clicky"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := Format(data, FormatOptions{Format: c.format})
			if err != nil {
				t.Fatalf("Format returned error: %v", err)
			}
			if out == "" {
				t.Fatalf("Format returned empty string for map[%s]any; want non-empty %s", "string", c.name)
			}
			// Markdown prettifies keys ("name" -> "Name:") so we
			// compare case-insensitively. JSON/YAML pin their casing
			// and pass through unchanged, but lower-casing them too
			// doesn't hurt and keeps the test simple.
			haystack := strings.ToLower(out)
			for _, want := range c.contains {
				if !strings.Contains(haystack, strings.ToLower(want)) {
					t.Errorf("%s output missing %q\nfull output: %q", c.name, want, out)
				}
			}
		})
	}
}
