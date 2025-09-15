package pdf_test

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/flanksource/clicky/formatters/pdf"
)

func TestChromiumConverter(t *testing.T) {
	converter := NewChromiumConverter()

	t.Run("Name", func(t *testing.T) {
		if converter.Name() != "chromium" {
			t.Errorf("Expected name 'chromium', got '%s'", converter.Name())
		}
	})

	t.Run("SupportedFormats", func(t *testing.T) {
		formats := converter.SupportedFormats()
		expectedFormats := []string{"pdf"}

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

	t.Run("ChromeDetection", func(t *testing.T) {
		// Test that the converter can detect its availability
		isAvailable := converter.IsAvailable()
		chromePath := converter.GetChromePath()

		if isAvailable && chromePath == "" {
			t.Error("Converter reports as available but has empty chrome path")
		}

		if !isAvailable && chromePath != "" {
			t.Error("Converter reports as not available but has a chrome path")
		}

		t.Logf("Chrome available: %v, path: %s", isAvailable, chromePath)
	})

	// Only run actual conversion tests if Chrome is available
	if !converter.IsAvailable() {
		t.Skip("Chrome/Chromium not available on this system")
		return
	}

	t.Run("ConvertSVGToPDF", func(t *testing.T) {
		// Create a simple test SVG
		svgContent := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="200" height="100" viewBox="0 0 200 100">
  <rect x="10" y="10" width="80" height="30" fill="blue"/>
  <text x="100" y="35" font-family="Arial, sans-serif" font-size="16" fill="red">Test SVG</text>
</svg>`

		// Create temporary SVG file
		svgFile, err := os.CreateTemp("", "test_*.svg")
		if err != nil {
			t.Fatalf("Failed to create temp SVG file: %v", err)
		}
		defer os.Remove(svgFile.Name())

		_, err = svgFile.WriteString(svgContent)
		if err != nil {
			t.Fatalf("Failed to write SVG content: %v", err)
		}
		svgFile.Close()

		// Create temporary output file
		pdfFile, err := os.CreateTemp("", "test_output_*.pdf")
		if err != nil {
			t.Fatalf("Failed to create temp PDF file: %v", err)
		}
		defer os.Remove(pdfFile.Name())
		pdfFile.Close()

		// Convert SVG to PDF
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		options := &ConvertOptions{
			Format: "pdf",
			Width:  200,
			Height: 100,
		}

		err = converter.Convert(ctx, svgFile.Name(), pdfFile.Name(), options)
		if err != nil {
			t.Fatalf("Failed to convert SVG to PDF: %v", err)
		}

		// Verify PDF file was created and has content
		info, err := os.Stat(pdfFile.Name())
		if err != nil {
			t.Fatalf("Output PDF file does not exist: %v", err)
		}

		if info.Size() < 100 { // PDF should be at least 100 bytes
			t.Errorf("PDF file seems too small: %d bytes", info.Size())
		}

		t.Logf("Successfully created PDF: %s (%d bytes)", pdfFile.Name(), info.Size())
	})

	t.Run("UnsupportedFormat", func(t *testing.T) {
		svgFile, err := os.CreateTemp("", "test_*.svg")
		if err != nil {
			t.Fatalf("Failed to create temp SVG file: %v", err)
		}
		defer os.Remove(svgFile.Name())
		svgFile.Close()

		pngFile, err := os.CreateTemp("", "test_output_*.png")
		if err != nil {
			t.Fatalf("Failed to create temp PNG file: %v", err)
		}
		defer os.Remove(pngFile.Name())
		pngFile.Close()

		ctx := context.Background()
		options := &ConvertOptions{Format: "png"}

		err = converter.Convert(ctx, svgFile.Name(), pngFile.Name(), options)
		if err == nil {
			t.Error("Expected error for unsupported PNG format, got none")
		}

		if !strings.Contains(err.Error(), "unsupported format") {
			t.Errorf("Expected 'unsupported format' error, got: %v", err)
		}
	})
}

func TestChromiumConverter_PlatformSpecific(t *testing.T) {
	converter := NewChromiumConverter()
	chromePath := converter.GetChromePath()

	if !converter.IsAvailable() {
		t.Skip("Chrome/Chromium not available on this system")
		return
	}

	t.Run("ChromePathValidation", func(t *testing.T) {
		switch runtime.GOOS {
		case "darwin":
			if !strings.Contains(chromePath, "Applications") {
				t.Logf("Warning: Chrome path doesn't contain 'Applications': %s", chromePath)
			}
		case "linux":
			// On Linux, Chrome can be in various locations
			if chromePath == "" {
				t.Error("Chrome path should not be empty on Linux when available")
			}
		case "windows":
			if !strings.Contains(chromePath, "chrome.exe") && !strings.Contains(chromePath, "chromium.exe") {
				t.Logf("Warning: Chrome path doesn't contain expected exe name: %s", chromePath)
			}
		}

		t.Logf("Platform: %s, Chrome path: %s", runtime.GOOS, chromePath)
	})
}

func TestChromiumConverter_ManagerIntegration(t *testing.T) {
	// Test that ChromiumConverter is properly integrated into the manager
	converters := GetAvailableConverters()
	t.Logf("Available converters: %v", converters)

	// Check if chromium is in the list when available
	converter := NewChromiumConverter()
	if converter.IsAvailable() {
		found := false
		for _, name := range converters {
			if name == "chromium" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Chromium converter should be in available converters list when Chrome is available")
		}
	}

	// Test getting converter by name
	if converter.IsAvailable() {
		retrievedConverter, err := GetConverter("chromium")
		if err != nil {
			t.Errorf("Failed to get chromium converter: %v", err)
		}
		if retrievedConverter.Name() != "chromium" {
			t.Errorf("Expected converter name 'chromium', got '%s'", retrievedConverter.Name())
		}
	}
}
