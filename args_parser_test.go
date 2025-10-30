package clicky

import (
	"os"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("ParseArgumentsAsMap", func() {
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
		tt := tt
		ginkgo.It(tt.name, func() {
			result, err := ParseArgumentsAsMap(tt.args)
			if tt.wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(tt.expected))
			}
		})
	}
})

var _ = ginkgo.Describe("ParseArgumentsWithFile", func() {
	var tmpDir string

	ginkgo.BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "clicky-test-*")
		Expect(err).ToNot(HaveOccurred())

		textFile := tmpDir + "/test.txt"
		err = os.WriteFile(textFile, []byte("hello world"), 0644)
		Expect(err).ToNot(HaveOccurred())

		jsonFile := tmpDir + "/test.json"
		err = os.WriteFile(jsonFile, []byte(`{"name":"john","age":30}`), 0644)
		Expect(err).ToNot(HaveOccurred())
	})

	ginkgo.AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	tests := []struct {
		name     string
		getArgs  func(string) []string
		expected map[string]any
		wantErr  bool
	}{
		{
			name: "string from file",
			getArgs: func(dir string) []string {
				return []string{"content@" + dir + "/test.txt"}
			},
			expected: map[string]any{"content": "hello world"},
			wantErr:  false,
		},
		{
			name: "json from file",
			getArgs: func(dir string) []string {
				return []string{"data:=@" + dir + "/test.json"}
			},
			expected: map[string]any{"data": map[string]any{"name": "john", "age": float64(30)}},
			wantErr:  false,
		},
		{
			name: "nonexistent file",
			getArgs: func(dir string) []string {
				return []string{"data@/nonexistent/file.txt"}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		ginkgo.It(tt.name, func() {
			args := tt.getArgs(tmpDir)
			result, err := ParseArgumentsAsMap(args)
			if tt.wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(tt.expected))
			}
		})
	}
})

var _ = ginkgo.Describe("ParseArgumentsWithQuery", func() {
	ginkgo.It("should separate data and query parameters", func() {
		args := []string{
			"name=john",
			"age:=30",
			"filter==active",
			"limit==10",
		}

		data, query, err := ParseArgumentsWithQuery(args)
		Expect(err).ToNot(HaveOccurred())

		expectedData := map[string]any{
			"name": "john",
			"age":  float64(30),
		}
		expectedQuery := map[string]string{
			"filter": "active",
			"limit":  "10",
		}

		Expect(data).To(Equal(expectedData))
		Expect(query).To(Equal(expectedQuery))
	})
})

var _ = ginkgo.Describe("MustParseArgumentsAsMap", func() {
	ginkgo.Context("when parsing succeeds", func() {
		ginkgo.It("should return parsed map", func() {
			args := []string{"name=john", "age:=30"}
			result := MustParseArgumentsAsMap(args)
			expected := map[string]any{"name": "john", "age": float64(30)}

			Expect(result).To(Equal(expected))
		})
	})

	ginkgo.Context("when parsing fails", func() {
		ginkgo.It("should panic", func() {
			Expect(func() {
				MustParseArgumentsAsMap([]string{"invalid:={bad json}"})
			}).To(Panic())
		})
	})
})

var _ = ginkgo.Describe("ParseArgumentsWithHeaders", func() {
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
		tt := tt
		ginkgo.It(tt.name, func() {
			data, headers, err := ParseArgumentsWithHeaders(tt.args)
			if tt.wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
				Expect(data).To(Equal(tt.expectedData))
				Expect(headers).To(Equal(tt.expectedHeaders))
			}
		})
	}
})

var _ = ginkgo.Describe("ParseArgumentsComplete", func() {
	ginkgo.It("should parse data, headers, and query parameters", func() {
		args := []string{
			"name=john",
			"age:=30",
			"User-Agent:MyApp/1.0",
			"filter==active",
			"limit==10",
		}

		data, headers, query, err := ParseArgumentsComplete(args)
		Expect(err).ToNot(HaveOccurred())

		expectedData := map[string]any{"name": "john", "age": float64(30)}
		expectedHeaders := map[string]string{"User-Agent": "MyApp/1.0"}
		expectedQuery := map[string]string{"filter": "active", "limit": "10"}

		Expect(data).To(Equal(expectedData))
		Expect(headers).To(Equal(expectedHeaders))
		Expect(query).To(Equal(expectedQuery))
	})
})

var _ = ginkgo.Describe("NestedBracketNotation", func() {
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
		tt := tt
		ginkgo.It(tt.name, func() {
			result, err := ParseArgumentsAsMap(tt.args)
			if tt.wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(tt.expected))
			}
		})
	}
})

var _ = ginkgo.Describe("HeadersFromFile", func() {
	ginkgo.It("should read header value from file", func() {
		tmpDir, err := os.MkdirTemp("", "clicky-test-*")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		headerFile := tmpDir + "/auth.txt"
		err = os.WriteFile(headerFile, []byte("Bearer token123"), 0644)
		Expect(err).ToNot(HaveOccurred())

		args := []string{"Authorization:@" + headerFile, "name=john"}
		data, headers, err := ParseArgumentsWithHeaders(args)
		Expect(err).ToNot(HaveOccurred())

		expectedData := map[string]any{"name": "john"}
		expectedHeaders := map[string]string{"Authorization": "Bearer token123"}

		Expect(data).To(Equal(expectedData))
		Expect(headers).To(Equal(expectedHeaders))
	})
})

var _ = ginkgo.Describe("BinaryFileHandling", func() {
	ginkgo.It("should base64 encode binary file content", func() {
		tmpDir, err := os.MkdirTemp("", "clicky-test-*")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		binaryFile := tmpDir + "/binary.dat"
		binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
		err = os.WriteFile(binaryFile, binaryContent, 0644)
		Expect(err).ToNot(HaveOccurred())

		args := []string{"data@" + binaryFile}
		result, err := ParseArgumentsAsMap(args)
		Expect(err).ToNot(HaveOccurred())

		expectedBase64 := "iVBORw0KGgo="
		Expect(result["data"]).To(Equal(expectedBase64))
	})
})

var _ = ginkgo.Describe("Escaping", func() {
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
		tt := tt
		ginkgo.It(tt.name, func() {
			result, err := ParseArgumentsAsMap(tt.args)
			if tt.wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(tt.expected))
			}
		})
	}
})

var _ = ginkgo.Describe("ArrayFromFile", func() {
	var tmpDir string
	var linesFile string

	ginkgo.BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "clicky-test-*")
		Expect(err).ToNot(HaveOccurred())

		linesFile = tmpDir + "/policies.txt"
		fileContent := `P001
P002
# This is a comment
P003

P004
`
		err = os.WriteFile(linesFile, []byte(fileContent), 0644)
		Expect(err).ToNot(HaveOccurred())
	})

	ginkgo.AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	tests := []struct {
		name     string
		getArgs  func(string) []string
		expected map[string]any
		wantErr  bool
	}{
		{
			name: "array from file with []key=@file syntax",
			getArgs: func(file string) []string {
				return []string{"[]PolicyNumber=@" + file}
			},
			expected: map[string]any{"PolicyNumber": []string{"P001", "P002", "P003", "P004"}},
			wantErr:  false,
		},
		{
			name: "array from file with key[]=@file syntax",
			getArgs: func(file string) []string {
				return []string{"PolicyNumber[]=@" + file}
			},
			expected: map[string]any{"PolicyNumber": []string{"P001", "P002", "P003", "P004"}},
			wantErr:  false,
		},
		{
			name: "nonexistent file",
			getArgs: func(file string) []string {
				return []string{"[]PolicyNumber=@/nonexistent/file.txt"}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		ginkgo.It(tt.name, func() {
			args := tt.getArgs(linesFile)
			result, err := ParseArgumentsAsMap(args)
			if tt.wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(tt.expected))
			}
		})
	}
})

var _ = ginkgo.Describe("Args Type", func() {
	ginkgo.Context("GetString", func() {
		ginkgo.It("should get string from string value", func() {
			args := Args{"name": "john", "city": "boston"}
			Expect(args.GetString("name")).To(Equal("john"))
		})

		ginkgo.It("should get first string from string slice", func() {
			args := Args{"names": []string{"john", "jane"}}
			Expect(args.GetString("names")).To(Equal("john"))
		})

		ginkgo.It("should get string from interface slice", func() {
			args := Args{"names": []interface{}{"john", "jane"}}
			Expect(args.GetString("names")).To(Equal("john"))
		})

		ginkgo.It("should return empty string for missing key", func() {
			args := Args{"name": "john"}
			Expect(args.GetString("missing")).To(Equal(""))
		})

		ginkgo.It("should convert number to string", func() {
			args := Args{"age": 30}
			Expect(args.GetString("age")).To(Equal("30"))
		})
	})

	ginkgo.Context("GetStringSlice", func() {
		ginkgo.It("should get string slice from string slice", func() {
			args := Args{"names": []string{"john", "jane", "joe"}}
			Expect(args.GetStringSlice("names")).To(Equal([]string{"john", "jane", "joe"}))
		})

		ginkgo.It("should wrap single string in slice", func() {
			args := Args{"name": "john"}
			Expect(args.GetStringSlice("name")).To(Equal([]string{"john"}))
		})

		ginkgo.It("should convert interface slice to string slice", func() {
			args := Args{"values": []interface{}{"a", "b", "c"}}
			Expect(args.GetStringSlice("values")).To(Equal([]string{"a", "b", "c"}))
		})

		ginkgo.It("should return empty slice for missing key", func() {
			args := Args{"name": "john"}
			Expect(args.GetStringSlice("missing")).To(HaveLen(0))
		})
	})

	ginkgo.Context("JSON marshaling", func() {
		ginkgo.It("should marshal and unmarshal correctly", func() {
			args := Args{"name": "john", "age": 30, "active": true}
			data, err := args.MarshalJSON()
			Expect(err).ToNot(HaveOccurred())

			var decoded Args
			err = decoded.UnmarshalJSON(data)
			Expect(err).ToNot(HaveOccurred())
			Expect(decoded.GetString("name")).To(Equal("john"))
		})
	})

	ginkgo.Context("ParseArguments", func() {
		ginkgo.It("should parse arguments into Args type", func() {
			args, err := ParseArguments([]string{"name=john", "age:=30"})
			Expect(err).ToNot(HaveOccurred())
			Expect(args.GetString("name")).To(Equal("john"))
			Expect(args["age"]).To(Equal(float64(30)))
		})
	})

	ginkgo.Context("MustParseArguments", func() {
		ginkgo.It("should parse successfully", func() {
			args := MustParseArguments([]string{"name=john", "age:=30"})
			Expect(args.GetString("name")).To(Equal("john"))
		})

		ginkgo.It("should panic on error", func() {
			Expect(func() {
				MustParseArguments([]string{"invalid:={bad json}"})
			}).To(Panic())
		})
	})
})
