package pdf

import (
	"fmt"
	"log"

	"github.com/flanksource/maroto/v2/pkg/components/col"
	"github.com/flanksource/maroto/v2/pkg/components/text"
	"github.com/flanksource/maroto/v2/pkg/consts/align"
	"github.com/flanksource/maroto/v2/pkg/consts/fontstyle"
	"github.com/flanksource/maroto/v2/pkg/core"
	"github.com/flanksource/maroto/v2/pkg/props"
)

// FontMetricsTester demonstrates Point vs MM conversions with visual bounding boxes
type FontMetricsTester struct {
	Debug bool
}

// NewFontMetricsTester creates a new font metrics testing component
func NewFontMetricsTester() *FontMetricsTester {
	return &FontMetricsTester{
		Debug: true,
	}
}

// Draw renders the font metrics tester with visual bounding boxes
func (fmtTester *FontMetricsTester) Draw(builder *Builder) error {
	if fmtTester.Debug {
		log.Printf("DEBUG: FontMetricsTester: rendering font metrics demonstration")
	}

	// Font sizes to test (in points)
	fontSizes := []float64{8, 10, 12, 14, 16}

	// Create test data showing Point -> MM conversions
	var testRows []core.Row

	// Header row
	headerRow := text.NewRow(6, "Font Metrics: Point vs MM Conversions",
		props.Text{
			Size:  14,
			Style: fontstyle.Bold,
			Align: align.Center,
			Color: &props.Color{Red: 0, Green: 0, Blue: 128}, // Dark blue
		})
	testRows = append(testRows, headerRow)

	// Font size demonstration rows
	for _, sizeInPt := range fontSizes {
		fontSize := NewFontSize(sizeInPt)
		sizeInMM := fontSize.ToMM()
		lineHeightMM := fontSize.LineHeight(1.2)

		// Create content showing the conversion
		content := fmt.Sprintf("Font: %.0fpt → %.2fmm (line: %.2fmm)",
			sizeInPt, sizeInMM.Float64(), lineHeightMM.Float64())

		// Use the actual font size for display
		textRow := text.NewRow(lineHeightMM.Float64(), content,
			props.Text{
				Size:  sizeInPt,
				Style: fontstyle.Normal,
				Align: align.Left,
				Color: &props.Color{Red: 64, Green: 64, Blue: 64}, // Dark gray
			})
		testRows = append(testRows, textRow)

		if fmtTester.Debug {
			log.Printf("DEBUG: FontMetricsTester: %s", content)
		}
	}

	// Unit conversion examples
	conversionRow := text.NewRow(6,
		fmt.Sprintf("Unit Conversions: 1pt = %.4fmm | 1mm = %.4fpt | 1rem = %.0fpt = %.2fmm",
			PointsToMM, MMToPoints, RemToPoints, RemToMM),
		props.Text{
			Size:  9,
			Style: fontstyle.Italic,
			Align: align.Center,
			Color: &props.Color{Red: 128, Green: 0, Blue: 128}, // Purple
		})
	testRows = append(testRows, conversionRow)

	// Baseline and metrics demonstration
	baselineRow := text.NewRow(8,
		"Baseline Demo: Αβγδ ← Greek | 中文 ← Chinese | Ĵust ← Latin",
		props.Text{
			Size:  12,
			Style: fontstyle.Normal,
			Align: align.Left,
			Color: &props.Color{Red: 196, Green: 0, Blue: 0}, // Red
		})
	testRows = append(testRows, baselineRow)

	// Add all rows to the builder
	builder.AddRows(testRows...)

	if fmtTester.Debug {
		log.Printf("DEBUG: FontMetricsTester: added %d demonstration rows", len(testRows))
	}

	return nil
}

// CreateFontMetricsColumn creates a column containing the font metrics tester
func CreateFontMetricsColumn(columnWidth int) core.Col {
	// Use the table-based approach instead
	fontMetricsTable := NewFontMetricsTable()
	return col.New(columnWidth).Add(fontMetricsTable)
}

// FontMetricsWrapper wraps the tester for use in columns
type FontMetricsWrapper struct {
	tester *FontMetricsTester
}

// FontMetricsTable creates a table-based font metrics display
func NewFontMetricsTable() *TableComponent {
	headers := []string{"Font Size", "Points", "MM", "Line Height"}
	rows := [][]string{}

	// Generate rows for different font sizes
	fontSizes := []float64{8, 10, 12, 14, 16}
	for _, sizeInPt := range fontSizes {
		fontSize := NewFontSize(sizeInPt)
		sizeInMM := fontSize.ToMM()
		lineHeightMM := fontSize.LineHeight(1.2)

		row := []string{
			fmt.Sprintf("%.0fpt font", sizeInPt),
			fmt.Sprintf("%.1fpt", sizeInPt),
			fmt.Sprintf("%.2fmm", sizeInMM.Float64()),
			fmt.Sprintf("%.2fmm", lineHeightMM.Float64()),
		}
		rows = append(rows, row)
	}

	// Add conversion constants row
	rows = append(rows, []string{
		"Constants",
		fmt.Sprintf("1pt = %.4fmm", PointsToMM),
		fmt.Sprintf("1mm = %.4fpt", MMToPoints),
		fmt.Sprintf("1rem = %.0fpt", RemToPoints),
	})

	// Convert [][]string to [][]any for TableComponent
	rowsAny := make([][]any, len(rows))
	for i, row := range rows {
		rowAny := make([]any, len(row))
		for j, cell := range row {
			rowAny[j] = cell
		}
		rowsAny[i] = rowAny
	}

	table := NewTableComponent(headers, rowsAny)
	table.HeaderStyle = "font-bold text-white bg-green-600 text-center text-sm"
	table.RowStyle = "text-xs text-gray-800 font-normal"

	return table
}
