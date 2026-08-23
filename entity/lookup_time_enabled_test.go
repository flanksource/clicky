package entity

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/flanksource/clicky/api"
)

// rangeFilter is a typed bound: nothing to enumerate, and the only thing worth
// describing about it is how precise its operands are.
func rangeFilter(key string, timeEnabled *bool) DynamicFilter {
	return DynamicFilter{
		Key: key, Label: key, Type: "from", TimeEnabled: timeEnabled,
		Options: func(context.Context, map[string]string, string, int) (map[string]api.Textable, int, error) {
			return nil, 0, nil
		},
	}
}

// A range control with nothing said about its clock is read as a whole-day
// bound, so a surface whose window is measured in minutes has to be able to say
// otherwise — and a control that has no clock to speak of must stay silent
// rather than assert one either way.
func TestLookupCarriesTheClockAFilterDeclares(t *testing.T) {
	enabled, disabled := true, false

	response := resolveFilters(t, map[string]string{},
		rangeFilter("instant", &enabled),
		rangeFilter("day", &disabled),
		rangeFilter("unspecified", nil),
	)

	for key, want := range map[string]*bool{
		"instant":     &enabled,
		"day":         &disabled,
		"unspecified": nil,
	} {
		got := response.Filters[key].TimeEnabled
		switch {
		case want == nil && got != nil:
			t.Errorf("%s reported timeEnabled=%v, want it unstated", key, *got)
		case want != nil && got == nil:
			t.Errorf("%s left timeEnabled unstated, want %v", key, *want)
		case want != nil && *got != *want:
			t.Errorf("%s timeEnabled=%v, want %v", key, *got, *want)
		}
	}

	// The wire shape is what the browser reads, and an omitted key is what tells
	// it to fall back to the control type rather than to a false it never sent.
	encoded, err := json.Marshal(response.Filters)
	if err != nil {
		t.Fatalf("marshal lookup filters: %v", err)
	}
	var wire map[string]map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("unmarshal lookup filters: %v", err)
	}
	if got := wire["instant"]["timeEnabled"]; got != true {
		t.Errorf("instant serialized timeEnabled=%v, want true", got)
	}
	if got := wire["day"]["timeEnabled"]; got != false {
		t.Errorf("day serialized timeEnabled=%v, want false", got)
	}
	if _, present := wire["unspecified"]["timeEnabled"]; present {
		t.Errorf("unspecified serialized a timeEnabled it never declared: %v", wire["unspecified"])
	}
}
