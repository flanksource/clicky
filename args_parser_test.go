package clicky

import (
	"os"
	"reflect"
	"testing"
)

func TestParseArgumentsAsMap(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]any
		wantErr  bool
	}{
		{
			name:     "string values",
			args:     []string{"name=john", "city=boston"},
			expected: map[string]any{"name": "john", "city": "boston"},
			wantErr:  false,
		},
		{
			name:     "json number",
			args:     []string{"age:=30", "price:=99.99"},
			expected: map[string]any{"age": float64(30), "price": 99.99},
			wantErr:  false,
		},
		{
			name:     "json boolean",
			args:     []string{"active:=true", "verified:=false"},
			expected: map[string]any{"active": true, "verified": false},
			wantErr:  false,
		},
		{
			name:     "json array",
			args:     []string{`tags:=["go","cli","http"]`},
			expected: map[string]any{"tags": []any{"go", "cli", "http"}},
			wantErr:  false,
		},
		{
			name:     "json object",
			args:     []string{`person:={"name":"john","age":30}`},
			expected: map[string]any{"person": map[string]any{"name": "john", "age": float64(30)}},
			wantErr:  false,
		},
		{
			name:     "mixed types",
			args:     []string{"name=john", "age:=30", "active:=true"},
			expected: map[string]any{"name": "john", "age": float64(30), "active": true},
			wantErr:  false,
		},
		{
			name:    "invalid json",
			args:    []string{"invalid:={bad json}"},
			wantErr: true,
		},
		{
			name:    "invalid format",
			args:    []string{"noequals"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseArgumentsAsMap(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgumentsAsMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseArgumentsAsMap() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseArgumentsWithFile(t *testing.T) {
	// Create temporary test files
	tmpDir := t.TempDir()

	// Create text file
	textFile := tmpDir + "/test.txt"
	if err := os.WriteFile(textFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create JSON file
	jsonFile := tmpDir + "/test.json"
	if err := os.WriteFile(jsonFile, []byte(`{"name":"john","age":30}`), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		args     []string
		expected map[string]any
		wantErr  bool
	}{
		{
			name:     "string from file",
			args:     []string{"content@" + textFile},
			expected: map[string]any{"content": "hello world"},
			wantErr:  false,
		},
		{
			name:     "json from file",
			args:     []string{"data:=@" + jsonFile},
			expected: map[string]any{"data": map[string]any{"name": "john", "age": float64(30)}},
			wantErr:  false,
		},
		{
			name:    "nonexistent file",
			args:    []string{"data@/nonexistent/file.txt"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseArgumentsAsMap(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgumentsAsMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseArgumentsAsMap() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseArgumentsWithQuery(t *testing.T) {
	args := []string{
		"name=john",
		"age:=30",
		"filter==active",
		"limit==10",
	}

	data, query, err := ParseArgumentsWithQuery(args)
	if err != nil {
		t.Fatalf("ParseArgumentsWithQuery() error = %v", err)
	}

	expectedData := map[string]any{
		"name": "john",
		"age":  float64(30),
	}
	expectedQuery := map[string]string{
		"filter": "active",
		"limit":  "10",
	}

	if !reflect.DeepEqual(data, expectedData) {
		t.Errorf("data = %v, want %v", data, expectedData)
	}
	if !reflect.DeepEqual(query, expectedQuery) {
		t.Errorf("query = %v, want %v", query, expectedQuery)
	}
}

func TestMustParseArgumentsAsMap(t *testing.T) {
	// Test successful case
	args := []string{"name=john", "age:=30"}
	result := MustParseArgumentsAsMap(args)
	expected := map[string]any{"name": "john", "age": float64(30)}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("MustParseArgumentsAsMap() = %v, want %v", result, expected)
	}

	// Test panic case
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("MustParseArgumentsAsMap() should have panicked")
		}
	}()

	MustParseArgumentsAsMap([]string{"invalid:={bad json}"})
}

func TestParseArgumentsWithHeaders(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedData    map[string]any
		expectedHeaders map[string]string
		wantErr         bool
	}{
		{
			name:            "headers only",
			args:            []string{"User-Agent:MyApp/1.0", "Authorization:Bearer token123"},
			expectedData:    map[string]any{},
			expectedHeaders: map[string]string{"User-Agent": "MyApp/1.0", "Authorization": "Bearer token123"},
			wantErr:         false,
		},
		{
			name:            "mixed headers and data",
			args:            []string{"name=john", "User-Agent:MyApp/1.0", "age:=30"},
			expectedData:    map[string]any{"name": "john", "age": float64(30)},
			expectedHeaders: map[string]string{"User-Agent": "MyApp/1.0"},
			wantErr:         false,
		},
		{
			name:            "data only",
			args:            []string{"name=john", "age:=30"},
			expectedData:    map[string]any{"name": "john", "age": float64(30)},
			expectedHeaders: map[string]string{},
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, headers, err := ParseArgumentsWithHeaders(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgumentsWithHeaders() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(data, tt.expectedData) {
				t.Errorf("ParseArgumentsWithHeaders() data = %v, want %v", data, tt.expectedData)
			}
			if !reflect.DeepEqual(headers, tt.expectedHeaders) {
				t.Errorf("ParseArgumentsWithHeaders() headers = %v, want %v", headers, tt.expectedHeaders)
			}
		})
	}
}

func TestParseArgumentsComplete(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedData    map[string]any
		expectedHeaders map[string]string
		expectedQuery   map[string]string
		wantErr         bool
	}{
		{
			name: "complete example",
			args: []string{
				"name=john",
				"age:=30",
				"User-Agent:MyApp/1.0",
				"filter==active",
				"limit==10",
			},
			expectedData:    map[string]any{"name": "john", "age": float64(30)},
			expectedHeaders: map[string]string{"User-Agent": "MyApp/1.0"},
			expectedQuery:   map[string]string{"filter": "active", "limit": "10"},
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, headers, query, err := ParseArgumentsComplete(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgumentsComplete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(data, tt.expectedData) {
				t.Errorf("ParseArgumentsComplete() data = %v, want %v", data, tt.expectedData)
			}
			if !reflect.DeepEqual(headers, tt.expectedHeaders) {
				t.Errorf("ParseArgumentsComplete() headers = %v, want %v", headers, tt.expectedHeaders)
			}
			if !reflect.DeepEqual(query, tt.expectedQuery) {
				t.Errorf("ParseArgumentsComplete() query = %v, want %v", query, tt.expectedQuery)
			}
		})
	}
}

func TestNestedBracketNotation(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]any
		wantErr  bool
	}{
		{
			name:     "simple nested object",
			args:     []string{"user[name]=john", "user[age]:=30"},
			expected: map[string]any{"user": map[string]any{"name": "john", "age": float64(30)}},
			wantErr:  false,
		},
		{
			name:     "simple array",
			args:     []string{"tags[]=go", "tags[]=cli"},
			expected: map[string]any{"tags": []any{"go", "cli"}},
			wantErr:  false,
		},
		{
			name:     "deep nested object",
			args:     []string{"user[profile][name]=john", "user[profile][location]=NYC"},
			expected: map[string]any{"user": map[string]any{"profile": map[string]any{"name": "john", "location": "NYC"}}},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseArgumentsAsMap(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgumentsAsMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseArgumentsAsMap() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHeadersFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create header file
	headerFile := tmpDir + "/auth.txt"
	if err := os.WriteFile(headerFile, []byte("Bearer token123"), 0644); err != nil {
		t.Fatal(err)
	}

	args := []string{"Authorization:@" + headerFile, "name=john"}
	data, headers, err := ParseArgumentsWithHeaders(args)

	if err != nil {
		t.Fatalf("ParseArgumentsWithHeaders() error = %v", err)
	}

	expectedData := map[string]any{"name": "john"}
	expectedHeaders := map[string]string{"Authorization": "Bearer token123"}

	if !reflect.DeepEqual(data, expectedData) {
		t.Errorf("data = %v, want %v", data, expectedData)
	}
	if !reflect.DeepEqual(headers, expectedHeaders) {
		t.Errorf("headers = %v, want %v", headers, expectedHeaders)
	}
}

func TestBinaryFileHandling(t *testing.T) {
	tmpDir := t.TempDir()

	// Create binary file (simulate with non-UTF8 content)
	binaryFile := tmpDir + "/binary.dat"
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
	if err := os.WriteFile(binaryFile, binaryContent, 0644); err != nil {
		t.Fatal(err)
	}

	args := []string{"data@" + binaryFile}
	result, err := ParseArgumentsAsMap(args)

	if err != nil {
		t.Fatalf("ParseArgumentsAsMap() error = %v", err)
	}

	// Should be base64 encoded
	expectedBase64 := "iVBORw0KGgo="
	if result["data"] != expectedBase64 {
		t.Errorf("binary file content = %v, want %v", result["data"], expectedBase64)
	}
}

func TestEscaping(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]any
		wantErr  bool
	}{
		{
			name:     "escaped equals",
			args:     []string{"key\\==value"},
			expected: map[string]any{"key=": "value"},
			wantErr:  false,
		},
		{
			name:     "escaped colon",
			args:     []string{"key\\:name=value"},
			expected: map[string]any{"key:name": "value"},
			wantErr:  false,
		},
		{
			name:     "escaped at sign",
			args:     []string{"email\\@domain=user@example.com"},
			expected: map[string]any{"email@domain": "user@example.com"},
			wantErr:  false,
		},
		{
			name:     "escaped backslash",
			args:     []string{"path=C\\:\\\\Users"},
			expected: map[string]any{"path": "C:\\Users"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseArgumentsAsMap(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgumentsAsMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseArgumentsAsMap() = %v, want %v", result, tt.expected)
			}
		})
	}
}
