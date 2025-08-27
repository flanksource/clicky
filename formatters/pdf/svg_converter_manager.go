package pdf

import (
	"context"
	"fmt"
	"sync"
)

// SVGConverterManager manages multiple SVG converters with fallback support
type SVGConverterManager struct {
	converters []SVGConverter
	preferred  string
	mu         sync.RWMutex
}

// NewSVGConverterManager creates a new converter manager
func NewSVGConverterManager() *SVGConverterManager {
	manager := &SVGConverterManager{
		converters: []SVGConverter{},
	}

	// Auto-detect and register available converters in priority order
	manager.autoDetectConverters()

	return manager
}

// autoDetectConverters discovers available converters on the system
func (m *SVGConverterManager) autoDetectConverters() {
	// Try converters in order of preference: Inkscape -> RSVG -> Playwright
	converters := []SVGConverter{
		NewInkscapeConverter(),
		NewRSVGConverter(),
		NewPlaywrightConverter(),
	}

	for _, converter := range converters {
		if converter.IsAvailable() {
			m.converters = append(m.converters, converter)
		}
	}
}

// SetPreferred sets the preferred converter by name
func (m *SVGConverterManager) SetPreferred(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if the preferred converter is available
	for _, converter := range m.converters {
		if converter.Name() == name {
			m.preferred = name
			return nil
		}
	}

	return fmt.Errorf("converter '%s' not available", name)
}

// GetPreferred returns the preferred converter name
func (m *SVGConverterManager) GetPreferred() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.preferred
}

// GetAvailableConverters returns a list of available converter names
func (m *SVGConverterManager) GetAvailableConverters() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, len(m.converters))
	for i, converter := range m.converters {
		names[i] = converter.Name()
	}
	return names
}

// GetConverter returns a converter by name
func (m *SVGConverterManager) GetConverter(name string) (SVGConverter, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, converter := range m.converters {
		if converter.Name() == name {
			return converter, nil
		}
	}

	return nil, fmt.Errorf("converter '%s' not found", name)
}

// GetBestConverter returns the best available converter for the given format
func (m *SVGConverterManager) GetBestConverter(format string) (SVGConverter, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.converters) == 0 {
		return nil, fmt.Errorf("no SVG converters available")
	}

	// If preferred converter is set and supports the format, use it
	if m.preferred != "" {
		for _, converter := range m.converters {
			if converter.Name() == m.preferred && m.supportsFormat(converter, format) {
				return converter, nil
			}
		}
	}

	// Find first converter that supports the format
	for _, converter := range m.converters {
		if m.supportsFormat(converter, format) {
			return converter, nil
		}
	}

	return nil, fmt.Errorf("no converter supports format '%s'", format)
}

// supportsFormat checks if a converter supports the given format
func (m *SVGConverterManager) supportsFormat(converter SVGConverter, format string) bool {
	supportedFormats := converter.SupportedFormats()
	for _, supportedFormat := range supportedFormats {
		if supportedFormat == format {
			return true
		}
	}
	return false
}

// Convert converts an SVG file using the best available converter
func (m *SVGConverterManager) Convert(ctx context.Context, svgPath, outputPath string, options *ConvertOptions) error {
	if options == nil {
		options = DefaultConvertOptions()
	}

	converter, err := m.GetBestConverter(options.Format)
	if err != nil {
		return fmt.Errorf("failed to get converter: %w", err)
	}

	return converter.Convert(ctx, svgPath, outputPath, options)
}

// ConvertWithFallback attempts conversion with fallback to other converters
func (m *SVGConverterManager) ConvertWithFallback(ctx context.Context, svgPath, outputPath string, options *ConvertOptions) error {
	m.mu.RLock()
	converters := make([]SVGConverter, len(m.converters))
	copy(converters, m.converters)
	m.mu.RUnlock()

	if options == nil {
		options = DefaultConvertOptions()
	}

	var lastErr error

	// Try converters in order, filtering by format support
	for _, converter := range converters {
		if !m.supportsFormat(converter, options.Format) {
			continue
		}

		err := converter.Convert(ctx, svgPath, outputPath, options)
		if err == nil {
			return nil // Success
		}

		lastErr = fmt.Errorf("%s: %w", converter.Name(), err)
	}

	if lastErr == nil {
		return fmt.Errorf("no converter supports format '%s'", options.Format)
	}

	return fmt.Errorf("all converters failed, last error: %w", lastErr)
}

// RefreshConverters re-detects available converters
func (m *SVGConverterManager) RefreshConverters() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.converters = []SVGConverter{}
	m.autoDetectConverters()

	// Reset preferred if it's no longer available
	if m.preferred != "" {
		found := false
		for _, converter := range m.converters {
			if converter.Name() == m.preferred {
				found = true
				break
			}
		}
		if !found {
			m.preferred = ""
		}
	}
}

// GetSupportedFormats returns all formats supported by at least one converter
func (m *SVGConverterManager) GetSupportedFormats() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	formatSet := make(map[string]bool)
	for _, converter := range m.converters {
		for _, format := range converter.SupportedFormats() {
			formatSet[format] = true
		}
	}

	formats := make([]string, 0, len(formatSet))
	for format := range formatSet {
		formats = append(formats, format)
	}

	return formats
}

// Close closes any converters that need cleanup (like Playwright)
func (m *SVGConverterManager) Close() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, converter := range m.converters {
		if closer, ok := converter.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}
