package pdf

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// InkscapeConverter implements SVGConverter using Inkscape
type InkscapeConverter struct{}

// NewInkscapeConverter creates a new Inkscape converter
func NewInkscapeConverter() *InkscapeConverter {
	return &InkscapeConverter{}
}

// Name returns the name of this converter
func (c *InkscapeConverter) Name() string {
	return "inkscape"
}

// IsAvailable checks if Inkscape is available in PATH
func (c *InkscapeConverter) IsAvailable() bool {
	_, err := exec.LookPath("inkscape")
	return err == nil
}

// SupportedFormats returns formats supported by Inkscape
func (c *InkscapeConverter) SupportedFormats() []string {
	return []string{"png", "pdf", "eps", "ps", "svg"}
}

// Convert converts an SVG file using Inkscape
func (c *InkscapeConverter) Convert(ctx context.Context, svgPath, outputPath string, options *ConvertOptions) error {
	if !c.IsAvailable() {
		return NewConverterError(c.Name(), "convert", fmt.Errorf("inkscape not found in PATH"))
	}

	if options == nil {
		options = DefaultConvertOptions()
	}

	args := []string{
		svgPath,
		"--export-filename=" + outputPath,
	}

	// Set output format
	format := strings.ToLower(options.Format)
	switch format {
	case "png":
		args = append(args, "--export-type=png")

		if options.Width > 0 {
			args = append(args, "--export-width="+strconv.Itoa(options.Width))
		}
		if options.Height > 0 {
			args = append(args, "--export-height="+strconv.Itoa(options.Height))
		}
		if options.DPI > 0 {
			args = append(args, "--export-dpi="+strconv.Itoa(options.DPI))
		}
		if options.BackgroundColor != "" {
			args = append(args, "--export-background="+options.BackgroundColor)
		}

	case "pdf":
		args = append(args, "--export-type=pdf")

	case "eps":
		args = append(args, "--export-type=eps")

	case "ps":
		args = append(args, "--export-type=ps")

	case "svg":
		args = append(args, "--export-type=svg")

	default:
		return NewConverterError(c.Name(), "convert", fmt.Errorf("unsupported format: %s", format))
	}

	cmd := exec.CommandContext(ctx, "inkscape", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return NewConverterError(c.Name(), "convert", fmt.Errorf("command failed: %w, output: %s", err, string(output)))
	}

	return nil
}

// ConvertToFormat is a convenience method that determines output path based on format
func (c *InkscapeConverter) ConvertToFormat(ctx context.Context, svgPath string, format string, options *ConvertOptions) (string, error) {
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
