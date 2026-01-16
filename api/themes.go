package api

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/commons/logger"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// Color represents a color with hex value and optional transparency.
type Color struct {
	Hex     string
	Opacity float64
}

// Font contains typography styling information including weight,
// size, colors, and text decorations.
type Font struct {
	Name          string
	Weight        string
	Size          float64
	Background    Color
	Foreground    Color
	Bold          bool
	Faint         bool
	Italic        bool
	Underline     bool
	Strikethrough bool
}

type LineStyle string

const (
	Solid  LineStyle = "solid"
	Dashed LineStyle = "dashed"
	Dotted LineStyle = "dotted"
	Double LineStyle = "double"
	None   LineStyle = "none"
)

type LineEndStyle string

const (
	LineEndStyleNone    LineEndStyle = "none"
	LineEndStyleArrow   LineEndStyle = "arrow"
	LineEndStyleDiamond LineEndStyle = "diamond"
)

type Line struct {
	Color      Color
	Style      LineStyle
	Width      float64
	EndStyle   LineEndStyle
	StartStyle LineEndStyle
}

type Circle struct {
	Color    Color
	Border   Line
	Diameter float64
}

// Padding defines spacing around content in CSS box model format using Point units.
type Padding struct {
	Top    Point
	Right  Point
	Bottom Point
	Left   Point
}

// Helper methods for conversion to MM (for layout calculations)

// TopMM returns the top padding converted to millimeters
func (p *Padding) TopMM() float64 {
	return p.Top.ToMM()
}

// RightMM returns the right padding converted to millimeters
func (p *Padding) RightMM() float64 {
	return p.Right.ToMM()
}

// BottomMM returns the bottom padding converted to millimeters
func (p *Padding) BottomMM() float64 {
	return p.Bottom.ToMM()
}

// LeftMM returns the left padding converted to millimeters
func (p *Padding) LeftMM() float64 {
	return p.Left.ToMM()
}

// Box represents a styled rectangular container with fill, borders, and padding.
type Box struct {
	Rectangle
	Fill    Color
	Border  Borders
	Padding Padding
}

type Borders struct {
	Left   Line
	Right  Line
	Top    Line
	Bottom Line
}

type Rectangle struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

type Position struct {
	X int `json:"x,omitempty"`
	Y int `json:"y,omitempty"`
}

func (p Position) RelativeTo(other Position) Position {
	return Position{
		X: p.X + other.X,
		Y: p.Y + other.Y,
	}
}

// Class groups styling properties that can be applied to text elements,
// similar to CSS classes but with structured property access.
type Class struct {
	Name       string
	Background *Color
	Foreground *Color
	Font       *Font
	Padding    *Padding
	Border     *Borders
}

// Theme provides a consistent color palette for semantic styling
// across different UI states (success, error, warning, etc.).
type Theme struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Error     lipgloss.Color
	Info      lipgloss.Color
	Muted     lipgloss.Color
}

func DefaultTheme() Theme {
	return Theme{
		Primary:   lipgloss.Color("#8A2BE2"), // BlueViolet
		Secondary: lipgloss.Color("#4169E1"), // RoyalBlue
		Success:   lipgloss.Color("#32CD32"), // LimeGreen
		Warning:   lipgloss.Color("#FFD700"), // Gold
		Error:     lipgloss.Color("#FF6347"), // Tomato
		Info:      lipgloss.Color("#00CED1"), // DarkTurquoise
		Muted:     lipgloss.Color("#808080"), // Gray
	}
}

func DarkTheme() Theme {
	return Theme{
		Primary:   lipgloss.Color("#BB86FC"), // Purple
		Secondary: lipgloss.Color("#03DAC6"), // Teal
		Success:   lipgloss.Color("#4CAF50"), // Green
		Warning:   lipgloss.Color("#FF9800"), // Orange
		Error:     lipgloss.Color("#F44336"), // Red
		Info:      lipgloss.Color("#2196F3"), // Blue
		Muted:     lipgloss.Color("#9E9E9E"), // Gray
	}
}

func LightTheme() Theme {
	return Theme{
		Primary:   lipgloss.Color("#6200EA"), // Deep Purple
		Secondary: lipgloss.Color("#00BCD4"), // Cyan
		Success:   lipgloss.Color("#388E3C"), // Dark Green
		Warning:   lipgloss.Color("#F57C00"), // Dark Orange
		Error:     lipgloss.Color("#D32F2F"), // Dark Red
		Info:      lipgloss.Color("#1976D2"), // Dark Blue
		Muted:     lipgloss.Color("#757575"), // Dark Gray
	}
}

// NoTTYTheme provides colorless output suitable for pipes and non-interactive contexts.
func NoTTYTheme() Theme {
	noColor := lipgloss.Color("")
	return Theme{
		Primary:   noColor,
		Secondary: noColor,
		Success:   noColor,
		Warning:   noColor,
		Error:     noColor,
		Info:      noColor,
		Muted:     noColor,
	}
}

// AutoTheme selects an appropriate theme by detecting terminal capabilities
// and background color, falling back to NoTTYTheme for non-interactive output.
func AutoTheme() Theme {
	if !isTerminal() {
		return NoTTYTheme()
	}

	// Detect terminal background and choose appropriate theme
	if termenv.HasDarkBackground() {
		return DarkTheme()
	}
	return LightTheme()
}

var terminalWidth = -1
var terminalDimensionsLogged = false

func GetTerminalWidth() int {
	if terminalWidth != -1 {
		return terminalWidth
	}
	width, height, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		return 120 // Default width
	}
	terminalWidth = width

	// Log terminal dimensions once at startup when trace logging is enabled
	if !terminalDimensionsLogged && logger.V(4).Enabled() {
		terminalDimensionsLogged = true

		// Create test lines with box drawing character
		halfWidth := width / 2
		line50 := strings.Repeat("─", halfWidth)
		line100 := strings.Repeat("─", width)

		logger.V(4).Infof("Terminal dimensions: width=%d, height=%d", width, height)
		_, _ = os.Stderr.WriteString(line50 + "\n")
		_, _ = os.Stderr.WriteString(line100 + "\n")
	}

	return width
}

var terminalHeight = -1

func GetTerminalLines() int {
	if terminalHeight != -1 {
		return terminalHeight
	}
	_, height, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		return 40 // Default height
	}
	terminalHeight = height
	return height
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}
