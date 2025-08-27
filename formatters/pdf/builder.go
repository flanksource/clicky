package pdf

import (
	"fmt"

	"github.com/flanksource/clicky/api"
	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"
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

	// Create Maroto configuration
	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithLeftMargin(10).
		WithRightMargin(10).
		WithTopMargin(10).
		WithBottomMargin(10).
		WithDebug(b.debugMode). // Enable debug mode if requested
		Build()

	// Create Maroto instance
	m := maroto.New(cfg)

	b.maroto = m
	b.config = &PageSize{
		Rectangle: api.Rectangle{Width: 210, Height: 297}, // A4 in mm
		Margins:   api.Padding{Top: 10, Right: 10, Bottom: 10, Left: 10},
	}
	b.pageNumbers = false

	return b
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
	b.maroto.RegisterHeader(headerRow)
}

// registerFooter registers the footer with Maroto
func (b *Builder) registerFooter() {
	if b.footer.IsEmpty() {
		return
	}

	footerRow := b.createTextRow(b.footer, 8)
	b.maroto.RegisterFooter(footerRow)
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
		b.AddText(child)
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
func (b *Builder) MoveBy(dx, dy int) *Builder {
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

// Output generates the final PDF content
func (b *Builder) Output() ([]byte, error) {
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

// Helper function to create text properties with alignment
func createTextProps(style fontstyle.Type, size float64, alignment align.Type, color *props.Color) *props.Text {
	return &props.Text{
		Style: style,
		Size:  size,
		Align: alignment,
		Color: color,
	}
}
