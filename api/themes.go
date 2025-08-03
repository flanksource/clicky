package api

import "github.com/charmbracelet/lipgloss"

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
