package styles

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
)

// ── Symbol Preset ─────────────────────────────────────────────────────────────

type SymbolPreset string

const (
	SymbolPresetUnicode SymbolPreset = "unicode"
	SymbolPresetNerd    SymbolPreset = "nerd"
	SymbolPresetASCII   SymbolPreset = "ascii"
)

func IsValidSymbolPreset(s string) bool {
	return s == string(SymbolPresetUnicode) || s == string(SymbolPresetNerd) || s == string(SymbolPresetASCII)
}

func ParseSymbolPreset(s string) SymbolPreset {
	switch s {
	case "nerd":
		return SymbolPresetNerd
	case "ascii":
		return SymbolPresetASCII
	default:
		return SymbolPresetUnicode
	}
}

type SpinnerType string

const (
	SpinnerStatus   SpinnerType = "status"
	SpinnerActivity SpinnerType = "activity"
)

// SPINNER_FRAMES mirrors oh-my-pi symbols.ts SPINNER_FRAMES.
var SpinnerFrames = map[SymbolPreset]map[SpinnerType][]string{
	SymbolPresetUnicode: {
		SpinnerStatus:   {"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		SpinnerActivity: {"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	},
	SymbolPresetNerd: {
		SpinnerStatus:   {"󱑖", "󱑋", "󱑌", "󱑍", "󱑎", "󱑏", "󱑐", "󱑑", "󱑒", "󱑓", "󱑔", "󱑕"},
		SpinnerActivity: {"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	},
	SymbolPresetASCII: {
		SpinnerStatus:   {"|", "/", "-", "\\"},
		SpinnerActivity: {"-", "\\", "|", "/"},
	},
}

func GetSpinnerFrames(preset SymbolPreset, typ SpinnerType) []string {
	if m, ok := SpinnerFrames[preset]; ok {
		if f, ok := m[typ]; ok {
			return f
		}
	}
	// fallback to unicode status
	return SpinnerFrames[SymbolPresetUnicode][SpinnerStatus]
}

// Box and separator presets (subset of symbols.ts).

type BoxPreset struct {
	TopLeft, TopRight, BottomLeft, BottomRight string
	Horizontal, Vertical, Cross                string
	TeeDown, TeeUp, TeeRight, TeeLeft          string
}

var BoxRoundPresets = map[SymbolPreset]BoxPreset{
	SymbolPresetUnicode: {"╭", "╮", "╰", "╯", "─", "│", "┼", "┬", "┴", "├", "┤"},
	SymbolPresetNerd:    {"╭", "╮", "╰", "╯", "─", "│", "┼", "┬", "┴", "├", "┤"},
	SymbolPresetASCII:   {"+", "+", "+", "+", "-", "|", "+", "+", "+", "+", "+"},
}

var BoxSharpPresets = map[SymbolPreset]BoxPreset{
	SymbolPresetUnicode: {"┌", "┐", "└", "┘", "─", "│", "┼", "┬", "┴", "├", "┤"},
	SymbolPresetNerd:    {"┌", "┐", "└", "┘", "─", "│", "┼", "┬", "┴", "├", "┤"},
	SymbolPresetASCII:   {"+", "+", "+", "+", "-", "|", "+", "+", "+", "+", "+"},
}

type SepPreset struct {
	Powerline, PowerlineThin, PowerlineLeft, PowerlineRight string
	Block, Space                                            string
	Dot, Slash, Pipe                                        string
}

var SepPresets = map[SymbolPreset]SepPreset{
	SymbolPresetUnicode: {"▕", "┆", "▶", "◀", "▌", " ", " · ", " / ", " │ "},
	SymbolPresetNerd:    {"\ue0b0", "\ue0b1", "\ue0b0", "\ue0b2", "█", " ", " · ", "\ue0bb", "\ue0b3"},
	SymbolPresetASCII:   {">", ">", ">", "<", "#", " ", " - ", " / ", " | "},
}

// ThinkingLevel mirrors ThinkingLevel | Effort in theme-class.ts.
type ThinkingLevel string

const (
	ThinkingOff     ThinkingLevel = "off"
	ThinkingMinimal ThinkingLevel = "minimal"
	ThinkingLow     ThinkingLevel = "low"
	ThinkingMedium  ThinkingLevel = "medium"
	ThinkingHigh    ThinkingLevel = "high"
	ThinkingXHigh   ThinkingLevel = "xhigh"
	ThinkingMax     ThinkingLevel = "max"
)

// LANG_BRAND_COLORS mirrors theme-class.ts LANG_BRAND_COLORS.
var LangBrandColors = map[string]string{
	"lang.javascript": "#f7df1e",
	"lang.python":     "#3776ab",
	"lang.ruby":       "#cc342d",
	"lang.julia":      "#9558b2",
}

// ── Color utilities (port of pi-utils/color.ts) ────────────────────────────

type RGB struct{ R, G, B int }

func hexToRGB(hex string) (RGB, bool) {
	h := strings.TrimPrefix(hex, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return RGB{}, false
	}
	r, err1 := strconv.ParseInt(h[0:2], 16, 0)
	g, err2 := strconv.ParseInt(h[2:4], 16, 0)
	b, err3 := strconv.ParseInt(h[4:6], 16, 0)
	if err1 != nil || err2 != nil || err3 != nil {
		return RGB{}, false
	}
	return RGB{int(r), int(g), int(b)}, true
}

var basicColors = [16]RGB{
	{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
	{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
	{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
	{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

var cubeSteps = [6]int{0, 95, 135, 175, 215, 255}

func paletteToRGB(index int) (RGB, bool) {
	if index < 0 || index > 255 {
		return RGB{}, false
	}
	if index < 16 {
		return basicColors[index], true
	}
	if index < 232 {
		n := index - 16
		r := cubeSteps[(n/36)%6]
		g := cubeSteps[(n/6)%6]
		b := cubeSteps[n%6]
		return RGB{r, g, b}, true
	}
	gray := 8 + (index-232)*10
	return RGB{gray, gray, gray}, true
}

func toRGB(value string) (RGB, bool) {
	if strings.HasPrefix(value, "#") {
		if len(value) != 4 && len(value) != 7 {
			return RGB{}, false
		}
		return hexToRGB(value)
	}
	// numeric string like "244" from JSON numbers parsed as string? handle
	if n, err := strconv.Atoi(value); err == nil {
		return paletteToRGB(n)
	}
	return RGB{}, false
}

func toRGBFromAny(v interface{}) (RGB, bool) {
	switch val := v.(type) {
	case string:
		if val == "" {
			return RGB{}, false
		}
		return toRGB(val)
	case float64:
		return paletteToRGB(int(val))
	case int:
		return paletteToRGB(val)
	default:
		return RGB{}, false
	}
}

type HSV struct{ H, S, V float64 }

func rgbToHSV(rgb RGB) HSV {
	r := float64(rgb.R) / 255
	g := float64(rgb.G) / 255
	b := float64(rgb.B) / 255
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	d := max - min
	var h float64
	if d != 0 {
		if max == r {
			h = (g - b) / d
			if g < b {
				h += 6
			}
		} else if max == g {
			h = (b-r)/d + 2
		} else {
			h = (r-g)/d + 4
		}
		h /= 6
	}
	var s float64
	if max != 0 {
		s = d / max
	}
	return HSV{h * 360, s, max}
}

func hsvToRGB(hsv HSV) RGB {
	h := math.Mod(math.Mod(hsv.H, 360)+360, 360)
	s := hsv.S
	v := hsv.V
	i := int(math.Floor(h / 60))
	f := h/60 - float64(i)
	p := v * (1 - s)
	q := v * (1 - f*s)
	t := v * (1 - (1-f)*s)
	var r, g, b float64
	switch i % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return RGB{int(math.Round(r * 255)), int(math.Round(g * 255)), int(math.Round(b * 255))}
}

func rgbToHex(rgb RGB) string {
	return fmt.Sprintf("#%02x%02x%02x", rgb.R, rgb.G, rgb.B)
}

func hexToHSV(hex string) (HSV, bool) {
	rgb, ok := hexToRGB(hex)
	if !ok {
		return HSV{}, false
	}
	return rgbToHSV(rgb), true
}

func hsvToHex(hsv HSV) string { return rgbToHex(hsvToRGB(hsv)) }

// AdjustHSV mirrors pi-utils adjustHsv: h additive degrees, s/v multipliers.
type HSVAdjustment struct{ H, S, V *float64 }

func AdjustHsv(hex string, adj HSVAdjustment) string {
	hsv, ok := hexToHSV(hex)
	if !ok {
		return hex
	}
	if adj.H != nil {
		hsv.H = math.Mod(hsv.H+*adj.H, 360)
		if hsv.H < 0 {
			hsv.H += 360
		}
	}
	if adj.S != nil {
		hsv.S = math.Max(0, math.Min(1, hsv.S**adj.S))
	}
	if adj.V != nil {
		hsv.V = math.Max(0, math.Min(1, hsv.V**adj.V))
	}
	return hsvToHex(hsv)
}
func adjustHsv(hex string, adj HSVAdjustment) string { return AdjustHsv(hex, adj) }

func floatPtr(v float64) *float64 { return &v }

// colorBlind adjustment: green -> blue (h+60, s*0.71)
var colorBlindAdjustment = HSVAdjustment{H: floatPtr(60), S: floatPtr(0.71)}

// Ansi256ToHex mirrors color.ts ansi256ToHex.
func Ansi256ToHex(index int) string {
	if c, ok := paletteToRGB(index); ok {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return "#000000"
}

// ColorLuma perceptual luma 0..1 (BT.709 over raw sRGB) — for light/dark classification.
func ColorLuma(value string) (float64, bool) {
	rgb, ok := toRGB(value)
	if !ok {
		return 0, false
	}
	return (0.2126*float64(rgb.R) + 0.7152*float64(rgb.G) + 0.0722*float64(rgb.B)) / 255.0, true
}

// ColorLumaAny accepts hex string or palette index (int/float64) like theme-class.ts.
func ColorLumaAny(v interface{}) (float64, bool) {
	rgb, ok := toRGBFromAny(v)
	if !ok {
		return 0, false
	}
	return (0.2126*float64(rgb.R) + 0.7152*float64(rgb.G) + 0.0722*float64(rgb.B)) / 255.0, true
}

func linearizeChannel(ch int) float64 {
	c := float64(ch) / 255.0
	if c <= 0.04045 {
		return c / 12.92
	}
	return pow((c+0.055)/1.055, 2.4)
}

// pow helper without math import for tiny usage (avoid init).
func pow(a, b float64) float64 {
	// use standard library for accuracy
	// inline to avoid import cycle - we can import math
	return mathPow(a, b)
}

// RelativeLuminance WCAG 2.x linearized sRGB.
func RelativeLuminance(value string) (float64, bool) {
	rgb, ok := toRGB(value)
	if !ok {
		return 0, false
	}
	return 0.2126*linearizeChannel(rgb.R) + 0.7152*linearizeChannel(rgb.G) + 0.0722*linearizeChannel(rgb.B), true
}

func RelativeLuminanceAny(v interface{}) (float64, bool) {
	rgb, ok := toRGBFromAny(v)
	if !ok {
		return 0, false
	}
	return 0.2126*linearizeChannel(rgb.R) + 0.7152*linearizeChannel(rgb.G) + 0.0722*linearizeChannel(rgb.B), true
}

// ResolveToHex mirrors color.ts resolveToHex.
func ResolveToHex(value string, isLight bool) string {
	if value == "" {
		if isLight {
			return "#000000"
		}
		return "#e5e5e7"
	}
	if strings.HasPrefix(value, "#") {
		return value
	}
	if n, err := strconv.Atoi(value); err == nil {
		return Ansi256ToHex(n)
	}
	return value
}

func resolveToHexAny(v interface{}, isLight bool) string {
	switch val := v.(type) {
	case string:
		if val == "" {
			if isLight {
				return "#000000"
			}
			return "#e5e5e7"
		}
		if strings.HasPrefix(val, "#") {
			return val
		}
		if n, err := strconv.Atoi(val); err == nil {
			return Ansi256ToHex(n)
		}
		return val
	case float64:
		return Ansi256ToHex(int(val))
	case int:
		return Ansi256ToHex(val)
	default:
		if isLight {
			return "#000000"
		}
		return "#e5e5e7"
	}
}

// colorToAnsi produces 24-bit ANSI for hex.
func colorToAnsi(hex string, mode ColorMode) string {
	rgb, ok := hexToRGB(hex)
	if !ok {
		return ""
	}
	if mode == ColorMode256 {
		// approximate to 256 palette? For now emit 24-bit even in 256 mode for fidelity.
		// Keep truecolor escape; terminal will downsample.
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", rgb.R, rgb.G, rgb.B)
}

func fgAnsi(value interface{}, mode ColorMode) string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return "\x1b[39m"
		}
		if strings.HasPrefix(v, "#") {
			return colorToAnsi(v, mode)
		}
		// var ref already resolved elsewhere; treat as hex
		if _, ok := hexToRGB(v); ok {
			return colorToAnsi(v, mode)
		}
		return "\x1b[39m"
	case float64:
		return fmt.Sprintf("\x1b[38;5;%dm", int(v))
	case int:
		return fmt.Sprintf("\x1b[38;5;%dm", v)
	default:
		return "\x1b[39m"
	}
}

func bgAnsi(value interface{}, mode ColorMode) string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return "\x1b[49m"
		}
		if strings.HasPrefix(v, "#") {
			rgb, ok := hexToRGB(v)
			if !ok {
				return "\x1b[49m"
			}
			return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", rgb.R, rgb.G, rgb.B)
		}
		return "\x1b[49m"
	case float64:
		return fmt.Sprintf("\x1b[48;5;%dm", int(v))
	case int:
		return fmt.Sprintf("\x1b[48;5;%dm", v)
	default:
		return "\x1b[49m"
	}
}

// ── Theme JSON schema (subset) ─────────────────────────────────────────────

type ThemeJSON struct {
	Name   string            `json:"name"`
	Vars   map[string]interface{} `json:"vars"`
	Colors map[string]interface{} `json:"colors"`
	Export map[string]interface{} `json:"export,omitempty"`
	Symbols *ThemeSymbolsJSON `json:"symbols,omitempty"`
}

type ThemeSymbolsJSON struct {
	Preset        *string           `json:"preset,omitempty"`
	Overrides     map[string]string `json:"overrides,omitempty"`
	SpinnerFrames interface{}       `json:"spinnerFrames,omitempty"`
}

// ── ColorMode ───────────────────────────────────────────────────────────────

type ColorMode string

const (
	ColorModeTrueColor ColorMode = "truecolor"
	ColorMode256       ColorMode = "256color"
)

func DetectColorMode() ColorMode {
	if os.Getenv("WT_SESSION") != "" {
		return ColorModeTrueColor
	}
	// TERM check for truecolor is best-effort; default to truecolor.
	term := os.Getenv("COLORTERM")
	if strings.Contains(term, "truecolor") || strings.Contains(term, "24bit") {
		return ColorModeTrueColor
	}
	// fallback truecolor (most modern terminals)
	return ColorModeTrueColor
}

// ── ThemePalette (resolved 40+ tokens) ─────────────────────────────────────

type ThemePalette struct {
	Accent             string
	Border             string
	BorderAccent       string
	BorderMuted        string
	Success            string
	Error              string
	Warning            string
	Muted              string
	Dim                string
	Text               string
	ThinkingText       string
	SelectedBg         string
	UserMessageBg      string
	UserMessageText    string
	CustomMessageBg    string
	CustomMessageText  string
	CustomMessageLabel string
	ToolPendingBg      string
	ToolSuccessBg      string
	ToolErrorBg        string
	ToolTitle          string
	ToolOutput         string
	MdHeading          string
	MdLink             string
	MdLinkUrl          string
	MdCode             string
	MdCodeBlock        string
	MdCodeBlockBorder  string
	MdQuote            string
	MdQuoteBorder      string
	MdHr               string
	MdListBullet       string
	ToolDiffAdded      string
	ToolDiffRemoved    string
	ToolDiffContext    string
	Link               string
	SyntaxComment      string
	SyntaxKeyword      string
	SyntaxFunction     string
	SyntaxVariable     string
	SyntaxString       string
	SyntaxNumber       string
	SyntaxType         string
	SyntaxOperator     string
	SyntaxPunctuation  string
	ThinkingOff        string
	ThinkingMinimal    string
	ThinkingLow        string
	ThinkingMedium     string
	ThinkingHigh       string
	ThinkingXHigh      string
	ThinkingMax        string
	BashMode           string
	PythonMode         string
	StatusLineBg       string
	StatusLineSep      string
	StatusLineModel    string
	StatusLinePath     string
	StatusLineGitClean string
	StatusLineGitDirty string
	StatusLineContext  string
	StatusLineSpend    string
	StatusLineStaged   string
	StatusLineDirty    string
	StatusLineUntracked string
	StatusLineOutput   string
	StatusLineCost     string
	StatusLineSubagents string
}

// ThemeColor and ThemeBg aliases for method signatures.
type ThemeColor string
type ThemeBg string

const (
	ThemeColorAccent             ThemeColor = "accent"
	ThemeColorBorder             ThemeColor = "border"
	ThemeColorBorderAccent       ThemeColor = "borderAccent"
	ThemeColorBorderMuted        ThemeColor = "borderMuted"
	ThemeColorSuccess            ThemeColor = "success"
	ThemeColorError              ThemeColor = "error"
	ThemeColorWarning            ThemeColor = "warning"
	ThemeColorMuted              ThemeColor = "muted"
	ThemeColorDim                ThemeColor = "dim"
	ThemeColorText               ThemeColor = "text"
	ThemeColorThinkingText       ThemeColor = "thinkingText"
	ThemeColorCustomMessageLabel ThemeColor = "customMessageLabel"
	ThemeColorToolTitle          ThemeColor = "toolTitle"
	ThemeColorToolOutput         ThemeColor = "toolOutput"
	ThemeColorMdHeading          ThemeColor = "mdHeading"
	ThemeColorMdLink             ThemeColor = "mdLink"
	ThemeColorMdLinkUrl          ThemeColor = "mdLinkUrl"
	ThemeColorMdCode             ThemeColor = "mdCode"
	ThemeColorMdCodeBlock        ThemeColor = "mdCodeBlock"
	ThemeColorMdCodeBlockBorder  ThemeColor = "mdCodeBlockBorder"
	ThemeColorMdQuote            ThemeColor = "mdQuote"
	ThemeColorMdQuoteBorder      ThemeColor = "mdQuoteBorder"
	ThemeColorMdHr               ThemeColor = "mdHr"
	ThemeColorMdListBullet       ThemeColor = "mdListBullet"
	ThemeColorToolDiffAdded      ThemeColor = "toolDiffAdded"
	ThemeColorToolDiffRemoved    ThemeColor = "toolDiffRemoved"
	ThemeColorToolDiffContext    ThemeColor = "toolDiffContext"
	ThemeColorLink               ThemeColor = "link"
	ThemeColorSyntaxComment      ThemeColor = "syntaxComment"
	ThemeColorSyntaxKeyword      ThemeColor = "syntaxKeyword"
	ThemeColorSyntaxFunction     ThemeColor = "syntaxFunction"
	ThemeColorSyntaxVariable     ThemeColor = "syntaxVariable"
	ThemeColorSyntaxString       ThemeColor = "syntaxString"
	ThemeColorSyntaxNumber       ThemeColor = "syntaxNumber"
	ThemeColorSyntaxType         ThemeColor = "syntaxType"
	ThemeColorSyntaxOperator     ThemeColor = "syntaxOperator"
	ThemeColorSyntaxPunctuation  ThemeColor = "syntaxPunctuation"
	ThemeColorThinkingOff        ThemeColor = "thinkingOff"
	ThemeColorThinkingMinimal    ThemeColor = "thinkingMinimal"
	ThemeColorThinkingLow        ThemeColor = "thinkingLow"
	ThemeColorThinkingMedium     ThemeColor = "thinkingMedium"
	ThemeColorThinkingHigh       ThemeColor = "thinkingHigh"
	ThemeColorThinkingXHigh      ThemeColor = "thinkingXhigh"
	ThemeColorThinkingMax        ThemeColor = "thinkingMax"
	ThemeColorBashMode           ThemeColor = "bashMode"
	ThemeColorPythonMode         ThemeColor = "pythonMode"
	ThemeColorStatusLineSep      ThemeColor = "statusLineSep"
	ThemeColorStatusLineModel    ThemeColor = "statusLineModel"
	ThemeColorStatusLinePath     ThemeColor = "statusLinePath"
	ThemeColorStatusLineGitClean ThemeColor = "statusLineGitClean"
	ThemeColorStatusLineGitDirty ThemeColor = "statusLineGitDirty"
	ThemeColorStatusLineContext  ThemeColor = "statusLineContext"
	ThemeColorStatusLineSpend    ThemeColor = "statusLineSpend"
	ThemeColorStatusLineStaged   ThemeColor = "statusLineStaged"
	ThemeColorStatusLineDirty    ThemeColor = "statusLineDirty"
	ThemeColorStatusLineUntracked ThemeColor = "statusLineUntracked"
	ThemeColorStatusLineOutput   ThemeColor = "statusLineOutput"
	ThemeColorStatusLineCost     ThemeColor = "statusLineCost"
	ThemeColorStatusLineSubagents ThemeColor = "statusLineSubagents"
	ThemeColorUserMessageText    ThemeColor = "userMessageText"
	ThemeColorCustomMessageText  ThemeColor = "customMessageText"
)

const (
	BgSelectedBg    ThemeBg = "selectedBg"
	BgUserMessageBg ThemeBg = "userMessageBg"
	BgCustomMessageBg ThemeBg = "customMessageBg"
	BgToolPendingBg ThemeBg = "toolPendingBg"
	BgToolSuccessBg ThemeBg = "toolSuccessBg"
	BgToolErrorBg   ThemeBg = "toolErrorBg"
	BgStatusLineBg  ThemeBg = "statusLineBg"
)

// ── Theme struct (Go port of theme-class.ts Theme) ─────────────────────────

type Theme struct {
	Name                  string
	Mode                  ColorMode
	SymbolPreset          SymbolPreset
	Palette               ThemePalette
	IsLightCache          bool
	StatusLineLuminance   *float64
	StatusContrastLuminance *float64

	fgAnsiMap map[ThemeColor]string
	bgAnsiMap map[ThemeBg]string
	hexFgMap  map[ThemeColor]string
	hexBgMap  map[ThemeBg]string
	symbols   map[string]string
}

var backgroundResetPattern = regexp.MustCompile("\x1b\\[(?:0|49)m")

// NewTheme constructs Theme from resolved palette.
func NewTheme(name string, palette ThemePalette, mode ColorMode, preset SymbolPreset) *Theme {
	isLight := false
	var luma *float64
	var contrast *float64
	if v, ok := ColorLuma(palette.StatusLineBg); ok {
		tmp := v
		luma = &tmp
		// also compute relative luminance for accentSurface
		if rl, ok2 := RelativeLuminance(palette.StatusLineBg); ok2 {
			tmp2 := rl
			contrast = &tmp2
		}
		if v > 0.5 {
			isLight = true
		}
	}
	fgMap := map[ThemeColor]string{}
	bgMap := map[ThemeBg]string{}
	hexFg := map[ThemeColor]string{}
	hexBg := map[ThemeBg]string{}

	// helper to populate
	setFg := func(k ThemeColor, v string) {
		fgMap[k] = fgAnsi(v, mode)
		hexFg[k] = ResolveToHex(v, isLight)
	}
	setBg := func(k ThemeBg, v string) {
		bgMap[k] = bgAnsi(v, mode)
		hexBg[k] = ResolveToHex(v, isLight)
	}

	setFg(ThemeColorAccent, palette.Accent)
	setFg(ThemeColorBorder, palette.Border)
	setFg(ThemeColorBorderAccent, palette.BorderAccent)
	setFg(ThemeColorBorderMuted, palette.BorderMuted)
	setFg(ThemeColorSuccess, palette.Success)
	setFg(ThemeColorError, palette.Error)
	setFg(ThemeColorWarning, palette.Warning)
	setFg(ThemeColorMuted, palette.Muted)
	setFg(ThemeColorDim, palette.Dim)
	setFg(ThemeColorText, palette.Text)
	setFg(ThemeColorThinkingText, palette.ThinkingText)
	setFg(ThemeColorCustomMessageLabel, palette.CustomMessageLabel)
	setFg(ThemeColorToolTitle, palette.ToolTitle)
	setFg(ThemeColorToolOutput, palette.ToolOutput)
	setFg(ThemeColorMdHeading, palette.MdHeading)
	setFg(ThemeColorMdLink, palette.MdLink)
	setFg(ThemeColorMdLinkUrl, palette.MdLinkUrl)
	setFg(ThemeColorMdCode, palette.MdCode)
	setFg(ThemeColorMdCodeBlock, palette.MdCodeBlock)
	setFg(ThemeColorMdCodeBlockBorder, palette.MdCodeBlockBorder)
	setFg(ThemeColorMdQuote, palette.MdQuote)
	setFg(ThemeColorMdQuoteBorder, palette.MdQuoteBorder)
	setFg(ThemeColorMdHr, palette.MdHr)
	setFg(ThemeColorMdListBullet, palette.MdListBullet)
	setFg(ThemeColorToolDiffAdded, palette.ToolDiffAdded)
	setFg(ThemeColorToolDiffRemoved, palette.ToolDiffRemoved)
	setFg(ThemeColorToolDiffContext, palette.ToolDiffContext)
	setFg(ThemeColorLink, palette.Link)
	setFg(ThemeColorSyntaxComment, palette.SyntaxComment)
	setFg(ThemeColorSyntaxKeyword, palette.SyntaxKeyword)
	setFg(ThemeColorSyntaxFunction, palette.SyntaxFunction)
	setFg(ThemeColorSyntaxVariable, palette.SyntaxVariable)
	setFg(ThemeColorSyntaxString, palette.SyntaxString)
	setFg(ThemeColorSyntaxNumber, palette.SyntaxNumber)
	setFg(ThemeColorSyntaxType, palette.SyntaxType)
	setFg(ThemeColorSyntaxOperator, palette.SyntaxOperator)
	setFg(ThemeColorSyntaxPunctuation, palette.SyntaxPunctuation)
	setFg(ThemeColorThinkingOff, palette.ThinkingOff)
	setFg(ThemeColorThinkingMinimal, palette.ThinkingMinimal)
	setFg(ThemeColorThinkingLow, palette.ThinkingLow)
	setFg(ThemeColorThinkingMedium, palette.ThinkingMedium)
	setFg(ThemeColorThinkingHigh, palette.ThinkingHigh)
	setFg(ThemeColorThinkingXHigh, palette.ThinkingXHigh)
	setFg(ThemeColorThinkingMax, palette.ThinkingMax)
	setFg(ThemeColorBashMode, palette.BashMode)
	setFg(ThemeColorPythonMode, palette.PythonMode)
	setFg(ThemeColorStatusLineSep, palette.StatusLineSep)
	setFg(ThemeColorStatusLineModel, palette.StatusLineModel)
	setFg(ThemeColorStatusLinePath, palette.StatusLinePath)
	setFg(ThemeColorStatusLineGitClean, palette.StatusLineGitClean)
	setFg(ThemeColorStatusLineGitDirty, palette.StatusLineGitDirty)
	setFg(ThemeColorStatusLineContext, palette.StatusLineContext)
	setFg(ThemeColorStatusLineSpend, palette.StatusLineSpend)
	setFg(ThemeColorStatusLineStaged, palette.StatusLineStaged)
	setFg(ThemeColorStatusLineDirty, palette.StatusLineDirty)
	setFg(ThemeColorStatusLineUntracked, palette.StatusLineUntracked)
	setFg(ThemeColorStatusLineOutput, palette.StatusLineOutput)
	setFg(ThemeColorStatusLineCost, palette.StatusLineCost)
	setFg(ThemeColorStatusLineSubagents, palette.StatusLineSubagents)
	setFg(ThemeColorUserMessageText, palette.UserMessageText)
	setFg(ThemeColorCustomMessageText, palette.CustomMessageText)

	setBg(BgSelectedBg, palette.SelectedBg)
	setBg(BgUserMessageBg, palette.UserMessageBg)
	setBg(BgCustomMessageBg, palette.CustomMessageBg)
	setBg(BgToolPendingBg, palette.ToolPendingBg)
	setBg(BgToolSuccessBg, palette.ToolSuccessBg)
	setBg(BgToolErrorBg, palette.ToolErrorBg)
	setBg(BgStatusLineBg, palette.StatusLineBg)

	t := &Theme{
		Name:                    name,
		Mode:                    mode,
		SymbolPreset:            preset,
		Palette:                 palette,
		IsLightCache:            isLight,
		StatusLineLuminance:     luma,
		StatusContrastLuminance: contrast,
		fgAnsiMap:               fgMap,
		bgAnsiMap:               bgMap,
		hexFgMap:                hexFg,
		hexBgMap:                hexBg,
		symbols:                 map[string]string{},
	}
	return t
}

func (t *Theme) IsLight() bool { return t.IsLightCache }

func (t *Theme) GetColorHex(c ThemeColor) string {
	if v, ok := t.hexFgMap[c]; ok {
		return v
	}
	return ""
}
func (t *Theme) GetBgHex(c ThemeBg) string {
	if v, ok := t.hexBgMap[c]; ok {
		return v
	}
	return ""
}

func (t *Theme) Fg(color ThemeColor, text string) string {
	ansi, ok := t.fgAnsiMap[color]
	if !ok {
		return text
	}
	return ansi + text + "\x1b[39m"
}
func (t *Theme) Bg(color ThemeBg, text string) string {
	ansi, ok := t.bgAnsiMap[color]
	if !ok {
		return text
	}
	return ansi + text + "\x1b[49m"
}
func (t *Theme) BgFill(color ThemeBg, text string) string {
	ansi, ok := t.bgAnsiMap[color]
	if !ok {
		return text
	}
	replaced := backgroundResetPattern.ReplaceAllString(text, "$&"+ansi)
	return ansi + replaced + "\x1b[49m"
}
func (t *Theme) GetFgAnsi(color ThemeColor) string {
	if v, ok := t.fgAnsiMap[color]; ok {
		return v
	}
	return ""
}
func (t *Theme) GetBgAnsi(color ThemeBg) string {
	if v, ok := t.bgAnsiMap[color]; ok {
		return v
	}
	return ""
}

// GetContrastFgAnsi mirrors theme-class.ts getContrastFgAnsi: luma>140 → black else white.
func (t *Theme) GetContrastFgAnsi(fillColor ThemeColor) string {
	ansi, ok := t.fgAnsiMap[fillColor]
	if !ok {
		// fallback to text color
		if txt, ok2 := t.fgAnsiMap[ThemeColorText]; ok2 {
			return txt
		}
		return "\x1b[38;2;255;255;255m"
	}
	re := regexp.MustCompile(`38;2;(\d+);(\d+);(\d+)`)
	m := re.FindStringSubmatch(ansi)
	if m == nil {
		if txt, ok2 := t.fgAnsiMap[ThemeColorText]; ok2 {
			return txt
		}
		return "\x1b[38;2;255;255;255m"
	}
	r, _ := strconv.Atoi(m[1])
	g, _ := strconv.Atoi(m[2])
	b, _ := strconv.Atoi(m[3])
	luma := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	if luma > 140 {
		return "\x1b[38;2;0;0;0m"
	}
	return "\x1b[38;2;255;255;255m"
}

func (t *Theme) GetColorMode() ColorMode { return t.Mode }

func (t *Theme) GetThinkingBorderColor(level ThinkingLevel) func(string) string {
	var c ThemeColor
	switch level {
	case ThinkingOff:
		c = ThemeColorThinkingOff
	case ThinkingMinimal:
		c = ThemeColorThinkingMinimal
	case ThinkingLow:
		c = ThemeColorThinkingLow
	case ThinkingMedium:
		c = ThemeColorThinkingMedium
	case ThinkingHigh:
		c = ThemeColorThinkingHigh
	case ThinkingXHigh, ThinkingMax:
		c = ThemeColorThinkingXHigh
		if level == ThinkingMax && t.Palette.ThinkingMax != "" {
			c = ThemeColorThinkingMax
		}
	default:
		c = ThemeColorThinkingOff
	}
	return func(s string) string { return t.Fg(c, s) }
}

// Lipgloss helpers — map ThemeColor to lipgloss.Color via hex.
func (t *Theme) LipglossFg(c ThemeColor) lipgloss.Color {
	return lipgloss.Color(t.GetColorHex(c))
}
func (t *Theme) LipglossBg(c ThemeBg) lipgloss.Color {
	return lipgloss.Color(t.GetBgHex(c))
}

// GetStatusLineStyle mirrors styles.go GetStatusLineStyle but theme-aware.
func (t *Theme) GetStatusLineStyle(preset string) lipgloss.Style {
	base := lipgloss.NewStyle().
		Background(lipgloss.Color(t.Palette.StatusLineBg)).
		Foreground(lipgloss.Color(t.GetColorHex(ThemeColorText)))
	switch preset {
	case "minimal":
		return base.Foreground(lipgloss.Color(t.GetColorHex(ThemeColorMuted)))
	case "compact":
		return base.Foreground(lipgloss.Color(t.GetColorHex(ThemeColorDim)))
	default:
		return base
	}
}

// ── Palette resolvers ───────────────────────────────────────────────────────

func resolveVar(value interface{}, vars map[string]interface{}, visited map[string]bool) interface{} {
	s, ok := value.(string)
	if !ok {
		return value
	}
	if s == "" || strings.HasPrefix(s, "#") {
		return s
	}
	// numeric string? keep as is for later conversion
	if _, err := strconv.Atoi(s); err == nil {
		// palette index referenced via var? but vars may hold numbers too
	}
	if visited[s] {
		return s
	}
	// vars lookup — key may be without $ prefix (dark.json uses "accent" not "$accent")
	if v, exists := vars[s]; exists {
		visited[s] = true
		return resolveVar(v, vars, visited)
	}
	// try with $ prefix
	if strings.HasPrefix(s, "$") {
		key := strings.TrimPrefix(s, "$")
		if v, exists := vars[key]; exists {
			visited[key] = true
			return resolveVar(v, vars, visited)
		}
	}
	return s
}

func resolveColors(colors map[string]interface{}, vars map[string]interface{}) map[string]string {
	out := map[string]string{}
	for k, v := range colors {
		resolved := resolveVar(v, vars, map[string]bool{})
		switch val := resolved.(type) {
		case string:
			out[k] = val
		case float64:
			// JSON numbers decode as float64 — convert via ansi256ToHex
			out[k] = Ansi256ToHex(int(val))
		case int:
			out[k] = Ansi256ToHex(val)
		default:
			out[k] = fmt.Sprint(val)
		}
		// If resolved is numeric palette index passed as string like "244" after var? Already handled.
		// For float64 case we already hexed; for string numeric we need to hex as well
		if _, err := strconv.Atoi(out[k]); err == nil {
			// string that is numeric palette index — convert
			if n, err2 := strconv.Atoi(out[k]); err2 == nil && n >= 0 && n <= 255 {
				// Check if original was number-like; hex it
				// But if out[k] came from numeric index it should have been float64 case already.
				// Still handle string numeric window.
				out[k] = Ansi256ToHex(n)
			}
		}
	}
	return out
}

func themeJSONToPalette(j ThemeJSON) ThemePalette {
	vars := map[string]interface{}{}
	for k, v := range j.Vars {
		vars[k] = v
	}
	resolved := resolveColors(j.Colors, vars)
	// helper
	get := func(key string) string {
		if v, ok := resolved[key]; ok {
			// if value is still var ref not resolved (missing), keep as is but try to resolve again
			if _, err := strconv.Atoi(v); err == nil {
				if n, e2 := strconv.Atoi(v); e2 == nil {
					return Ansi256ToHex(n)
				}
			}
			return v
		}
		return ""
	}
	// Normalize empty text fallback handling: ResolveToHex will handle at Theme creation, keep "" as is.
	return ThemePalette{
		Accent:             get("accent"),
		Border:             get("border"),
		BorderAccent:       get("borderAccent"),
		BorderMuted:        get("borderMuted"),
		Success:            get("success"),
		Error:              get("error"),
		Warning:            get("warning"),
		Muted:              get("muted"),
		Dim:                get("dim"),
		Text:               get("text"),
		ThinkingText:       get("thinkingText"),
		SelectedBg:         get("selectedBg"),
		UserMessageBg:      get("userMessageBg"),
		UserMessageText:    get("userMessageText"),
		CustomMessageBg:    get("customMessageBg"),
		CustomMessageText:  get("customMessageText"),
		CustomMessageLabel: get("customMessageLabel"),
		ToolPendingBg:      get("toolPendingBg"),
		ToolSuccessBg:      get("toolSuccessBg"),
		ToolErrorBg:        get("toolErrorBg"),
		ToolTitle:          get("toolTitle"),
		ToolOutput:         get("toolOutput"),
		MdHeading:          get("mdHeading"),
		MdLink:             get("mdLink"),
		MdLinkUrl:          get("mdLinkUrl"),
		MdCode:             get("mdCode"),
		MdCodeBlock:        get("mdCodeBlock"),
		MdCodeBlockBorder:  get("mdCodeBlockBorder"),
		MdQuote:            get("mdQuote"),
		MdQuoteBorder:      get("mdQuoteBorder"),
		MdHr:               get("mdHr"),
		MdListBullet:       get("mdListBullet"),
		ToolDiffAdded:      get("toolDiffAdded"),
		ToolDiffRemoved:    get("toolDiffRemoved"),
		ToolDiffContext:    get("toolDiffContext"),
		Link:               get("link"),
		SyntaxComment:      get("syntaxComment"),
		SyntaxKeyword:      get("syntaxKeyword"),
		SyntaxFunction:     get("syntaxFunction"),
		SyntaxVariable:     get("syntaxVariable"),
		SyntaxString:       get("syntaxString"),
		SyntaxNumber:       get("syntaxNumber"),
		SyntaxType:         get("syntaxType"),
		SyntaxOperator:     get("syntaxOperator"),
		SyntaxPunctuation:  get("syntaxPunctuation"),
		ThinkingOff:        get("thinkingOff"),
		ThinkingMinimal:    get("thinkingMinimal"),
		ThinkingLow:        get("thinkingLow"),
		ThinkingMedium:     get("thinkingMedium"),
		ThinkingHigh:       get("thinkingHigh"),
		ThinkingXHigh:      get("thinkingXhigh"),
		ThinkingMax:        get("thinkingMax"),
		BashMode:           get("bashMode"),
		PythonMode:         get("pythonMode"),
		StatusLineBg:       get("statusLineBg"),
		StatusLineSep:      get("statusLineSep"),
		StatusLineModel:    get("statusLineModel"),
		StatusLinePath:     get("statusLinePath"),
		StatusLineGitClean: get("statusLineGitClean"),
		StatusLineGitDirty: get("statusLineGitDirty"),
		StatusLineContext:  get("statusLineContext"),
		StatusLineSpend:    get("statusLineSpend"),
		StatusLineStaged:   get("statusLineStaged"),
		StatusLineDirty:    get("statusLineDirty"),
		StatusLineUntracked: get("statusLineUntracked"),
		StatusLineOutput:   get("statusLineOutput"),
		StatusLineCost:     get("statusLineCost"),
		StatusLineSubagents: get("statusLineSubagents"),
	}
}

// Built-in palettes (resolved, lipgloss-compatible hex).
// Rose Pine remains dark default for biggz-ai TUI; these mirror oh-my-pi dark.json/light.json resolved vars.

var DarkPalette = ThemePalette{
	Accent:             "#febc38",
	Border:             "#178fb9",
	BorderAccent:       "#0088fa",
	BorderMuted:        "#3d424a",
	Success:            "#89d281",
	Error:              "#fc3a4b",
	Warning:            "#e4c00f",
	Muted:              "#777d88",
	Dim:                "#5f6673",
	Text:               "",
	ThinkingText:       "#777d88",
	SelectedBg:         "#31363f",
	UserMessageBg:      "#221d1a",
	UserMessageText:    "",
	CustomMessageBg:    "#2a2530",
	CustomMessageText:  "",
	CustomMessageLabel: "#b281d6",
	ToolPendingBg:      "#1d2129",
	ToolSuccessBg:      "#161a1f",
	ToolErrorBg:        "#291d1d",
	ToolTitle:          "",
	ToolOutput:         "#777d88",
	MdHeading:          "#febc38",
	MdLink:             "#0088fa",
	MdLinkUrl:          "#5f6673",
	MdCode:             "#e5c1ff",
	MdCodeBlock:        "#9CDCFE",
	MdCodeBlockBorder:  "#777d88",
	MdQuote:            "#777d88",
	MdQuoteBorder:      "#3d424a",
	MdHr:               "#3d424a",
	MdListBullet:       "#febc38",
	ToolDiffAdded:      "#89d281",
	ToolDiffRemoved:    "#fc3a4b",
	ToolDiffContext:    "#777d88",
	Link:               "#0088fa",
	SyntaxComment:      "#6A9955",
	SyntaxKeyword:      "#569CD6",
	SyntaxFunction:     "#DCDCAA",
	SyntaxVariable:     "#9CDCFE",
	SyntaxString:       "#CE9178",
	SyntaxNumber:       "#B5CEA8",
	SyntaxType:         "#4EC9B0",
	SyntaxOperator:     "#D4D4D4",
	SyntaxPunctuation:  "#D4D4D4",
	ThinkingOff:        "#3d424a",
	ThinkingMinimal:    "#5f6673",
	ThinkingLow:        "#178fb9",
	ThinkingMedium:     "#0088fa",
	ThinkingHigh:       "#b281d6",
	ThinkingXHigh:      "#e5c1ff",
	ThinkingMax:        "#e5c1ff",
	BashMode:           "#0088fa",
	PythonMode:         "#e4c00f",
	StatusLineBg:       "#121212",
	StatusLineSep:      "#808080", // 244 → #808080
	StatusLineModel:    "#d787af",
	StatusLinePath:     "#00afaf",
	StatusLineGitClean: "#5faf5f",
	StatusLineGitDirty: "#d7af5f",
	StatusLineContext:  "#8787af",
	StatusLineSpend:    "#5fafaf",
	StatusLineStaged:   "#5faf00", // 70 → #5faf00
	StatusLineDirty:    "#d7af00", // 178 → #d7af00
	StatusLineUntracked: "#00afff", // 39 → #00afff
	StatusLineOutput:   "#ff5faf", // 205
	StatusLineCost:     "#ff5faf",
	StatusLineSubagents: "#febc38",
}

var LightPalette = ThemePalette{
	Accent:             "#5a8080",
	Border:             "#547da7",
	BorderAccent:       "#5a8080",
	BorderMuted:        "#b0b0b0",
	Success:            "#588458",
	Error:              "#aa5555",
	Warning:            "#9a7326",
	Muted:              "#6c6c6c",
	Dim:                "#767676",
	Text:               "",
	ThinkingText:       "#6c6c6c",
	SelectedBg:         "#d0d0e0",
	UserMessageBg:      "#e8e8e8",
	UserMessageText:    "",
	CustomMessageBg:    "#ede7f6",
	CustomMessageText:  "",
	CustomMessageLabel: "#7e57c2",
	ToolPendingBg:      "#e8e8f0",
	ToolSuccessBg:      "#e8f0e8",
	ToolErrorBg:        "#f0e8e8",
	ToolTitle:          "",
	ToolOutput:         "#6c6c6c",
	MdHeading:          "#9a7326",
	MdLink:             "#547da7",
	MdLinkUrl:          "#767676",
	MdCode:             "#5a8080",
	MdCodeBlock:        "#588458",
	MdCodeBlockBorder:  "#6c6c6c",
	MdQuote:            "#6c6c6c",
	MdQuoteBorder:      "#6c6c6c",
	MdHr:               "#6c6c6c",
	MdListBullet:       "#588458",
	ToolDiffAdded:      "#588458",
	ToolDiffRemoved:    "#aa5555",
	ToolDiffContext:    "#6c6c6c",
	SyntaxComment:      "#008000",
	SyntaxKeyword:      "#0000FF",
	SyntaxFunction:     "#795E26",
	SyntaxVariable:     "#001080",
	SyntaxString:       "#A31515",
	SyntaxNumber:       "#098658",
	SyntaxType:         "#267F99",
	SyntaxOperator:     "#000000",
	SyntaxPunctuation:  "#000000",
	ThinkingOff:        "#b0b0b0",
	ThinkingMinimal:    "#767676",
	ThinkingLow:        "#547da7",
	ThinkingMedium:     "#5a8080",
	ThinkingHigh:       "#875f87",
	ThinkingXHigh:      "#8b008b",
	ThinkingMax:        "#8b008b",
	BashMode:           "#588458",
	PythonMode:         "#9a7326",
	StatusLineBg:       "#e0e0e0",
	StatusLineSep:      "#808080",
	StatusLineModel:    "#875f87",
	StatusLinePath:     "#005f87",
	StatusLineGitClean: "#005f00",
	StatusLineGitDirty: "#af5f00",
	StatusLineContext:  "#5f5f87",
	StatusLineSpend:    "#005f5f",
	StatusLineStaged:   "#008700", // 28 → #008700
	StatusLineDirty:    "#af8700", // 136 → #af8700
	StatusLineUntracked: "#0087af", // 31 → #0087af
	StatusLineOutput:   "#af5faf", // 133 → #af5faf
	StatusLineCost:     "#af5faf",
	StatusLineSubagents: "#5a8080",
}

// ── Theme registry & loader ─────────────────────────────────────────────────

var builtinPalettes = map[string]ThemePalette{
	"dark":  DarkPalette,
	"light": LightPalette,
}

var currentTheme *Theme
var themeEpoch atomic.Int64
var colorBlindEnabled bool

func GetThemeEpoch() int64 { return themeEpoch.Load() }
func GetColorBlindMode() bool { return colorBlindEnabled }
func SetColorBlindMode(enabled bool) {
	if colorBlindEnabled == enabled {
		return
	}
	colorBlindEnabled = enabled
	BumpThemeEpoch()
}
func IsColorBlindMode() bool { return colorBlindEnabled }
func BumpThemeEpoch() int64 { return themeEpoch.Add(1) }

// IsPrettyEnabled guards TUI theming (mirrors JS BIGGZ_PRETTY / PI_SUBAGENT_CHILD).
func IsPrettyEnabled() bool {
	if os.Getenv("BIGGZ_PRETTY") == "0" {
		return false
	}
	if os.Getenv("PI_SUBAGENT_CHILD") == "1" {
		return false
	}
	return true
}

// GetTheme returns a Theme for name with optional preset override.
// Tries JSON file on disk (custom themes dir), falls back to hardcoded Go palettes, then Rose Pine.
func GetTheme(name string, preset ...SymbolPreset) *Theme {
	p := SymbolPresetUnicode
	if len(preset) > 0 && preset[0] != "" {
		p = preset[0]
	}
	mode := DetectColorMode()
	// Try loading JSON from candidate paths (mirrors oh-my-pi getCustomThemesDir + embedded fallback)
	var palette *ThemePalette
	for _, cand := range candidateThemePaths(name) {
		if data, err := os.ReadFile(cand); err == nil {
			if tj, err2 := parseThemeJSON(data); err2 == nil {
				pal := themeJSONToPalette(tj)
				if tj.Symbols != nil && tj.Symbols.Preset != nil && IsValidSymbolPreset(*tj.Symbols.Preset) && len(preset) == 0 {
					p = SymbolPreset(*tj.Symbols.Preset)
				}
				palette = &pal
				name = tj.Name
				break
			}
		}
	}
	// Fallback to builtin palettes (includes solarized-osaka parsed from hardcoded JSON below)
	if palette == nil {
		if bp, ok := builtinPalettes[name]; ok {
			palette = &bp
		} else if name == "solarized-osaka" {
			palette = builtinSolarizedPalette()
		}
	}
	if palette == nil {
		// ultimate fallback — Rose Pine dark
		tmp := DarkPalette
		palette = &tmp
		name = "dark"
	}
	// colorBlindMode swap: shift green diff add toward blue (mirrors loader.ts)
	if colorBlindEnabled && strings.HasPrefix(palette.ToolDiffAdded, "#") {
		tmp := *palette
		tmp.ToolDiffAdded = AdjustHsv(tmp.ToolDiffAdded, colorBlindAdjustment)
		palette = &tmp
	}
	th := NewTheme(name, *palette, mode, p)
	return th
}

func candidateThemePaths(name string) []string {
	var out []string
	// 1. Custom themes dir: ~/.pi/agent/themes (mirrors oh-my-pi)
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		out = append(out, filepath.Join(home, ".pi", "agent", "themes", name+".json"))
		out = append(out, filepath.Join(home, ".pi", "themes", name+".json"))
	}
	// 2. Project local themes: ./.pi/themes
	out = append(out, filepath.Join(".pi", "themes", name+".json"))
	// 3. Bundled assets relative to module root (for tests/dev) — include both repo-root and package-relative variants
	out = append(out, filepath.Join("internal", "assets", "pi", "themes", name+".json"))
	out = append(out, filepath.Join("..", "..", "assets", "pi", "themes", name+".json"))
	out = append(out, filepath.Join("..", "..", "..", "assets", "pi", "themes", name+".json"))
	// 3b. Absolute via runtime caller (file location) — robust for tests regardless of cwd
	if _, file, _, ok := runtime.Caller(0); ok && file != "" {
		dir := filepath.Dir(file)
		// theme.go is at internal/tui/styles/theme.go, assets is at internal/assets/pi/themes
		out = append(out, filepath.Join(dir, "..", "..", "assets", "pi", "themes", name+".json"))
		out = append(out, filepath.Join(dir, "..", "..", "..", "assets", "pi", "themes", name+".json"))
		// Also try absolute cleaned
		abs, _ := filepath.Abs(filepath.Join(dir, "..", "..", "assets", "pi", "themes", name+".json"))
		out = append(out, abs)
	}
	// 4. Absolute fallback via executable dir
	if exe, err := os.Executable(); err == nil && exe != "" {
		dir := filepath.Dir(exe)
		out = append(out, filepath.Join(dir, "themes", name+".json"))
		out = append(out, filepath.Join(dir, "..", "internal", "assets", "pi", "themes", name+".json"))
	}
	return out
}

func builtinSolarizedPalette() *ThemePalette {
	// Try to load solarized-osaka.json from assets if available, else hardcoded fallback
	for _, cand := range candidateThemePaths("solarized-osaka") {
		if data, err := os.ReadFile(cand); err == nil {
			if tj, err2 := parseThemeJSON(data); err2 == nil {
				pal := themeJSONToPalette(tj)
				return &pal
			}
		}
	}
	// Hardcoded fallback mirroring solarized-osaka.json vars/colors
	pal := ThemePalette{
		Accent: "#278bd3", Border: "#2a2a2a", BorderAccent: "#278bd3", BorderMuted: "#2a2a2a",
		Success: "#859900", Error: "#dc312e", Warning: "#b58900", Muted: "#586e74", Dim: "#647a82",
		Text: "#839495", ThinkingText: "#586e74", SelectedBg: "#1a1a1a", UserMessageBg: "#1a1a1a",
		UserMessageText: "#839495", CustomMessageBg: "#0a0a0a", CustomMessageText: "#839495", CustomMessageLabel: "#2aa298",
		ToolPendingBg: "#1a1a1a", ToolSuccessBg: "#1a1a1a", ToolErrorBg: "#1a1a1a", ToolTitle: "#278bd3", ToolOutput: "#839495",
		MdHeading: "#278bd3", MdLink: "#2aa298", MdLinkUrl: "#586e74", MdCode: "#2aa298", MdCodeBlock: "#839495",
		MdCodeBlockBorder: "#2a2a2a", MdQuote: "#586e74", MdQuoteBorder: "#2aa298", MdHr: "#2a2a2a", MdListBullet: "#2aa298",
		ToolDiffAdded: "#859900", ToolDiffRemoved: "#dc312e", ToolDiffContext: "#586e74", Link: "#2aa298",
		SyntaxComment: "#586e74", SyntaxKeyword: "#859900", SyntaxFunction: "#278bd3", SyntaxVariable: "#839495", SyntaxString: "#2aa298", SyntaxNumber: "#6d72c5", SyntaxType: "#b58900", SyntaxOperator: "#859900", SyntaxPunctuation: "#ca4c16",
		ThinkingOff: "#647a82", ThinkingMinimal: "#859900", ThinkingLow: "#278bd3", ThinkingMedium: "#2aa298", ThinkingHigh: "#6d72c5", ThinkingXHigh: "#d33682", ThinkingMax: "#dc312e", BashMode: "#b58900", PythonMode: "#b58900",
		StatusLineBg: "#0a0a0a", StatusLineSep: "#2a2a2a", StatusLineModel: "#278bd3", StatusLinePath: "#2aa298", StatusLineGitClean: "#859900", StatusLineGitDirty: "#b58900", StatusLineContext: "#6d72c5", StatusLineSpend: "#2aa298", StatusLineStaged: "#859900", StatusLineDirty: "#b58900", StatusLineUntracked: "#278bd3", StatusLineOutput: "#d33682", StatusLineCost: "#d33682", StatusLineSubagents: "#278bd3",
	}
	return &pal
}

func parseThemeJSON(data []byte) (ThemeJSON, error) {
	var j ThemeJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return j, err
	}
	return j, nil
}

// LoadThemeFromFile loads a theme JSON from disk (for watcher reload).
func LoadThemeFromFile(path string, preset SymbolPreset) (*Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	j, err := parseThemeJSON(data)
	if err != nil {
		return nil, err
	}
	pal := themeJSONToPalette(j)
	if j.Symbols != nil && j.Symbols.Preset != nil && IsValidSymbolPreset(*j.Symbols.Preset) {
		preset = SymbolPreset(*j.Symbols.Preset)
	}
	mode := DetectColorMode()
	return NewTheme(j.Name, pal, mode, preset), nil
}

// InitCurrentTheme initializes the global current theme based on terminal background.
func InitCurrentTheme() *Theme {
	if !IsPrettyEnabled() {
		th := GetTheme("dark")
		currentTheme = th
		return th
	}
	bg := DetectTerminalBackground()
	name := "dark"
	if bg == "light" {
		name = "light"
	}
	th := GetTheme(name)
	currentTheme = th
	return th
}

func CurrentTheme() *Theme {
	if currentTheme == nil {
		return InitCurrentTheme()
	}
	return currentTheme
}

func SetCurrentTheme(th *Theme) {
	currentTheme = th
	BumpThemeEpoch()
}

// ── Terminal background detection (tiered fallback) ─────────────────────────

var terminalReportedAppearance string
var macOSReportedAppearance string

func shouldUseMacOSAppearanceFallback() bool {
	return os.Getenv("ZELLIJ") != "" && os.Getenv("GOOS") == "darwin" || (os.Getenv("ZELLIJ") != "" && isDarwin())
}

func isDarwin() bool {
	return os.Getenv("GOOS") == "darwin" || strings.Contains(strings.ToLower(os.Getenv("OSTYPE")), "darwin")
}

// DetectTerminalBackground mirrors theme.ts detectTerminalBackground with tiered fallback.
func DetectTerminalBackground() string {
	// Tier 1: terminal-reported appearance from OSC 11 (if available via env stub)
	if !shouldUseMacOSAppearanceFallback() && terminalReportedAppearance != "" {
		return terminalReportedAppearance
	}
	// Tier 2: COLORFGBG env var
	if colorfgbg := os.Getenv("COLORFGBG"); colorfgbg != "" {
		parts := strings.Split(colorfgbg, ";")
		if len(parts) >= 2 {
			if bg, err := strconv.Atoi(parts[1]); err == nil {
				if bg < 8 {
					return "dark"
				}
				return "light"
			}
		}
	}
	// Tier 3: host macOS appearance fallback (only for broken paths like Zellij on macOS)
	if shouldUseMacOSAppearanceFallback() {
		if mac := detectMacOSAppearance(); mac != "" {
			return mac
		}
		if macOSReportedAppearance != "" {
			return macOSReportedAppearance
		}
	}
	return "dark"
}

func detectMacOSAppearance() string {
	// Stub: check env BIGGZ_MAC_APPEARANCE or try `defaults read` via os exec? Keep stub.
	if v := os.Getenv("BIGGZ_MAC_APPEARANCE"); v == "dark" || v == "light" {
		return v
	}
	// Allow override via AppleInterfaceStyle env for tests
	if v := os.Getenv("AppleInterfaceStyle"); strings.EqualFold(v, "dark") {
		return "dark"
	}
	if v := os.Getenv("AppleInterfaceStyle"); strings.EqualFold(v, "light") {
		return "light"
	}
	return ""
}

// OnTerminalAppearanceChange mirrors theme.ts onTerminalAppearanceChange.
func OnTerminalAppearanceChange(mode string) {
	if mode != "dark" && mode != "light" {
		return
	}
	if terminalReportedAppearance == mode {
		return
	}
	terminalReportedAppearance = mode
	// If auto theme, re-evaluate — for now just bump epoch
	BumpThemeEpoch()
}

// ── FSWatcher with debounce 100ms ───────────────────────────────────────────

func WatchThemeFile(path string, onChange func(*Theme)) (func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return func() {}, err
	}
	// Watch directory, not file (more reliable across editors atomic writes)
	dir := path
	if idx := strings.LastIndex(path, "/"); idx != -1 {
		dir = path[:idx]
	} else if idx := strings.LastIndex(path, "\\"); idx != -1 {
		dir = path[:idx]
	}
	// Fallback to "." if no separator
	if dir == path {
		dir = "."
	}
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return func() {}, err
	}
	base := path
	if idx := strings.LastIndex(path, "/"); idx != -1 {
		base = path[idx+1:]
	} else if idx := strings.LastIndex(path, "\\"); idx != -1 {
		base = path[idx+1:]
	}

	done := make(chan struct{})
	go func() {
		var timer *time.Timer
		defer watcher.Close()
		for {
			select {
			case <-done:
				if timer != nil {
					timer.Stop()
				}
				return
			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Filter to our file only
				evBase := ev.Name
				if idx := strings.LastIndex(ev.Name, "/"); idx != -1 {
					evBase = ev.Name[idx+1:]
				} else if idx := strings.LastIndex(ev.Name, "\\"); idx != -1 {
					evBase = ev.Name[idx+1:]
				}
				if evBase != base {
					continue
				}
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(100*time.Millisecond, func() {
					if _, err := os.Stat(path); err != nil {
						return
					}
					th, err := LoadThemeFromFile(path, SymbolPresetUnicode)
					if err != nil {
						return
					}
					SetCurrentTheme(th)
					if onChange != nil {
						onChange(th)
					}
				})
			case <-watcher.Errors:
				// ignore
			}
		}
	}()
	stop := func() {
		close(done)
		watcher.Close()
	}
	return stop, nil
}

// WatchThemesDir watches a themes directory for any json change (debounce 100ms).
func WatchThemesDir(dir string, onChange func(*Theme)) (func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return func() {}, err
	}
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return func() {}, err
	}
	done := make(chan struct{})
	go func() {
		var timer *time.Timer
		defer watcher.Close()
		for {
			select {
			case <-done:
				if timer != nil {
					timer.Stop()
				}
				return
			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}
				if !strings.HasSuffix(strings.ToLower(ev.Name), ".json") {
					continue
				}
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(100*time.Millisecond, func() {
					th, err := LoadThemeFromFile(ev.Name, SymbolPresetUnicode)
					if err != nil {
						return
					}
					SetCurrentTheme(th)
					if onChange != nil {
						onChange(th)
					}
				})
			case <-watcher.Errors:
			}
		}
	}()
	return func() { close(done); watcher.Close() }, nil
}

// ── Helpers for math without importing math at top (avoid circular) ─────────

func mathPow(a, b float64) float64 { return math.Pow(a, b) }
