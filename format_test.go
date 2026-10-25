package clicky

import (
	"encoding/json"
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
		name     string
		format   string
		contains []string
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

func TestFormatWithContextCallbacks(t *testing.T) {
	t.Cleanup(ClearFormatCallbacks)

	type callbackContext struct {
		RequestID string
	}

	ctx := &callbackContext{RequestID: "req-42"}
	var beforeSeen any

	AddFormatCallback(FormatCallback{
		BeforeFormat: func(callbackCtx any, manager *FormatManager, options FormatOptions, before any) any {
			if manager == nil {
				t.Fatal("expected manager")
			}
			gotCtx, ok := callbackCtx.(*callbackContext)
			if !ok {
				t.Fatalf("callback ctx type = %T", callbackCtx)
			}
			if gotCtx.RequestID != ctx.RequestID {
				t.Fatalf("request id = %q, want %q", gotCtx.RequestID, ctx.RequestID)
			}

			payload, ok := before.(map[string]any)
			if !ok {
				t.Fatalf("before type = %T", before)
			}
			payload["request_id"] = gotCtx.RequestID
			return payload
		},
		AfterFormat: func(callbackCtx any, manager *FormatManager, options FormatOptions, before any, out string) string {
			beforeSeen = before
			return out + "\n# callback"
		},
	})

	output, err := FormatWithContext(ctx, map[string]any{"name": "Ada"}, FormatOptions{Format: "json"})
	if err != nil {
		t.Fatalf("FormatWithContext() error = %v", err)
	}

	var payload map[string]any
	raw := output[:len(output)-len("\n# callback")]
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("invalid json output: %v\n%s", err, output)
	}

	if payload["request_id"] != ctx.RequestID {
		t.Fatalf("request_id = %#v, want %q", payload["request_id"], ctx.RequestID)
	}

	beforePayload, ok := beforeSeen.(map[string]any)
	if !ok {
		t.Fatalf("after callback before type = %T", beforeSeen)
	}
	if beforePayload["request_id"] != ctx.RequestID {
		t.Fatalf("after callback before.request_id = %#v, want %q", beforePayload["request_id"], ctx.RequestID)
	}
}
