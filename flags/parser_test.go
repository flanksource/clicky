package flags

import (
	"reflect"
	"testing"
	"time"

	"github.com/flanksource/commons/duration"
)

// Test structs with various embedding patterns

type BaseOptions struct {
	Name  string `flag:"name" help:"Name field" default:"default"`
	Count int    `flag:"count" help:"Count field" default:"5"`
}

type EmbeddedOptions struct {
	BaseOptions
	Active bool `flag:"active" help:"Active flag" default:"true"`
}

type MultiLevelOptions struct {
	EmbeddedOptions
	Extra string `flag:"extra" help:"Extra field"`
}

type MixedOptions struct {
	Direct string `flag:"direct" help:"Direct field"`
	BaseOptions
	Another int `flag:"another" help:"Another field"`
}

type ComplexEmbedding struct {
	CommonFields
	SpecialFields
	Direct string `flag:"direct" help:"Direct field"`
}

type CommonFields struct {
	ID   string `flag:"id" help:"ID field"`
	Name string `flag:"name" help:"Name field"`
}

type SpecialFields struct {
	Tags     []string          `flag:"tags" help:"Tags field"`
	Duration duration.Duration `flag:"duration" help:"Duration field"`
	Since    time.Time         `flag:"since" help:"Time field"`
}

func TestParseStructFields_Simple(t *testing.T) {
	fields, err := ParseStructFields(reflect.TypeOf(BaseOptions{}))
	if err != nil {
		t.Fatalf("ParseStructFields failed: %v", err)
	}

	if len(fields) != 2 {
		t.Fatalf("Expected 2 fields, got %d", len(fields))
	}

	// Check first field
	if fields[0].FlagName != "name" {
		t.Errorf("Expected flag name 'name', got '%s'", fields[0].FlagName)
	}
	if fields[0].DefaultValue != "default" {
		t.Errorf("Expected default 'default', got '%s'", fields[0].DefaultValue)
	}

	// Check second field
	if fields[1].FlagName != "count" {
		t.Errorf("Expected flag name 'count', got '%s'", fields[1].FlagName)
	}
}

func TestParseStructFields_SingleLevelEmbedding(t *testing.T) {
	fields, err := ParseStructFields(reflect.TypeOf(EmbeddedOptions{}))
	if err != nil {
		t.Fatalf("ParseStructFields failed: %v", err)
	}

	// Should have 3 fields: name, count (from BaseOptions), and active
	if len(fields) != 3 {
		t.Fatalf("Expected 3 fields, got %d", len(fields))
	}

	flagNames := make(map[string]bool)
	for _, f := range fields {
		flagNames[f.FlagName] = true
	}

	expectedFlags := []string{"name", "count", "active"}
	for _, expected := range expectedFlags {
		if !flagNames[expected] {
			t.Errorf("Missing expected flag: %s", expected)
		}
	}
}

func TestParseStructFields_MultiLevelEmbedding(t *testing.T) {
	fields, err := ParseStructFields(reflect.TypeOf(MultiLevelOptions{}))
	if err != nil {
		t.Fatalf("ParseStructFields failed: %v", err)
	}

	// Should have 4 fields: name, count, active, extra
	if len(fields) != 4 {
		t.Fatalf("Expected 4 fields, got %d", len(fields))
	}

	flagNames := make(map[string]bool)
	for _, f := range fields {
		flagNames[f.FlagName] = true
	}

	expectedFlags := []string{"name", "count", "active", "extra"}
	for _, expected := range expectedFlags {
		if !flagNames[expected] {
			t.Errorf("Missing expected flag: %s", expected)
		}
	}
}

func TestParseStructFields_MixedDirectAndEmbedded(t *testing.T) {
	fields, err := ParseStructFields(reflect.TypeOf(MixedOptions{}))
	if err != nil {
		t.Fatalf("ParseStructFields failed: %v", err)
	}

	// Should have 4 fields: direct, name, count, another
	if len(fields) != 4 {
		t.Fatalf("Expected 4 fields, got %d", len(fields))
	}

	flagNames := make(map[string]bool)
	for _, f := range fields {
		flagNames[f.FlagName] = true
	}

	expectedFlags := []string{"direct", "name", "count", "another"}
	for _, expected := range expectedFlags {
		if !flagNames[expected] {
			t.Errorf("Missing expected flag: %s", expected)
		}
	}
}

func TestParseStructFields_MultipleEmbeddings(t *testing.T) {
	fields, err := ParseStructFields(reflect.TypeOf(ComplexEmbedding{}))
	if err != nil {
		t.Fatalf("ParseStructFields failed: %v", err)
	}

	// Should have: id, name (from CommonFields), tags, duration, since (from SpecialFields), direct
	if len(fields) != 6 {
		t.Fatalf("Expected 6 fields, got %d", len(fields))
	}

	flagNames := make(map[string]bool)
	for _, f := range fields {
		flagNames[f.FlagName] = true
	}

	expectedFlags := []string{"id", "name", "tags", "duration", "since", "direct"}
	for _, expected := range expectedFlags {
		if !flagNames[expected] {
			t.Errorf("Missing expected flag: %s", expected)
		}
	}
}

func TestGetFieldByPath_Direct(t *testing.T) {
	opts := BaseOptions{Name: "test", Count: 10}
	v := reflect.ValueOf(&opts).Elem()

	// Get name field
	field := GetFieldByPath(v, []int{0})
	if field.String() != "test" {
		t.Errorf("Expected 'test', got '%s'", field.String())
	}

	// Get count field
	field = GetFieldByPath(v, []int{1})
	if field.Int() != 10 {
		t.Errorf("Expected 10, got %d", field.Int())
	}
}

func TestGetFieldByPath_Embedded(t *testing.T) {
	opts := EmbeddedOptions{
		BaseOptions: BaseOptions{Name: "embedded", Count: 20},
		Active:      true,
	}
	v := reflect.ValueOf(&opts).Elem()

	// BaseOptions is at index 0, Name is at index 0 within BaseOptions
	field := GetFieldByPath(v, []int{0, 0})
	if field.String() != "embedded" {
		t.Errorf("Expected 'embedded', got '%s'", field.String())
	}

	// BaseOptions is at index 0, Count is at index 1 within BaseOptions
	field = GetFieldByPath(v, []int{0, 1})
	if field.Int() != 20 {
		t.Errorf("Expected 20, got %d", field.Int())
	}

	// Active is at index 1
	field = GetFieldByPath(v, []int{1})
	if !field.Bool() {
		t.Error("Expected true for Active field")
	}
}

func TestGetFieldByPath_MultiLevel(t *testing.T) {
	opts := MultiLevelOptions{
		EmbeddedOptions: EmbeddedOptions{
			BaseOptions: BaseOptions{Name: "multilevel", Count: 30},
			Active:      false,
		},
		Extra: "extra-value",
	}
	v := reflect.ValueOf(&opts).Elem()

	// EmbeddedOptions[0] -> BaseOptions[0] -> Name[0]
	field := GetFieldByPath(v, []int{0, 0, 0})
	if field.String() != "multilevel" {
		t.Errorf("Expected 'multilevel', got '%s'", field.String())
	}

	// EmbeddedOptions[0] -> Active[1]
	field = GetFieldByPath(v, []int{0, 1})
	if field.Bool() {
		t.Error("Expected false for Active field")
	}

	// Extra[1]
	field = GetFieldByPath(v, []int{1})
	if field.String() != "extra-value" {
		t.Errorf("Expected 'extra-value', got '%s'", field.String())
	}
}

func TestParseStructFields_FieldPaths(t *testing.T) {
	fields, err := ParseStructFields(reflect.TypeOf(EmbeddedOptions{}))
	if err != nil {
		t.Fatalf("ParseStructFields failed: %v", err)
	}

	// Find the 'name' field which is embedded
	var nameField *FieldInfo
	for i := range fields {
		if fields[i].FlagName == "name" {
			nameField = &fields[i]
			break
		}
	}

	if nameField == nil {
		t.Fatal("Could not find 'name' field")
	}

	// The path should be [0, 0] (BaseOptions is field 0, Name is field 0 within it)
	expectedPath := []int{0, 0}
	if !reflect.DeepEqual(nameField.FieldPath, expectedPath) {
		t.Errorf("Expected path %v, got %v", expectedPath, nameField.FieldPath)
	}
}
