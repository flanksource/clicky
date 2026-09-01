package mcp

import (
	"net/http"
	"testing"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/rpc"
)

// EffectiveToolHints is the one place an operation's safety is derived, and a
// second derivation elsewhere would not conflict loudly — it fills a hint only
// while that hint is still unset, so whichever source ran first would win in
// silence. That is how an entity-level verb table briefly made `update`
// non-destructive.
//
// The table below is therefore the contract, asserted end to end through the
// operation shape a generated entity command actually produces.
func TestEffectiveToolHintsDerivesVerbSafety(t *testing.T) {
	tests := []struct {
		verb                              string
		method                            string
		readOnly, destructive, idempotent bool
	}{
		{verb: "list", method: http.MethodGet, readOnly: true, destructive: false, idempotent: true},
		{verb: "get", method: http.MethodGet, readOnly: true, destructive: false, idempotent: true},
		{verb: "create", method: http.MethodPost, readOnly: false, destructive: false, idempotent: false},
		{verb: "update", method: http.MethodPut, readOnly: false, destructive: true, idempotent: true},
		{verb: "delete", method: http.MethodDelete, readOnly: false, destructive: true, idempotent: true},
	}
	for _, tt := range tests {
		t.Run(tt.verb, func(t *testing.T) {
			hints := EffectiveToolHints(&rpc.RPCOperation{
				Name:   "invoice " + tt.verb,
				Method: tt.method,
				Clicky: &entity.ClickyOperationMeta{Entity: "invoice", Verb: tt.verb},
			})

			assertHint(t, "ReadOnlyHint", hints.ReadOnlyHint, tt.readOnly)
			assertHint(t, "DestructiveHint", hints.DestructiveHint, tt.destructive)
			assertHint(t, "IdempotentHint", hints.IdempotentHint, tt.idempotent)
		})
	}
}

// A hint the operation declares is its own answer, and derivation must not
// second-guess it — an author who says a custom update only reads means it.
func TestEffectiveToolHintsPrefersDeclaredHints(t *testing.T) {
	readOnly, nonDestructive := true, false
	hints := EffectiveToolHints(&rpc.RPCOperation{
		Name:   "invoice update",
		Method: http.MethodPut,
		Clicky: &entity.ClickyOperationMeta{
			Entity: "invoice", Verb: "update",
			ToolHints: entity.MCPToolHints{ReadOnlyHint: &readOnly, DestructiveHint: &nonDestructive},
		},
	})

	assertHint(t, "ReadOnlyHint", hints.ReadOnlyHint, true)
	assertHint(t, "DestructiveHint", hints.DestructiveHint, false)
}

// An operation clicky knows nothing about declares nothing, so a consumer can
// tell "did not say" apart from "said no".
func TestEffectiveToolHintsLeavesUnknownVerbsUnset(t *testing.T) {
	hints := EffectiveToolHints(&rpc.RPCOperation{Name: "widget frobnicate", Method: "TRACE"})

	if hints.ReadOnlyHint != nil {
		t.Errorf("ReadOnlyHint = %v, want nil", *hints.ReadOnlyHint)
	}
	if hints.DestructiveHint != nil {
		t.Errorf("DestructiveHint = %v, want nil", *hints.DestructiveHint)
	}
}

func assertHint(t *testing.T, name string, got *bool, want bool) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want %v", name, want)
	}
	if *got != want {
		t.Errorf("%s = %v, want %v", name, *got, want)
	}
}
