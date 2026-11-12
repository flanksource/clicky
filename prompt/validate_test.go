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

// scaleSchema mirrors a fixture form whose "cluster" field only exists, and is
// only required, while "scale" is true.
const scaleSchema = `{"type":"object","required":["db"],
	"properties":{"db":{"type":"string"},"scale":{"type":"boolean"}},
	"allOf":[{"if":{"properties":{"scale":{"const":true}},"required":["scale"]},
		"then":{"required":["cluster"],"properties":{"cluster":{"type":"string"}}}}]}`

func TestValidateAllOfIfThenRequiresFieldOnlyWhenIfMatches(t *testing.T) {
	schema := json.RawMessage(scaleSchema)
	err := Validate(schema, map[string]any{"db": "x", "scale": true})
	if err == nil || err.Error() != `missing required field "cluster"` {
		t.Fatalf("expected missing cluster when scale=true, got %v", err)
	}
	if err := Validate(schema, map[string]any{"db": "x", "scale": true, "cluster": 7.0}); err == nil {
		t.Fatal("expected the then-branch property type to be enforced")
	}
	for _, values := range []map[string]any{
		{"db": "x", "scale": false},
		{"db": "x"},
		{"db": "x", "scale": true, "cluster": "c1"},
	} {
		if err := Validate(schema, values); err != nil {
			t.Fatalf("expected %v to pass: %v", values, err)
		}
	}
}

func TestValidateAllOfElseBranchAppliesWhenIfFails(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"mode":{"type":"string"}},
		"allOf":[{"if":{"properties":{"mode":{"enum":["auto"]}},"required":["mode"]},
			"then":{"properties":{"interval":{"type":"integer"}}},
			"else":{"required":["target"]}}]}`)
	if err := Validate(schema, map[string]any{"mode": "manual"}); err == nil || err.Error() != `missing required field "target"` {
		t.Fatalf("expected else-branch to require target, got %v", err)
	}
	if err := Validate(schema, map[string]any{"mode": "auto", "interval": 1.5}); err == nil {
		t.Fatal("expected then-branch integer type to reject 1.5")
	}
	if err := Validate(schema, map[string]any{"mode": "auto"}); err != nil {
		t.Fatalf("expected then-branch to not require target: %v", err)
	}
}

func TestValidateConstRejectsOtherValue(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"ack":{"type":"boolean","const":true},"v":{"const":3}}}`)
	if err := Validate(schema, map[string]any{"ack": false}); err == nil {
		t.Fatal("expected ack=false to violate const true")
	}
	if err := Validate(schema, map[string]any{"v": 4.0}); err == nil {
		t.Fatal("expected v=4 to violate const 3")
	}
	if err := Validate(schema, map[string]any{"ack": true, "v": 3.0}); err != nil {
		t.Fatalf("expected const-matching values to pass: %v", err)
	}
}

func TestValidateAllOfUnconditionalMemberMergesLikeRoot(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","allOf":[{"required":["name"],"properties":{"name":{"type":"string"}}}]}`)
	if err := Validate(schema, map[string]any{}); err == nil || err.Error() != `missing required field "name"` {
		t.Fatalf("expected unconditional member to require name, got %v", err)
	}
	if err := Validate(schema, map[string]any{"name": 1.0}); err == nil {
		t.Fatal("expected unconditional member property type to be enforced")
	}
}

func TestValidateNestedObjectNamesFieldPath(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"a":{"type":"object","required":["b"],
		"properties":{"b":{"type":"string"},"c":{"type":"object","properties":{"d":{"type":"integer"}}}}}}}`)
	cases := map[string]map[string]any{
		`missing required field "a.b"`:             {"a": map[string]any{}},
		`field "a.c.d": expected integer, got 1.5`: {"a": map[string]any{"b": "x", "c": map[string]any{"d": 1.5}}},
		`field "a": expected object, got string`:   {"a": "flat"},
	}
	for want, values := range cases {
		if err := Validate(schema, values); err == nil || err.Error() != want {
			t.Fatalf("expected %q for %v, got %v", want, values, err)
		}
	}
	if err := Validate(schema, map[string]any{"a": map[string]any{"b": "x", "c": map[string]any{"d": 2.0}}}); err != nil {
		t.Fatalf("expected a valid nested object to pass: %v", err)
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

func TestValidateAllOfCombinedConstraints(t *testing.T) {
	cases := []struct {
		name, schema string
		values       map[string]any
		want         string
	}{
		{
			name: "a later property schema keeps the earlier enum",
			schema: `{"type":"object","properties":{"mode":{"enum":["manual"],"type":"string"}},
				"allOf":[{"properties":{"mode":{"type":"string"}}}]}`,
			values: map[string]any{"mode": "auto"},
			want:   `field "mode": value "auto" is not one of the allowed options`,
		},
		{
			name: "a root if/then applies",
			schema: `{"type":"object","if":{"properties":{"scale":{"const":true}},"required":["scale"]},
				"then":{"required":["cluster"]}}`,
			values: map[string]any{"scale": true},
			want:   `missing required field "cluster"`,
		},
		{
			name: "a conditional member's own required applies",
			schema: `{"type":"object","allOf":[{"required":["db"],
				"if":{"properties":{"scale":{"const":true}},"required":["scale"]},"then":{"required":["cluster"]}}]}`,
			values: map[string]any{"scale": true, "cluster": "c1"},
			want:   `missing required field "db"`,
		},
		{
			name: "a type-only condition selects then",
			schema: `{"type":"object","allOf":[{"if":{"properties":{"count":{"type":"integer"}},"required":["count"]},
				"then":{"required":["unit"]}}]}`,
			values: map[string]any{"count": float64(1)},
			want:   `missing required field "unit"`,
		},
		{
			name: "an absent optional condition property satisfies if",
			schema: `{"type":"object","allOf":[{"if":{"properties":{"scale":{"const":true}}},
				"then":{"required":["cluster"]},"else":{"required":["target"]}}]}`,
			values: map[string]any{},
			want:   `missing required field "cluster"`,
		},
		{
			name:   "a null allOf member is an invalid schema",
			schema: `{"type":"object","allOf":[null]}`,
			values: map[string]any{},
			want:   `invalid schema: #.allOf[0] is null`,
		},
		{
			name: "an undecidable condition is rejected",
			schema: `{"type":"object","allOf":[{"if":{"properties":{"n":{"minimum":1}}},
				"then":{"required":["unit"]}}]}`,
			values: map[string]any{"n": float64(2)},
			want:   `invalid schema: #.allOf[0].if: property "n": condition states no const, enum, type, not, properties or required`,
		},
	}
	for _, tc := range cases {
		err := Validate(json.RawMessage(tc.schema), tc.values)
		if err == nil || err.Error() != tc.want {
			t.Errorf("%s: expected %q, got %v", tc.name, tc.want, err)
		}
	}
}
