package pdf

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/flanksource/commons/logger"
)

// ChromiumConverter implements SVGConverter using Chrome/Chromium headless mode
type ChromiumConverter struct {
	chromePath string
}

// NewChromiumConverter creates a new Chromium converter
func NewChromiumConverter() *ChromiumConverter {
	converter := &ChromiumConverter{}
	converter.chromePath = converter.detectChromePath()
	return converter
}

// Name returns the name of this converter
func (c *ChromiumConverter) Name() string {
	return "chromium"
}

// IsAvailable checks if Chrome/Chromium is available on the system
func (c *ChromiumConverter) IsAvailable() bool {
	return c.chromePath != ""
}

// SupportedFormats returns formats supported by Chrome/Chromium
func (c *ChromiumConverter) SupportedFormats() []string {
	return []string{"pdf"}
}

// Convert converts an SVG file using Chrome/Chromium headless mode
func (c *ChromiumConverter) Convert(ctx context.Context, svgPath, outputPath string, options *ConvertOptions) error {
	if !c.IsAvailable() {
		return NewConverterError(c.Name(), "convert", fmt.Errorf("chrome/chromium not found"))
	}

	if options == nil {
		options = DefaultConvertOptions()
	}

	// Only support PDF format for Chrome
	format := strings.ToLower(options.Format)
	if format != "pdf" {
		return NewConverterError(c.Name(), "convert", fmt.Errorf("unsupported format: %s (chrome only supports PDF)", format))
	}

	logger.Infof("[%s] converting %s to PDF using %s", c.Name(), svgPath, c.chromePath)

	// Build Chrome command arguments
	args := []string{
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--disable-dev-shm-usage",
		"--disable-pdf-tagging",
		"--no-pdf-header-footer",
		"--print-to-pdf=" + outputPath,
	}

	// Add paper size if specified in width/height
	if options.Width > 0 && options.Height > 0 {
		// Convert pixels to inches (assuming 96 DPI)
		widthInch := float64(options.Width) / 96.0
		heightInch := float64(options.Height) / 96.0
		args = append(args, fmt.Sprintf("--print-to-pdf-paper-width=%.2f", widthInch))
		args = append(args, fmt.Sprintf("--print-to-pdf-paper-height=%.2f", heightInch))
	}

	// Add the SVG file path (convert to file:// URL)
	svgURL := "file://" + filepath.Clean(svgPath)
	args = append(args, svgURL)

	cmd := exec.CommandContext(ctx, c.chromePath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return NewConverterError(c.Name(), "convert", fmt.Errorf("chrome command failed: %w, output: %s", err, string(output)))
	}

	// Verify the output file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return NewConverterError(c.Name(), "convert", fmt.Errorf("output PDF file was not created"))
	}

	return nil
}

// ConvertToFormat is a convenience method that determines output path based on format
func (c *ChromiumConverter) ConvertToFormat(ctx context.Context, svgPath, format string, options *ConvertOptions) (string, error) {
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

// detectChromePath attempts to find Chrome or Chromium executable on the system
func (c *ChromiumConverter) detectChromePath() string {
	switch runtime.GOOS {
	case "darwin": // macOS
		return c.detectChromeMacOS()
	case "linux":
		return c.detectChromeLinux()
	case "windows":
		return c.detectChromeWindows()
	default:
		return ""
	}
}

// detectChromeMacOS finds Chrome on macOS
func (c *ChromiumConverter) detectChromeMacOS() string {
	paths := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
	}

	for _, path := range paths {
		if c.isExecutable(path) {
			return path
		}
	}

	return ""
}

// detectChromeLinux finds Chrome on Linux
func (c *ChromiumConverter) detectChromeLinux() string {
	// First try common names in PATH
	pathNames := []string{
		"google-chrome-stable",
		"google-chrome",
		"chromium",
		"chromium-browser",
	}

	for _, name := range pathNames {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	// Then try common installation paths
	paths := []string{
		"/usr/bin/google-chrome-stable",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/opt/google/chrome/chrome",
		"/snap/bin/chromium",
	}

	for _, path := range paths {
		if c.isExecutable(path) {
			return path
		}
	}

	return ""
}

// detectChromeWindows finds Chrome on Windows
func (c *ChromiumConverter) detectChromeWindows() string {
	// First try common names in PATH
	pathNames := []string{
		"chrome.exe",
		"chromium.exe",
		"google-chrome.exe",
	}

	for _, name := range pathNames {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	// Then try common installation paths
	paths := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Chromium\Application\chromium.exe`,
		`C:\Program Files (x86)\Chromium\Application\chromium.exe`,
	}

	for _, path := range paths {
		if c.isExecutable(path) {
			return path
		}
	}

	return ""
}

// isExecutable checks if a file exists and is executable
func (c *ChromiumConverter) isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// On Unix-like systems, check execute permission
	if runtime.GOOS != "windows" {
		return info.Mode()&0111 != 0
	}

	// On Windows, just check if file exists (executable check is more complex)
	return !info.IsDir()
}

// GetChromePath returns the detected Chrome path (for testing/debugging)
func (c *ChromiumConverter) GetChromePath() string {
	return c.chromePath
}
