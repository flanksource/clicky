package clicky

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
	"gopkg.in/yaml.v3"
)

func TestDumpSchemaFlag(t *testing.T) {
	// Create a test schema
	testSchema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "id",
				Type:   "string",
				Style:  "text-blue-600 font-bold",
				Label:  "Order ID",
			},
			{
				Name:   "total",
				Type:   "float",
				Format: "currency",
				Style:  "text-green-600",
			},
			{
				Name:   "status",
				Type:   "string",
				ColorOptions: map[string]string{
					"green":  "completed",
					"yellow": "pending",
					"red":    "failed",
				},
			},
		},
	}

	// Create a temporary schema file
	schemaFile, err := os.CreateTemp("", "test-schema-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp schema file: %v", err)
	}
	defer os.Remove(schemaFile.Name())

	// Write schema to file
	schemaYAML, err := yaml.Marshal(testSchema)
	if err != nil {
		t.Fatalf("Failed to marshal schema: %v", err)
	}
	if _, err := schemaFile.Write(schemaYAML); err != nil {
		t.Fatalf("Failed to write schema file: %v", err)
	}
	schemaFile.Close()

	// Create test data file
	dataFile, err := os.CreateTemp("", "test-data-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp data file: %v", err)
	}
	defer os.Remove(dataFile.Name())

	dataJSON := `{"id":"ORD-001","total":99.99,"status":"completed"}`
	if _, err := dataFile.WriteString(dataJSON); err != nil {
		t.Fatalf("Failed to write data file: %v", err)
	}
	dataFile.Close()

	t.Run("DumpSchema outputs to stderr", func(t *testing.T) {
		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// Create schema formatter
		sf, err := LoadSchemaFromYAML(schemaFile.Name())
		if err != nil {
			t.Fatalf("Failed to load schema: %v", err)
		}

		// Format with DumpSchema enabled
		options := formatters.FormatOptions{
			Format:     "json",
			DumpSchema: true,
		}

		// Capture stdout to discard output
		oldStdout := os.Stdout
		os.Stdout, _ = os.Open(os.DevNull)
		defer func() { os.Stdout = oldStdout }()

		err = sf.FormatFiles([]string{dataFile.Name()}, options)
		if err != nil {
			t.Errorf("FormatFiles failed: %v", err)
		}

		// Restore stderr and read captured output
		w.Close()
		os.Stderr = oldStderr
		var buf bytes.Buffer
		io.Copy(&buf, r)
		stderrOutput := buf.String()

		// Verify stderr contains schema dump
		if !strings.Contains(stderrOutput, "=== Schema Dump ===") {
			t.Errorf("Expected schema dump header in stderr, got: %s", stderrOutput)
		}
		if !strings.Contains(stderrOutput, "fields:") {
			t.Errorf("Expected 'fields:' in schema dump, got: %s", stderrOutput)
		}
		if !strings.Contains(stderrOutput, "name: id") {
			t.Errorf("Expected 'name: id' in schema dump, got: %s", stderrOutput)
		}
		if !strings.Contains(stderrOutput, "format: currency") {
			t.Errorf("Expected 'format: currency' in schema dump, got: %s", stderrOutput)
		}
	})

	t.Run("DumpSchema disabled - no stderr output", func(t *testing.T) {
		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// Create schema formatter
		sf, err := LoadSchemaFromYAML(schemaFile.Name())
		if err != nil {
			t.Fatalf("Failed to load schema: %v", err)
		}

		// Format with DumpSchema disabled
		options := formatters.FormatOptions{
			Format:     "json",
			DumpSchema: false,
		}

		// Capture stdout to discard output
		oldStdout := os.Stdout
		os.Stdout, _ = os.Open(os.DevNull)
		defer func() { os.Stdout = oldStdout }()

		err = sf.FormatFiles([]string{dataFile.Name()}, options)
		if err != nil {
			t.Errorf("FormatFiles failed: %v", err)
		}

		// Restore stderr and read captured output
		w.Close()
		os.Stderr = oldStderr
		var buf bytes.Buffer
		io.Copy(&buf, r)
		stderrOutput := buf.String()

		// Verify stderr does NOT contain schema dump
		if strings.Contains(stderrOutput, "=== Schema Dump ===") {
			t.Errorf("Unexpected schema dump in stderr when DumpSchema is false: %s", stderrOutput)
		}
	})
}