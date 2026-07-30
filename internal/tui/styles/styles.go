package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	Primary   = lipgloss.Color("#7C3AED") // violet
	Secondary = lipgloss.Color("#06B6D4") // cyan
	Success   = lipgloss.Color("#10B981") // emerald
	Warning   = lipgloss.Color("#F59E0B") // amber
	Error     = lipgloss.Color("#EF4444") // red
	Muted     = lipgloss.Color("#6B7280") // gray
	Text      = lipgloss.Color("#F3F4F6") // light gray
	Bg        = lipgloss.Color("#1F2937") // dark bg

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
