package styles

import "github.com/charmbracelet/lipgloss"

// Rose Pine palette — single source of truth (Gentleman-Cute refresh).
var (
	ColorBase     = lipgloss.Color("#191724")
	ColorSurface  = lipgloss.Color("#1f1d2e")
	ColorOverlay  = lipgloss.Color("#6e6a86")
	ColorText     = lipgloss.Color("#e0def4")
	ColorSubtext  = lipgloss.Color("#908caa")
	ColorLavender = lipgloss.Color("#c4a7e7")
	ColorGreen    = lipgloss.Color("#9ccfd8")
	ColorPeach    = lipgloss.Color("#f6c177")
	ColorRed      = lipgloss.Color("#eb6f92")
	ColorBlue     = lipgloss.Color("#31748f")
	ColorMauve    = lipgloss.Color("#ebbcba")
	ColorYellow   = lipgloss.Color("#f1ca93")
	ColorTeal     = lipgloss.Color("#9ccfd8")
)

const Cursor = "▸ "

func Tagline(version string) string {
	return "Gentle-AI " + version + " — Ecosystem, Frameworks, Workflows"
}

var (
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorLavender).
			Bold(true)

	HeadingStyle = lipgloss.NewStyle().
			Foreground(ColorMauve).
			Bold(true)

	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorSubtext)

	SubtextStyle = lipgloss.NewStyle().
			Foreground(ColorSubtext)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(ColorLavender).
			Bold(true)

	UnselectedStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorGreen)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorRed)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorYellow)

	FrameStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorLavender).
			Padding(1, 2)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorOverlay).
			Padding(0, 1)

	ProgressFilled = lipgloss.NewStyle().
			Foreground(ColorGreen)

	ProgressEmpty = lipgloss.NewStyle().
			Foreground(ColorOverlay)

	PercentStyle = lipgloss.NewStyle().
			Foreground(ColorPeach).
			Bold(true)
)

var (
	// Legacy aliases — now map to Rose Pine (single source of truth).
	Primary   = ColorLavender
	Secondary = ColorGreen
	Success   = ColorGreen
	Warning   = ColorYellow
	Error     = ColorRed
	Muted     = ColorSubtext
	Text      = ColorText
	Bg        = ColorBase

	// Title style
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		Padding(0, 1).
		MarginBottom(1)

	// Section header
	Section = lipgloss.NewStyle().
		Bold(true).
		Foreground(Secondary).
		MarginTop(1).
		MarginBottom(1)

	// Menu item
	MenuItem = lipgloss.NewStyle().
		Foreground(Text).
		Padding(0, 2)

	MenuItemSelected = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(Primary).
		Padding(0, 2)

	MenuItemKey = lipgloss.NewStyle().
		Foreground(Secondary).
		Bold(true)

	MenuItemDesc = lipgloss.NewStyle().
		Foreground(Muted).
		Italic(true)

	// Status
	StatusEnabled  = lipgloss.NewStyle().Foreground(Success).Bold(true)
	StatusDisabled = lipgloss.NewStyle().Foreground(Error).Bold(true)
	StatusInfo     = lipgloss.NewStyle().Foreground(Muted)

	// Help text
	Help = lipgloss.NewStyle().
		Foreground(Muted).
		PaddingTop(1)

	HelpKey = lipgloss.NewStyle().
		Foreground(Secondary).
		Bold(true)

	HelpDesc = lipgloss.NewStyle().
		Foreground(Muted)

	// Buttons
	Button = lipgloss.NewStyle().
		Background(Primary).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 3).
		MarginRight(1)

	ButtonActive = lipgloss.NewStyle().
		Background(Secondary).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 3).
		MarginRight(1)

	ButtonInactive = lipgloss.NewStyle().
		Background(Muted).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 3)

	// Checkbox
	CheckboxSelected = lipgloss.NewStyle().Foreground(Success).SetString("● ")
	CheckboxEmpty    = lipgloss.NewStyle().Foreground(Muted).SetString("○ ")

	// Input
	InputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Secondary).
			Padding(0, 1)

	// Loading spinner
	Spinner = lipgloss.NewStyle().Foreground(Secondary)

	// Error box
	ErrorBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Error).
			Padding(1, 2).
			Foreground(Error)

	// Warning box
	WarningBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Warning).
			Padding(1, 2).
			Foreground(Warning)

	// Success box
	SuccessBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Success).
			Padding(1, 2).
			Foreground(Success)

	// App border
	AppStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(1, 2)
)
