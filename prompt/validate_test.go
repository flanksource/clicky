package prompt

import (
	"encoding/json"
	"testing"
)

func TestValidateIntegerRejectsFractionalNumber(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"n":{"type":"integer"}}}`)
	if err := Validate(schema, map[string]any{"n": 1.5}); err == nil {
		t.Fatal("expected 1.5 to be rejected for an integer field")
	}
	if err := Validate(schema, map[string]any{"n": float64(2)}); err != nil {
		t.Fatalf("expected whole number 2 to pass integer validation: %v", err)
	}
}

func TestValidateNumberAcceptsFraction(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"n":{"type":"number"}}}`)
	if err := Validate(schema, map[string]any{"n": 1.5}); err != nil {
		t.Fatalf("expected 1.5 to pass number validation: %v", err)
	}
}

func TestValidateMultiSelectRejectsDuplicateSelection(t *testing.T) {
	schema := MultiSelectSchema("Pick", []string{"a", "b", "c"}, 0)
	if err := Validate(schema, map[string]any{"choice": []any{"1", "1"}}); err == nil {
		t.Fatal("expected a duplicate selection to be rejected by uniqueItems")
	}
	if err := Validate(schema, map[string]any{"choice": []any{"0", "1"}}); err != nil {
		t.Fatalf("expected distinct selections to pass: %v", err)
	}
}

func TestValidateMultiSelectEnforcesMaxItems(t *testing.T) {
	schema := MultiSelectSchema("Pick", []string{"a", "b", "c"}, 2)
	if err := Validate(schema, map[string]any{"choice": []any{"0", "1", "2"}}); err == nil {
		t.Fatal("expected an over-limit selection to be rejected by maxItems")
	}
	if err := Validate(schema, map[string]any{"choice": []any{"0", "1"}}); err != nil {
		t.Fatalf("expected an at-limit selection to pass: %v", err)
	}
}
