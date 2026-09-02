package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/screens"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
)

// fixture describes one help screen for gallery mirroring oh-my-pi gallery-fixtures.
type fixture struct {
	ScreenID  int               `json:"screenID"`
	Title     string            `json:"title"`
	Paragraph string            `json:"paragraph"`
	Keys      []screens.HelpKey `json:"keys"`
	Width     int               `json:"width"`
	Preview   string            `json:"preview"`
}

func main() {
	outDirs := []string{"docs/gallery", ".biggz/gallery"}
	// Allow override via first arg.
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		outDirs = []string{os.Args[1]}
	}
	// Ensure BIGGZ_PRETTY check doesn't mute styles when generating gallery.
	// Force pretty for capture so rendering matches live TUI.
	_ = styles.IsPrettyEnabled()

	ok := false
	for _, dir := range outDirs {
		if err := generate(dir); err != nil {
			fmt.Fprintf(os.Stderr, "gallery generate %s: %v\n", dir, err)
		} else {
			fmt.Printf("gallery written to %s\n", dir)
			ok = true
		}
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "gallery: no output dir writable")
		os.Exit(1)
	}
}

func generate(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	// Help screen IDs curated from screens/help.go helpData.
	helpIDs := []int{0, 12, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 14, 15, 16, 18, 19, 20}
	var fixtures []fixture

	widths := []int{80, 100}
	for _, id := range helpIDs {
		h := screens.GetHelp(id)
		// Normalize via repair+latex for stable preview.
		h.Paragraph = screens.LatexToUnicode(screens.RepairOrphanClosingFence(h.Paragraph))
		for _, w := range widths {
			// Deterministic 80/100 via HelpOverlayWidth(w) + VisibleWidth check.
			overlay := screens.HelpOverlayWidth(id, w)
			// Verify wrapping matches live View() truncation at same width.
			for _, line := range strings.Split(overlay, "\n") {
				if screens.VisibleWidth(line) > w {
					overlay = strings.ReplaceAll(overlay, line, screens.TruncateToWidth(line, w))
				}
			}
			fname := filepath.Join(outDir, fmt.Sprintf("help-%02d-%d.ansi", id, w))
			if err := os.WriteFile(fname, []byte(overlay), 0o644); err != nil {
				return err
			}
		}
		// Build JSON fixture entry (width 80 representative).
		preview := strings.ReplaceAll(h.Paragraph, "\n", " ")
		if len(preview) > 120 {
			preview = preview[:120] + "…"
		}
		fixtures = append(fixtures, fixture{
			ScreenID:  id,
			Title:     h.Title,
			Paragraph: h.Paragraph,
			Keys:      h.Keys,
			Width:     80,
			Preview:   preview,
		})
		// Per-screen markdown txt (plain without ANSI) for quick review.
		txtPath := filepath.Join(outDir, fmt.Sprintf("help-%02d.txt", id))
		var b strings.Builder
		b.WriteString("# " + h.Title + "\n\n")
		b.WriteString(screens.RepairOrphanClosingFence(h.Paragraph) + "\n\n")
		b.WriteString("## Keys\n")
		for _, k := range h.Keys {
			b.WriteString(fmt.Sprintf("- %s — %s\n", k.Key, k.Desc))
		}
		if err := os.WriteFile(txtPath, []byte(screens.LatexToUnicode(b.String())), 0o644); err != nil {
			return err
		}
	}

	// Dashboard fixtures.
	dash := screens.NewDashboardModel()
	dashView := dash.View()
	if err := os.WriteFile(filepath.Join(outDir, "dashboard.ansi"), []byte(dashView), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "dashboard.txt"), []byte(stripANSI(dashView)), 0o644); err != nil {
		return err
	}

	// FleetRow spinner gallery: N live blocks phase-locked vs static.
	var fleetLines []string
	for i := 0; i < 3; i++ {
		l1, l2, _ := screens.RenderFleetRow(80, screens.FleetRowInput{
			Glyph:        "",
			Agent:        fmt.Sprintf("agent-%d", i+1),
			Model:        "claude-sonnet",
			State:        "running",
			ElapsedSec:   12 + i*5,
			WindowTokens: 1200 + i*200,
			SpentTokens:  800 + i*100,
			Tool:         "edit",
			Activity:     "applying patch",
		})
		fleetLines = append(fleetLines, l1, l2)
	}
	// Capture shared spinner frame index for determinism.
	screens.AdvanceSpinnerFrame()
	spinner := screens.GetSpinnerFrame()
	fleetContent := fmt.Sprintf("shared spinner frame: %q idx=%d\n\n", spinner, screens.GetSharedSpinnerIndex()) + strings.Join(fleetLines, "\n")
	if err := os.WriteFile(filepath.Join(outDir, "fleet-live.ansi"), []byte(fleetContent), 0o644); err != nil {
		return err
	}

	// OutputBlock gallery lifecycle.
	states := []string{"streaming", "progress", "success", "error"}
	for _, st := range states {
		lines := screens.RenderOutputBlock(screens.OutputBlockOptions{
			Header: "tool: read",
			State:  st,
			Width:  80,
			Sections: []screens.OutputBlockSection{
				{Label: "output", Lines: []string{"sample line for " + st, "second line — width aware"}},
			},
		})
		if err := os.WriteFile(filepath.Join(outDir, fmt.Sprintf("output-%s.ansi", st)), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			return err
		}
	}

	// Fixtures JSON (mirrors oh-my-pi gallery-fixtures index).
	fixturePath := filepath.Join(outDir, "fixtures.json")
	jb, err := json.MarshalIndent(fixtures, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(fixturePath, jb, 0o644); err != nil {
		return err
	}

	// README placeholder.
	readme := `# biggz-ai Gallery

Generated by ` + "`go run ./scripts/gallery`" + `.

This gallery mirrors ` + "`oh-my-pi/packages/coding-agent/src/cli/gallery-cli.ts`" + ` fixtures.
Each help screen is rendered at widths 80/100 via ` + "`screens.HelpOverlay`" + ` and
` + "`screens.RenderFleetRow`" + `/` + "`RenderOutputBlock`" + ` phase-locked 80ms spinner.

Outputs:
- ` + "`help-*.ansi`" + ` — ANSI rendered help overlays (width variants)
- ` + "`help-*.txt`" + ` — plain markdown fixtures
- ` + "`dashboard.ansi/.txt`" + ` — dashboard screen
- ` + "`fleet-live.ansi`" + ` — N live FleetRows sharing one 80ms ticker (GetSpinnerFrame phase-locked)
- ` + "`output-*.ansi`" + ` — OutputBlock lifecycle (streaming/progress/success/error)
- ` + "`fixtures.json`" + ` — machine-readable help fixtures (title/paragraph/keys)

Regenerate:

` + "```bash" + `
go run ./scripts/gallery
# or custom dir
go run ./scripts/gallery -- docs/gallery
` + "```" + `

PNG via kitty-sixel:
The harness writes ANSI text by default (no extra deps). For pixel-perfect PNG,
pipe an ` + "`.ansi`" + ` file through a truecolor terminal recorder (VHS/kitty)
or ` + "`lipgloss`" + ` renderer; the ANSI already carries 24-bit color escapes.

Pretty guard:
` + "`BIGGZ_PRETTY=0`" + ` or ` + "`PI_SUBAGENT_CHILD=1`" + ` disables spinner animation and falls back to static glyph ` + "`·`" + `; the gallery respects the same guard.
`
	if err := os.WriteFile(filepath.Join(outDir, "README.md"), []byte(readme), 0o644); err != nil {
		return err
	}

	// Also ensure kitty-sixel note file exists for harness parity.
	sixelNote := "kitty-sixel harness: ANSI files can be rendered via `kitty +kitten icat` or VHS. PNG output not auto-generated in CI to avoid heavy deps.\n"
	_ = os.WriteFile(filepath.Join(outDir, ".sixel-note"), []byte(sixelNote), 0o644)

	return nil
}

func stripANSI(s string) string {
	// Minimal ANSI strip for .txt
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if !inEsc && s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			inEsc = true
			i++
			continue
		}
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
