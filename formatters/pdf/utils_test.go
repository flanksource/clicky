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
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	ledongpdf "github.com/ledongthuc/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// ExtractTextFromPDF extracts all text content from a PDF byte array
// using the ledongthuc/pdf library for actual text extraction
func ExtractTextFromPDF(pdfData []byte) (string, error) {
	// Create a reader from the PDF data
	reader := bytes.NewReader(pdfData)

	// Parse the PDF
	pdfReader, err := ledongpdf.NewReader(reader, int64(len(pdfData)))
	if err != nil {
		return "", fmt.Errorf("failed to create PDF reader: %w", err)
	}

	// Extract text from all pages
	var allText strings.Builder
	numPages := pdfReader.NumPage()

	for pageNum := 1; pageNum <= numPages; pageNum++ {
		page := pdfReader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		// Extract text from the page
		textContent, err := page.GetPlainText(nil)
		if err != nil {
			// Continue with other pages even if one fails
			continue
		}

		if allText.Len() > 0 {
			allText.WriteString("\n")
		}
		allText.WriteString(textContent)
	}

	return allText.String(), nil
}

// ExtractTextFromPage extracts text content from a specific page of a PDF
func ExtractTextFromPage(pdfData []byte, pageNum int) (string, error) {
	// Create a reader from the PDF data
	reader := bytes.NewReader(pdfData)

	// Parse the PDF
	pdfReader, err := ledongpdf.NewReader(reader, int64(len(pdfData)))
	if err != nil {
		return "", fmt.Errorf("failed to create PDF reader: %w", err)
	}

	// Check if page number is valid
	if pageNum < 1 || pageNum > pdfReader.NumPage() {
		return "", fmt.Errorf("page number %d out of range (1-%d)", pageNum, pdfReader.NumPage())
	}

	// Get the specific page
	page := pdfReader.Page(pageNum)
	if page.V.IsNull() {
		return "", fmt.Errorf("page %d is null", pageNum)
	}

	// Extract text from the page
	textContent, err := page.GetPlainText(nil)
	if err != nil {
		return "", fmt.Errorf("failed to extract text from page %d: %w", pageNum, err)
	}

	return textContent, nil
}

// assertPDFTextOrder verifies that text appears in the PDF in the expected order
func assertPDFTextOrder(t *testing.T, pdfData []byte, orderedTexts []string) {
	t.Helper()

	// Extract text from the PDF
	extractedText, err := ExtractTextFromPDF(pdfData)
	if err != nil {
		t.Errorf("Failed to extract text from PDF: %v", err)
		return
	}

	// Check that texts appear in the correct order
	lastIndex := -1
	for _, expected := range orderedTexts {
		index := strings.Index(extractedText[lastIndex+1:], expected)
		if index == -1 {
			t.Errorf("PDF does not contain expected text in order: %q", expected)
			if lastIndex >= 0 {
				t.Logf("Last found text was at position %d", lastIndex)
			}
			break
		}
		lastIndex = lastIndex + 1 + index
	}

	if len(orderedTexts) > 0 {
		t.Logf("✓ PDF contains all %d text segments in correct order", len(orderedTexts))
	}
}

// GetPDFInfo returns basic information about a PDF
func GetPDFInfo(pdfData []byte) (pages, size int, err error) {
	reader := bytes.NewReader(pdfData)
	ctx, err := api.ReadContext(reader, model.NewDefaultConfiguration())
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read PDF: %w", err)
	}

	return ctx.PageCount, len(pdfData), nil
}

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

// assertNoImageLoadErrors checks specifically for image loading errors
func assertNoImageLoadErrors(t *testing.T, pdfData []byte) {
	t.Helper()

	// Extract text from the PDF
	extractedText, err := ExtractTextFromPDF(pdfData)
	if err != nil {
		t.Errorf("Failed to extract text from PDF: %v", err)
		return
	}

	// Check for image-related error patterns
	imageErrorPatterns := []string{
		"could not load image",
		"Image Placeholder",
		"image file not found",
		"failed to download image",
		"failed to convert",
	}

	for _, pattern := range imageErrorPatterns {
		if strings.Contains(strings.ToLower(extractedText), strings.ToLower(pattern)) {
			t.Errorf("PDF contains image loading error: %q", pattern)
			// Show context
			index := strings.Index(strings.ToLower(extractedText), strings.ToLower(pattern))
			start := maxInt(0, index-30)
			end := minInt(len(extractedText), index+len(pattern)+30)
			t.Logf("Context: ...%s...", extractedText[start:end])
		}
	}
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
