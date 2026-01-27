//go:build pdf

// Package pdf provides test utilities for validating PDF content and structure.
//
// This package includes utilities for PDF content verification with integration
// for the pdfcpu library. Currently focused on structural validation with
// infrastructure in place for future text extraction capabilities.
//
// Key Features:
// - PDF structure validation using pdfcpu
// - Basic PDF metadata extraction (page count, file size)
// - Infrastructure for future text content verification
// - Comprehensive test helpers for PDF widget validation
package pdf_test

import (
	"os"
	"strings"
	"testing"
)

// SVG Test Utilities

// createTestSVG creates a simple test SVG for testing purposes
func createTestSVG() string {
	return `<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
  <rect x="10" y="10" width="80" height="80" fill="blue" />
  <circle cx="50" cy="50" r="20" fill="red" />
  <text x="50" y="30" text-anchor="middle" fill="white">Test</text>
</svg>`
}

// createComplexTestSVG creates a more complex test SVG with various elements
func createComplexTestSVG() string {
	return `<svg width="200" height="150" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="grad1" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" style="stop-color:rgb(255,255,0);stop-opacity:1" />
      <stop offset="100%" style="stop-color:rgb(255,0,0);stop-opacity:1" />
    </linearGradient>
  </defs>
  <rect x="10" y="10" width="180" height="130" fill="url(#grad1)" stroke="black" stroke-width="2"/>
  <circle cx="50" cy="50" r="30" fill="blue" opacity="0.7" />
  <ellipse cx="150" cy="50" rx="40" ry="25" fill="green" opacity="0.7" />
  <polygon points="100,20 120,60 80,60" fill="purple" />
  <path d="M 50 100 Q 100 50 150 100" stroke="orange" stroke-width="3" fill="none" />
  <text x="100" y="130" text-anchor="middle" font-family="Arial" font-size="14" fill="black">Complex SVG Test</text>
</svg>`
}

// writeTestSVG writes test SVG content to a temporary file and returns the path
func writeTestSVG(t *testing.T, svgContent string) string {
	t.Helper()

	tmpFile := t.TempDir() + "/test.svg"
	err := writeToFile(tmpFile, svgContent)
	if err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	return tmpFile
}

// writeToFile writes content to a file
func writeToFile(path, content string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}

// assertFileExists checks if a file exists at the given path
func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Expected file does not exist: %s", path)
	}
}

// assertFileNotEmpty checks if a file exists and is not empty
func assertFileNotEmpty(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Errorf("Expected file does not exist: %s", path)
		return
	}
	if err != nil {
		t.Errorf("Failed to stat file %s: %v", path, err)
		return
	}

	if info.Size() == 0 {
		t.Errorf("File exists but is empty: %s", path)
	}
}

// Error Detection Functions

// assertPDFDoesNotContainErrors checks that the PDF doesn't contain any error messages
func assertPDFDoesNotContainErrors(t *testing.T, pdfData []byte) {
	t.Helper()

	// Extract text from the PDF
	extractedText, err := ExtractTextFromPDF(pdfData)
	if err != nil {
		t.Errorf("Failed to extract text from PDF: %v", err)
		return
	}

	// List of error patterns to check for
	errorPatterns := []string{
		"could not load image",
		"SVG Rendering Error",
		"Image Placeholder", // This appears when image loading fails
		"failed to",
		"error:",
		"Error:",
	}

	// Check for each error pattern
	for _, pattern := range errorPatterns {
		if strings.Contains(extractedText, pattern) {
			t.Errorf("PDF contains error message: %q", pattern)
			// Show context around the error
			index := strings.Index(extractedText, pattern)
			start := maxInt(0, index-50)
			end := minInt(len(extractedText), index+len(pattern)+50)
			t.Logf("Context: ...%s...", extractedText[start:end])
		}
	}

	t.Logf("✓ PDF contains no error messages")
}

// assertNoSVGRenderingErrors checks specifically for SVG rendering errors
func assertNoSVGRenderingErrors(t *testing.T, pdfData []byte) {
	t.Helper()

	// Extract text from the PDF
	extractedText, err := ExtractTextFromPDF(pdfData)
	if err != nil {
		t.Errorf("Failed to extract text from PDF: %v", err)
		return
	}

	// Check for SVG-related error patterns
	svgErrorPatterns := []string{
		"SVG Rendering Error",
		"SVG conversion failed",
		"failed to convert SVG",
		"Invalid SVG",
		"could not extract SVG",
	}

	for _, pattern := range svgErrorPatterns {
		if strings.Contains(extractedText, pattern) {
			t.Errorf("PDF contains SVG rendering error: %q", pattern)
			// Show context
			index := strings.Index(extractedText, pattern)
			start := maxInt(0, index-30)
			end := minInt(len(extractedText), index+len(pattern)+30)
			t.Logf("Context: ...%s...", extractedText[start:end])
		}
	}
}

// Helper functions

// maxInt returns the larger of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minInt returns the smaller of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
