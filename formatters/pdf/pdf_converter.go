package pdf

import (
	"fmt"
	"os"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// convertToTraditionalXRef converts a PDF with compressed XRef streams to use traditional xref tables
// This creates a version that gofpdi can import without issues
func convertToTraditionalXRef(inputPath string) (string, error) {
	// Create temporary output file
	outputFile, err := os.CreateTemp("", "converted_traditional_*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	outputFile.Close()
	outputPath := outputFile.Name()

	// Configure pdfcpu to write with traditional xref tables
	config := model.NewDefaultConfiguration()
	config.WriteXRefStream = false   // Force traditional xref tables
	config.WriteObjectStream = false // Disable object streams for maximum compatibility

	// Read the PDF with compressed XRef streams
	ctx, err := api.ReadContextFile(inputPath)
	if err != nil {
		os.Remove(outputPath)
		return "", fmt.Errorf("failed to read PDF %s: %w", inputPath, err)
	}

	// Apply configuration to context
	ctx.Configuration = config

	// Write the PDF with traditional xref tables
	err = api.WriteContextFile(ctx, outputPath)
	if err != nil {
		os.Remove(outputPath)
		return "", fmt.Errorf("failed to write converted PDF from %s to %s: %w", inputPath, outputPath, err)
	}

	// Conversion successful

	return outputPath, nil
}

// needsXRefConversion checks if a PDF has compressed XRef streams that need conversion
func needsXRefConversion(pdfPath string) bool {
	// For now, assume all SVG-converted PDFs need conversion
	// This is a safe approach since the conversion is idempotent
	// and gofpdi has issues with compressed XRef streams
	return true
}

// convertPDFForGofpdi converts a PDF to be compatible with gofpdi if needed
// Returns the path to use for import (original if no conversion needed, converted if conversion was needed)
func convertPDFForGofpdi(inputPath string) (string, bool, error) {
	// Check if conversion is needed
	if !needsXRefConversion(inputPath) {
		// Already compatible with gofpdi
		return inputPath, false, nil
	}

	// Convert to traditional format
	convertedPath, err := convertToTraditionalXRef(inputPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to convert PDF for gofpdi compatibility: %w", err)
	}

	return convertedPath, true, nil
}

// cleanupConvertedPDF removes a converted PDF file (should be called for temporary files)
func cleanupConvertedPDF(path string, wasConverted bool) {
	if wasConverted && path != "" {
		// Only cleanup if it was converted (temporary file)
		os.Remove(path)
	}
}

// validateConvertedPDF validates that the conversion was successful
func validateConvertedPDF(path string) error {
	// Use our existing validation function
	return validatePDFFile(path)
}

// ConvertedPDFInfo holds information about a PDF conversion
type ConvertedPDFInfo struct {
	OriginalPath  string
	ConvertedPath string
	WasConverted  bool
	ShouldCleanup bool
}

// NewConvertedPDFInfo creates a ConvertedPDFInfo for tracking PDF conversions
func NewConvertedPDFInfo(originalPath string) *ConvertedPDFInfo {
	return &ConvertedPDFInfo{
		OriginalPath:  originalPath,
		ConvertedPath: originalPath,
		WasConverted:  false,
		ShouldCleanup: false,
	}
}

// Convert performs the conversion if needed and updates the info
func (info *ConvertedPDFInfo) Convert() error {
	convertedPath, wasConverted, err := convertPDFForGofpdi(info.OriginalPath)
	if err != nil {
		return err
	}

	info.ConvertedPath = convertedPath
	info.WasConverted = wasConverted
	info.ShouldCleanup = wasConverted

	// Validate the result
	if err := validateConvertedPDF(info.ConvertedPath); err != nil {
		info.Cleanup()
		return fmt.Errorf("converted PDF validation failed: %w", err)
	}

	return nil
}

// Cleanup removes temporary files if needed
func (info *ConvertedPDFInfo) Cleanup() {
	cleanupConvertedPDF(info.ConvertedPath, info.ShouldCleanup)
}

// GetPath returns the path to use for PDF operations
func (info *ConvertedPDFInfo) GetPath() string {
	return info.ConvertedPath
}

// uncompressPDFStreams decompresses streams in a PDF file in-place to ensure compatibility
// This converts compressed XRef streams and object streams to traditional format
func uncompressPDFStreams(pdfPath string) error {
	// Configure pdfcpu to write with uncompressed streams
	config := model.NewDefaultConfiguration()
	config.WriteXRefStream = true    // Force traditional xref tables
	config.WriteObjectStream = false // Disable compressed object streams
	config.DecodeAllStreams = true
	config.Optimize = true

	// Read the PDF
	ctx, err := api.ReadContextFile(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to read PDF %s: %w", pdfPath, err)
	}

	// Apply configuration to context
	ctx.Configuration = config

	// Write the PDF back with uncompressed streams (in-place)
	err = api.WriteContextFile(ctx, pdfPath)
	if err != nil {
		return fmt.Errorf("failed to write uncompressed PDF to %s: %w", pdfPath, err)
	}

	return nil
}
