package pdf

import (
	"context"
	"fmt"
)

// SVGConverter defines the interface for converting SVG files to other formats
type SVGConverter interface {
	// Name returns the name of the converter
	Name() string
	
	// IsAvailable checks if the converter is available on the system
	IsAvailable() bool
	
	// SupportedFormats returns the list of output formats this converter supports
	SupportedFormats() []string
	
	// Convert converts an SVG file to the specified format
	Convert(ctx context.Context, svgPath, outputPath string, options *ConvertOptions) error
}

// ConvertOptions holds options for SVG conversion
type ConvertOptions struct {
	// Output format (png, jpg, jpeg, pdf, eps, ps)
	Format string
	
	// Output width in pixels (0 = auto)
	Width int
	
	// Output height in pixels (0 = auto)
	Height int
	
	// DPI for raster formats (0 = default)
	DPI int
	
	// Background color (transparent if empty)
	BackgroundColor string
	
	// Quality for JPEG (0-100, 0 = default)
	Quality int
}

// DefaultConvertOptions returns default conversion options
func DefaultConvertOptions() *ConvertOptions {
	return &ConvertOptions{
		Format:  "png",
		Width:   0,
		Height:  0,
		DPI:     96,
		Quality: 90,
	}
}

// ConverterType represents the type of SVG converter
type ConverterType string

const (
	ConverterTypeInkscape   ConverterType = "inkscape"
	ConverterTypeRSVG       ConverterType = "rsvg"
	ConverterTypePlaywright ConverterType = "playwright"
)

// ConverterError represents an error from a converter
type ConverterError struct {
	Converter string
	Operation string
	Err       error
}

func (e *ConverterError) Error() string {
	return fmt.Sprintf("%s converter %s failed: %v", e.Converter, e.Operation, e.Err)
}

func (e *ConverterError) Unwrap() error {
	return e.Err
}

// NewConverterError creates a new converter error
func NewConverterError(converter, operation string, err error) error {
	return &ConverterError{
		Converter: converter,
		Operation: operation,
		Err:       err,
	}
}