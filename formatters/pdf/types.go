package pdf

import (
	"strings"

	"github.com/flanksource/clicky/api"
)

type (
	VerticalPosition   string
	HorizontalPosition string
	InsidePosition     string
	LabelPosition      struct {
		Vertical   VerticalPosition
		Horizontal HorizontalPosition
		Inside     InsidePosition
	}
)

const (
	VerticalTop      VerticalPosition   = "top"
	VerticalBottom   VerticalPosition   = "bottom"
	VerticalCenter   VerticalPosition   = "" // center is the default
	HorizontalLeft   HorizontalPosition = "left"
	HorizontalRight  HorizontalPosition = "right"
	HorizontalCenter HorizontalPosition = "" // center is the default
	InsideTop        InsidePosition     = "" // inside is the default
	InsideBottom     InsidePosition     = "outside"
)

type Positionable struct {
	Position *LabelPosition
	// If both position and absolute is provided, absolute is relative to position
	Absolute *api.Position
}

// ParsePosition parses position strings like "center", "top-left", "bottom-right-outside"
func ParsePosition(s string) LabelPosition {
	if s == "" {
		return LabelPosition{} // Default center position
	}

	parts := strings.Split(strings.ToLower(s), "-")
	pos := LabelPosition{}

	for _, part := range parts {
		switch part {
		case "top":
			pos.Vertical = VerticalTop
		case "bottom":
			pos.Vertical = VerticalBottom
		case "center", "middle":
			pos.Vertical = VerticalCenter
		case "left":
			pos.Horizontal = HorizontalLeft
		case "right":
			pos.Horizontal = HorizontalRight
		case "outside":
			pos.Inside = InsideBottom
		case "inside":
			pos.Inside = InsideTop
		}
	}

	return pos
}

type Label struct {
	Positionable
	api.Text
}

type Line struct {
	Positionable
	api.Line
	Labels []Label
}

type Circle struct {
	Positionable
	api.Circle
	Labels []Label
}

type Box struct {
	api.Rectangle
	Borders *api.Borders
	Labels  []Label
	Lines   []Line
}
