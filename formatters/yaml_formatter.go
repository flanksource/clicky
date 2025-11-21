package formatters

import (
	"encoding/json"
	"reflect"

	"gopkg.in/yaml.v3"

	"github.com/flanksource/clicky/api"
)

// YAMLFormatter handles YAML formatting
type YAMLFormatter struct{}

// NewYAMLFormatter creates a new YAML formatter
func NewYAMLFormatter() *YAMLFormatter {
	return &YAMLFormatter{}
}

// Format formats data as YAML
func (f *YAMLFormatter) Format(data interface{}) (string, error) {
	if data == nil {
		return "null", nil
	}
	// Convert to PrettyData and use FormatPrettyData
	prettyData, err := ToPrettyData(data)
	if err != nil || prettyData == nil || prettyData.Original == nil {
		// Fallback to direct YAML serialization
		return "", err
	}
	return f.FormatPrettyData(prettyData)
}

// FormatPrettyData formats PrettyData as YAML using the original data if available
func (f *YAMLFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	return f.FormatValue(data.Original)
}

// FormatValue is a helper to format any value as YAML
func (f *YAMLFormatter) FormatValue(data interface{}) (string, error) {
	if data == nil {
		return "null", nil
	}

	// Check if the data has yaml tags
	if HasYAMLTags(data) {
		// Use yaml.Marshal directly when yaml tags are present
		if b, err := yaml.Marshal(data); err != nil {
			return "", err
		} else {
			return string(b), nil
		}
	}

	// No yaml tags - marshal to JSON first, then convert to YAML
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	var yamlData interface{}
	if err := json.Unmarshal(jsonBytes, &yamlData); err != nil {
		return "", err
	}

	yamlBytes, err := yaml.Marshal(yamlData)
	if err != nil {
		return "", err
	}

	return string(yamlBytes), nil
}

// HasYAMLTags checks if a value or its nested structures contain yaml struct tags
func HasYAMLTags(data interface{}) bool {
	if data == nil {
		return false
	}

	val := reflect.ValueOf(data)
	return hasYAMLTagsValue(val)
}

func hasYAMLTagsValue(val reflect.Value) bool {
	// Handle pointers
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return false
		}
		val = val.Elem()
	}

	// Handle interfaces
	if val.Kind() == reflect.Interface {
		if val.IsNil() {
			return false
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Struct:
		typ := val.Type()
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)

			// Check if this field has a yaml tag
			if _, ok := field.Tag.Lookup("yaml"); ok {
				return true
			}

			// Check nested structs recursively
			if field.Type.Kind() == reflect.Struct {
				fieldVal := val.Field(i)
				if hasYAMLTagsValue(fieldVal) {
					return true
				}
			}
		}
		return false

	case reflect.Slice, reflect.Array:
		// Check the element type for yaml tags
		if val.Len() > 0 {
			return hasYAMLTagsValue(val.Index(0))
		}
		// Check the element type itself
		elemType := val.Type().Elem()
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}
		if elemType.Kind() == reflect.Struct {
			// Create a zero value to check tags
			zeroVal := reflect.New(elemType).Elem()
			return hasYAMLTagsValue(zeroVal)
		}
		return false

	case reflect.Map:
		// Maps don't have yaml tags on their keys/values
		// Check if any map values are structs with yaml tags
		for _, key := range val.MapKeys() {
			mapVal := val.MapIndex(key)
			if hasYAMLTagsValue(mapVal) {
				return true
			}
		}
		return false

	default:
		return false
	}
}
