package flags

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/spf13/pflag"
)

type jsonFlagValue struct{ value reflect.Value }

// String omits unset defaults and exposes serialization failures as help text
// because pflag calls it during registration without an error return channel.
func (v *jsonFlagValue) String() string {
	if !v.value.IsValid() || v.value.IsZero() {
		return ""
	}
	data, err := json.Marshal(v.value.Interface())
	if err != nil {
		return fmt.Sprintf("invalid JSON: %v", err)
	}
	return string(data)
}

// Set replaces the value only after strict decoding succeeds.
func (v *jsonFlagValue) Set(raw string) error {
	value, err := parseJSONFlag(v.value.Type(), raw)
	if err == nil {
		v.value = value
	}
	return err
}

// Type distinguishes opaque JSON from object and array flags in generated schemas.
func (v *jsonFlagValue) Type() string {
	t := v.value.Type()
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		if t == reflect.TypeFor[json.RawMessage]() {
			return "json-value"
		}
		return "json-array"
	}
	return "json-object"
}

// parseJSONFlag decodes exactly one value and rejects unknown object fields.
func parseJSONFlag(t reflect.Type, raw string) (reflect.Value, error) {
	value := reflect.New(t)
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(value.Interface()); err != nil {
		return reflect.Value{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return reflect.Value{}, fmt.Errorf("exactly one JSON value is required")
	}
	return value.Elem(), nil
}

// isJSONField selects structured types while preserving native flag bindings.
func isJSONField(t reflect.Type) bool {
	if t == reflect.TypeFor[*bool]() || t.String() == "time.Time" {
		return false
	}
	switch t.Kind() {
	case reflect.Struct, reflect.Map, reflect.Pointer, reflect.Array:
		return true
	case reflect.Slice:
		return t.Elem().Kind() != reflect.String && t.Elem().Kind() != reflect.Int
	}
	return false
}

// bindJSONFlag validates an explicit default before registering the flag.
func bindJSONFlag(fs *pflag.FlagSet, info FieldInfo, fv *FlagValue) {
	fv.JSON = &jsonFlagValue{value: reflect.New(info.FieldType).Elem()}
	if info.DefaultValue != "" {
		if err := fv.JSON.Set(info.DefaultValue); err != nil {
			panic(fmt.Sprintf("flag %s default: %v", info.FlagName, err))
		}
	}
	if info.FlagName != "" {
		fs.VarP(fv.JSON, info.FlagName, info.ShortFlag, info.Help)
	}
}
