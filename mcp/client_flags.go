package mcp

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

var reservedToolFlags = map[string]bool{
	"allow-tool": true, "deny-tool": true, "help": true,
	"json": true, "refresh": true, "timeout": true,
}

type toolSchema struct {
	Properties map[string]json.RawMessage `json:"properties"`
	Required   []string                   `json:"required"`
}

type schemaProperty struct {
	Type        any               `json:"type"`
	Description string            `json:"description"`
	Enum        []json.RawMessage `json:"enum"`
	Default     json.RawMessage   `json:"default"`
	AnyOf       []json.RawMessage `json:"anyOf"`
	OneOf       []json.RawMessage `json:"oneOf"`
}

type boundToolFlag struct {
	property   string
	flagName   string
	typeName   string
	hasDefault bool
	enum       []string
	stringVal  *string
	intVal     *int64
	floatVal   *float64
	boolVal    *bool
}

// parseToolSchema validates the object boundary needed for flag synthesis.
func parseToolSchema(raw json.RawMessage) (*toolSchema, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return &toolSchema{Properties: map[string]json.RawMessage{}}, nil
	}
	var schema toolSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("parse tool input schema: %w", err)
	}
	if schema.Properties == nil {
		schema.Properties = map[string]json.RawMessage{}
	}
	return &schema, nil
}

// bindToolFlags translates one cached JSON Schema into typed Cobra flags and
// returns the bindings used to assemble a protocol argument object.
func bindToolFlags(cmd *cobra.Command, raw json.RawMessage) ([]*boundToolFlag, error) {
	schema, err := parseToolSchema(raw)
	if err != nil {
		return nil, err
	}
	required := map[string]bool{}
	for _, property := range schema.Required {
		required[property] = true
	}
	names := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	used := map[string]string{}
	bindings := make([]*boundToolFlag, 0, len(names))
	for _, propertyName := range names {
		flagName := kebabFlagName(propertyName)
		if reservedToolFlags[flagName] {
			flagName = "p-" + flagName
		}
		if prior, exists := used[flagName]; exists {
			return nil, fmt.Errorf("schema properties %q and %q both map to --%s", prior, propertyName, flagName)
		}
		used[flagName] = propertyName

		var property schemaProperty
		if err := json.Unmarshal(schema.Properties[propertyName], &property); err != nil {
			return nil, fmt.Errorf("parse schema property %q: %w", propertyName, err)
		}
		binding, err := bindToolFlag(cmd, propertyName, flagName, property)
		if err != nil {
			return nil, err
		}
		if required[propertyName] && !binding.hasDefault {
			if err := cmd.MarkFlagRequired(flagName); err != nil {
				return nil, err
			}
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

func bindToolFlag(cmd *cobra.Command, propertyName, flagName string, property schemaProperty) (*boundToolFlag, error) {
	typeName := schemaType(property)
	help := property.Description
	enum := enumStrings(property.Enum)
	if len(enum) > 0 {
		help = strings.TrimSpace(help + " (one of: " + strings.Join(enum, ", ") + ")")
	}
	binding := &boundToolFlag{
		property: propertyName, flagName: flagName, typeName: typeName,
		hasDefault: len(property.Default) > 0 && string(property.Default) != "null", enum: enum,
	}

	switch typeName {
	case "integer":
		value, err := rawIntDefault(property.Default)
		if err != nil {
			return nil, fmt.Errorf("invalid default for %q: %w", propertyName, err)
		}
		binding.intVal = cmd.Flags().Int64(flagName, value, help)
	case "number":
		value, err := rawFloatDefault(property.Default)
		if err != nil {
			return nil, fmt.Errorf("invalid default for %q: %w", propertyName, err)
		}
		binding.floatVal = cmd.Flags().Float64(flagName, value, help)
	case "boolean":
		value, err := rawBoolDefault(property.Default)
		if err != nil {
			return nil, fmt.Errorf("invalid default for %q: %w", propertyName, err)
		}
		binding.boolVal = cmd.Flags().Bool(flagName, value, help)
	case "array", "object":
		value := ""
		if binding.hasDefault {
			value = string(property.Default)
		}
		help = strings.TrimSpace(help + " (JSON " + typeName + ")")
		binding.stringVal = cmd.Flags().String(flagName, value, help)
	default:
		value := ""
		if binding.hasDefault {
			if err := json.Unmarshal(property.Default, &value); err != nil {
				return nil, fmt.Errorf("invalid default for %q: %w", propertyName, err)
			}
		}
		binding.stringVal = cmd.Flags().String(flagName, value, help)
	}
	return binding, nil
}

// assembleArguments converts changed flags and schema defaults into the typed
// map sent in CallToolRequest.
func assembleArguments(cmd *cobra.Command, bindings []*boundToolFlag) (map[string]any, error) {
	arguments := map[string]any{}
	for _, binding := range bindings {
		if !cmd.Flags().Changed(binding.flagName) && !binding.hasDefault {
			continue
		}
		var value any
		switch binding.typeName {
		case "integer":
			value = *binding.intVal
		case "number":
			value = *binding.floatVal
		case "boolean":
			value = *binding.boolVal
		case "array", "object":
			if err := json.Unmarshal([]byte(*binding.stringVal), &value); err != nil {
				return nil, fmt.Errorf("--%s must be a JSON %s: %w", binding.flagName, binding.typeName, err)
			}
			if binding.typeName == "array" {
				if _, ok := value.([]any); !ok {
					return nil, fmt.Errorf("--%s must be a JSON array", binding.flagName)
				}
			} else if _, ok := value.(map[string]any); !ok {
				return nil, fmt.Errorf("--%s must be a JSON object", binding.flagName)
			}
		default:
			value = *binding.stringVal
		}
		if len(binding.enum) > 0 && !stringInSlice(fmt.Sprint(value), binding.enum) {
			return nil, fmt.Errorf("--%s must be one of: %s", binding.flagName, strings.Join(binding.enum, ", "))
		}
		arguments[binding.property] = value
	}
	return arguments, nil
}

func schemaType(property schemaProperty) string {
	if len(property.AnyOf) > 0 || len(property.OneOf) > 0 {
		return "string"
	}
	if value, ok := property.Type.(string); ok && value != "" {
		return value
	}
	return "string"
}

func enumStrings(raw []json.RawMessage) []string {
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		var value any
		decoder := json.NewDecoder(strings.NewReader(string(item)))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err == nil {
			values = append(values, fmt.Sprint(value))
		}
	}
	return values
}

func rawIntDefault(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, err
	}
	return strconv.ParseInt(number.String(), 10, 64)
}

func rawFloatDefault(raw json.RawMessage) (float64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	return strconv.ParseFloat(string(raw), 64)
}

func rawBoolDefault(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return false, nil
	}
	return strconv.ParseBool(string(raw))
}

var repeatedHyphens = regexp.MustCompile(`-+`)

func kebabFlagName(value string) string {
	var out strings.Builder
	runes := []rune(value)
	for i, current := range runes {
		if current == '_' || current == ' ' || current == '.' {
			out.WriteRune('-')
			continue
		}
		if unicode.IsUpper(current) && i > 0 {
			previous := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || (unicode.IsUpper(previous) && nextLower) {
				out.WriteRune('-')
			}
		}
		out.WriteRune(unicode.ToLower(current))
	}
	return strings.Trim(repeatedHyphens.ReplaceAllString(out.String(), "-"), "-")
}

func stringInSlice(value string, values []string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
