package api

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

type Color struct {
	Hex     string
	Opacity float64
}

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

type Padding struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

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

type Class struct {
	Name       string
	Background *Color
	Foreground *Color
	Font       *Font
	Padding    *Padding
	Border     *Borders
}

// Theme defines color schemes and styles
type Theme struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Error     lipgloss.Color
	Info      lipgloss.Color
	Muted     lipgloss.Color
}

// DefaultTheme returns the default color theme
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

// DarkTheme returns a dark color theme
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

// LightTheme returns a light color theme
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

// NoTTYTheme returns a theme for non-terminal output (no colors)
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

// AutoTheme automatically selects a theme based on the terminal environment
func AutoTheme() Theme {
	// Check if output is a terminal
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

func GetTerminalWidth() int {
	if terminalWidth != -1 {
		return terminalWidth
	}
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80 // Default width
	}
	terminalWidth = width
	return width
}

// isTerminal checks if stdout is a terminal
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
