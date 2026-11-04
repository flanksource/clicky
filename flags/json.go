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

func (v *jsonFlagValue) String() string {
	data, err := json.Marshal(v.value.Interface())
	if err != nil {
		panic(err)
	}
	return string(data)
}
func (v *jsonFlagValue) Set(raw string) error {
	value, err := parseJSONFlag(v.value.Type(), raw)
	if err == nil {
		v.value = value
	}
	return err
}
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
