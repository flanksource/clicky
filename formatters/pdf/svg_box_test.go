package pdf

import (
	"fmt"
	"os"
	"testing"

	"github.com/flanksource/clicky/api"
)

// TestAllLabelPositions tests every possible label position
func TestAllLabelPositions(t *testing.T) {
	positions := []struct {
		name     string
		position LabelPosition
	}{
		{"center", LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter}},
		{"top", LabelPosition{Vertical: VerticalTop}},
		{"bottom", LabelPosition{Vertical: VerticalBottom}},
		{"left", LabelPosition{Horizontal: HorizontalLeft}},
		{"right", LabelPosition{Horizontal: HorizontalRight}},
		{"top-left", LabelPosition{Vertical: VerticalTop, Horizontal: HorizontalLeft}},
		{"top-right", LabelPosition{Vertical: VerticalTop, Horizontal: HorizontalRight}},
		{"bottom-left", LabelPosition{Vertical: VerticalBottom, Horizontal: HorizontalLeft}},
		{"bottom-right", LabelPosition{Vertical: VerticalBottom, Horizontal: HorizontalRight}},
		{"top-outside", LabelPosition{Vertical: VerticalTop, Inside: InsideBottom}},
		{"bottom-outside", LabelPosition{Vertical: VerticalBottom, Inside: InsideBottom}},
		{"left-outside", LabelPosition{Horizontal: HorizontalLeft, Inside: InsideBottom}},
		{"right-outside", LabelPosition{Horizontal: HorizontalRight, Inside: InsideBottom}},
	}

	// Create output directory
	os.MkdirAll("out", 0o755)

	for _, pos := range positions {
		t.Run(pos.name, func(t *testing.T) {
			box := SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  200,
						Height: 150,
					},
					Fill: api.Color{Hex: "#f0f0f0"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#333"}, Width: 2},
						Bottom: api.Line{Color: api.Color{Hex: "#333"}, Width: 2},
						Left:   api.Line{Color: api.Color{Hex: "#333"}, Width: 2},
						Right:  api.Line{Color: api.Color{Hex: "#333"}, Width: 2},
					},
				},
				Labels: []Label{
					{
						Text: api.Text{
							Content: fmt.Sprintf("Label: %s", pos.name),
							Class: api.Class{
								Font: &api.Font{Size: 14},
							},
						},
						Positionable: Positionable{
							Position: &pos.position,
						},
					},
				},
			}

			svgData, err := box.GenerateSVG()
			if err != nil {
				t.Fatalf("Failed to generate SVG: %v", err)
			}

			// Save to file
			filename := fmt.Sprintf("out/label_position_%s.svg", pos.name)
			err = os.WriteFile(filename, svgData, 0o644)
			if err != nil {
				t.Fatalf("Failed to write SVG file: %v", err)
			}

			t.Logf("SVG saved to: %s", filename)
		})
	}
}

// TestOperations tests each type of operation (holes, dados, rabbets)
func TestOperations(t *testing.T) {
	tests := []struct {
		name string
		box  SVGBox
	}{
		{
			name: "holes",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  300,
						Height: 200,
					},
					Fill: api.Color{Hex: "#ffffff"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Bottom: api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Left:   api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Right:  api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
					},
				},
				Circles: []CircleShape{
					{X: 50, Y: 50, Diameter: 20, Label: "C1"},
					{X: 150, Y: 50, Diameter: 25, Label: "C2"},
					{X: 250, Y: 50, Diameter: 20, Label: "C3"},
					{X: 50, Y: 150, Diameter: 30, Label: "C4"},
					{X: 150, Y: 150, Diameter: 35, Label: "C5"},
					{X: 250, Y: 150, Diameter: 30, Label: "C6"},
				},
				Labels: []Label{
					{
						Text: api.Text{Content: "Circles Example"},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalTop},
						},
					},
				},
			},
		},
		{
			name: "dados",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  300,
						Height: 200,
					},
					Fill: api.Color{Hex: "#ffffff"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Bottom: api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Left:   api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Right:  api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
					},
				},
				Cuts: []Cut{
					{Orientation: "vertical", Position: 75, Width: 10, Label: "Cut1"},
					{Orientation: "vertical", Position: 150, Width: 15, Label: "Cut2"},
					{Orientation: "vertical", Position: 225, Width: 10, Label: "Cut3"},
					{Orientation: "horizontal", Position: 100, Width: 12, Label: "Cut4"},
				},
				Labels: []Label{
					{
						Text: api.Text{Content: "Cuts Example"},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalTop},
						},
					},
				},
			},
		},
		{
			name: "rabbets",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  300,
						Height: 200,
					},
					Fill: api.Color{Hex: "#ffffff"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Bottom: api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Left:   api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Right:  api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
					},
				},
				EdgeCuts: []EdgeCut{
					{Edge: "left", Width: 20, Label: "R1"},
					{Edge: "right", Width: 20, Label: "R2"},
					{Edge: "top", Width: 15, Label: "R3"},
					{Edge: "bottom", Width: 15, Label: "R4"},
				},
				Labels: []Label{
					{
						Text: api.Text{Content: "EdgeCuts Example"},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
						},
					},
				},
			},
		},
		{
			name: "combined_operations",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  400,
						Height: 300,
					},
					Fill: api.Color{Hex: "#f5f5f5"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
						Bottom: api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
						Left:   api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
						Right:  api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
					},
				},
				Circles: []CircleShape{
					{X: 100, Y: 75, Diameter: 25, Label: "C1"},
					{X: 300, Y: 75, Diameter: 25, Label: "C2"},
					{X: 100, Y: 225, Diameter: 25, Label: "C3"},
					{X: 300, Y: 225, Diameter: 25, Label: "C4"},
				},
				Cuts: []Cut{
					{Orientation: "vertical", Position: 200, Width: 15, Label: "Center Cut"},
				},
				EdgeCuts: []EdgeCut{
					{Edge: "left", Width: 25, Label: "L"},
					{Edge: "right", Width: 25, Label: "R"},
				},
				Labels: []Label{
					{
						Text: api.Text{Content: "Combined Operations"},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalTop, Inside: InsideBottom},
						},
					},
				},
				ShowDimensions: true,
				ActualWidth:    400,
				ActualHeight:   300,
				DimensionUnit:  "mm",
			},
		},
	}

	// Create output directory
	os.MkdirAll("out", 0o755)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svgData, err := tt.box.GenerateSVG()
			if err != nil {
				t.Fatalf("Failed to generate SVG: %v", err)
			}

			// Save to file
			filename := fmt.Sprintf("out/operations_%s.svg", tt.name)
			err = os.WriteFile(filename, svgData, 0o644)
			if err != nil {
				t.Fatalf("Failed to write SVG file: %v", err)
			}

			t.Logf("SVG saved to: %s", filename)
		})
	}
}

// TestMeasureLines tests measure lines with different configurations
func TestMeasureLines(t *testing.T) {
	tests := []struct {
		name string
		box  SVGBox
	}{
		{
			name: "horizontal_measure_lines",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  300,
						Height: 200,
					},
					Fill: api.Color{Hex: "#ffffff"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Bottom: api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Left:   api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Right:  api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
					},
				},
				MeasureLines: []MeasureLine{
					{X1: 0, Y1: 200, X2: 300, Y2: 200, Label: "300mm", Offset: 20, ShowArrows: true},
					{X1: 0, Y1: 200, X2: 100, Y2: 200, Label: "100mm", Offset: 40, ShowArrows: true},
					{X1: 100, Y1: 200, X2: 200, Y2: 200, Label: "100mm", Offset: 40, ShowArrows: true},
					{X1: 200, Y1: 200, X2: 300, Y2: 200, Label: "100mm", Offset: 40, ShowArrows: true},
				},
				Labels: []Label{
					{
						Text: api.Text{Content: "Horizontal Measure Lines"},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
						},
					},
				},
			},
		},
		{
			name: "vertical_measure_lines",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  300,
						Height: 200,
					},
					Fill: api.Color{Hex: "#ffffff"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Bottom: api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Left:   api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Right:  api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
					},
				},
				MeasureLines: []MeasureLine{
					{X1: 0, Y1: 0, X2: 0, Y2: 200, Label: "200mm", Offset: -20, ShowArrows: true},
					{X1: 0, Y1: 0, X2: 0, Y2: 50, Label: "50mm", Offset: -40, ShowArrows: true},
					{X1: 0, Y1: 50, X2: 0, Y2: 150, Label: "100mm", Offset: -40, ShowArrows: true},
					{X1: 0, Y1: 150, X2: 0, Y2: 200, Label: "50mm", Offset: -40, ShowArrows: true},
				},
				Labels: []Label{
					{
						Text: api.Text{Content: "Vertical Measure Lines"},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
						},
					},
				},
			},
		},
		{
			name: "mixed_measure_lines",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  400,
						Height: 300,
					},
					Fill: api.Color{Hex: "#f0f0f0"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
						Bottom: api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
						Left:   api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
						Right:  api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
					},
				},
				MeasureLines: []MeasureLine{
					// Horizontal dimensions
					{X1: 0, Y1: 300, X2: 400, Y2: 300, Label: "400mm", Offset: 20, ShowArrows: true},
					{X1: 0, Y1: 0, X2: 150, Y2: 0, Label: "150mm", Offset: -20, ShowArrows: true, Style: "dashed"},
					{X1: 250, Y1: 0, X2: 400, Y2: 0, Label: "150mm", Offset: -20, ShowArrows: true, Style: "dashed"},
					// Vertical dimensions
					{X1: 0, Y1: 0, X2: 0, Y2: 300, Label: "300mm", Offset: -20, ShowArrows: true},
					{X1: 400, Y1: 0, X2: 400, Y2: 100, Label: "100mm", Offset: 20, ShowArrows: true, Style: "dashed"},
					{X1: 400, Y1: 200, X2: 400, Y2: 300, Label: "100mm", Offset: 20, ShowArrows: true, Style: "dashed"},
				},
				Circles: []CircleShape{
					{X: 150, Y: 100, Diameter: 30, Label: ""},
					{X: 250, Y: 100, Diameter: 30, Label: ""},
					{X: 150, Y: 200, Diameter: 30, Label: ""},
					{X: 250, Y: 200, Diameter: 30, Label: ""},
				},
				Labels: []Label{
					{
						Text: api.Text{Content: "Mixed Measure Lines"},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
						},
					},
				},
			},
		},
	}

	// Create output directory
	os.MkdirAll("out", 0o755)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svgData, err := tt.box.GenerateSVG()
			if err != nil {
				t.Fatalf("Failed to generate SVG: %v", err)
			}

			// Save to file
			filename := fmt.Sprintf("out/measure_lines_%s.svg", tt.name)
			err = os.WriteFile(filename, svgData, 0o644)
			if err != nil {
				t.Fatalf("Failed to write SVG file: %v", err)
			}

			t.Logf("SVG saved to: %s", filename)
		})
	}
}

// TestCollisionAvoidance tests label collision scenarios
func TestCollisionAvoidance(t *testing.T) {
	tests := []struct {
		name string
		box  SVGBox
	}{
		{
			name: "overlapping_center_labels",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  250,
						Height: 200,
					},
					Fill: api.Color{Hex: "#ffffff"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#333"}, Width: 1},
						Bottom: api.Line{Color: api.Color{Hex: "#333"}, Width: 1},
						Left:   api.Line{Color: api.Color{Hex: "#333"}, Width: 1},
						Right:  api.Line{Color: api.Color{Hex: "#333"}, Width: 1},
					},
				},
				Labels: []Label{
					{
						Text: api.Text{Content: "Center Label 1", Class: api.Class{Font: &api.Font{Size: 16}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
						},
					},
					{
						Text: api.Text{Content: "Center Label 2", Class: api.Class{Font: &api.Font{Size: 14}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
						},
					},
					{
						Text: api.Text{Content: "Center Label 3", Class: api.Class{Font: &api.Font{Size: 12}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
						},
					},
				},
			},
		},
		{
			name: "crowded_edges",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  300,
						Height: 250,
					},
					Fill: api.Color{Hex: "#f5f5f5"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
						Bottom: api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
						Left:   api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
						Right:  api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
					},
				},
				Labels: []Label{
					// Top edge labels
					{
						Text: api.Text{Content: "Top Left", Class: api.Class{Font: &api.Font{Size: 12}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalTop, Horizontal: HorizontalLeft},
						},
					},
					{
						Text: api.Text{Content: "Top Center", Class: api.Class{Font: &api.Font{Size: 12}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalTop, Horizontal: HorizontalCenter},
						},
					},
					{
						Text: api.Text{Content: "Top Right", Class: api.Class{Font: &api.Font{Size: 12}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalTop, Horizontal: HorizontalRight},
						},
					},
					// Bottom edge labels
					{
						Text: api.Text{Content: "Bottom Left", Class: api.Class{Font: &api.Font{Size: 12}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalBottom, Horizontal: HorizontalLeft},
						},
					},
					{
						Text: api.Text{Content: "Bottom Center", Class: api.Class{Font: &api.Font{Size: 12}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalBottom, Horizontal: HorizontalCenter},
						},
					},
					{
						Text: api.Text{Content: "Bottom Right", Class: api.Class{Font: &api.Font{Size: 12}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalBottom, Horizontal: HorizontalRight},
						},
					},
				},
			},
		},
		{
			name: "labels_with_operations",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  350,
						Height: 250,
					},
					Fill: api.Color{Hex: "#ffffff"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Bottom: api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Left:   api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Right:  api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
					},
				},
				Circles: []CircleShape{
					{X: 175, Y: 125, Diameter: 50, Label: "Main Hole"},
				},
				Labels: []Label{
					{
						Text: api.Text{Content: "This label should avoid the hole", Class: api.Class{Font: &api.Font{Size: 14}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
						},
					},
					{
						Text: api.Text{Content: "Top Label", Class: api.Class{Font: &api.Font{Size: 12}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalTop, Horizontal: HorizontalCenter},
						},
					},
					{
						Text: api.Text{Content: "Bottom Label", Class: api.Class{Font: &api.Font{Size: 12}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalBottom, Horizontal: HorizontalCenter},
						},
					},
				},
			},
		},
	}

	// Create output directory
	os.MkdirAll("out", 0o755)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svgData, err := tt.box.GenerateSVG()
			if err != nil {
				t.Fatalf("Failed to generate SVG: %v", err)
			}

			// Save to file
			filename := fmt.Sprintf("out/collision_%s.svg", tt.name)
			err = os.WriteFile(filename, svgData, 0o644)
			if err != nil {
				t.Fatalf("Failed to write SVG file: %v", err)
			}

			t.Logf("SVG saved to: %s", filename)
		})
	}
}

// TestViewboxAdjustment tests that viewbox adjusts to fit all content
func TestViewboxAdjustment(t *testing.T) {
	tests := []struct {
		name string
		box  SVGBox
	}{
		{
			name: "outside_labels",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  200,
						Height: 150,
					},
					Fill: api.Color{Hex: "#e0e0e0"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
						Bottom: api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
						Left:   api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
						Right:  api.Line{Color: api.Color{Hex: "#000"}, Width: 2},
					},
				},
				Labels: []Label{
					{
						Text: api.Text{Content: "Top Outside Label - Should Not Be Cut Off"},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalTop, Inside: InsideBottom},
						},
					},
					{
						Text: api.Text{Content: "Bottom Outside Label - Should Not Be Cut Off"},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalBottom, Inside: InsideBottom},
						},
					},
					{
						Text: api.Text{Content: "Left Outside Label"},
						Positionable: Positionable{
							Position: &LabelPosition{Horizontal: HorizontalLeft, Inside: InsideBottom},
						},
					},
					{
						Text: api.Text{Content: "Right Outside Label"},
						Positionable: Positionable{
							Position: &LabelPosition{Horizontal: HorizontalRight, Inside: InsideBottom},
						},
					},
				},
			},
		},
		{
			name: "extended_measure_lines",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  250,
						Height: 200,
					},
					Fill: api.Color{Hex: "#ffffff"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Bottom: api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Left:   api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Right:  api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
					},
				},
				MeasureLines: []MeasureLine{
					// Far offset measure lines that should expand viewbox
					{X1: 0, Y1: 200, X2: 250, Y2: 200, Label: "250mm Total", Offset: 60, ShowArrows: true},
					{X1: 0, Y1: 0, X2: 0, Y2: 200, Label: "200mm Height", Offset: -60, ShowArrows: true},
					{X1: 250, Y1: 0, X2: 250, Y2: 200, Label: "200mm", Offset: 40, ShowArrows: true},
					{X1: 0, Y1: 0, X2: 250, Y2: 0, Label: "250mm Width", Offset: -40, ShowArrows: true},
				},
				Labels: []Label{
					{
						Text: api.Text{Content: "Extended Measure Lines"},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
						},
					},
				},
			},
		},
	}

	// Create output directory
	os.MkdirAll("out", 0o755)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svgData, err := tt.box.GenerateSVG()
			if err != nil {
				t.Fatalf("Failed to generate SVG: %v", err)
			}

			// Save to file
			filename := fmt.Sprintf("out/viewbox_%s.svg", tt.name)
			err = os.WriteFile(filename, svgData, 0o644)
			if err != nil {
				t.Fatalf("Failed to write SVG file: %v", err)
			}

			t.Logf("SVG saved to: %s", filename)
		})
	}
}

// TestSmartCollisionAvoidance tests collision avoidance when enabled
func TestSmartCollisionAvoidance(t *testing.T) {
	tests := []struct {
		name string
		box  SVGBox
	}{
		{
			name: "smart_positioning_with_hole",
			box: SVGBox{
				Box: api.Box{
					Rectangle: api.Rectangle{
						Width:  400,
						Height: 300,
					},
					Fill: api.Color{Hex: "#ffffff"},
					Border: api.Borders{
						Top:    api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Bottom: api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Left:   api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
						Right:  api.Line{Color: api.Color{Hex: "#000"}, Width: 1},
					},
				},
				Circles: []CircleShape{
					{X: 200, Y: 150, Diameter: 60, Label: "Large Hole"},
				},
				Labels: []Label{
					{
						Text: api.Text{Content: "Center Label - Should Move Away From Hole", Class: api.Class{Font: &api.Font{Size: 14}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
						},
					},
					{
						Text: api.Text{Content: "Another Center Label", Class: api.Class{Font: &api.Font{Size: 12}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
						},
					},
					{
						Text: api.Text{Content: "Third Center Label", Class: api.Class{Font: &api.Font{Size: 10}}},
						Positionable: Positionable{
							Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
						},
					},
				},
				EnableCollisionAvoidance: true,
			},
		},
	}

	// Create output directory
	os.MkdirAll("out", 0o755)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svgData, err := tt.box.GenerateSVG()
			if err != nil {
				t.Fatalf("Failed to generate SVG: %v", err)
			}

			// Save to file
			filename := fmt.Sprintf("out/smart_collision_%s.svg", tt.name)
			err = os.WriteFile(filename, svgData, 0o644)
			if err != nil {
				t.Fatalf("Failed to write SVG file: %v", err)
			}

			t.Logf("SVG saved to: %s", filename)
		})
	}
}
