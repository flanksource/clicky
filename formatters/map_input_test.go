package formatters

import (
	"strings"
	"testing"
)

// TestMapStringAnyToMarkdown asserts that a top-level
// map[string]any (or any non-struct map kind) can flow through the
// MarkdownFormatter path that uses the ToPrettyData conversion.
//
// The map-handling logic was added to ToPrettyDataWithOptions but never
// mirrored into ToPrettyData, so any formatter that calls the older
// ToPrettyData returns an empty string and an "expected struct, got map"
// error for map input. This test exercises each of those entry points.
func TestMapStringAnyToMarkdown(t *testing.T) {
	data := map[string]any{
		"name":  "clicky",
		"count": 42,
		"nested": map[string]any{
			"key": "value",
		},
	}

	// We only need to see the keys/values make it through. Using
	// lower-case contains so we don't pin prettified casing.
	expected := []string{"name", "count", "nested", "key", "value", "clicky", "42"}

	formatters := []struct {
		name string
		run  func(t *testing.T) string
	}{
		{
			name: "MarkdownFormatter.Format",
			run: func(t *testing.T) string {
				out, err := NewMarkdownFormatter().Format(data)
				if err != nil {
					t.Fatalf("returned error: %v", err)
				}
				return out
			},
		},
		{
			name: "FormatManager.Markdown",
			run: func(t *testing.T) string {
				out, err := NewFormatManager().Markdown(data)
				if err != nil {
					t.Fatalf("returned error: %v", err)
				}
				return out
			},
		},
		{
			name: "ToPrettyData",
			run: func(t *testing.T) string {
				pd, err := ToPrettyData(data)
				if err != nil {
					t.Fatalf("returned error: %v", err)
				}
				if pd == nil {
					t.Fatalf("returned nil PrettyData")
				}
				// Render the PrettyData as markdown so we can assert
				// on the same output shape as the other subtests.
				out, err := NewMarkdownFormatter().FormatPrettyData(pd, FormatOptions{})
				if err != nil {
					t.Fatalf("FormatPrettyData returned error: %v", err)
				}
				return out
			},
		},
	}

	for _, f := range formatters {
		t.Run(f.name, func(t *testing.T) {
			out := f.run(t)
			if out == "" {
				t.Fatalf("got empty string, want non-empty output containing %v", expected)
			}
			lower := strings.ToLower(out)
			for _, want := range expected {
				if !strings.Contains(lower, want) {
					t.Errorf("output missing %q\nfull output: %q", want, out)
				}
			}
		})
	}
}

// TestMapStringAnyWithNestedTable guards the richer case of a top-level
// map whose value is a slice of maps. The map-handling code in
// ToPrettyDataWithOptions already nests the slice as a sub-table, so a
// proper fix to ToPrettyData should produce the same nested output.
func TestMapStringAnyWithNestedTable(t *testing.T) {
	data := map[string]any{
		"label": "servers",
		"servers": []any{
			map[string]any{"name": "web-01", "status": "up"},
			map[string]any{"name": "db-01", "status": "up"},
		},
	}

	out, err := NewMarkdownFormatter().Format(data)
	if err != nil {
		t.Fatalf("MarkdownFormatter.Format returned error: %v", err)
	}

	lower := strings.ToLower(out)
	for _, want := range []string{"label", "servers", "web-01", "db-01"} {
		if !strings.Contains(lower, want) {
			t.Errorf("output missing %q\nfull output: %q", want, out)
		}
	}
}
