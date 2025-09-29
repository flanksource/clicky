package pdf

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/flanksource/maroto/v2"
	"github.com/flanksource/maroto/v2/pkg/components/col"
	"github.com/flanksource/maroto/v2/pkg/components/row"
	"github.com/flanksource/maroto/v2/pkg/components/text"
	"github.com/flanksource/maroto/v2/pkg/config"
	"github.com/flanksource/maroto/v2/pkg/consts/fontfamily"
	"github.com/flanksource/maroto/v2/pkg/consts/pagesize"
	"github.com/flanksource/maroto/v2/pkg/core"
	"github.com/flanksource/maroto/v2/pkg/fpdf"
	"github.com/flanksource/maroto/v2/pkg/props"

	"github.com/flanksource/clicky/api"
)

// PageSize represents the page configuration
type PageSize struct {
	api.Rectangle `json:"rectangle,omitempty"`
	Margins       api.Padding `json:"margins,omitempty"`
}

// Widget interface for all PDF widgets
type Widget interface {
	// Draw draws the widget using the builder
	Draw(b *Builder) error
}

// Builder wraps Maroto for PDF generation
type Builder struct {
	maroto           core.Maroto
	config           *PageSize
	style            *StyleConverter
	header           api.Text
	footer           api.Text
	pageNumbers      bool
	debugMode        bool
	converterManager *SVGConverterManager
}

// BuilderOption is a function that configures a Builder
type BuilderOption func(*Builder)

// WithDebug enables debug mode which shows grid lines
func WithDebug(enabled bool) BuilderOption {
	return func(b *Builder) {
		b.debugMode = enabled
	}
}

// WithPageSize sets the page size
func WithPageSize(size pagesize.Type) BuilderOption {
	return func(b *Builder) {
		// This will be applied when creating the Maroto instance
	}
}

// NewBuilder creates a new PDF builder using Maroto
func NewBuilder(opts ...BuilderOption) *Builder {
	b := &Builder{
		style:     NewStyleConverter(),
		debugMode: false,
	}

	// Apply options
	for _, opt := range opts {
		opt(b)
	}

	// Create Maroto configuration with Unicode font support
	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithLeftMargin(5).
		WithRightMargin(5).
		WithTopMargin(5).
		WithBottomMargin(5).
		WithDefaultFont(&props.Font{
			Family: fontfamily.Arial,
			Size:   12.0,
		}).                     // Use Arial for better Unicode support
		WithDebug(b.debugMode). // Enable debug mode if requested
		Build()

	// Create Maroto instance
	m := maroto.New(cfg)

	b.maroto = m
	b.config = &PageSize{
		Rectangle: api.Rectangle{Width: 210, Height: 297}, // A4 in mm
		Margins:   api.Padding{Top: 5, Right: 5, Bottom: 5, Left: 5},
	}
	b.pageNumbers = false

	return b
}

func (b *Builder) SaveTo(path string) error {
	if b.maroto == nil {
		return fmt.Errorf("builder not initialized: maroto instance is nil (use NewBuilder() to create a proper builder)")
	}

	data, err := b.Output()
	if err != nil {
		return err
	}

	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// SetHeader sets the header text for all pages
func (b *Builder) SetHeader(header api.Text) {
	b.header = header
	if !header.IsEmpty() {
		b.registerHeader()
	}
}

// SetFooter sets the footer text for all pages
func (b *Builder) SetFooter(footer api.Text) {
	b.footer = footer
	if !footer.IsEmpty() {
		b.registerFooter()
	}
}

// EnablePageNumbers enables page numbering
func (b *Builder) EnablePageNumbers() {
	b.pageNumbers = true
}

// registerHeader registers the header with Maroto
func (b *Builder) registerHeader() {
	if b.header.IsEmpty() {
		return
	}

	headerRow := b.createTextRow(b.header, 10)
	if err := b.maroto.RegisterHeader(headerRow); err != nil {
		// Log error but don't fail, just continue without header
		fmt.Printf("Warning: failed to register header: %v\n", err)
	}
}

// registerFooter registers the footer with Maroto
func (b *Builder) registerFooter() {
	if b.footer.IsEmpty() {
		return
	}

	footerRow := b.createTextRow(b.footer, 8)
	if err := b.maroto.RegisterFooter(footerRow); err != nil {
		// Log error but don't fail, just continue without footer
		fmt.Printf("Warning: failed to register footer: %v\n", err)
	}
}

// createTextRow creates a Maroto row with text
func (b *Builder) createTextRow(t api.Text, height float64) core.Row {
	textProps := b.style.ConvertToTextProps(t.Class)

	// Create text component
	textCol := col.New(12).Add(
		text.New(t.Content, *textProps),
	)

	return row.New(height).Add(textCol)
}

// AddPage is not needed with Maroto as it handles pages automatically
func (b *Builder) AddPage() {
	// Maroto handles pages automatically
	// This method is kept for API compatibility
}

// Write writes text to the PDF
func (b *Builder) Write(text api.Text) *Builder {
	b.AddText(text)
	return b
}

// AddText adds text to the PDF
func (b *Builder) AddText(t api.Text) {
	// Calculate height based on font size
	height := b.style.CalculateTextHeight(t.Class)

	// Add main text
	if t.Content != "" {
		textRow := b.createTextRow(t, height)
		b.maroto.AddRows(textRow)
	}

	// Add children
	for _, child := range t.Children {
		// Only add if child is a Text type (PDF builder doesn't support other Textable types yet)
		if textChild, ok := child.(api.Text); ok {
			b.AddText(textChild)
		}
		// Other Textable types like icons are not yet supported in PDF
	}
}

// AddRow adds a custom row to the PDF
func (b *Builder) AddRow(height float64, columns ...core.Col) {
	r := row.New(height)
	for _, col := range columns {
		r.Add(col)
	}
	b.maroto.AddRows(r)
}

// AddRows adds multiple rows to the PDF
func (b *Builder) AddRows(rows ...core.Row) {
	b.maroto.AddRows(rows...)
}

// DrawWidget draws a widget
func (b *Builder) DrawWidget(widget Widget) error {
	return widget.Draw(b)
}

// MoveBy adds vertical spacing
func (b *Builder) MoveBy(_, dy int) *Builder {
	if dy > 0 {
		// Add empty row for vertical spacing
		b.maroto.AddRows(row.New(float64(dy)))
	}
	// Horizontal movement is handled by column positioning in Maroto
	return b
}

// MoveTo is not directly supported in Maroto's grid system
func (b *Builder) MoveTo(pos api.Position) *Builder {
	// Maroto uses a grid system, not absolute positioning
	// This is kept for API compatibility but has limited effect
	return b
}

// GetMaroto returns the underlying Maroto instance for direct access
func (b *Builder) GetMaroto() core.Maroto {
	return b.maroto
}

// GetStyleConverter returns the style converter
func (b *Builder) GetStyleConverter() *StyleConverter {
	return b.style
}

// GetConverterManager returns the SVG converter manager, creating it if necessary
func (b *Builder) GetConverterManager() *SVGConverterManager {
	if b.converterManager == nil {
		b.converterManager = NewSVGConverterManager()
	}
	return b.converterManager
}

// GetFpdf returns the underlying gofpdf instance for advanced operations like PDF imports
func (b *Builder) GetFpdf() interface{} {
	provider := b.maroto.GetProvider()

	// The provider is a gofpdf provider that wraps a gofpdf.Fpdf instance
	// We need to use reflection or a type assertion to access it
	// For now, return the provider itself which implements the Fpdf interface
	return provider
}

// generateColumnLabel creates Excel-style column labels (A, B, C... Z, AA, AB, etc.)
func generateColumnLabel(index int) string {
	var result string
	for index >= 0 {
		result = string(rune('A'+index%26)) + result
		index = index/26 - 1
	}
	return result
}

// drawDebugGrid draws a red grid with position markers for debugging layout
func (b *Builder) drawDebugGrid() error {
	if !b.debugMode {
		return nil // Only draw grid in debug mode
	}

	provider := b.maroto.GetProvider()
	if provider == nil {
		return fmt.Errorf("provider is nil")
	}

	drawingHelper := fpdf.NewDrawingHelper(provider)
	if drawingHelper == nil {
		return fmt.Errorf("drawingHelper is nil")
	}

	fpdfInterface := drawingHelper.GetFpdf()
	if fpdfInterface == nil {
		return fmt.Errorf("fpdfInterface is nil")
	}

	// Type assert to access required fpdf methods
	fpdfObj, ok := fpdfInterface.(interface {
		SetDrawColor(r, g, b int)
		SetTextColor(r, g, b int)
		Line(x1, y1, x2, y2 float64)
		SetLineCapStyle(styleStr string)
		SetLineWidth(width float64)
		SetDashPattern(dashArray []float64, dashPhase float64)
		SetFont(familyStr, styleStr string, size float64)
		Text(x, y float64, txtStr string)
		GetMargins() (left, top, right, bottom float64)
		GetPageSize() (width, height float64)
	})
	if !ok {
		return fmt.Errorf("fpdf interface does not support required methods for grid drawing")
	}

	// Get page dimensions and margins
	pageWidth, pageHeight := fpdfObj.GetPageSize()
	leftMargin, topMargin, rightMargin, bottomMargin := fpdfObj.GetMargins()

	// First draw dashed red margin lines
	fpdfObj.SetDrawColor(128, 0, 0)                // Dark red for margins
	fpdfObj.SetDashPattern([]float64{2.0, 1.0}, 0) // 2mm dash, 1mm gap
	fpdfObj.SetLineWidth(0.5)                      // Ensure 0.5pt thickness

	// Left margin line
	fpdfObj.Line(leftMargin, 0, leftMargin, pageHeight)
	// Right margin line
	fpdfObj.Line(pageWidth-rightMargin, 0, pageWidth-rightMargin, pageHeight)
	// Top margin line
	fpdfObj.Line(0, topMargin, pageWidth, topMargin)
	// Bottom margin line
	fpdfObj.Line(0, pageHeight-bottomMargin, pageWidth, pageHeight-bottomMargin)

	// Set light gray color for full-page grid with thin lines
	fpdfObj.SetDrawColor(192, 192, 192)    // Light gray instead of bright purple
	fpdfObj.SetDashPattern([]float64{}, 0) // Reset to solid lines for grid
	fpdfObj.SetLineWidth(0.2)              // 0.2pt line thickness

	// Draw vertical grid lines every 5mm across full page width
	verticalLines := 0
	for x := float64(0); x <= pageWidth; x += 5 {
		fpdfObj.Line(x, 0, x, pageHeight)
		verticalLines++
	}

	// Draw horizontal grid lines every 5mm across full page height
	horizontalLines := 0
	for y := float64(0); y <= pageHeight; y += 5 {
		fpdfObj.Line(0, y, pageWidth, y)
		horizontalLines++
	}

	// Set font and text color for grid labels
	fpdfObj.SetFont("Arial", "", 8)
	fpdfObj.SetTextColor(64, 64, 64) // Dark gray for labels

	// Add column labels (A, B, C...) at top of every 5th column (25mm intervals)
	columnLabels := 0
	for x := float64(0); x <= pageWidth; x += 25 {
		if x+25 <= pageWidth { // Only label if there's a full column
			letter := generateColumnLabel(columnLabels)
			fpdfObj.Text(x+10, 2, letter) // Center in column (x+12.5-2.5 for text width)
			columnLabels++
		}
	}

	// Add row labels (1, 2, 3...) at left of every 5th row (25mm intervals)
	rowLabels := 0
	for y := float64(25); y <= pageHeight; y += 25 { // Start from 25 to skip first row with column headers
		rowNumber := rowLabels + 1
		fpdfObj.Text(2, y-10, fmt.Sprintf("%d", rowNumber)) // Center in row (y-12.5+2.5 for text height)
		rowLabels++
	}

	return nil
}

// Output generates the final PDF content
func (b *Builder) Output() ([]byte, error) {
	if b.maroto == nil {
		return nil, fmt.Errorf("builder not initialized: maroto instance is nil (use NewBuilder() to create a proper builder)")
	}

	// Draw debug grid if in debug mode (before generating final PDF)
	if b.debugMode {
		if err := b.drawDebugGrid(); err != nil {
			log.Printf("WARNING: Failed to draw debug grid: %v", err)
			// Continue with PDF generation even if grid drawing fails
		}
	}

	// Generate the PDF document
	document, err := b.maroto.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	// Get the bytes
	return document.GetBytes(), nil
}

// Build is an alias for Output to match the expected interface
func (b *Builder) Build() ([]byte, error) {
	return b.Output()
}
