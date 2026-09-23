package api

import (
	"os"
	"sync/atomic"

	"github.com/flanksource/clicky/api/tailwind"
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

// ThemeColor is a terminal color as lipgloss accepts it: a hex value such as
// "#8A2BE2", an ANSI index such as "21", or "" for no color. It is a plain
// string so the theme does not tie package api to lipgloss; renderers convert
// it with lipgloss.Color(theme.X).
type ThemeColor string

// Theme provides a consistent color palette for semantic styling
// across different UI states (success, error, warning, etc.).
type Theme struct {
	Primary   ThemeColor
	Secondary ThemeColor
	Success   ThemeColor
	Warning   ThemeColor
	Error     ThemeColor
	Info      ThemeColor
	Muted     ThemeColor
}

func DefaultTheme() Theme {
	return Theme{
		Primary:   "#8A2BE2", // BlueViolet
		Secondary: "#4169E1", // RoyalBlue
		Success:   "#32CD32", // LimeGreen
		Warning:   "#FFD700", // Gold
		Error:     "#FF6347", // Tomato
		Info:      "#00CED1", // DarkTurquoise
		Muted:     "#808080", // Gray
	}
}

func DarkTheme() Theme {
	return Theme{
		Primary:   "#BB86FC", // Purple
		Secondary: "#03DAC6", // Teal
		Success:   "#4CAF50", // Green
		Warning:   "#FF9800", // Orange
		Error:     "#F44336", // Red
		Info:      "#2196F3", // Blue
		Muted:     "#9E9E9E", // Gray
	}
}

func LightTheme() Theme {
	return Theme{
		Primary:   "#6200EA", // Deep Purple
		Secondary: "#00BCD4", // Cyan
		Success:   "#388E3C", // Dark Green
		Warning:   "#F57C00", // Dark Orange
		Error:     "#D32F2F", // Dark Red
		Info:      "#1976D2", // Dark Blue
		Muted:     "#757575", // Dark Gray
	}
}

// NoTTYTheme provides colorless output suitable for pipes and non-interactive contexts.
func NoTTYTheme() Theme {
	noColor := ThemeColor("")
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

const (
	// unmeasuredTerminalSize marks the cache as empty. Any non-positive size
	// reported by the terminal is treated as unmeasured too — see
	// GetTerminalWidth.
	unmeasuredTerminalSize = -1

	defaultTerminalWidth = 120
	defaultTerminalLines = 40
)

var (
	terminalWidth  atomic.Int32
	terminalHeight atomic.Int32
)

func init() {
	InvalidateTerminalSize()
	tailwind.TerminalWidthFunc = GetTerminalWidth
	tailwind.TerminalHeightFunc = GetTerminalLines
	watchTerminalResize()
}

// InvalidateTerminalSize drops the cached terminal size so the next read
// re-measures. Called on SIGWINCH; the size is otherwise sampled once per
// process and a resize would leave every frame laid out for the old width.
func InvalidateTerminalSize() {
	terminalWidth.Store(unmeasuredTerminalSize)
	terminalHeight.Store(unmeasuredTerminalSize)
}

// SetTerminalWidth pins the width every width-aware renderer lays out against.
// A non-positive width is not a width: it clears the cache, so the next read
// re-measures rather than laying every frame out against a bogus size.
func SetTerminalWidth(width int) {
	if width <= 0 {
		terminalWidth.Store(unmeasuredTerminalSize)
		return
	}
	terminalWidth.Store(int32(width))
}

// GetTerminalWidth returns the terminal width, always positive: it falls back
// to defaultTerminalWidth whenever the width cannot be measured.
//
// term.GetSize reports (0, 0, nil) — success, zero size — for a pty created
// without a window size, which every non-interactive `script`/CI capture hits.
// Caching that zero pinned the width at 0 for the whole process, and
// width-aware renderers (api/meta.go buildLipglossTree) then truncated every
// label to a single `…`. A non-positive size is unmeasured, never a real width,
// so it is neither returned nor cached.
func GetTerminalWidth() int {
	if w := terminalWidth.Load(); w > 0 {
		return int(w)
	}
	width, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil || width <= 0 {
		return defaultTerminalWidth
	}
	terminalWidth.Store(int32(width))

	return width
}

// SetTerminalLines pins the height, with the same rule as SetTerminalWidth: a
// non-positive height clears the cache instead of being stored as one.
func SetTerminalLines(height int) {
	if height <= 0 {
		terminalHeight.Store(unmeasuredTerminalSize)
		return
	}
	terminalHeight.Store(int32(height))
}

// GetTerminalLines returns the terminal height, always positive, falling back
// to defaultTerminalLines. Non-positive sizes are unmeasured — see
// GetTerminalWidth.
func GetTerminalLines() int {
	if h := terminalHeight.Load(); h > 0 {
		return int(h)
	}
	_, height, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil || height <= 0 {
		return defaultTerminalLines
	}
	terminalHeight.Store(int32(height))
	return height
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}
