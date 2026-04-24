package clicky

import (
	"encoding/json"
	"testing"
)

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
