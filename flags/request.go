package flags

import (
	"encoding/csv"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/flanksource/commons/duration"
)

// PopulateFromRequest fills optsValue by parsing flagMap and args directly,
// without touching any shared pflag pointer. This is the entry point used by
// the HTTP data-func dispatcher in cobra_command.go.
//
// The CLI path keeps the existing pflag-backed pipeline (AssignFieldValue);
// only the HTTP path benefits from per-request isolation. Concurrent requests
// allocate their own optsValue, so no locking is required.
//
// Precedence per field: explicit flagMap[FlagName] → args (for the IsArgs
// field) → DefaultValue → type zero value. Stdin is intentionally skipped on
// this path — HTTP requests have no terminal.
func PopulateFromRequest(optsValue reflect.Value, fields []FieldInfo, flagMap map[string]string, args []string) error {
	for _, info := range fields {
		fieldValue := GetFieldByPath(optsValue, info.FieldPath)
		if !fieldValue.IsValid() || !fieldValue.CanSet() {
			return fmt.Errorf("cannot set field %s", info.FieldName)
		}

		raw, hasRaw := pickRawValue(info, flagMap)
		if err := assignFieldFromRequest(fieldValue, info, raw, hasRaw, args); err != nil {
			return fmt.Errorf("field %s: %w", info.FieldName, err)
		}
	}
	return nil
}

// pickRawValue chooses the string input for a field, honouring the precedence
// flagMap → DefaultValue. Args and stdin are handled inside the per-type
// branch because their semantics differ (args may be a slice; stdin is
// off for HTTP).
func pickRawValue(info FieldInfo, flagMap map[string]string) (string, bool) {
	if info.FlagName != "" {
		if v, ok := flagMap[info.FlagName]; ok {
			return v, true
		}
	}
	if info.DefaultValue != "" {
		return info.DefaultValue, true
	}
	return "", false
}

func assignFieldFromRequest(fieldValue reflect.Value, info FieldInfo, raw string, hasRaw bool, args []string) error {
	switch info.FieldType.Kind() {
	case reflect.String:
		val := raw
		if !hasRaw && info.IsArgs && len(args) > 0 {
			val = args[0]
		}
		loaded, err := loadFromFileOrURL(val)
		if err != nil {
			return err
		}
		fieldValue.SetString(loaded)
		return nil

	case reflect.Int:
		if !hasRaw {
			fieldValue.SetInt(0)
			return nil
		}
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("parsing int: %w", err)
		}
		fieldValue.SetInt(int64(n))
		return nil

	case reflect.Bool:
		if !hasRaw {
			fieldValue.SetBool(false)
			return nil
		}
		b, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("parsing bool: %w", err)
		}
		fieldValue.SetBool(b)
		return nil

	case reflect.Slice:
		return assignSliceFromRequest(fieldValue, info, raw, hasRaw, args)

	default:
		switch info.FieldType.String() {
		case "duration.Duration":
			if !hasRaw {
				fieldValue.Set(reflect.ValueOf(duration.Duration(0)))
				return nil
			}
			d, err := duration.ParseDuration(strings.TrimSpace(raw))
			if err != nil {
				return fmt.Errorf("parsing duration: %w", err)
			}
			fieldValue.Set(reflect.ValueOf(d))
			return nil

		case "time.Time":
			if !hasRaw {
				return nil
			}
			t, err := parseTime(raw)
			if err != nil {
				return fmt.Errorf("parsing time: %w", err)
			}
			fieldValue.Set(reflect.ValueOf(t))
			return nil
		}
	}
	return nil
}

func assignSliceFromRequest(fieldValue reflect.Value, info FieldInfo, raw string, hasRaw bool, args []string) error {
	elemKind := info.FieldType.Elem().Kind()

	var tokens []string
	switch {
	case hasRaw:
		parsed, err := readAsCSVRecord(raw)
		if err != nil {
			return fmt.Errorf("parsing CSV: %w", err)
		}
		tokens = parsed
	case info.IsArgs && len(args) > 0:
		tokens = args
	}

	switch elemKind {
	case reflect.String:
		if len(tokens) == 1 {
			lines, err := loadLinesFromFileOrURL(tokens[0])
			if err != nil {
				return err
			}
			tokens = lines
		}
		if tokens == nil {
			fieldValue.Set(reflect.Zero(info.FieldType))
			return nil
		}
		return assignValue(fieldValue, tokens)

	case reflect.Int:
		if tokens == nil {
			fieldValue.Set(reflect.Zero(info.FieldType))
			return nil
		}
		out := make([]int, 0, len(tokens))
		for _, tok := range tokens {
			n, err := strconv.Atoi(strings.TrimSpace(tok))
			if err != nil {
				return fmt.Errorf("parsing int slice element %q: %w", tok, err)
			}
			out = append(out, n)
		}
		return assignValue(fieldValue, out)
	}
	return nil
}

// readAsCSVRecord mirrors pflag.stringSliceValue.Set: parse the raw string as
// one CSV record. An empty string yields nil so the field is left at the zero
// value rather than a one-element [""] slice.
func readAsCSVRecord(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	rec, err := csv.NewReader(strings.NewReader(raw)).Read()
	if err != nil {
		return nil, err
	}
	return rec, nil
}
