package pdf

import (
	"bytes"
	"fmt"
	
	"github.com/ajstarks/svgo"
	"github.com/flanksource/clicky/api"
)

// SVGBox generates an SVG representation of a box with labels and borders
type SVGBox struct {
	api.Box              // Inherits Rectangle, Fill, Border, Padding
	Labels               []Label
	Lines                []Line
	Circles              []CircleShape
	Cuts                 []Cut
	EdgeCuts             []EdgeCut
	MeasureLines         []MeasureLine
	ShowDimensions       bool
	DimensionUnit        string
	ActualWidth          float64
	ActualHeight         float64
	SVGPadding           float64 // Padding for the SVG canvas
	EnableCollisionAvoidance bool // Enable automatic collision avoidance
}

// CircleShape represents a circular shape in the box
type CircleShape struct {
	X        float64
	Y        float64
	Diameter float64
	Depth    float64
	Label    string
}

// Cut represents a rectangular cut in the box
type Cut struct {
	Orientation string  // "horizontal" or "vertical"
	Position    float64
	Width       float64
	Depth       float64
	Label       string
}

// EdgeCut represents a cut along an edge of the box
type EdgeCut struct {
	Edge  string // "left", "right", "top", "bottom"
	Width float64
	Depth float64
	Label string
}

// MeasureLine represents a dimension line with arrows and label
type MeasureLine struct {
	X1         float64
	Y1         float64
	X2         float64
	Y2         float64
	Label      string
	Offset     float64 // Distance from the measured object
	ShowArrows bool
	Style      string // "solid" or "dashed"
}

// LabelBounds represents the rectangular bounds of a label
type LabelBounds struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
	Label  *Label
}

// PositionedLabel represents a label with calculated position
type PositionedLabel struct {
	Label   Label
	X       int
	Y       int
	Anchor  string
	Bounds  LabelBounds
}

// GenerateSVG creates an SVG representation of the box
func (b SVGBox) GenerateSVG() ([]byte, error) {
	var buf bytes.Buffer
	canvas := svg.New(&buf)
	
	// Calculate dynamic padding based on labels and measure lines
	padding := b.calculateDynamicPadding()
	
	width := float64(b.Rectangle.Width)
	height := float64(b.Rectangle.Height)
	actualW := b.ActualWidth
	actualH := b.ActualHeight
	if actualW == 0 {
		actualW = width
	}
	if actualH == 0 {
		actualH = height
	}
	
	svgWidth := int(width + padding.Left + padding.Right)
	svgHeight := int(height + padding.Top + padding.Bottom)
	boxX := int(padding.Left)
	boxY := int(padding.Top)
	
	// Start SVG
	canvas.Start(svgWidth, svgHeight)
	
	// Draw main rectangle with fill
	fillColor := b.colorToHex(b.Fill)
	
	// Draw the main box (initially without stroke, we'll draw borders separately)
	style := fmt.Sprintf("fill:%s;stroke:none", fillColor)
	canvas.Rect(boxX, boxY, int(width), int(height), style)
	
	// Draw borders
	b.drawBorders(canvas, boxX, boxY, int(width), int(height))
	
	// Draw circles
	for _, circle := range b.Circles {
		cx := boxX + int(circle.X)
		cy := boxY + int(circle.Y)
		r := int(circle.Diameter / 2)
		canvas.Circle(cx, cy, r, "fill:rgba(0,0,0,0.2);stroke:black;stroke-width:0.5")
		
		if circle.Label != "" {
			labelY := cy + r + 12
			canvas.Text(cx, labelY, circle.Label, "text-anchor:middle;font-size:10px;fill:#666")
		}
	}
	
	// Draw cuts
	for _, cut := range b.Cuts {
		var dx, dy, dw, dh int
		
		if cut.Orientation == "vertical" {
			dx = boxX + int(cut.Position-cut.Width/2)
			dy = boxY
			dw = int(cut.Width)
			dh = int(height)
		} else {
			dx = boxX
			dy = boxY + int(cut.Position-cut.Width/2)
			dw = int(width)
			dh = int(cut.Width)
		}
		
		canvas.Rect(dx, dy, dw, dh, "fill:rgba(0,0,0,0.15);stroke:black;stroke-width:0.5;stroke-dasharray:3,3")
		
		if cut.Label != "" {
			labelX := dx + dw/2
			labelY := dy + dh/2
			canvas.Text(labelX, labelY, cut.Label, "text-anchor:middle;font-size:10px;fill:#666")
		}
	}
	
	// Draw edge cuts
	for _, edgeCut := range b.EdgeCuts {
		var rx, ry, rw, rh int
		
		switch edgeCut.Edge {
		case "left":
			rx = boxX
			ry = boxY
			rw = int(edgeCut.Width)
			rh = int(height)
		case "right":
			rx = boxX + int(width) - int(edgeCut.Width)
			ry = boxY
			rw = int(edgeCut.Width)
			rh = int(height)
		case "top":
			rx = boxX
			ry = boxY
			rw = int(width)
			rh = int(edgeCut.Width)
		case "bottom":
			rx = boxX
			ry = boxY + int(height) - int(edgeCut.Width)
			rw = int(width)
			rh = int(edgeCut.Width)
		}
		
		canvas.Rect(rx, ry, rw, rh, "fill:rgba(0,0,0,0.1);stroke:black;stroke-width:0.5;stroke-dasharray:4,2")
		
		if edgeCut.Label != "" {
			labelX := rx + rw/2
			labelY := ry + rh/2
			canvas.Text(labelX, labelY, edgeCut.Label, "text-anchor:middle;font-size:10px;fill:#666")
		}
	}
	
	// Draw measure lines
	for _, ml := range b.MeasureLines {
		b.drawMeasureLine(canvas, ml, boxX, boxY)
	}
	
	// Draw labels with collision avoidance
	if b.EnableCollisionAvoidance {
		positionedLabels := b.calculateCollisionFreePositions(boxX, boxY, int(width), int(height))
		for _, pl := range positionedLabels {
			b.drawPositionedLabel(canvas, pl)
		}
	} else {
		for _, label := range b.Labels {
			b.drawLabel(canvas, label, boxX, boxY, int(width), int(height))
		}
	}
	
	// Draw dimensions if enabled
	if b.ShowDimensions {
		unit := b.DimensionUnit
		if unit == "" {
			unit = "mm"
		}
		
		// Width dimension (bottom)
		widthText := fmt.Sprintf("%.0f%s", actualW, unit)
		canvas.Text(boxX+int(width)/2, boxY+int(height)+30, widthText, 
			"text-anchor:middle;font-size:12px;fill:#333")
		
		// Draw dimension line
		y := boxY + int(height) + 20
		canvas.Line(boxX, y, boxX+int(width), y, "stroke:#333;stroke-width:0.5")
		// Arrows
		canvas.Line(boxX, y-3, boxX, y+3, "stroke:#333;stroke-width:0.5")
		canvas.Line(boxX+int(width), y-3, boxX+int(width), y+3, "stroke:#333;stroke-width:0.5")
		
		// Height dimension (left)
		heightText := fmt.Sprintf("%.0f%s", actualH, unit)
		// Rotate text for vertical dimension
		transform := fmt.Sprintf("rotate(-90 %d %d)", boxX-30, boxY+int(height)/2)
		canvas.Gtransform(transform)
		canvas.Text(boxX-30, boxY+int(height)/2, heightText, 
			"text-anchor:middle;font-size:12px;fill:#333")
		canvas.Gend()
		
		// Draw dimension line
		x := boxX - 20
		canvas.Line(x, boxY, x, boxY+int(height), "stroke:#333;stroke-width:0.5")
		// Arrows
		canvas.Line(x-3, boxY, x+3, boxY, "stroke:#333;stroke-width:0.5")
		canvas.Line(x-3, boxY+int(height), x+3, boxY+int(height), "stroke:#333;stroke-width:0.5")
	}
	
	canvas.End()
	return buf.Bytes(), nil
}

// drawBorders draws individual borders with custom styles
func (b SVGBox) drawBorders(canvas *svg.SVG, x, y, w, h int) {
	// Helper function to get border style string from Line
	getLineStyle := func(line api.Line) string {
		color := b.colorToHex(line.Color)
		width := int(line.Width)
		if width == 0 {
			width = 1
		}
		
		style := fmt.Sprintf("stroke:%s;stroke-width:%d", color, width)
		
		if line.Style == api.Dashed {
			style += ";stroke-dasharray:5,5"
		} else if line.Style == api.Dotted {
			style += ";stroke-dasharray:1,2"
		}
		
		return style
	}
	
	// Draw each border
	// Top border
	if b.Border.Top.Width > 0 || b.Border.Top.Color.Hex != "" {
		canvas.Line(x, y, x+w, y, getLineStyle(b.Border.Top))
	}
	// Bottom border
	if b.Border.Bottom.Width > 0 || b.Border.Bottom.Color.Hex != "" {
		canvas.Line(x, y+h, x+w, y+h, getLineStyle(b.Border.Bottom))
	}
	// Left border
	if b.Border.Left.Width > 0 || b.Border.Left.Color.Hex != "" {
		canvas.Line(x, y, x, y+h, getLineStyle(b.Border.Left))
	}
	// Right border
	if b.Border.Right.Width > 0 || b.Border.Right.Color.Hex != "" {
		canvas.Line(x+w, y, x+w, y+h, getLineStyle(b.Border.Right))
	}
}

// drawLabel draws a label at the specified position
func (b SVGBox) drawLabel(canvas *svg.SVG, label Label, boxX, boxY, boxW, boxH int) {
	text := label.Text.Content
	if text == "" {
		return
	}
	
	// Calculate label position
	var x, y int
	anchor := "middle"
	
	// Default to center if no position specified
	pos := "center"
	if label.Position != nil && label.Position.Horizontal != "" {
		if label.Position.Vertical != "" {
			pos = string(label.Position.Vertical) + "-" + string(label.Position.Horizontal)
		} else {
			pos = string(label.Position.Horizontal)
		}
	} else if label.Position != nil && label.Position.Vertical != "" {
		pos = string(label.Position.Vertical)
	}
	
	// Parse position and calculate coordinates
	switch pos {
	case "center":
		x = boxX + boxW/2
		y = boxY + boxH/2
	case "top":
		x = boxX + boxW/2
		y = boxY - 10
	case "bottom":
		x = boxX + boxW/2
		y = boxY + boxH + 20
	case "left":
		x = boxX - 10
		y = boxY + boxH/2
		anchor = "end"
	case "right":
		x = boxX + boxW + 10
		y = boxY + boxH/2
		anchor = "start"
	case "top-left":
		x = boxX + 20
		y = boxY + 20
		anchor = "start"
	case "top-right":
		x = boxX + boxW - 20
		y = boxY + 20
		anchor = "end"
	case "bottom-left":
		x = boxX + 20
		y = boxY + boxH - 10
		anchor = "start"
	case "bottom-right":
		x = boxX + boxW - 20
		y = boxY + boxH - 10
		anchor = "end"
	default:
		// Default to center
		x = boxX + boxW/2
		y = boxY + boxH/2
	}
	
	// Apply absolute position offset if specified
	if label.Absolute != nil {
		x += label.Absolute.X
		y += label.Absolute.Y
	}
	
	// Determine font properties
	fontSize := 14
	fontWeight := "normal"
	fontColor := "#000"
	
	if label.Text.Class.Font != nil {
		if label.Text.Class.Font.Size > 0 {
			fontSize = int(label.Text.Class.Font.Size)
		}
		if label.Text.Class.Font.Bold {
			fontWeight = "bold"
		}
	}
	
	if label.Text.Class.Foreground != nil {
		fontColor = b.colorToHex(*label.Text.Class.Foreground)
	}
	
	style := fmt.Sprintf("text-anchor:%s;font-size:%dpx;font-weight:%s;fill:%s", 
		anchor, fontSize, fontWeight, fontColor)
	
	canvas.Text(x, y, text, style)
}

// drawMeasureLine draws a measure line with arrows and label
func (b SVGBox) drawMeasureLine(canvas *svg.SVG, ml MeasureLine, boxX, boxY int) {
	// Adjust coordinates relative to box position
	x1 := boxX + int(ml.X1)
	y1 := boxY + int(ml.Y1)
	x2 := boxX + int(ml.X2)
	y2 := boxY + int(ml.Y2)
	
	isHorizontal := ml.Y1 == ml.Y2
	isVertical := ml.X1 == ml.X2
	
	// Apply offset
	if isHorizontal {
		y1 += int(ml.Offset)
		y2 += int(ml.Offset)
	} else if isVertical {
		x1 += int(ml.Offset)
		x2 += int(ml.Offset)
	}
	
	// Draw extension lines (thin lines from object to measure line)
	if ml.Offset != 0 {
		if isHorizontal {
			// Vertical extension lines
			canvas.Line(boxX+int(ml.X1), boxY+int(ml.Y1), x1, y1, "stroke:#666;stroke-width:0.5")
			canvas.Line(boxX+int(ml.X2), boxY+int(ml.Y2), x2, y2, "stroke:#666;stroke-width:0.5")
		} else if isVertical {
			// Horizontal extension lines
			canvas.Line(boxX+int(ml.X1), boxY+int(ml.Y1), x1, y1, "stroke:#666;stroke-width:0.5")
			canvas.Line(boxX+int(ml.X2), boxY+int(ml.Y2), x2, y2, "stroke:#666;stroke-width:0.5")
		}
	}
	
	// Draw main measure line
	style := "stroke:#000;stroke-width:0.8"
	if ml.Style == "dashed" {
		style += ";stroke-dasharray:5,3"
	}
	canvas.Line(x1, y1, x2, y2, style)
	
	// Draw arrows if enabled
	if ml.ShowArrows {
		if isHorizontal {
			// Left arrow
			canvas.Polygon([]int{x1, x1+8, x1+8}, []int{y1, y1-4, y1+4}, "fill:#000")
			// Right arrow
			canvas.Polygon([]int{x2, x2-8, x2-8}, []int{y2, y2-4, y2+4}, "fill:#000")
		} else if isVertical {
			// Top arrow
			canvas.Polygon([]int{x1, x1-4, x1+4}, []int{y1, y1+8, y1+8}, "fill:#000")
			// Bottom arrow
			canvas.Polygon([]int{x2, x2-4, x2+4}, []int{y2, y2-8, y2-8}, "fill:#000")
		}
	}
	
	// Draw label
	if ml.Label != "" {
		var labelX, labelY int
		if isHorizontal {
			labelX = (x1 + x2) / 2
			labelY = y1 + 15 // Place label below the line
		} else if isVertical {
			labelX = x1 - 15 // Place label to the left of the line
			labelY = (y1 + y2) / 2
		}
		
		canvas.Text(labelX, labelY, ml.Label, "text-anchor:middle;font-size:11px;fill:#333")
	}
}

// PaddingBox represents padding for all four sides
type PaddingBox struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// calculateDynamicPadding calculates padding needed to ensure all elements are visible
func (b SVGBox) calculateDynamicPadding() PaddingBox {
	padding := PaddingBox{
		Top:    50,
		Right:  50,
		Bottom: 50,
		Left:   50,
	}
	
	// Start with SVGPadding if specified
	if b.SVGPadding > 0 {
		padding.Top = b.SVGPadding
		padding.Right = b.SVGPadding
		padding.Bottom = b.SVGPadding
		padding.Left = b.SVGPadding
	}
	
	// Check labels for outside positions
	for _, label := range b.Labels {
		if label.Position == nil {
			continue
		}
		
		// Estimate text size (rough approximation)
		textWidth := float64(len(label.Text.Content) * 8)
		textHeight := 20.0
		
		switch label.Position.Vertical {
		case VerticalTop:
			if label.Position.Inside == InsideBottom { // outside
				padding.Top = max(padding.Top, textHeight+30)
			}
		case VerticalBottom:
			if label.Position.Inside == InsideBottom { // outside
				padding.Bottom = max(padding.Bottom, textHeight+30)
			}
		}
		
		switch label.Position.Horizontal {
		case HorizontalLeft:
			if label.Position.Inside == InsideBottom { // outside
				padding.Left = max(padding.Left, textWidth+20)
			}
		case HorizontalRight:
			if label.Position.Inside == InsideBottom { // outside
				padding.Right = max(padding.Right, textWidth+20)
			}
		}
	}
	
	// Check measure lines
	for _, ml := range b.MeasureLines {
		if ml.Y1 == ml.Y2 { // Horizontal
			if ml.Offset > 0 {
				padding.Bottom = max(padding.Bottom, ml.Offset+30)
			} else if ml.Offset < 0 {
				padding.Top = max(padding.Top, -ml.Offset+30)
			}
		} else if ml.X1 == ml.X2 { // Vertical
			if ml.Offset > 0 {
				padding.Right = max(padding.Right, ml.Offset+30)
			} else if ml.Offset < 0 {
				padding.Left = max(padding.Left, -ml.Offset+30)
			}
		}
	}
	
	// Check dimensions if enabled
	if b.ShowDimensions {
		padding.Bottom = max(padding.Bottom, 40)
		padding.Left = max(padding.Left, 40)
	}
	
	return padding
}

// calculateCollisionFreePositions calculates positions for all labels avoiding collisions
func (b SVGBox) calculateCollisionFreePositions(boxX, boxY, boxW, boxH int) []PositionedLabel {
	var positioned []PositionedLabel
	var occupiedBounds []LabelBounds
	
	// Add obstacles (circles, cuts, edge cuts) to occupied bounds
	obstacles := b.getObstacleBounds(boxX, boxY, boxW, boxH)
	occupiedBounds = append(occupiedBounds, obstacles...)
	
	for _, label := range b.Labels {
		pos := b.findBestLabelPosition(label, boxX, boxY, boxW, boxH, occupiedBounds)
		positioned = append(positioned, pos)
		occupiedBounds = append(occupiedBounds, pos.Bounds)
	}
	
	return positioned
}

// getObstacleBounds returns bounds of all obstacles (holes, dados, rabbets)
func (b SVGBox) getObstacleBounds(boxX, boxY, boxW, boxH int) []LabelBounds {
	var bounds []LabelBounds
	
	// Add circles as obstacles
	for _, circle := range b.Circles {
		cx := float64(boxX) + circle.X
		cy := float64(boxY) + circle.Y
		r := circle.Diameter / 2
		bounds = append(bounds, LabelBounds{
			X:      cx - r - 5, // Add 5px buffer
			Y:      cy - r - 5,
			Width:  circle.Diameter + 10,
			Height: circle.Diameter + 10,
		})
	}
	
	// Add cuts as obstacles
	for _, cut := range b.Cuts {
		var dx, dy, dw, dh float64
		
		if cut.Orientation == "vertical" {
			dx = float64(boxX) + cut.Position - cut.Width/2
			dy = float64(boxY)
			dw = cut.Width
			dh = float64(boxH)
		} else {
			dx = float64(boxX)
			dy = float64(boxY) + cut.Position - cut.Width/2
			dw = float64(boxW)
			dh = cut.Width
		}
		
		bounds = append(bounds, LabelBounds{
			X:      dx - 5,
			Y:      dy - 5,
			Width:  dw + 10,
			Height: dh + 10,
		})
	}
	
	// Add edge cuts as obstacles
	for _, edgeCut := range b.EdgeCuts {
		var rx, ry, rw, rh float64
		
		switch edgeCut.Edge {
		case "left":
			rx = float64(boxX)
			ry = float64(boxY)
			rw = edgeCut.Width
			rh = float64(boxH)
		case "right":
			rx = float64(boxX+boxW) - edgeCut.Width
			ry = float64(boxY)
			rw = edgeCut.Width
			rh = float64(boxH)
		case "top":
			rx = float64(boxX)
			ry = float64(boxY)
			rw = float64(boxW)
			rh = edgeCut.Width
		case "bottom":
			rx = float64(boxX)
			ry = float64(boxY+boxH) - edgeCut.Width
			rw = float64(boxW)
			rh = edgeCut.Width
		}
		
		bounds = append(bounds, LabelBounds{
			X:      rx - 5,
			Y:      ry - 5,
			Width:  rw + 10,
			Height: rh + 10,
		})
	}
	
	return bounds
}

// findBestLabelPosition finds the best collision-free position for a label
func (b SVGBox) findBestLabelPosition(label Label, boxX, boxY, boxW, boxH int, occupied []LabelBounds) PositionedLabel {
	// Estimate label size
	textWidth := float64(len(label.Text.Content) * 8)
	textHeight := 20.0
	
	if label.Text.Class.Font != nil && label.Text.Class.Font.Size > 0 {
		fontSize := float64(label.Text.Class.Font.Size)
		textWidth = float64(len(label.Text.Content)) * fontSize * 0.6
		textHeight = fontSize + 5
	}
	
	// Define possible positions to try (in order of preference)
	positions := []struct {
		x, y   int
		anchor string
		name   string
	}{
		{boxX + boxW/2, boxY + boxH/2, "middle", "center"},
		{boxX + boxW/2, boxY + boxH/2 - 15, "middle", "center-above"},
		{boxX + boxW/2, boxY + boxH/2 + 15, "middle", "center-below"},
		{boxX + boxW/2 - 20, boxY + boxH/2, "middle", "center-left"},
		{boxX + boxW/2 + 20, boxY + boxH/2, "middle", "center-right"},
		{boxX + 20, boxY + 20, "start", "top-left"},
		{boxX + boxW - 20, boxY + 20, "end", "top-right"},
		{boxX + 20, boxY + boxH - 10, "start", "bottom-left"},
		{boxX + boxW - 20, boxY + boxH - 10, "end", "bottom-right"},
	}
	
	// Use original position if specified and available
	if label.Position != nil {
		originalPos := b.calculateOriginalPosition(label, boxX, boxY, boxW, boxH)
		bounds := b.calculateLabelBounds(originalPos.X, originalPos.Y, textWidth, textHeight, originalPos.Anchor)
		if !b.boundsCollide(bounds, occupied) {
			return originalPos
		}
		// If original position collides, fall back to alternatives
	}
	
	// Try each position until we find one without collisions
	for _, pos := range positions {
		bounds := b.calculateLabelBounds(pos.x, pos.y, textWidth, textHeight, pos.anchor)
		if !b.boundsCollide(bounds, occupied) {
			return PositionedLabel{
				Label:  label,
				X:      pos.x,
				Y:      pos.y,
				Anchor: pos.anchor,
				Bounds: bounds,
			}
		}
	}
	
	// If no position works, use center as fallback
	centerBounds := b.calculateLabelBounds(boxX+boxW/2, boxY+boxH/2, textWidth, textHeight, "middle")
	return PositionedLabel{
		Label:  label,
		X:      boxX + boxW/2,
		Y:      boxY + boxH/2,
		Anchor: "middle",
		Bounds: centerBounds,
	}
}

// calculateOriginalPosition calculates the original intended position for a label
func (b SVGBox) calculateOriginalPosition(label Label, boxX, boxY, boxW, boxH int) PositionedLabel {
	var x, y int
	anchor := "middle"
	
	// Default to center if no position specified
	pos := "center"
	if label.Position != nil && label.Position.Horizontal != "" {
		if label.Position.Vertical != "" {
			pos = string(label.Position.Vertical) + "-" + string(label.Position.Horizontal)
		} else {
			pos = string(label.Position.Horizontal)
		}
	} else if label.Position != nil && label.Position.Vertical != "" {
		pos = string(label.Position.Vertical)
	}
	
	// Calculate position
	switch pos {
	case "center":
		x = boxX + boxW/2
		y = boxY + boxH/2
	case "top":
		x = boxX + boxW/2
		y = boxY - 10
	case "bottom":
		x = boxX + boxW/2
		y = boxY + boxH + 20
	case "left":
		x = boxX - 10
		y = boxY + boxH/2
		anchor = "end"
	case "right":
		x = boxX + boxW + 10
		y = boxY + boxH/2
		anchor = "start"
	case "top-left":
		x = boxX + 20
		y = boxY + 20
		anchor = "start"
	case "top-right":
		x = boxX + boxW - 20
		y = boxY + 20
		anchor = "end"
	case "bottom-left":
		x = boxX + 20
		y = boxY + boxH - 10
		anchor = "start"
	case "bottom-right":
		x = boxX + boxW - 20
		y = boxY + boxH - 10
		anchor = "end"
	default:
		x = boxX + boxW/2
		y = boxY + boxH/2
	}
	
	// Apply absolute position offset if specified
	if label.Absolute != nil {
		x += label.Absolute.X
		y += label.Absolute.Y
	}
	
	// Estimate text size for bounds
	textWidth := float64(len(label.Text.Content) * 8)
	textHeight := 20.0
	if label.Text.Class.Font != nil && label.Text.Class.Font.Size > 0 {
		fontSize := float64(label.Text.Class.Font.Size)
		textWidth = float64(len(label.Text.Content)) * fontSize * 0.6
		textHeight = fontSize + 5
	}
	
	bounds := b.calculateLabelBounds(x, y, textWidth, textHeight, anchor)
	
	return PositionedLabel{
		Label:  label,
		X:      x,
		Y:      y,
		Anchor: anchor,
		Bounds: bounds,
	}
}

// calculateLabelBounds calculates the rectangular bounds for a label at the given position
func (b SVGBox) calculateLabelBounds(x, y int, textWidth, textHeight float64, anchor string) LabelBounds {
	var boundsX, boundsY float64
	
	switch anchor {
	case "start":
		boundsX = float64(x)
		boundsY = float64(y) - textHeight/2
	case "end":
		boundsX = float64(x) - textWidth
		boundsY = float64(y) - textHeight/2
	case "middle":
		boundsX = float64(x) - textWidth/2
		boundsY = float64(y) - textHeight/2
	default:
		boundsX = float64(x) - textWidth/2
		boundsY = float64(y) - textHeight/2
	}
	
	return LabelBounds{
		X:      boundsX,
		Y:      boundsY,
		Width:  textWidth,
		Height: textHeight,
	}
}

// boundsCollide checks if a label bounds collides with any occupied bounds
func (b SVGBox) boundsCollide(labelBounds LabelBounds, occupied []LabelBounds) bool {
	for _, occ := range occupied {
		if labelBounds.X < occ.X+occ.Width &&
			labelBounds.X+labelBounds.Width > occ.X &&
			labelBounds.Y < occ.Y+occ.Height &&
			labelBounds.Y+labelBounds.Height > occ.Y {
			return true
		}
	}
	return false
}

// drawPositionedLabel draws a positioned label
func (b SVGBox) drawPositionedLabel(canvas *svg.SVG, pl PositionedLabel) {
	label := pl.Label
	text := label.Text.Content
	if text == "" {
		return
	}
	
	// Determine font properties
	fontSize := 14
	fontWeight := "normal"
	fontColor := "#000"
	
	if label.Text.Class.Font != nil {
		if label.Text.Class.Font.Size > 0 {
			fontSize = int(label.Text.Class.Font.Size)
		}
		if label.Text.Class.Font.Bold {
			fontWeight = "bold"
		}
	}
	
	if label.Text.Class.Foreground != nil {
		fontColor = b.colorToHex(*label.Text.Class.Foreground)
	}
	
	style := fmt.Sprintf("text-anchor:%s;font-size:%dpx;font-weight:%s;fill:%s", 
		pl.Anchor, fontSize, fontWeight, fontColor)
	
	canvas.Text(pl.X, pl.Y, text, style)
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// ImportFromSVG parses SVG content and adds elements to this SVGBox
func (b *SVGBox) ImportFromSVG(svgContent string) error {
	importer := NewSVGImporter()
	elements, err := importer.ImportSVG(svgContent)
	if err != nil {
		return fmt.Errorf("failed to import SVG: %w", err)
	}
	
	// Add imported elements to the SVGBox
	b.Circles = append(b.Circles, elements.Circles...)
	b.Cuts = append(b.Cuts, elements.Cuts...)
	b.EdgeCuts = append(b.EdgeCuts, elements.EdgeCuts...)
	b.Labels = append(b.Labels, elements.Labels...)
	
	return nil
}

// colorToHex converts an api.Color to a hex string
func (b SVGBox) colorToHex(color api.Color) string {
	if color.Hex != "" {
		if len(color.Hex) > 0 && color.Hex[0] != '#' {
			return "#" + color.Hex
		}
		return color.Hex
	}
	
	// Default to black if no color specified
	return "#000000"
}