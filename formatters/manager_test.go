package formatters

import (
	"strings"
	"testing"
)

type TestStruct struct {
	Name   string `json:"name" pretty:"label=Name"`
	Age    int    `json:"age" pretty:"label=Age,format=number"`
	Email  string `json:"email" pretty:"label=Email Address"`
	Hidden string `pretty:"hide"`
}

func TestFormatManager(t *testing.T) {
	manager := NewFormatManager()
	
	testData := TestStruct{
		Name:   "John Doe",
		Age:    30,
		Email:  "john@example.com",
		Hidden: "this should not appear",
	}
	
	t.Run("JSON", func(t *testing.T) {
		result, err := manager.JSON(testData)
		if err != nil {
			t.Fatalf("JSON format failed: %v", err)
		}
		if !strings.Contains(result, "\"name\"") {
			t.Error("JSON output should contain name field")
		}
		if !strings.Contains(result, "\"age\"") {
			t.Error("JSON output should contain age field")
		}
	})
	
	t.Run("YAML", func(t *testing.T) {
		result, err := manager.YAML(testData)
		if err != nil {
			t.Fatalf("YAML format failed: %v", err)
		}
		if !strings.Contains(result, "name:") {
			t.Error("YAML output should contain name field")
		}
		if !strings.Contains(result, "age:") {
			t.Error("YAML output should contain age field")
		}
	})
	
	t.Run("CSV", func(t *testing.T) {
		result, err := manager.CSV([]TestStruct{testData})
		if err != nil {
			t.Fatalf("CSV format failed: %v", err)
		}
		if !strings.Contains(result, "name") {
			t.Error("CSV output should contain name header")
		}
		if !strings.Contains(result, "John Doe") {
			t.Error("CSV output should contain John Doe")
		}
		// Should not contain hidden field
		if strings.Contains(result, "this should not appear") {
			t.Error("CSV output should not contain hidden field value")
		}
	})
	
	t.Run("Markdown", func(t *testing.T) {
		result, err := manager.Markdown([]TestStruct{testData})
		if err != nil {
			t.Fatalf("Markdown format failed: %v", err)
		}
		if !strings.Contains(result, "| name") {
			t.Error("Markdown output should contain name column")
		}
		if !strings.Contains(result, "John Doe") {
			t.Error("Markdown output should contain John Doe")
		}
	})
	
	t.Run("ToPrettyData", func(t *testing.T) {
		prettyData, err := manager.ToPrettyData(testData)
		if err != nil {
			t.Fatalf("ToPrettyData failed: %v", err)
		}
		if prettyData == nil {
			t.Fatal("PrettyData should not be nil")
		}
		if prettyData.Schema == nil {
			t.Fatal("Schema should not be nil")
		}
		if len(prettyData.Schema.Fields) != 3 {
			t.Errorf("Expected 3 fields (excluding hidden), got %d", len(prettyData.Schema.Fields))
		}
	})
	
	t.Run("Format with string", func(t *testing.T) {
		result, err := manager.Format("json", testData)
		if err != nil {
			t.Fatalf("Format(json) failed: %v", err)
		}
		if !strings.Contains(result, "\"name\"") {
			t.Error("Format(json) output should contain name field")
		}
		
		result, err = manager.Format("yaml", testData)
		if err != nil {
			t.Fatalf("Format(yaml) failed: %v", err)
		}
		if !strings.Contains(result, "name:") {
			t.Error("Format(yaml) output should contain name field")
		}
	})
	
	t.Run("Unsupported Format", func(t *testing.T) {
		_, err := manager.Format("unknown", testData)
		if err == nil {
			t.Error("Format should return error for unsupported format")
		}
	})
}

func TestParsePrettyTag(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		tag       string
		wantLabel string
		wantFormat string
	}{
		{
			name:      "empty tag",
			fieldName: "TestField",
			tag:       "",
			wantLabel: "TestField",
			wantFormat: "",
		},
		{
			name:      "label only",
			fieldName: "TestField",
			tag:       "label=Custom Label",
			wantLabel: "Custom Label",
			wantFormat: "",
		},
		{
			name:      "format only",
			fieldName: "TestField",
			tag:       "format=currency",
			wantLabel: "TestField",
			wantFormat: "currency",
		},
		{
			name:      "label and format",
			fieldName: "TestField",
			tag:       "label=Price,format=currency",
			wantLabel: "Price",
			wantFormat: "currency",
		},
		{
			name:      "table format",
			fieldName: "Items",
			tag:       "table",
			wantLabel: "Items",
			wantFormat: "table",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := ParsePrettyTag(tt.fieldName, tt.tag)
			if field.Label != tt.wantLabel {
				t.Errorf("Label = %q, want %q", field.Label, tt.wantLabel)
			}
			if field.Format != tt.wantFormat {
				t.Errorf("Format = %q, want %q", field.Format, tt.wantFormat)
			}
		})
	}
}

func TestPrettifyFieldName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"firstName", "First Name"},
		{"first_name", "First Name"},
		{"first-name", "First Name"},
		{"HTTPRequest", "Httprequest"}, // Not perfect but acceptable
		{"userID", "User Id"},
		{"simple", "Simple"},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := PrettifyFieldName(tt.input)
			if got != tt.want {
				t.Errorf("PrettifyFieldName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}