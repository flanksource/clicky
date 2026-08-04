package rpc

import (
	"reflect"
	"testing"

	"github.com/flanksource/clicky/flags"
)

// A repeatable flag reaches the CLI as a CSV record (pflag's stringSlice
// encoding). A JSON body must produce the same encoding, or the same action
// behaves differently over HTTP than on the command line.
func TestConvertValueToStringEncodesArraysAsCSV(t *testing.T) {
	cases := map[string]struct {
		value any
		want  string
	}{
		"string array":     {[]any{"path=/a", "id=2"}, "path=/a,id=2"},
		"single element":   {[]any{"only=1"}, "only=1"},
		"empty array":      {[]any{}, ""},
		"embedded comma":   {[]any{"a=1,2", "b=3"}, `"a=1,2",b=3`},
		"embedded quote":   {[]any{`a="q"`}, `"a=""q"""`},
		"numeric elements": {[]any{float64(1), float64(2)}, "1,2"},
		"plain string":     {"path=/a", "path=/a"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := convertValueToString(tc.value); got != tc.want {
				t.Errorf("convertValueToString(%v) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

// The encoding is only useful if the flag decoder reverses it, so assert the
// full round trip rather than the encoding alone.
func TestArrayFlagRoundTripsThroughPopulateFromRequest(t *testing.T) {
	type actionFlags struct {
		Select []string `flag:"select"`
	}
	want := []string{"path=/sink/B", "id=2"}

	fields, err := flags.ParseStructFields(reflect.TypeOf(actionFlags{}))
	if err != nil {
		t.Fatalf("ParseStructFields: %v", err)
	}
	var decoded actionFlags
	flagMap := map[string]string{"select": convertValueToString([]any{"path=/sink/B", "id=2"})}
	if err := flags.PopulateFromRequest(reflect.ValueOf(&decoded).Elem(), fields, flagMap, nil); err != nil {
		t.Fatalf("PopulateFromRequest: %v", err)
	}
	if !reflect.DeepEqual(decoded.Select, want) {
		t.Errorf("decoded = %q, want %q", decoded.Select, want)
	}
}
