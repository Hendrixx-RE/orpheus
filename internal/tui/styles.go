package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Name      string
	Base      lipgloss.Color
	Surface   lipgloss.Color
	Border    lipgloss.Color
	BorderFoc lipgloss.Color
	Text      lipgloss.Color
	Muted     lipgloss.Color
	Green     lipgloss.Color
	Yellow    lipgloss.Color
	Red       lipgloss.Color
	Cyan      lipgloss.Color
	Purple    lipgloss.Color
	Orange    lipgloss.Color
}

var Themes = []Theme{
	{
		Name:      "Gruvbox Retro",
		Base:      lipgloss.Color("#282828"),
		Surface:   lipgloss.Color("#3c3836"),
		Border:    lipgloss.Color("#504945"),
		BorderFoc: lipgloss.Color("#d79921"),
		Text:      lipgloss.Color("#ebdbb2"),
		Muted:     lipgloss.Color("#a89984"),
		Green:     lipgloss.Color("#b8bb26"),
		Yellow:    lipgloss.Color("#fabd2f"),
		Red:       lipgloss.Color("#fb4934"),
		Cyan:      lipgloss.Color("#8ec07c"),
		Purple:    lipgloss.Color("#d3869b"),
		Orange:    lipgloss.Color("#fe8019"),
	},
	{
		Name:      "Catppuccin",
		Base:      lipgloss.Color("#1e1e2e"),
		Surface:   lipgloss.Color("#313244"),
		Border:    lipgloss.Color("#45475a"),
		BorderFoc: lipgloss.Color("#cba6f7"),
		Text:      lipgloss.Color("#cdd6f4"),
		Muted:     lipgloss.Color("#6c7086"),
		Green:     lipgloss.Color("#a6e3a1"),
		Yellow:    lipgloss.Color("#f9e2af"),
		Red:       lipgloss.Color("#f38ba8"),
		Cyan:      lipgloss.Color("#89dceb"),
		Purple:    lipgloss.Color("#cba6f7"),
		Orange:    lipgloss.Color("#fab387"),
	},
	{
		Name:      "Monokai",
		Base:      lipgloss.Color("#272822"),
		Surface:   lipgloss.Color("#3e3d32"),
		Border:    lipgloss.Color("#49483e"),
		BorderFoc: lipgloss.Color("#66d9ef"),
		Text:      lipgloss.Color("#f8f8f2"),
		Muted:     lipgloss.Color("#75715e"),
		Green:     lipgloss.Color("#a6e22e"),
		Yellow:    lipgloss.Color("#e6db74"),
		Red:       lipgloss.Color("#f92672"),
		Cyan:      lipgloss.Color("#66d9ef"),
		Purple:    lipgloss.Color("#ae81ff"),
		Orange:    lipgloss.Color("#fd971f"),
	},
}

var (
	colorBase      = Themes[0].Base
	colorSurface   = Themes[0].Surface
	colorBorder    = Themes[0].Border
	colorBorderFoc = Themes[0].BorderFoc
	colorText      = Themes[0].Text
	colorMuted     = Themes[0].Muted
	colorGreen     = Themes[0].Green
	colorYellow    = Themes[0].Yellow
	colorRed       = Themes[0].Red
	colorCyan      = Themes[0].Cyan
	colorPurple    = Themes[0].Purple
	colorOrange    = Themes[0].Orange

	stylePanel         lipgloss.Style
	stylePanelFocused  lipgloss.Style
	styleTitle         lipgloss.Style
	styleSelected      lipgloss.Style
	styleOrphan        lipgloss.Style
	styleExplicit      lipgloss.Style
	styleKey           lipgloss.Style
	styleVal           lipgloss.Style
	styleDimmed        lipgloss.Style
	styleAILabel       lipgloss.Style
	styleVerdict       lipgloss.Style
	styleSidebarActive lipgloss.Style
	styleStatusBar     lipgloss.Style
	styleDivider       lipgloss.Style
)

func init() {
	ApplyTheme(Themes[0])
}

func ApplyTheme(t Theme) {
	colorBase = t.Base
	colorSurface = t.Surface
	colorBorder = t.Border
	colorBorderFoc = t.BorderFoc
	colorText = t.Text
	colorMuted = t.Muted
	colorGreen = t.Green
	colorYellow = t.Yellow
	colorRed = t.Red
	colorCyan = t.Cyan
	colorPurple = t.Purple
	colorOrange = t.Orange

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

	styleOrphan = lipgloss.NewStyle().Foreground(colorRed)
	styleExplicit = lipgloss.NewStyle().Foreground(colorCyan)

	styleKey = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	styleVal = lipgloss.NewStyle().Foreground(colorText)
	styleDimmed = lipgloss.NewStyle().Foreground(colorMuted)
	styleAILabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	styleVerdict = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)

	styleSidebarActive = lipgloss.NewStyle().
		Foreground(colorYellow).
		Bold(true)

	styleStatusBar = lipgloss.NewStyle().
		Foreground(colorMuted)

	styleDivider = lipgloss.NewStyle().Foreground(colorBorder)
}
