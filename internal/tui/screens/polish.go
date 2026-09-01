package screens

import (
	"fmt"
	"strings"
)

// FleetRowInput holds data for a 2-line fleet row.
type FleetRowInput struct {
	Glyph       string
	Agent       string
	Model       string
	State       string
	ElapsedSec  int
	WindowTokens int
	SpentTokens  int
	Tool         string
	Activity     string
}

// RenderFleetRow renders 2-line row per POLISH-TUI-01.
// L1 = glyph agent model·state + fixed elapsed 5c + tokens 10c; L2 dim tool/activity.
// Height always 2. Truncation preserves right columns.
func RenderFleetRow(width int, in FleetRowInput) (string, string, int) {
	if width <= 0 {
		width = 80
	}
	// Build left part: glyph + agent/model·state
	left := strings.TrimSpace(in.Glyph + " " + strings.TrimSpace(in.Agent+" "+in.Model) + "·" + strings.TrimSpace(in.State))
	left = strings.Join(strings.Fields(left), " ")
	if left == "·" || left == "" {
		left = strings.TrimSpace(in.Glyph + " " + in.State)
	}
	leftBudget := RowLeftBudget(width)
	// Ensure leftBudget leaves room for right columns? Use FixedRightWidth =16.
	// RowLeftBudget already (width-6)/2, but cap to width-FixedRightWidth-1 to guarantee right fits.
	maxLeft := width - FixedRightWidth - 1
	if maxLeft < 1 {
		maxLeft = 1
	}
	if leftBudget > maxLeft {
		leftBudget = maxLeft
	}
	leftTrunc := TruncateToWidth(left, leftBudget)

	elapsedStr := fmt.Sprintf("%ds", in.ElapsedSec)
	elapsedStr = RightAlign(elapsedStr, 5)
	tokStr := formatFleetTokens(in.WindowTokens, in.SpentTokens)
	tokStr = RightAlign(tokStr, 10)

	// L1 = left padded? leftTrunc + space + elapsed + space + tokens, right-aligned.
	// Need to ensure VisibleWidth(L1) <= width
	// Right part is elapsed(5)+1+tokens(10)=16 visible plus one separator space between left and right => at least 1
	// So L1 is leftTrunc + " " + elapsed + " " + tokens with widths fixed.
	elapsedStyled := "\x1b[2m" + elapsedStr + "\x1b[0m"
	tokensStyled := "\x1b[2m" + tokStr + "\x1b[0m"
	l1 := leftTrunc + " " + elapsedStyled + " " + tokensStyled

	// Ensure l1 not exceed width by truncating left further if needed (right never truncated)
	for VisibleWidth(l1) > width && VisibleWidth(leftTrunc) > 1 {
		leftTrunc = TruncateToWidth(leftTrunc, VisibleWidth(leftTrunc)-1)
		l1 = leftTrunc + " " + elapsedStyled + " " + tokensStyled
	}

	// L2 dim tool/activity
	activity := strings.TrimSpace(in.Tool)
	if activity == "" {
		activity = strings.TrimSpace(in.Activity)
	} else if strings.TrimSpace(in.Activity) != "" && !strings.Contains(activity, in.Activity) {
		activity = activity + " " + in.Activity
	}
	activity = strings.Join(strings.Fields(activity), " ")
	if VisibleWidth(activity) > width {
		activity = TruncateToWidth(activity, width)
	}
	l2 := "\x1b[2m" + activity + "\x1b[0m"

	return l1, l2, 2
}

// WorkflowRowInput holds workflow hierarchy data.
type WorkflowRowInput struct {
	Name         string
	State        string
	Gate         string
	Next         string
	Output       string
	Failure      string
	NestedLevel  int
}

// RenderWorkflowRow renders workflow 2-line per POLISH-TUI-02 with │ dim guide.
func RenderWorkflowRow(width int, in WorkflowRowInput) (string, string, int) {
	if width <= 0 {
		width = 80
	}
	l1Left := strings.TrimSpace(in.Name + "·" + in.State)
	l1Left = strings.Join(strings.Fields(l1Left), " ")
	if VisibleWidth(l1Left) > width {
		l1Left = TruncateToWidth(l1Left, width)
	}
	l1 := l1Left

	// L2 dim gate/next/output + failure inline; nested prefix │ dim
	var parts []string
	if g := strings.TrimSpace(in.Gate); g != "" {
		parts = append(parts, g)
	}
	if n := strings.TrimSpace(in.Next); n != "" {
		parts = append(parts, n)
	}
	if o := strings.TrimSpace(in.Output); o != "" {
		parts = append(parts, o)
	}
	l2Core := strings.Join(parts, " ")
	if f := strings.TrimSpace(in.Failure); f != "" {
		if l2Core != "" {
			l2Core += " — " + f
		} else {
			l2Core = f
		}
	}
	l2Core = strings.Join(strings.Fields(l2Core), " ")
	// prefix for nested
	prefix := ""
	if in.NestedLevel > 0 {
		// dim │ per depth
		prefix = strings.Repeat("│ ", in.NestedLevel)
		prefix = "\x1b[2m" + prefix + "\x1b[0m"
	}
	// Truncate l2Core to fit width minus prefix visible
	prefixW := VisibleWidth(prefix)
	budget := width - prefixW
	if budget < 1 {
		budget = 1
	}
	if VisibleWidth(l2Core) > budget {
		l2Core = TruncateToWidth(l2Core, budget)
	}
	l2 := prefix + "\x1b[2m" + l2Core + "\x1b[0m"

	return l1, l2, 2
}

// HeaderInput for collapsed header 2 groups.
type HeaderInput struct {
	Running   int
	Queued    int
	CapUsed   int
	CapLimit  int
	PaneWarn  bool
	ElapsedSec int
	TokensSpent int
	TokensWindow int
}

// RenderHeader renders collapsed header 2 groups per POLISH-TUI-03: g1 muted·g2 dim (≤2 numerics +1 hint)
func RenderHeader(in HeaderInput) string {
	g1 := fmt.Sprintf("%d running·%d queued·cap %d/%d", in.Running, in.Queued, in.CapUsed, in.CapLimit)
	if in.PaneWarn {
		g1 += "·pane ⚠"
	}
	elapsed := fmt.Sprintf("%ds", in.ElapsedSec)
	tok := formatFleetTokens(in.TokensWindow, in.TokensSpent)
	g2 := elapsed + "·" + tok
	// g1 muted, g2 dim via ANSI; separator · dim
	muted := "\x1b[90m" + g1 + "\x1b[0m"
	dim := "\x1b[2m" + g2 + "\x1b[0m"
	return muted + " · " + dim
}

// PanesModel holds collapsible panes section.
type PanesModel struct {
	Collapsed bool
	Rows      []string
}

// PanesHeader is the collapsible section header.
const PanesHeader = "── panes ──"

// View renders panes section per POLISH-TUI-04.
func (m PanesModel) View() string {
	var b strings.Builder
	b.WriteString(PanesHeader + "\n")
	if m.Collapsed {
		return b.String()
	}
	for _, r := range m.Rows {
		b.WriteString(r + "\n")
	}
	return b.String()
}

// VisibleWorkflowRowsGeneric generic version for any slice.

func VisibleWorkflowRowsGeneric[T any](rows []T, limit int) ([]T, int, string) {
	if limit <= 0 || len(rows) <= limit {
		return rows, 0, ""
	}
	hidden := len(rows) - limit
	return rows[:limit], hidden, fmt.Sprintf("… +%d hidden", hidden)
}

// VisibleWorkflowRowsStrings for string rows helper.

func VisibleWorkflowRowsStrings(rows []string, limit int) ([]string, string) {
	vis, _, tail := VisibleWorkflowRowsGeneric(rows, limit)
	// tail already formatted, but generic returns tail string only if hidden>0, need to check
	if tail == "" {
		// need to compute tail correctly
		if len(rows) > limit && limit > 0 {
			return vis, fmt.Sprintf("… +%d hidden", len(rows)-limit)
		}
		return vis, ""
	}
	return vis, tail
}

// ── Rank4/5: CachedOutputBlock helpers (Go variant, trivial) ──
// Mirrors oh-my-pi output-block.ts + render-utils.ts tail-window logic.

const (
	TruncateTitleLength   = 60
	TruncatePreviewLength = 120
	PreviewCollapsedLines = 3
	PreviewCollapsedItems = 8
)

func PreviewWindowRows(termRows int) int {
	if termRows <= 0 {
		termRows = 30
	}
	rows := termRows - 20
	if rows < 6 {
		rows = 6
	}
	return rows
}

func CapPreviewLines(lines []string, maxRows int, expanded bool) []string {
	if expanded {
		return lines
	}
	if maxRows <= 0 {
		maxRows = PreviewCollapsedLines
	}
	if len(lines) <= maxRows {
		return lines
	}
	visible := maxRows - 1
	if visible <= 0 {
		return []string{fmt.Sprintf("… %d earlier lines", len(lines))}
	}
	start := len(lines) - visible
	hidden := len(lines) - visible
	marker := fmt.Sprintf("… %d earlier lines", hidden)
	out := make([]string, 0, visible+1)
	out = append(out, marker)
	out = append(out, lines[start:]...)
	return out
}

// OutputBlockOptions mirrors oh-my-pi OutputBlockOptions (subset).
type OutputBlockOptions struct {
	Header      string
	HeaderMeta  string
	State       string // pending|running|success|error|warning
	Width       int
	BorderColor string
	Sections    []OutputBlockSection
}

type OutputBlockSection struct {
	Label     string
	Lines     []string
	Separator bool
}

func borderColorForState(state, override string) string {
	if override != "" {
		return override
	}
	switch state {
	case "error":
		return "error"
	case "warning":
		return "warning"
	case "running", "pending":
		return "accent"
	default:
		return "dim"
	}
}

// RenderOutputBlock renders a bordered card (header + sections) at given width.
// Emulates oh-my-pi renderOutputBlock but via Go lipgloss-agnostic strings.
func RenderOutputBlock(opts OutputBlockOptions) []string {
	width := opts.Width
	if width <= 0 {
		width = 80
	}
	border := borderColorForState(opts.State, opts.BorderColor)
	h := "─"
	tl, tr, bl, br := "╭", "╮", "╰", "╯"
	capStr := h + h + h
	label := strings.TrimSpace(opts.Header)
	if opts.HeaderMeta != "" {
		if label != "" {
			label += " · " + opts.HeaderMeta
		} else {
			label = opts.HeaderMeta
		}
	}
	topLabel := ""
	if label != "" {
		topLabel = " " + TruncateToWidth(label, width-6) + " "
	}
	_ = border // keep for parity; top includes borderColor hint
	top := tl + capStr + topLabel + strings.Repeat(h, max(0, width-len(capStr)-VisibleWidth(topLabel)-2)) + tr + " [" + border + "]"
	var lines []string
	lines = append(lines, top)
	if len(opts.Sections) == 0 {
		opts.Sections = []OutputBlockSection{{Lines: []string{}}}
	}
	for _, sec := range opts.Sections {
		if sec.Label != "" {
			lbl := " " + sec.Label + " "
			lines = append(lines, "├"+capStr+lbl+strings.Repeat(h, max(0, width-len(capStr)-VisibleWidth(lbl)-2))+"┤")
		} else if sec.Separator {
			lines = append(lines, "├"+capStr+strings.Repeat(h, max(0, width-len(capStr)-2))+"┤")
		}
		for _, l := range sec.Lines {
			for _, wrapped := range WrapTextWithAnsi(l, max(1, width-4)) {
				inner := wrapped + strings.Repeat(" ", max(0, width-4-VisibleWidth(wrapped)))
				lines = append(lines, "│ "+inner+" │")
			}
		}
	}
	bottom := bl + capStr + strings.Repeat(h, max(0, width-len(capStr)-2)) + br
	lines = append(lines, bottom)
	return lines
}

// CachedOutputBlock caches last render (dedupes visibleWidth computations).
type CachedOutputBlock struct {
	lastKey   string
	lastLines []string
}

func (c *CachedOutputBlock) Render(opts OutputBlockOptions) []string {
	key := fmt.Sprintf("%d|%s|%s|%s|%s|%d", opts.Width, opts.Header, opts.HeaderMeta, opts.State, opts.BorderColor, len(opts.Sections))
	for _, s := range opts.Sections {
		key += "|" + s.Label + fmt.Sprint(len(s.Lines))
		for _, l := range s.Lines {
			key += "|" + l
		}
	}
	if c.lastKey == key && c.lastLines != nil {
		return c.lastLines
	}
	lines := RenderOutputBlock(opts)
	c.lastKey = key
	c.lastLines = lines
	return lines
}

func (c *CachedOutputBlock) Invalidate() {
	c.lastKey = ""
	c.lastLines = nil
}
