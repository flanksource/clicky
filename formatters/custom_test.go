package formatters

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestRegisterFormatter(t *testing.T) {
	// Clean up before test
	ClearCustomFormatters()
	defer ClearCustomFormatters()

	// Register a simple test formatter
	RegisterFormatter("test", func(data interface{}, opts FormatOptions) (string, error) {
		return "TEST: " + fmt.Sprint(data), nil
	})

	// Verify it was registered
	fn, exists := GetCustomFormatter("test")
	if !exists {
		t.Fatal("Expected formatter to be registered")
	}

	// Test the formatter
	result, err := fn("hello", FormatOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "TEST: hello"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestRegisterFormatterCaseInsensitive(t *testing.T) {
	ClearCustomFormatters()
	defer ClearCustomFormatters()

	// Register with uppercase
	RegisterFormatter("UPPER", func(data interface{}, opts FormatOptions) (string, error) {
		return strings.ToUpper(fmt.Sprint(data)), nil
	})

	// Retrieve with lowercase
	fn, exists := GetCustomFormatter("upper")
	if !exists {
		t.Fatal("Expected formatter to be found with lowercase name")
	}

	result, _ := fn("test", FormatOptions{})
	if result != "TEST" {
		t.Errorf("Expected TEST, got %s", result)
	}

	// Retrieve with mixed case
	fn, exists = GetCustomFormatter("UpPeR")
	if !exists {
		t.Fatal("Expected formatter to be found with mixed case")
	}
}

func TestCustomFormatterTakesPrecedence(t *testing.T) {
	ClearCustomFormatters()
	defer ClearCustomFormatters()

	// Register a custom formatter that overrides a built-in format
	RegisterFormatter("json", func(data interface{}, opts FormatOptions) (string, error) {
		return "CUSTOM JSON OUTPUT", nil
	})

	// Use it through FormatManager
	manager := NewFormatManager()
	result, err := manager.FormatWithOptions(FormatOptions{Format: "json"}, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "CUSTOM JSON OUTPUT"
	if result != expected {
		t.Errorf("Expected custom formatter to take precedence. Got %q", result)
	}
}

func TestListCustomFormatters(t *testing.T) {
	ClearCustomFormatters()
	defer ClearCustomFormatters()

	// Register multiple formatters
	formatters := []string{"formatter1", "formatter2", "formatter3"}
	for _, name := range formatters {
		RegisterFormatter(name, func(data interface{}, opts FormatOptions) (string, error) {
			return name, nil
		})
	}

	// List them
	list := ListCustomFormatters()
	if len(list) != len(formatters) {
		t.Fatalf("Expected %d formatters, got %d", len(formatters), len(list))
	}

	// Verify they're sorted
	for i := 0; i < len(list)-1; i++ {
		if list[i] > list[i+1] {
			t.Errorf("List is not sorted: %v", list)
			break
		}
	}
}

func TestUnregisterFormatter(t *testing.T) {
	ClearCustomFormatters()
	defer ClearCustomFormatters()

	// Register a formatter
	RegisterFormatter("test", func(data interface{}, opts FormatOptions) (string, error) {
		return "test", nil
	})

	// Verify it exists
	_, exists := GetCustomFormatter("test")
	if !exists {
		t.Fatal("Expected formatter to exist")
	}

	// Unregister it
	UnregisterFormatter("test")

	// Verify it's gone
	_, exists = GetCustomFormatter("test")
	if exists {
		t.Fatal("Expected formatter to be removed")
	}
}

func TestConcurrentRegistration(t *testing.T) {
	ClearCustomFormatters()
	defer ClearCustomFormatters()

	// Test thread safety by registering many formatters concurrently
	var wg sync.WaitGroup
	count := 100

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("formatter%d", n)
			RegisterFormatter(name, func(data interface{}, opts FormatOptions) (string, error) {
				return name, nil
			})
		}(i)
	}

	wg.Wait()

	// Verify all formatters were registered
	list := ListCustomFormatters()
	if len(list) != count {
		t.Errorf("Expected %d formatters, got %d", count, len(list))
	}
}

func TestFormatterWithError(t *testing.T) {
	ClearCustomFormatters()
	defer ClearCustomFormatters()

	// Register a formatter that returns an error
	RegisterFormatter("error", func(data interface{}, opts FormatOptions) (string, error) {
		return "", fmt.Errorf("intentional error")
	})

	manager := NewFormatManager()
	_, err := manager.FormatWithOptions(FormatOptions{Format: "error"}, "test")
	if err == nil {
		t.Fatal("Expected error from custom formatter")
	}

	if !strings.Contains(err.Error(), "intentional error") {
		t.Errorf("Expected error message to contain 'intentional error', got: %v", err)
	}
}
