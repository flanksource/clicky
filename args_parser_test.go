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

func TestArrayFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file with multiple lines
	linesFile := tmpDir + "/policies.txt"
	fileContent := `P001
P002
# This is a comment
P003

P004
`
	if err := os.WriteFile(linesFile, []byte(fileContent), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		args     []string
		expected map[string]any
		wantErr  bool
	}{
		{
			name:     "array from file with []key=@file syntax",
			args:     []string{"[]PolicyNumber=@" + linesFile},
			expected: map[string]any{"PolicyNumber": []string{"P001", "P002", "P003", "P004"}},
			wantErr:  false,
		},
		{
			name:     "array from file with key[]=@file syntax",
			args:     []string{"PolicyNumber[]=@" + linesFile},
			expected: map[string]any{"PolicyNumber": []string{"P001", "P002", "P003", "P004"}},
			wantErr:  false,
		},
		{
			name:    "nonexistent file",
			args:    []string{"[]PolicyNumber=@/nonexistent/file.txt"},
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

func TestArgsType(t *testing.T) {
	t.Run("GetString from string value", func(t *testing.T) {
		args := Args{"name": "john", "city": "boston"}
		if got := args.GetString("name"); got != "john" {
			t.Errorf("GetString() = %v, want john", got)
		}
	})

	t.Run("GetString from string slice returns first", func(t *testing.T) {
		args := Args{"names": []string{"john", "jane"}}
		if got := args.GetString("names"); got != "john" {
			t.Errorf("GetString() = %v, want john", got)
		}
	})

	t.Run("GetString from interface slice", func(t *testing.T) {
		args := Args{"names": []interface{}{"john", "jane"}}
		if got := args.GetString("names"); got != "john" {
			t.Errorf("GetString() = %v, want john", got)
		}
	})

	t.Run("GetString from missing key", func(t *testing.T) {
		args := Args{"name": "john"}
		if got := args.GetString("missing"); got != "" {
			t.Errorf("GetString() = %v, want empty string", got)
		}
	})

	t.Run("GetString from number", func(t *testing.T) {
		args := Args{"age": 30}
		if got := args.GetString("age"); got != "30" {
			t.Errorf("GetString() = %v, want 30", got)
		}
	})

	t.Run("GetStringSlice from string slice", func(t *testing.T) {
		args := Args{"names": []string{"john", "jane", "joe"}}
		got := args.GetStringSlice("names")
		want := []string{"john", "jane", "joe"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetStringSlice() = %v, want %v", got, want)
		}
	})

	t.Run("GetStringSlice from single string", func(t *testing.T) {
		args := Args{"name": "john"}
		got := args.GetStringSlice("name")
		want := []string{"john"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetStringSlice() = %v, want %v", got, want)
		}
	})

	t.Run("GetStringSlice from interface slice", func(t *testing.T) {
		args := Args{"values": []interface{}{"a", "b", "c"}}
		got := args.GetStringSlice("values")
		want := []string{"a", "b", "c"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetStringSlice() = %v, want %v", got, want)
		}
	})

	t.Run("GetStringSlice from missing key", func(t *testing.T) {
		args := Args{"name": "john"}
		got := args.GetStringSlice("missing")
		if len(got) != 0 {
			t.Errorf("GetStringSlice() = %v, want empty slice", got)
		}
	})

	t.Run("JSON marshaling", func(t *testing.T) {
		args := Args{"name": "john", "age": 30, "active": true}
		data, err := args.MarshalJSON()
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var decoded Args
		if err := decoded.UnmarshalJSON(data); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if decoded.GetString("name") != "john" {
			t.Errorf("After unmarshal, name = %v, want john", decoded.GetString("name"))
		}
	})

	t.Run("ParseArguments function", func(t *testing.T) {
		args, err := ParseArguments([]string{"name=john", "age:=30"})
		if err != nil {
			t.Fatalf("ParseArguments() error = %v", err)
		}

		if args.GetString("name") != "john" {
			t.Errorf("name = %v, want john", args.GetString("name"))
		}

		if args["age"] != float64(30) {
			t.Errorf("age = %v, want 30", args["age"])
		}
	})

	t.Run("MustParseArguments panics on error", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("MustParseArguments() should have panicked")
			}
		}()

		MustParseArguments([]string{"invalid:={bad json}"})
	})

	t.Run("MustParseArguments success", func(t *testing.T) {
		args := MustParseArguments([]string{"name=john", "age:=30"})
		if args.GetString("name") != "john" {
			t.Errorf("name = %v, want john", args.GetString("name"))
		}
	})
}
