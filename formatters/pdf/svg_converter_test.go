package pdf

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestSVGConverterManager(t *testing.T) {
	manager := NewSVGConverterManager()

	// Test auto-detection
	converters := manager.GetAvailableConverters()
	t.Logf("Available converters: %v", converters)

	if len(converters) == 0 {
		t.Skip("No SVG converters available on this system")
	}

	// Test getting supported formats
	formats := manager.GetSupportedFormats()
	t.Logf("Supported formats: %v", formats)

	if len(formats) == 0 {
		t.Error("Expected at least one supported format")
	}
}

func TestInkscapeConverter(t *testing.T) {
	converter := NewInkscapeConverter()

	t.Run("Name", func(t *testing.T) {
		if converter.Name() != "inkscape" {
			t.Errorf("Expected name 'inkscape', got '%s'", converter.Name())
		}
	})

	t.Run("SupportedFormats", func(t *testing.T) {
		formats := converter.SupportedFormats()
		expectedFormats := []string{"png", "pdf", "eps", "ps", "svg"}

		if len(formats) != len(expectedFormats) {
			t.Errorf("Expected %d formats, got %d", len(expectedFormats), len(formats))
		}

		for _, expected := range expectedFormats {
			found := false
			for _, format := range formats {
				if format == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected format '%s' not found", expected)
			}
		}
	})

	t.Run("IsAvailable", func(t *testing.T) {
		_, err := exec.LookPath("inkscape")
		expectedAvailable := err == nil

		if converter.IsAvailable() != expectedAvailable {
			t.Errorf("IsAvailable() = %v, expected %v", converter.IsAvailable(), expectedAvailable)
		}
	})

	if !converter.IsAvailable() {
		t.Skip("Inkscape not available, skipping conversion tests")
		return
	}

	testConversion(t, converter)
}

func TestRSVGConverter(t *testing.T) {
	converter := NewRSVGConverter()

	t.Run("Name", func(t *testing.T) {
		if converter.Name() != "rsvg-convert" {
			t.Errorf("Expected name 'rsvg-convert', got '%s'", converter.Name())
		}
	})

	t.Run("SupportedFormats", func(t *testing.T) {
		formats := converter.SupportedFormats()
		expectedFormats := []string{"png", "pdf", "ps", "eps", "svg"}

		if len(formats) != len(expectedFormats) {
			t.Errorf("Expected %d formats, got %d", len(expectedFormats), len(formats))
		}
	})

	t.Run("IsAvailable", func(t *testing.T) {
		_, err := exec.LookPath("rsvg-convert")
		expectedAvailable := err == nil

		if converter.IsAvailable() != expectedAvailable {
			t.Errorf("IsAvailable() = %v, expected %v", converter.IsAvailable(), expectedAvailable)
		}
	})

	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available, skipping conversion tests")
		return
	}

	testConversion(t, converter)
}

func TestPlaywrightConverter(t *testing.T) {
	converter := NewPlaywrightConverter()

	t.Run("Name", func(t *testing.T) {
		if converter.Name() != "playwright" {
			t.Errorf("Expected name 'playwright', got '%s'", converter.Name())
		}
	})

	t.Run("SupportedFormats", func(t *testing.T) {
		formats := converter.SupportedFormats()
		expectedFormats := []string{"png", "jpg", "jpeg", "pdf"}

		if len(formats) != len(expectedFormats) {
			t.Errorf("Expected %d formats, got %d", len(expectedFormats), len(formats))
		}
	})

	// Note: IsAvailable() for Playwright initializes the playwright instance
	// which can be slow and may fail in CI environments, so we test it carefully
	t.Run("IsAvailable", func(t *testing.T) {
		// Set a timeout for the availability check
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		done := make(chan bool)
		var available bool

		go func() {
			available = converter.IsAvailable()
			done <- true
		}()

		select {
		case <-done:
			t.Logf("Playwright available: %v", available)
		case <-ctx.Done():
			t.Log("Playwright availability check timed out, assuming not available")
			available = false
		}
	})

	if !converter.IsAvailable() {
		t.Fail()
		return
	}

	defer func() {
		// Clean up Playwright resources
		if err := converter.Close(); err != nil {
			t.Logf("Error closing Playwright converter: %v", err)
		}
	}()

	testConversion(t, converter)
}

// testConversion is a helper function that tests conversion functionality
func testConversion(t *testing.T, converter SVGConverter) {
	t.Helper()

	// Create test SVG
	svgContent := CreateTestSVG()
	svgPath := WriteTestSVG(t, svgContent)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test PNG conversion (all converters should support this)
	t.Run("ConvertToPNG", func(t *testing.T) {
		outputPath := strings.TrimSuffix(svgPath, ".svg") + "_png.png"
		defer os.Remove(outputPath) // Clean up

		options := &ConvertOptions{
			Format: "png",
			Width:  200,
			Height: 200,
			DPI:    96,
		}

		err := converter.Convert(ctx, svgPath, outputPath, options)
		if err != nil {
			t.Fatalf("PNG conversion failed: %v", err)
		}

		AssertFileExists(t, outputPath)
		AssertFileNotEmpty(t, outputPath)
	})

	// Test conversion with different options
	t.Run("ConvertWithOptions", func(t *testing.T) {
		if !supportsFormat(converter, "png") {
			t.Skip("Converter doesn't support PNG")
		}

		outputPath := strings.TrimSuffix(svgPath, ".svg") + "_options.png"
		defer os.Remove(outputPath)

		options := &ConvertOptions{
			Format:          "png",
			Width:           100,
			Height:          100,
			DPI:             72,
			BackgroundColor: "white",
		}

		err := converter.Convert(ctx, svgPath, outputPath, options)
		if err != nil {
			t.Fatalf("Conversion with options failed: %v", err)
		}

		AssertFileExists(t, outputPath)
		AssertFileNotEmpty(t, outputPath)
	})

	// Test ConvertToFormat convenience method if available
	t.Run("ConvertToFormat", func(t *testing.T) {
		switch c := converter.(type) {
		case *InkscapeConverter:
			outputPath, err := c.ConvertToFormat(ctx, svgPath, "png", nil)
			if err != nil {
				t.Fatalf("ConvertToFormat failed: %v", err)
			}
			defer os.Remove(outputPath)
			AssertFileExists(t, outputPath)
			AssertFileNotEmpty(t, outputPath)
		case *RSVGConverter:
			outputPath, err := c.ConvertToFormat(ctx, svgPath, "png", nil)
			if err != nil {
				t.Fatalf("ConvertToFormat failed: %v", err)
			}
			defer os.Remove(outputPath)
			AssertFileExists(t, outputPath)
			AssertFileNotEmpty(t, outputPath)
		case *PlaywrightConverter:
			outputPath, err := c.ConvertToFormat(ctx, svgPath, "png", nil)
			if err != nil {
				t.Fatalf("ConvertToFormat failed: %v", err)
			}
			defer os.Remove(outputPath)
			AssertFileExists(t, outputPath)
			AssertFileNotEmpty(t, outputPath)
		default:
			t.Skip("ConvertToFormat not implemented for this converter")
		}
	})
}

func TestManagerConversion(t *testing.T) {
	manager := NewSVGConverterManager()

	if len(manager.GetAvailableConverters()) == 0 {
		t.Skip("No converters available")
	}

	// Create test SVG
	svgContent := CreateTestSVG()
	svgPath := WriteTestSVG(t, svgContent)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("Convert", func(t *testing.T) {
		outputPath := strings.TrimSuffix(svgPath, ".svg") + "_manager.png"
		defer os.Remove(outputPath)

		options := &ConvertOptions{
			Format: "png",
			Width:  150,
			Height: 150,
		}

		err := manager.Convert(ctx, svgPath, outputPath, options)
		if err != nil {
			t.Fatalf("Manager conversion failed: %v", err)
		}

		AssertFileExists(t, outputPath)
		AssertFileNotEmpty(t, outputPath)
	})

	t.Run("ConvertWithFallback", func(t *testing.T) {
		outputPath := strings.TrimSuffix(svgPath, ".svg") + "_fallback.png"
		defer os.Remove(outputPath)

		options := &ConvertOptions{
			Format: "png",
		}

		err := manager.ConvertWithFallback(ctx, svgPath, outputPath, options)
		if err != nil {
			t.Fatalf("Manager fallback conversion failed: %v", err)
		}

		AssertFileExists(t, outputPath)
		AssertFileNotEmpty(t, outputPath)
	})
}

func TestManagerPreferences(t *testing.T) {
	manager := NewSVGConverterManager()

	availableConverters := manager.GetAvailableConverters()
	if len(availableConverters) == 0 {
		t.Skip("No converters available")
	}

	// Test setting preferred converter
	preferredName := availableConverters[0]
	err := manager.SetPreferred(preferredName)
	if err != nil {
		t.Fatalf("Failed to set preferred converter: %v", err)
	}

	if manager.GetPreferred() != preferredName {
		t.Errorf("Expected preferred converter '%s', got '%s'", preferredName, manager.GetPreferred())
	}

	// Test setting invalid preferred converter
	err = manager.SetPreferred("nonexistent")
	if err == nil {
		t.Error("Expected error when setting nonexistent converter as preferred")
	}
}

func TestConvertOptions(t *testing.T) {
	options := DefaultConvertOptions()

	if options.Format != "png" {
		t.Errorf("Expected default format 'png', got '%s'", options.Format)
	}

	if options.DPI != 96 {
		t.Errorf("Expected default DPI 96, got %d", options.DPI)
	}

	if options.Quality != 90 {
		t.Errorf("Expected default quality 90, got %d", options.Quality)
	}
}

func TestConverterError(t *testing.T) {
	err := NewConverterError("test-converter", "test-operation", os.ErrNotExist)

	expectedMsg := "test-converter converter test-operation failed: file does not exist"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}

	// Test unwrapping
	if unwrappedErr := err.(*ConverterError).Unwrap(); unwrappedErr != os.ErrNotExist {
		t.Errorf("Expected unwrapped error to be os.ErrNotExist, got %v", unwrappedErr)
	}
}

// Helper function to check if a converter supports a format
func supportsFormat(converter SVGConverter, format string) bool {
	supportedFormats := converter.SupportedFormats()
	for _, supportedFormat := range supportedFormats {
		if supportedFormat == format {
			return true
		}
	}
	return false
}
