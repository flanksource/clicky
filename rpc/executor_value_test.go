package rpc

import "testing"

// A flag value is a string, so every JSON type has to survive being encoded
// into one and decoded back by the handler. Arrays already did; nested objects
// fell through to Go's `map[a:1]` bracket form, which is not valid JSON and
// leaves the handler nothing it can parse.
func TestConvertValueToStringEncodesNestedObjectsAsJSON(t *testing.T) {
	got := convertValueToString(map[string]interface{}{"tags": []interface{}{"headers"}, "rate": float64(25)})

	// Key order is not guaranteed, so assert on both valid renderings rather
	// than pinning one.
	want := map[string]bool{
		`{"rate":25,"tags":["headers"]}`: true,
		`{"tags":["headers"],"rate":25}`: true,
	}
	if !want[got] {
		t.Errorf("convertValueToString(nested object) = %q, want compact JSON", got)
	}
}

func TestConvertValueToStringKeepsExistingEncodings(t *testing.T) {
	for _, test := range []struct {
		name  string
		value interface{}
		want  string
	}{
		{"string", "plain", "plain"},
		{"whole number", float64(25), "25"},
		{"fractional number", 2.5, "2.5"},
		{"true", true, "true"},
		{"false", false, "false"},
		{"nil", nil, ""},
		{"array", []interface{}{"a", "b"}, "a,b"},
	} {
		if got := convertValueToString(test.value); got != test.want {
			t.Errorf("%s: convertValueToString(%v) = %q, want %q",
				test.name, test.value, got, test.want)
		}
	}
}
