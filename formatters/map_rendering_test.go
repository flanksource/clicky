package formatters

import (
	"strings"
	"testing"
)

type MapRenderNestedStruct struct {
	ID          int     `json:"id" pretty:"label=ID"`
	Name        string  `json:"name" pretty:"label=Name"`
	Description *string `json:"description,omitempty" pretty:"label=Description,omitempty"`
	Active      bool    `json:"active" pretty:"label=Active"`
}

type MapRenderTestStruct struct {
	StringMap  map[string]string                 `json:"string_map" pretty:"label=String Map"`
	StructMap  map[string]MapRenderNestedStruct  `json:"struct_map" pretty:"label=Struct Map"`
	PointerMap map[string]*MapRenderNestedStruct `json:"pointer_map" pretty:"label=Pointer Map"`
}

func mapRenderStringPtr(s string) *string { return &s }

func createMapRenderTestData() *MapRenderTestStruct {
	return &MapRenderTestStruct{
		StringMap: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		StructMap: map[string]MapRenderNestedStruct{
			"item1": {ID: 101, Name: "Map Item 1", Description: mapRenderStringPtr("First"), Active: true},
		},
		PointerMap: map[string]*MapRenderNestedStruct{
			"ptr1": {ID: 201, Name: "Ptr Item 1", Description: mapRenderStringPtr("Desc"), Active: true},
			"ptr2": nil,
		},
	}
}

func TestMarkdownNoPointerAddresses(t *testing.T) {
	manager := NewFormatManager()
	data := createMapRenderTestData()

	output, err := manager.Markdown(data)
	if err != nil {
		t.Fatalf("Markdown formatting failed: %v", err)
	}

	if strings.Contains(output, "0x") {
		t.Errorf("Markdown output contains pointer addresses (0x...)")
		t.Logf("Output:\n%s", output)
	}
}

func TestMarkdownNoMapPattern(t *testing.T) {
	manager := NewFormatManager()
	data := createMapRenderTestData()

	output, err := manager.Markdown(data)
	if err != nil {
		t.Fatalf("Markdown formatting failed: %v", err)
	}

	if strings.Contains(output, "map[") {
		t.Errorf("Markdown output contains raw map[] pattern")
		// Find and show context
		idx := strings.Index(output, "map[")
		start := idx - 50
		if start < 0 {
			start = 0
		}
		end := idx + 100
		if end > len(output) {
			end = len(output)
		}
		t.Logf("Found 'map[' at position %d", idx)
		t.Logf("Context: ...%s...", output[start:end])
	}
}

func TestHTMLNoPointerAddresses(t *testing.T) {
	manager := NewFormatManager()
	data := createMapRenderTestData()

	output, err := manager.HTML(data)
	if err != nil {
		t.Fatalf("HTML formatting failed: %v", err)
	}

	if strings.Contains(output, "0x") {
		t.Errorf("HTML output contains pointer addresses (0x...)")
		// Find and show context
		idx := strings.Index(output, "0x")
		start := idx - 50
		if start < 0 {
			start = 0
		}
		end := idx + 100
		if end > len(output) {
			end = len(output)
		}
		t.Logf("Found '0x' at position %d", idx)
		t.Logf("Context: ...%s...", output[start:end])
	}
}

func TestHTMLNoMapPattern(t *testing.T) {
	manager := NewFormatManager()
	data := createMapRenderTestData()

	output, err := manager.HTML(data)
	if err != nil {
		t.Fatalf("HTML formatting failed: %v", err)
	}

	if strings.Contains(output, "map[") {
		t.Errorf("HTML output contains raw map[] pattern")
		// Find and show context
		idx := strings.Index(output, "map[")
		start := idx - 50
		if start < 0 {
			start = 0
		}
		end := idx + 100
		if end > len(output) {
			end = len(output)
		}
		t.Logf("Found 'map[' at position %d", idx)
		t.Logf("Context: ...%s...", output[start:end])
	}
}

type SliceOfMapsTestStruct struct {
	ConfigList []map[string]interface{} `json:"config_list" pretty:"label=Configuration List"`
}

func createSliceOfMapsTestData() *SliceOfMapsTestStruct {
	return &SliceOfMapsTestStruct{
		ConfigList: []map[string]interface{}{
			{"name": "config1", "enabled": true, "value": 100},
			{"name": "config2", "enabled": false, "value": 200},
		},
	}
}

func TestSliceOfMapsNoPointerAddresses(t *testing.T) {
	manager := NewFormatManager()
	data := createSliceOfMapsTestData()

	testCases := []struct {
		name   string
		format func(interface{}) (string, error)
	}{
		{"JSON", manager.JSON},
		{"YAML", manager.YAML},
		{"CSV", manager.CSV},
		{"Markdown", manager.Markdown},
		{"Pretty", manager.Pretty},
		{"HTML", manager.HTML},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := tc.format(data)
			if err != nil {
				t.Fatalf("Format %s failed: %v", tc.name, err)
			}

			if strings.Contains(output, "0x") {
				t.Errorf("Format %s contains pointer addresses", tc.name)
				// Find and show context
				idx := strings.Index(output, "0x")
				start := idx - 50
				if start < 0 {
					start = 0
				}
				end := idx + 100
				if end > len(output) {
					end = len(output)
				}
				t.Logf("Found '0x' at position %d", idx)
				t.Logf("Context: ...%s...", output[start:end])
			}

			// Additional check: ensure map data is present
			if !strings.Contains(output, "config1") || !strings.Contains(output, "config2") {
				t.Errorf("Format %s missing expected data (config1, config2)", tc.name)
			}

			t.Logf("%s output:\n%s", tc.name, output)
		})
	}
}
