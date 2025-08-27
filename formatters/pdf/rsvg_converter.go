package pdf

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// RSVGConverter implements SVGConverter using rsvg-convert
type RSVGConverter struct{}

// NewRSVGConverter creates a new RSVG converter
func NewRSVGConverter() *RSVGConverter {
	return &RSVGConverter{}
}

// Name returns the name of this converter
func (c *RSVGConverter) Name() string {
	return "rsvg-convert"
}

// IsAvailable checks if rsvg-convert is available in PATH
func (c *RSVGConverter) IsAvailable() bool {
	_, err := exec.LookPath("rsvg-convert")
	return err == nil
}

// SupportedFormats returns formats supported by rsvg-convert
func (c *RSVGConverter) SupportedFormats() []string {
	return []string{"png", "pdf", "ps", "eps", "svg"}
}

// Convert converts an SVG file using rsvg-convert
func (c *RSVGConverter) Convert(ctx context.Context, svgPath, outputPath string, options *ConvertOptions) error {
	if !c.IsAvailable() {
		return NewConverterError(c.Name(), "convert", fmt.Errorf("rsvg-convert not found in PATH"))
	}

	if options == nil {
		options = DefaultConvertOptions()
	}

	args := []string{}

	// Set output format
	format := strings.ToLower(options.Format)
	switch format {
	case "png":
		args = append(args, "--format=png")

	case "pdf":
		args = append(args, "--format=pdf")

	case "ps":
		args = append(args, "--format=ps")

	case "eps":
		args = append(args, "--format=eps")

	case "svg":
		args = append(args, "--format=svg")

	default:
		return NewConverterError(c.Name(), "convert", fmt.Errorf("unsupported format: %s", format))
	}

	// Set dimensions
	if options.Width > 0 {
		args = append(args, "--width="+strconv.Itoa(options.Width))
	}
	if options.Height > 0 {
		args = append(args, "--height="+strconv.Itoa(options.Height))
	}

	// Set DPI
	if options.DPI > 0 {
		args = append(args, "--dpi-x="+strconv.Itoa(options.DPI))
		args = append(args, "--dpi-y="+strconv.Itoa(options.DPI))
	}

	// Set background color
	if options.BackgroundColor != "" {
		args = append(args, "--background-color="+options.BackgroundColor)
	}

	// Set output file
	args = append(args, "--output="+outputPath)

	// Input file
	args = append(args, svgPath)

	cmd := exec.CommandContext(ctx, "rsvg-convert", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return NewConverterError(c.Name(), "convert", fmt.Errorf("command failed: %w, output: %s", err, string(output)))
	}

	return nil
}

// ConvertToFormat is a convenience method that determines output path based on format
func (c *RSVGConverter) ConvertToFormat(ctx context.Context, svgPath string, format string, options *ConvertOptions) (string, error) {
	if options == nil {
		options = DefaultConvertOptions()
	}
	options.Format = format

	ext := "." + strings.ToLower(format)
	outputPath := strings.TrimSuffix(svgPath, filepath.Ext(svgPath)) + ext

	err := c.Convert(ctx, svgPath, outputPath, options)
	if err != nil {
		return "", err
	}

	return outputPath, nil
}
