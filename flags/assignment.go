package flags

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
)

const ARGS = "__args__"

// AssignFieldValue assigns a flag value to a struct field using the field path
func AssignFieldValue(structValue reflect.Value, fv *FlagValue, args []string, isStdinAvailable bool) error {
	fieldValue := GetFieldByPath(structValue, fv.FieldPath)

	if !fieldValue.IsValid() || !fieldValue.CanSet() {
		return fmt.Errorf("cannot set field at path %v", fv.FieldPath)
	}

	// Process based on type
	switch fv.FieldType.Kind() {
	case reflect.String:
		val := *fv.StringPtr

		// Priority: args first, then stdin, then flag value
		if val == "" && fv.IsArgs && len(args) > 0 {
			val = args[0]
		} else if val == "" && fv.IsStdin && len(args) > 0 {
			// stdin:"true" can also use args as fallback
			val = args[0]
		} else if val == "" && fv.IsStdin && isStdinAvailable {
			if lines, err := loadFromStdin(); err != nil {
				return err
			} else if len(lines) > 0 {
				val = lines[0]
			}
		}

		if loaded, err := loadFromFileOrURL(val); err != nil {
			return fmt.Errorf("loading %s: %w", fv.FieldName, err)
		} else {
			fieldValue.SetString(loaded)
		}

	case reflect.Int:
		fieldValue.SetInt(int64(*fv.IntPtr))

	case reflect.Bool:
		fieldValue.SetBool(*fv.BoolPtr)

	case reflect.Slice:
		switch fv.FieldType.Elem().Kind() {
		case reflect.String:
			val := *fv.StringSlicePtr

			// Priority: args first, then stdin, then flag value
			if len(val) == 0 && fv.IsArgs && len(args) > 0 {
				val = args
			} else if len(val) == 0 && fv.IsStdin && len(args) > 0 {
				// stdin:"true" can also use args as fallback
				val = args
			} else if len(val) == 0 && fv.IsStdin && isStdinAvailable {
				if val, err := loadFromStdin(); err != nil {
					return err
				} else {
					fieldValue.Set(reflect.ValueOf(val))
					return nil
				}
			}

			if len(val) == 1 {
				if lines, err := loadLinesFromFileOrURL(val[0]); err != nil {
					return err
				} else {
					fieldValue.Set(reflect.ValueOf(lines))
				}
			} else {
				fieldValue.Set(reflect.ValueOf(val))
			}

		case reflect.Int:
			fieldValue.Set(reflect.ValueOf(*fv.IntSlicePtr))
		}

	default:
		// Handle special types
		typeName := fv.FieldType.String()
		switch typeName {
		case "duration.Duration":
			fieldValue.Set(reflect.ValueOf(*fv.DurationPtr))

		case "time.Time":
			fieldValue.Set(reflect.ValueOf(*fv.TimePtr))
		}
	}

	return nil
}

// loadFromStdin reads lines from stdin
func loadFromStdin() ([]string, error) {
	if stat, _ := os.Stdin.Stat(); stat != nil && (stat.Mode()&os.ModeCharDevice) != 0 {
		return nil, nil
	} else if stat != nil {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		var lines []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, line)
			}
		}
		return lines, nil
	}
	return nil, nil
}

// loadFromFileOrURL loads content from a file or URL
func loadFromFileOrURL(path string) (string, error) {
	if !strings.HasPrefix(path, "@") {
		return path, nil
	}
	path = strings.TrimPrefix(path, "@")
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		resp, err := http.Get(path)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// loadLinesFromFileOrURL loads lines from a file or URL
func loadLinesFromFileOrURL(path string) ([]string, error) {
	content, err := loadFromFileOrURL(path)
	if err != nil {
		return nil, err
	}

	return parseLines(content), nil
}

// parseLines parses lines from content, skipping empty lines and comments
func parseLines(content string) []string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines
}
