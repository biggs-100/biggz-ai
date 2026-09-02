package styles

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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

// ── Rank3: Theme engine status-line + card tokens (single source via theme.go) ──
// Rose Pine remains dark default for TitleStyle/PanelStyle; these tokens mirror
// oh-my-pi dark.json 40-token palette (see theme.go DarkPalette / dark.json).
// Single source: Go DarkPalette + JSON dark.json + solarized-osaka.json (second built-in).
// Hex values below match ansi256ToHex(244)=#808080, 70=#5faf00, 178=#d7af00, 39=#00afff, 205=#ff5faf
// so styles and theme engine stay unified (no divergent palettes).
var (
	ColorStatusLineBg      = lipgloss.Color("#121212") // DarkPalette.StatusLineBg via theme.go (single source)
	ColorStatusLineFg      = ColorText
	ColorStatusLineSep     = lipgloss.Color("#808080") // 244 → #808080 via theme.go DarkPalette
	ColorStatusLineModel   = lipgloss.Color("#d787af")
	ColorStatusLinePath    = lipgloss.Color("#00afaf")
	ColorStatusLineGitClean = lipgloss.Color("#5faf5f")
	ColorStatusLineGitDirty = lipgloss.Color("#d7af5f")
	ColorStatusLineContext = lipgloss.Color("#8787af")
	ColorStatusLineSpend   = lipgloss.Color("#5fafaf")
	ColorStatusLineStaged  = lipgloss.Color("#5faf00")   // 70 → #5faf00 via theme.go
	ColorStatusLineDirty   = lipgloss.Color("#d7af00")   // 178 → #d7af00 via theme.go
	ColorStatusLineUntracked = lipgloss.Color("#00afff") // 39 → #00afff via theme.go
	ColorStatusLineOutput  = lipgloss.Color("#ff5faf")   // 205 → #ff5faf via theme.go
	ColorStatusLineCost    = lipgloss.Color("#ff5faf")   // 205
	ColorStatusLineSubagents = ColorLavender // accent alias, theme accent is #febc38
	ColorToolPendingBg     = lipgloss.Color("#1d2129")   // DarkPalette.ToolPendingBg
	ColorToolSuccessBg     = lipgloss.Color("#161a1f")   // DarkPalette.ToolSuccessBg
	ColorToolErrorBg       = lipgloss.Color("#291d1d")   // DarkPalette.ToolErrorBg
	ColorToolTitle         = ColorText
	ColorAccent            = lipgloss.Color("#febc38")   // DarkPalette.Accent
)

var (
	StatusLineStyle = lipgloss.NewStyle().
			Background(ColorStatusLineBg).
			Foreground(ColorStatusLineFg)

	StatusLineModelStyle = lipgloss.NewStyle().
			Foreground(ColorStatusLineModel).
			Bold(true)

	StatusLinePathStyle = lipgloss.NewStyle().
			Foreground(ColorStatusLinePath)

	StatusLineGitCleanStyle = lipgloss.NewStyle().
			Foreground(ColorStatusLineGitClean)

	StatusLineGitDirtyStyle = lipgloss.NewStyle().
			Foreground(ColorStatusLineGitDirty)

	StatusLineContextStyle = lipgloss.NewStyle().
			Foreground(ColorStatusLineContext)

	StatusLineCostStyle = lipgloss.NewStyle().
			Foreground(ColorStatusLineCost)

	ToolPendingStyle = lipgloss.NewStyle().
			Background(ColorToolPendingBg).
			Foreground(ColorText)

	ToolSuccessStyle = lipgloss.NewStyle().
			Background(ColorToolSuccessBg).
			Foreground(ColorText)
)

// ── PR2: Pill tokens with icon/color/spinner per state ─────────────────────
var (
	PillRunningStyle  = lipgloss.NewStyle().Background(ColorToolPendingBg).Foreground(ColorLavender).Bold(true).Padding(0, 1).MarginRight(1)
	PillQueuedStyle   = lipgloss.NewStyle().Background(ColorOverlay).Foreground(ColorText).Padding(0, 1).MarginRight(1)
	PillCompleteStyle = lipgloss.NewStyle().Background(ColorToolSuccessBg).Foreground(ColorGreen).Padding(0, 1).MarginRight(1)
	PillFailedStyle   = lipgloss.NewStyle().Background(ColorToolErrorBg).Foreground(ColorRed).Bold(true).Padding(0, 1).MarginRight(1)

	// Token aliases per spec (PillRunning/Queued/Complete/Failed)
	PillRunning  = PillRunningStyle
	PillQueued   = PillQueuedStyle
	PillComplete = PillCompleteStyle
	PillFailed   = PillFailedStyle
)

// PillStyle returns lipgloss style per pill state. Honors BIGGZ_PRETTY=0 plain fallback and TERM=dumb strip ANSI.
func PillStyle(state string) lipgloss.Style {
	if !IsPrettyEnabled() {
		return lipgloss.NewStyle()
	}
	if os.Getenv("TERM") == "dumb" {
		return lipgloss.NewStyle()
	}
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running", "pending", "streaming", "live", "in_progress", "in-progress":
		return PillRunningStyle
	case "queued":
		return PillQueuedStyle
	case "complete", "success", "done":
		return PillCompleteStyle
	case "failed", "error":
		return PillFailedStyle
	default:
		return lipgloss.NewStyle().Background(ColorSurface).Foreground(ColorText).Padding(0, 1).MarginRight(1)
	}
}

// PillIcon returns icon per state, with BIGGZ_PRETTY=0 ASCII and TERM=dumb fallbacks.
func PillIcon(state string) string {
	if !IsPrettyEnabled() || os.Getenv("TERM") == "dumb" {
		switch strings.ToLower(strings.TrimSpace(state)) {
		case "running":
			return ">"
		case "queued":
			return "-"
		case "complete", "success":
			return "+"
		case "failed", "error":
			return "x"
		default:
			return "·"
		}
	}
	if os.Getenv("BIGGZ_NO_ANIMATION") == "1" || os.Getenv("GENTLE_AI_NO_ANIMATION") == "1" {
		if strings.EqualFold(strings.TrimSpace(state), "running") {
			return "·"
		}
	}
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running":
		return "⠋"
	case "queued":
		return "◷"
	case "complete", "success":
		return "✓"
	case "failed", "error":
		return "✗"
	default:
		return "·"
	}
}

// GetStatusLineStyle returns preset-aware status-line base style.
// When BIGGZ_PRETTY=0 or PI_SUBAGENT_CHILD=1, falls back to Rose Pine static style.
// Otherwise delegates to the active Theme (dark/light auto via COLORFGBG → theme.go).
func GetStatusLineStyle(preset string) lipgloss.Style {
	if IsPrettyEnabled() {
		th := CurrentTheme()
		if th != nil {
			return th.GetStatusLineStyle(preset)
		}
	}
	switch preset {
	case "minimal":
		return StatusLineStyle.Copy().Foreground(ColorSubtext)
	case "compact":
		return StatusLineStyle.Copy().Foreground(ColorMauve)
	default:
		return StatusLineStyle
	}
}

// GetTheme is a package-level alias for theme.go GetTheme (preset optional).
// Kept here for single-source ergonomics: styles.GetTheme("dark", styles.SymbolPresetNerd)
// Previously divergent palettes (Go styles vs pi/themes/solarized-osaka.json) now unified
// via theme.go loader (GetTheme reads JSON on disk with fallback to DarkPalette/Rose Pine).
func GetThemeByName(name string) *Theme { return GetTheme(name) }

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

	// Table styles — shared single source (Rose Pine)
	TableHeader = lipgloss.NewStyle().
			Foreground(ColorSubtext).
			Background(ColorSurface).
			Bold(true).
			Padding(0, 1)

	TableSelected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(Primary).
			Bold(true).
			Padding(0, 1)

	PreviewPane = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorOverlay).
			Padding(0, 1)

	ModalOverlay = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorPeach).
			Padding(1, 2).
			Background(ColorBase)
)
