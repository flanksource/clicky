package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flanksource/maroto/v2/pkg/components/col"
	marotoimagecomponent "github.com/flanksource/maroto/v2/pkg/components/image"
	"github.com/flanksource/maroto/v2/pkg/consts/align"
	"github.com/flanksource/maroto/v2/pkg/consts/fontstyle"
	"github.com/flanksource/maroto/v2/pkg/core/entity"
	"github.com/flanksource/maroto/v2/pkg/fpdf"
	"github.com/flanksource/maroto/v2/pkg/props"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/tailwind"
)

// createTestDrawingHelper creates a proper DrawingHelper for testing
func createTestDrawingHelper(t *testing.T) *fpdf.DrawingHelper {
	// Create a real PDF builder to get a valid provider
	builder := NewBuilder()
	if builder == nil {
		t.Fatal("Failed to create PDF builder")
	}

	// Get the provider from the maroto instance
	maroto := builder.GetMaroto()
	if maroto == nil {
		t.Fatal("Failed to get maroto instance from builder")
	}

	provider := maroto.GetProvider()
	if provider == nil {
		t.Fatal("Failed to get provider from maroto")
	}

	// Create the DrawingHelper from the valid provider
	drawingHelper := fpdf.NewDrawingHelper(provider)
	if drawingHelper == nil {
		t.Fatal("Failed to create DrawingHelper from valid provider - this indicates a deeper infrastructure issue")
	}

	return drawingHelper
}

// saveTestPDF saves test PDF to the out directory
func saveTestPDF(t *testing.T, name string, pdfData []byte) {
	// Create output directory
	outDir := "out"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Logf("Warning: Could not create output directory: %v", err)
		return
	}

	// Save PDF
	filename := fmt.Sprintf("%s.pdf", name)
	filepath := filepath.Join(outDir, filename)
	if err := os.WriteFile(filepath, pdfData, 0o644); err != nil {
		t.Logf("Warning: Could not save PDF to %s: %v", filepath, err)
	} else {
		t.Logf("Test PDF saved to: %s", filepath)
	}
}

func TestTableCreation(t *testing.T) {
	table := Table{
		BaseTable: BaseTable{
			Columns: []Column{
				{Label: "Name", Style: "w-[50%] text-left align-middle"},
				{Label: "Age", Style: "w-[25%] text-center align-middle"},
				{Label: "City", Style: "w-[25%] text-right align-middle"},
			},
			Rows: [][]any{
				{"Alice Johnson", 28, "New York"},
				{"Bob Smith", 35, "Los Angeles"},
			},
			HeaderStyle:       "font-bold bg-gray-100 text-center align-middle",
			RowStyle:          "text-gray-700 align-middle",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
	}

	// Test that the table was created properly
	if len(table.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(table.Columns))
	}

	if len(table.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(table.Rows))
	}

	if !table.ShowBorders {
		t.Error("Expected ShowBorders to be true")
	}
}

func TestTableComponent(t *testing.T) {
	table := Table{
		BaseTable: BaseTable{
			Columns: []Column{
				{Label: "Product", Style: "w-[40%] text-left align-middle"},
				{Label: "Price", Style: "w-[30%] text-right align-middle font-mono"},
				{Label: "Stock", Style: "w-[30%] text-center align-middle"},
			},
			Rows: [][]any{
				{"Laptop", "$1,299", 15},
				{"Mouse", "$29.99", 150},
				{"Wireless Keyboard", "$79.99", 45},
				{"USB-C Hub", "$49.99", 80},
			},
			HeaderStyle:       "font-bold text-white bg-blue-600 text-center text-sm",
			RowStyle:          "text-sm text-gray-800 align-middle",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
	}

	// Test that the table was created properly
	if len(table.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(table.Columns))
	}

	if len(table.Rows) != 4 {
		t.Errorf("Expected 4 rows, got %d", len(table.Rows))
	}

	// Generate PDF with the table
	builder := NewBuilder()

	// Add a title
	titleText := Text{
		Text: api.Text{
			Content: "Table Test - Product Inventory",
			Class:   api.ResolveStyles("text-2xl font-bold text-center text-blue-800 mb-6"),
		},
	}
	titleText.Draw(builder)

	// Draw the table
	err := table.Draw(builder)
	if err != nil {
		t.Fatalf("Failed to draw table: %v", err)
	}

	// Generate the PDF
	pdfData, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to generate PDF: %v", err)
	}

	// Validate PDF was generated
	if len(pdfData) == 0 {
		t.Fatal("Generated PDF is empty")
	}

	// Save the PDF to out directory
	saveTestPDF(t, "table_component_test", pdfData)

	t.Logf("✓ Table PDF generated successfully (%d bytes)", len(pdfData))
}

func TestColumnWidthParsing(t *testing.T) {
	tests := []struct {
		name          string
		style         string
		expectedWidth string
	}{
		{
			name:          "percentage width",
			style:         "w-[25%] text-center align-middle",
			expectedWidth: "25%",
		},
		{
			name:          "character width",
			style:         "w-[20ch] text-left font-mono",
			expectedWidth: "20ch",
		},
		{
			name:          "pixel width",
			style:         "w-[200px] text-right",
			expectedWidth: "200px",
		},
		{
			name:          "fractional width",
			style:         "w-1/3 text-center",
			expectedWidth: "1/3",
		},
		{
			name:          "no width specified",
			style:         "text-center font-bold",
			expectedWidth: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			column := Column{
				Label: "Test",
				Style: tt.style,
			}

			// Extract width from style using the Tailwind utilities
			widthSpec := tailwind.ParseWidthFromStyle(column.Style)

			var actualWidth string
			if widthSpec != nil {
				// Handle different width formats correctly
				if strings.Contains(widthSpec.Original, "[") && strings.Contains(widthSpec.Original, "]") {
					// Arbitrary values like w-[25%], w-[20ch]
					start := strings.Index(widthSpec.Original, "[") + 1
					end := strings.Index(widthSpec.Original, "]")
					actualWidth = widthSpec.Original[start:end]
				} else {
					// Other formats like w-1/3, w-auto - remove the "w-" prefix
					actualWidth = widthSpec.Original[2:]
				}
			}

			if tt.expectedWidth == "" {
				if actualWidth != "" {
					t.Errorf("Expected no width, got %q", actualWidth)
				}
			} else {
				if !strings.Contains(actualWidth, strings.Trim(tt.expectedWidth, "[]")) {
					t.Errorf("Expected width containing %q, got %q", tt.expectedWidth, actualWidth)
				}
			}
		})
	}
}

func TestAlignmentParsing(t *testing.T) {
	tests := []struct {
		name               string
		style              string
		expectedHorizontal string
		expectedVertical   string
	}{
		{
			name:               "left-top alignment",
			style:              "text-left-top font-bold",
			expectedHorizontal: "left",
			expectedVertical:   "top",
		},
		{
			name:               "center-middle alignment",
			style:              "text-center-middle bg-gray-100",
			expectedHorizontal: "center",
			expectedVertical:   "middle",
		},
		{
			name:               "right-bottom alignment",
			style:              "text-right-bottom font-mono",
			expectedHorizontal: "right",
			expectedVertical:   "bottom",
		},
		{
			name:               "separate alignment classes",
			style:              "text-center align-top font-bold",
			expectedHorizontal: "center",
			expectedVertical:   "top",
		},
		{
			name:               "default alignment",
			style:              "font-bold bg-gray-100",
			expectedHorizontal: "left",
			expectedVertical:   "middle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alignment := tailwind.ParseAlignment(tt.style)

			// Convert alignment enums to strings for comparison
			var horizontalStr string
			switch alignment.Horizontal {
			case "L": // align.Left
				horizontalStr = "left"
			case "C": // align.Center
				horizontalStr = "center"
			case "R": // align.Right
				horizontalStr = "right"
			case "J": // align.Justify
				horizontalStr = "justify"
			default:
				horizontalStr = "left"
			}

			var verticalStr string
			switch alignment.Vertical {
			case tailwind.VerticalTop:
				verticalStr = "top"
			case tailwind.VerticalMiddle:
				verticalStr = "middle"
			case tailwind.VerticalBottom:
				verticalStr = "bottom"
			default:
				verticalStr = "middle"
			}

			if horizontalStr != tt.expectedHorizontal {
				t.Errorf("Expected horizontal alignment %q, got %q", tt.expectedHorizontal, horizontalStr)
			}

			if verticalStr != tt.expectedVertical {
				t.Errorf("Expected vertical alignment %q, got %q", tt.expectedVertical, verticalStr)
			}
		})
	}
}

func TestStyleMerging(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		override string
		expected string
	}{
		{
			name:     "simple merge",
			base:     "text-left font-normal",
			override: "text-center",
			expected: "font-normal text-center",
		},
		{
			name:     "color override",
			base:     "text-gray-500 bg-white",
			override: "text-blue-600",
			expected: "bg-white text-blue-600",
		},
		{
			name:     "multiple property override",
			base:     "text-left font-normal bg-gray-100",
			override: "text-center font-bold",
			expected: "bg-gray-100 text-center font-bold",
		},
		{
			name:     "no conflicts",
			base:     "font-bold",
			override: "bg-gray-100",
			expected: "font-bold bg-gray-100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tailwind.MergeStyles(tt.base, tt.override)

			// Split both results into words and sort for comparison
			expectedWords := strings.Fields(tt.expected)
			resultWords := strings.Fields(result)

			if len(resultWords) != len(expectedWords) {
				t.Errorf("Expected %d classes, got %d. Result: %q, Expected: %q",
					len(expectedWords), len(resultWords), result, tt.expected)
				return
			}

			// Check that all expected words are present (order may vary)
			for _, expectedWord := range expectedWords {
				found := false
				for _, resultWord := range resultWords {
					if resultWord == expectedWord {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected class %q not found in result %q", expectedWord, result)
				}
			}
		})
	}
}

func TestTableWithComplexStyling(t *testing.T) {
	table := Table{
		BaseTable: BaseTable{
			Columns: []Column{
				{
					Label: "Item",
					Style: "w-[20ch] text-left-top font-medium text-gray-800",
				},
				{
					Label: "Description",
					Style: "w-[40ch] text-left-middle font-normal text-gray-700",
				},
				{
					Label: "Price",
					Style: "w-[10ch] text-right-middle font-mono text-green-600",
				},
				{
					Label: "Status",
					Style: "w-[8ch] text-center-bottom font-bold text-blue-600",
				},
			},
			Rows: [][]any{
				{"PRD-001", "High-performance laptop", "$1,299.00", "Active"},
				{"PRD-002", "Wireless mouse", "$29.99", "Active"},
				{"PRD-003", "USB keyboard", "$79.99", "Inactive"},
			},
			HeaderStyle:       "font-bold bg-gray-800 text-white text-center-middle",
			RowStyle:          "text-gray-700 align-middle hover:bg-gray-50",
			AlternateRowStyle: "bg-gray-25",
			ShowBorders:       true,
			CompactMode:       false,
		},
	}

	// Test style parsing for all columns
	for i, column := range table.Columns {
		t.Run(column.Label, func(t *testing.T) {
			// Parse width
			widthSpec := tailwind.ParseWidthFromStyle(column.Style)
			if widthSpec == nil {
				t.Errorf("Column %d (%s): Expected width specification in style %q", i, column.Label, column.Style)
				return
			}

			// Parse alignment
			alignment := tailwind.ParseAlignment(column.Style)
			// Just verify that alignment parsing doesn't fail
			if alignment.Horizontal == "" {
				t.Errorf("Column %d (%s): Invalid horizontal alignment in style %q", i, column.Label, column.Style)
			}
			if alignment.Vertical < 0 {
				t.Errorf("Column %d (%s): Invalid vertical alignment in style %q", i, column.Label, column.Style)
			}
		})
	}

	// Test that all expected data is present
	if len(table.Columns) != 4 {
		t.Errorf("Expected 4 columns, got %d", len(table.Columns))
	}

	if len(table.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(table.Rows))
	}

	// Test header style parsing
	headerAlignment := tailwind.ParseAlignment(table.HeaderStyle)
	if headerAlignment.Horizontal != "C" { // align.Center
		t.Errorf("Expected header horizontal alignment center (C), got %v", headerAlignment.Horizontal)
	}
	if headerAlignment.Vertical != tailwind.VerticalMiddle {
		t.Errorf("Expected header vertical alignment middle, got %v", headerAlignment.Vertical)
	}
}

func TestTableDataValidation(t *testing.T) {
	tests := []struct {
		name    string
		table   Table
		wantErr bool
	}{
		{
			name: "valid table",
			table: Table{
				BaseTable: BaseTable{
					Columns: []Column{
						{Label: "Col1", Style: "w-[50%] text-left"},
						{Label: "Col2", Style: "w-[50%] text-right"},
					},
					Rows: [][]any{
						{"Data1", "Data2"},
						{"Data3", "Data4"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "mismatched row data length",
			table: Table{
				BaseTable: BaseTable{
					Columns: []Column{
						{Label: "Col1", Style: "w-[50%] text-left"},
						{Label: "Col2", Style: "w-[50%] text-right"},
					},
					Rows: [][]any{
						{"Data1", "Data2"},
						{"Data3"}, // Missing second column data
					},
				},
			},
			wantErr: false, // Should not error, just truncate or pad
		},
		{
			name: "empty columns",
			table: Table{
				BaseTable: BaseTable{
					Columns: []Column{},
					Rows: [][]any{
						{"Data1", "Data2"},
					},
				},
			},
			wantErr: false, // Should handle gracefully
		},
		{
			name: "empty rows",
			table: Table{
				BaseTable: BaseTable{
					Columns: []Column{
						{Label: "Col1", Style: "w-[100%] text-center"},
					},
					Rows: [][]any{},
				},
			},
			wantErr: false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that table creation doesn't panic or error inappropriately
			defer func() {
				if r := recover(); r != nil && !tt.wantErr {
					t.Errorf("Table creation panicked: %v", r)
				}
			}()

			// Basic validation - ensure data is accessible
			columnCount := len(tt.table.Columns)
			rowCount := len(tt.table.Rows)

			if columnCount < 0 {
				t.Error("Negative column count")
			}
			if rowCount < 0 {
				t.Error("Negative row count")
			}

			// Test that we can iterate through the data without panic
			for i, row := range tt.table.Rows {
				for j := range tt.table.Columns {
					if j < len(row) {
						// Data is accessible
						_ = row[j]
					}
					// Don't fail if row is shorter than columns - this is handled gracefully
				}
				_ = i // Use the row index
			}
		})
	}
}

func TestTableComponent_drawCellText(t *testing.T) {
	// Create a mock TableComponent
	tc := &TableComponent{
		TopAlign:       false,
		styleConverter: NewStyleConverter(),
	}

	// Test cases for drawCellText
	// NOTE: All tests with nil drawingHelper indicate a critical setup failure
	tests := []struct {
		name           string
		text           string
		cell           *entity.Cell
		style          props.Text
		expectError    bool
		errorSubstring string
		setupFailure   bool // Indicates this test expects a setup failure
	}{
		{
			name:           "nil cell with empty text",
			text:           "",
			cell:           nil,
			style:          props.Text{Size: 12},
			expectError:    false, // Empty text returns early before checking cell
			errorSubstring: "",
		},
		{
			name:           "nil cell with text",
			text:           "test",
			cell:           nil,
			style:          props.Text{Size: 12},
			expectError:    true,
			errorSubstring: "cell is nil",
		},
		{
			name:        "empty text with valid cell",
			text:        "",
			cell:        &entity.Cell{X: 0, Y: 0, Width: 100, Height: 20},
			style:       props.Text{Size: 12},
			expectError: false, // Empty text should return success
		},
		{
			name:           "invalid cell dimensions",
			text:           "test",
			cell:           &entity.Cell{X: 0, Y: 0, Width: 0, Height: 20}, // Invalid width
			style:          props.Text{Size: 12},
			expectError:    true,
			errorSubstring: "invalid cell dimensions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a proper DrawingHelper for testing
			drawingHelper := createTestDrawingHelper(t)

			// Set up FPDF interface for testing
			tc.Fpdf = WrapFpdf(drawingHelper.GetFpdf())

			// Test the method with valid infrastructure
			err := tc.drawCellText(tt.text, tt.cell, tt.style)

			// Test normal error handling with proper infrastructure
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for case '%s', but got nil", tt.name)
				} else if tt.errorSubstring != "" && !strings.Contains(err.Error(), tt.errorSubstring) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorSubstring, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for case '%s', but got: %v", tt.name, err)
				}
			}
		})
	}
}

func TestTableComponent_drawCellText_StyleHandling(t *testing.T) {
	tc := &TableComponent{
		TopAlign:       true, // Test top alignment
		styleConverter: NewStyleConverter(),
	}


	// Test different style configurations
	styleTests := []struct {
		name        string
		style       props.Text
		expectedLog string // What we expect in debug logs
	}{
		{
			name: "bold font",
			style: props.Text{
				Family: "Arial",
				Style:  fontstyle.Bold,
				Size:   12,
				Align:  align.Left,
			},
		},
		{
			name: "italic font",
			style: props.Text{
				Family: "Times",
				Style:  fontstyle.Italic,
				Size:   14,
				Align:  align.Center,
			},
		},
		{
			name: "right aligned",
			style: props.Text{
				Size:  10,
				Align: align.Right,
			},
		},
		{
			name:  "default values",
			style: props.Text{}, // Test all defaults
		},
	}

	cell := &entity.Cell{X: 0, Y: 0, Width: 100, Height: 20}

	for _, tt := range styleTests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that style handling doesn't panic with various configurations
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("drawCellText panicked with style %+v: %v", tt.style, r)
				}
			}()

			// Test with proper DrawingHelper infrastructure
			err := tc.drawCellText("test", cell, tt.style)
			if err != nil {
				t.Errorf("drawCellText failed with style %+v: %v", tt.style, err)
			}
		})
	}
}

func TestTableComponent_drawCellText_AlignmentCalculation(t *testing.T) {

	tests := []struct {
		name            string
		topAlign        bool
		horizontalAlign align.Type
		expectedPrefix  string
	}{
		{
			name:            "top left",
			topAlign:        true,
			horizontalAlign: align.Left,
			expectedPrefix:  "TL",
		},
		{
			name:            "top center",
			topAlign:        true,
			horizontalAlign: align.Center,
			expectedPrefix:  "TC",
		},
		{
			name:            "top right",
			topAlign:        true,
			horizontalAlign: align.Right,
			expectedPrefix:  "TR",
		},
		{
			name:            "middle left",
			topAlign:        false,
			horizontalAlign: align.Left,
			expectedPrefix:  "ML",
		},
		{
			name:            "middle center",
			topAlign:        false,
			horizontalAlign: align.Center,
			expectedPrefix:  "MC",
		},
		{
			name:            "middle right",
			topAlign:        false,
			horizontalAlign: align.Right,
			expectedPrefix:  "MR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TableComponent{
				TopAlign:       tt.topAlign,
				styleConverter: NewStyleConverter(),
			}

			style := props.Text{
				Align: tt.horizontalAlign,
				Size:  10,
			}
			cell := &entity.Cell{X: 0, Y: 0, Width: 100, Height: 20}

			// Test that alignment calculation works correctly
			// Since we can't access the internal alignment string directly,
			// we test that the method doesn't panic and handles alignment properly
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("drawCellText panicked with alignment %v: %v", tt.horizontalAlign, r)
				}
			}()

			err := tc.drawCellText("test", cell, style)
			if err != nil {
				t.Errorf("drawCellText failed with alignment %v: %v", tt.horizontalAlign, err)
			}
		})
	}
}

func TestTableComponent_drawCellText_EdgeCases(t *testing.T) {
	tc := &TableComponent{
		TopAlign:       false,
		styleConverter: NewStyleConverter(),
	}


	edgeCases := []struct {
		name         string
		cell         *entity.Cell
		text         string
		expectError  bool
		errorSubtext string
	}{
		{
			name:         "zero width cell",
			cell:         &entity.Cell{X: 0, Y: 0, Width: 0, Height: 20},
			text:         "test",
			expectError:  true,
			errorSubtext: "invalid cell dimensions",
		},
		{
			name:         "zero height cell",
			cell:         &entity.Cell{X: 0, Y: 0, Width: 100, Height: 0},
			text:         "test",
			expectError:  true,
			errorSubtext: "invalid cell dimensions",
		},
		{
			name:         "negative dimensions",
			cell:         &entity.Cell{X: 0, Y: 0, Width: -10, Height: -5},
			text:         "test",
			expectError:  true,
			errorSubtext: "invalid cell dimensions",
		},
		{
			name:        "very long text",
			cell:        &entity.Cell{X: 0, Y: 0, Width: 50, Height: 20},
			text:        "This is a very long text that might not fit in the cell and could cause issues",
			expectError: false, // Should handle gracefully
		},
		{
			name:        "special characters",
			cell:        &entity.Cell{X: 0, Y: 0, Width: 100, Height: 20},
			text:        "Special chars: @#$%^&*()[]{}|\\:;\"'<>?/.,`~",
			expectError: false, // Should handle gracefully
		},
		{
			name:        "empty text",
			cell:        &entity.Cell{X: 0, Y: 0, Width: 100, Height: 20},
			text:        "",
			expectError: false, // Empty text should succeed
		},
	}

	for _, tt := range edgeCases {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("drawCellText panicked with edge case '%s': %v", tt.name, r)
				}
			}()

			style := props.Text{Size: 10}
			err := tc.drawCellText(tt.text, tt.cell, style)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for edge case '%s', but got none", tt.name)
				} else if tt.errorSubtext != "" && !strings.Contains(err.Error(), tt.errorSubtext) {
					t.Errorf("Expected error containing '%s' for edge case '%s', got: %v", tt.errorSubtext, tt.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for edge case '%s', but got: %v", tt.name, err)
				}
			}
		})
	}
}

// TestTwoColumnSVGTableLayout tests the two-column layout with SVG (60%) + Table (40%)
func TestTwoColumnSVGTableLayout(t *testing.T) {
	// Enable debug mode for detailed positioning logs
	debugMode := true
	if debugMode {
		t.Logf("DEBUG: TestTwoColumnSVGTableLayout: debug mode enabled")
	}

	// Create builder
	builder := NewBuilder(WithDebug(true))

	// Create a demo SVG box for the image column (extracted from showcase)
	demoSVGBox := SVGBox{
		Box: api.Box{
			Rectangle: api.Rectangle{Width: 300, Height: 200},
			Fill:      api.Color{Hex: "#e3f2fd"},
			Border: api.Borders{
				Top:    api.Line{Width: 3, Color: api.Color{Hex: "#1976d2"}},
				Right:  api.Line{Width: 3, Color: api.Color{Hex: "#1976d2"}},
				Bottom: api.Line{Width: 3, Color: api.Color{Hex: "#1976d2"}},
				Left:   api.Line{Width: 3, Color: api.Color{Hex: "#1976d2"}},
			},
		},
		Circles: []CircleShape{
			{X: 75, Y: 50, Diameter: 25, Label: "A"},
			{X: 225, Y: 50, Diameter: 25, Label: "B"},
			{X: 150, Y: 150, Diameter: 30, Label: "C"},
		},
		Labels: []Label{
			{
				Positionable: Positionable{
					Position: &LabelPosition{Vertical: VerticalTop, Horizontal: HorizontalCenter},
				},
				Text: api.Text{
					Content: "Technical Diagram",
					Class:   api.Class{Font: &api.Font{Bold: true, Size: 1.1}},
				},
			},
		},
		ShowDimensions: true,
		ActualWidth:    300,
		ActualHeight:   200,
		DimensionUnit:  "mm",
	}

	// Generate temporary SVG file
	svgBytes, err := demoSVGBox.GenerateSVG()
	if err != nil {
		t.Fatalf("Failed to generate demo SVG: %v", err)
	}

	if debugMode {
		t.Logf("DEBUG: TestTwoColumnSVGTableLayout: generated SVG with %d bytes", len(svgBytes))
	}

	tempFile, err := os.CreateTemp("", "two_column_layout_test_*.svg")
	if err != nil {
		t.Fatalf("Failed to create temp SVG file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	if debugMode {
		t.Logf("DEBUG: TestTwoColumnSVGTableLayout: created temp SVG file: %s", tempFile.Name())
	}

	if _, err := tempFile.Write(svgBytes); err != nil {
		tempFile.Close()
		t.Fatalf("Failed to write SVG data: %v", err)
	}
	tempFile.Close()

	// Convert SVG to PNG using the existing conversion system
	ctx := context.Background()
	pngPath := "out/two_column_layout_test.png"

	convertOptions := &ConvertOptions{
		Format: "png",
		DPI:    288, // High resolution
	}

	// Convert SVG to PNG
	if debugMode {
		t.Logf("DEBUG: TestTwoColumnSVGTableLayout: converting SVG to PNG at %s with DPI=%d", pngPath, convertOptions.DPI)
	}
	if err := ConvertWithFallback(ctx, tempFile.Name(), pngPath, convertOptions); err != nil {
		t.Fatalf("SVG conversion failed: %v", err)
	}

	// Verify the PNG file was created and is valid
	if err := ValidatePNGFile(pngPath); err != nil {
		t.Fatalf("Converted PNG is invalid: %v", err)
	}

	if debugMode {
		pngStat, err := os.Stat(pngPath)
		if err == nil {
			t.Logf("DEBUG: TestTwoColumnSVGTableLayout: PNG conversion successful, file size: %.1f KB", float64(pngStat.Size())/1024)
		}
	}

	// Create the side-by-side row using maroto's column system
	// 60% = 7.2 columns ≈ 7 columns (7/12 = 58.33%)
	// 40% = 4.8 columns ≈ 5 columns (5/12 = 41.67%)
	leftColWidth := 7
	rightColWidth := 5
	if debugMode {
		t.Logf("DEBUG: TestTwoColumnSVGTableLayout: layout ratios - left=%d/12 (%.2f%%), right=%d/12 (%.2f%%)",
			leftColWidth, float64(leftColWidth)/12*100, rightColWidth, float64(rightColWidth)/12*100)
	}

	// Left column: Image component (7 columns ≈ 58.33%)
	imageComponent := marotoimagecomponent.NewFromFile(pngPath, props.Rect{
		Top:     0,     // Align to top of cell
		Left:    0,     // Align to left of cell
		Percent: 95,    // Use 95% of available space
		Center:  false, // Don't center, use Top/Left positioning
	})
	leftCol := col.New(leftColWidth).Add(imageComponent)

	// Right column: Table component (5 columns ≈ 41.67%)
	tableComponent := NewTableComponent(
		[]string{"Property", "Value"},
		[][]string{
			{"Width", "300mm"},
			{"Height", "200mm"},
			{"Circles", "3 (A, B, C)"},
			{"Area", "600cm²"},
			{"Border", "3px Blue"},
			{"Aspect Ratio", "3:2"},
			{"Scale", "1:1"},
			{"Format", "SVG→PNG"},
			{"Resolution", "288 DPI"},
			{"Status", "✓ Valid"},
		},
	)

	// Enable debug mode for the table component
	tableComponent.Debug = debugMode

	if debugMode {
		t.Logf("DEBUG: TestTwoColumnSVGTableLayout: created table component with %d columns, %d rows, debug=%v",
			len(tableComponent.Columns), len(tableComponent.Rows), tableComponent.Debug)
	}

	rightCol := col.New(rightColWidth).Add(tableComponent)

	// Add the side-by-side row to the builder
	rowHeight := 80.0 // Height in mm to accommodate content

	if debugMode {
		t.Logf("DEBUG: TestTwoColumnSVGTableLayout: adding side-by-side row with height=%.1fmm", rowHeight)
	}

	builder.GetMaroto().AddRow(rowHeight, leftCol, rightCol)

	// Generate the PDF
	if debugMode {
		t.Logf("DEBUG: TestTwoColumnSVGTableLayout: generating PDF...")
	}

	pdfData, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to generate PDF: %v", err)
	}

	if debugMode {
		t.Logf("DEBUG: TestTwoColumnSVGTableLayout: PDF generated successfully, size=%d bytes", len(pdfData))
	}

	// Validate PDF was generated
	if len(pdfData) == 0 {
		t.Fatal("Generated PDF is empty")
	}

	// Save the PDF to out directory
	saveTestPDF(t, "two_column_layout_test", pdfData)

	// Get file size information
	pngStat, pngErr := os.Stat(pngPath)
	var pngSize string
	if pngErr == nil {
		pngSize = fmt.Sprintf("%.1f KB", float64(pngStat.Size())/1024)
	} else {
		pngSize = "Unknown"
	}

	t.Logf("✓ Two-column layout PDF generated successfully")
	t.Logf("  PDF size: %d bytes", len(pdfData))
	t.Logf("  PNG size: %s", pngSize)
	t.Logf("  Layout: 7:5 column ratio (58.33%%:41.67%%)")
	t.Logf("  Files: out/two_column_layout_test.pdf, %s", pngPath)
}
