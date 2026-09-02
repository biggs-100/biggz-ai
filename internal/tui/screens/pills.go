package screens

import (
	"fmt"
	"os"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
)

type Pill struct{ Label, State, Tool string }

const pillCollapseLimit = 3

func CollapsePills(pills []Pill, limit int) ([]Pill, string) {
	if limit <= 0 {
		limit = pillCollapseLimit
	}
	if len(pills) <= limit {
		return pills, ""
	}
	return pills[:limit], fmt.Sprintf("… +%d hidden", len(pills)-limit)
}

func RenderPills(pills []Pill) string {
	if len(pills) == 0 {
		return ""
	}
	if !styles.IsPrettyEnabled() || os.Getenv("TERM") == "dumb" {
		vis, suffix := CollapsePills(pills, pillCollapseLimit)
		parts := make([]string, 0, len(vis))
		for _, p := range vis {
			parts = append(parts, p.Label)
		}
		s := strings.Join(parts, " ")
		if suffix != "" {
			if s != "" {
				s += " "
			}
			s += suffix
		}
		return s
	}
	vis, suffix := CollapsePills(pills, pillCollapseLimit)
	var rendered []string
	for _, p := range vis {
		label := p.Label
		if strings.EqualFold(strings.TrimSpace(p.State), "running") {
			spinner := GetSpinnerFrame()
			if spinner == "·" && tuiAnimationsDisabled() {
				label = "· " + label
			} else {
				label = spinner + " " + label
			}
		}
		rendered = append(rendered, styles.PillStyle(p.State).Render(label))
	}
	s := strings.Join(rendered, " ")
	if suffix != "" {
		s += " \x1b[2m" + suffix + "\x1b[22m"
	}
	return s
}

func RenderPill(label, state string) string { return RenderPills([]Pill{{Label: label, State: state}}) }
func HighlightCode(text string) string {
	if !styles.IsPrettyEnabled() || os.Getenv("TERM") == "dumb" || strings.TrimSpace(text) == "" {
		return text
	}
	return styles.HelpStyle.Render(text)
}
