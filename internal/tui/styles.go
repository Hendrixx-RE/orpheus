package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Gruvbox Dark
	colorBase      = lipgloss.Color("#282828")
	colorBorder    = lipgloss.Color("#504945")
	colorBorderFoc = lipgloss.Color("#d79921")
	colorText      = lipgloss.Color("#ebdbb2")
	colorMuted     = lipgloss.Color("#a89984")
	colorGreen     = lipgloss.Color("#b8bb26")
	colorYellow    = lipgloss.Color("#fabd2f")
	colorRed       = lipgloss.Color("#fb4934")
	colorCyan      = lipgloss.Color("#8ec07c")
	colorPurple    = lipgloss.Color("#d3869b")
	colorOrange    = lipgloss.Color("#fe8019")

	stylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder)

	stylePanelFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorderFoc)

	styleTitle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	styleSelected = lipgloss.NewStyle().
			Foreground(colorBase).
			Background(colorYellow).
			Bold(true)

	styleOrphan   = lipgloss.NewStyle().Foreground(colorRed)
	styleExplicit = lipgloss.NewStyle().Foreground(colorCyan)
	styleDep      = lipgloss.NewStyle().Foreground(colorMuted)

	styleHealthHigh = lipgloss.NewStyle().Foreground(colorRed)
	styleHealthMid  = lipgloss.NewStyle().Foreground(colorOrange)
	styleHealthLow  = lipgloss.NewStyle().Foreground(colorGreen)

	styleKey     = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	styleVal     = lipgloss.NewStyle().Foreground(colorText)
	styleDimmed  = lipgloss.NewStyle().Foreground(colorMuted)
	styleAILabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	styleVerdict = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)

	styleSidebarActive = lipgloss.NewStyle().
				Foreground(colorYellow).
				Bold(true)

	styleStatusBar = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleDivider = lipgloss.NewStyle().Foreground(colorBorder)
)
