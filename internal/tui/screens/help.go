package screens

import "github.com/biggz-ai/biggz/internal/tui/styles"

// HelpContent describes the help for a screen.
type HelpContent struct {
	Title    string
	Keys     []HelpKey
	Paragraph string
}

// HelpKey describes a keyboard shortcut.
type HelpKey struct {
	Key  string
	Desc string
}

// helpData maps screen IDs to help content.
var helpData = map[int]HelpContent{
	0: {
		Title: "Welcome / Main Menu",
		Keys: []HelpKey{
			{"↑↓", "Navigate menu items"},
			{"ENTER", "Select highlighted item"},
			{"i / c / s / m / q", "Shortcut keys for each screen"},
			{"?", "Toggle this help"},
			{"ESC", "Go back / quit from main menu"},
			{"CTRL+C", "Force quit"},
		},
		Paragraph: "biggs-ai is a Review-Driven Development harness for AI coding agents. Use this TUI to install, configure, and monitor biggz-ai.",
	},
	1: {
		Title: "Install",
		Keys: []HelpKey{
			{"ENTER", "Start installation"},
			{"↑↓", "Select agent (if multiple detected)"},
			{"R", "Retry after error / reinstall"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Installs biggz-ai in your AI coding agent. Detects OpenCode, Claude Code, or Qwen Code. Deploys skills, commands, MCP, persona, and agent configuration.",
	},
	2: {
		Title: "Configuration",
		Keys: []HelpKey{
			{"← →", "Switch between tabs (Models / Persona / Delivery / Save)"},
			{"↑↓", "Navigate items in current tab"},
			{"ENTER", "Select / toggle / save"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Configure model assignments for each SDD phase, choose the agent persona, and set delivery strategy and review budget. All changes are persisted to opencode.json.",
	},
	3: {
		Title: "Status",
		Keys: []HelpKey{
			{"R", "Refresh all status data"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Shows real-time status of RDD (Review-Driven Development), active SDD changes, and system health. Data is fetched from the live system on refresh.",
	},
	4: {
		Title: "BigMem Memory",
		Keys: []HelpKey{
			{"R", "Load memories from BigMem"},
			{"↑↓", "Navigate memory list"},
			{"ENTER", "View memory details"},
			{"ESC", "Back to list / main menu"},
		},
		Paragraph: "Browse persistent memories stored in BigMem. Each entry is a decision, bug fix, discovery, or convention saved automatically by the orchestrator or sub-agents.",
	},
	5: {
		Title: "Backup & Restore",
		Keys: []HelpKey{
			{"ENTER", "Create or restore a backup"},
			{"↑↓", "Select backup entry"},
			{"R", "Refresh backup list"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Create snapshots of biggz-ai state and restore them later. Backups include skills, configuration, and persistent memory.",
	},
	6: {
		Title: "Profiles",
		Keys: []HelpKey{
			{"R", "Refresh profile list"},
			{"S", "Save current config as a named profile"},
			{"ENTER", "Load selected profile"},
			{"↑↓", "Navigate profile list"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Save and load named configurations. Profiles store model assignments, delivery strategy, review budget, and persona settings as a complete snapshot of opencode.json.",
	},
	7: {
		Title: "Update",
		Keys: []HelpKey{
			{"ENTER", "Check for updates"},
			{"R", "Refresh check"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Check for new versions of biggz-ai on GitHub. When an update is available, you can download and install it directly from this screen.",
	},
	8: {
		Title: "Uninstall",
		Keys: []HelpKey{
			{"ENTER", "Start uninstall"},
			{"Y", "Confirm uninstall"},
			{"N", "Cancel uninstall"},
			{"R", "Retry after error"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Remove biggz-ai from your system. Deletes ~/.biggz/, removes the biggz-orchestrator agent from opencode.json, and strips the persona from AGENTS.md. gentle-ai data is NOT affected.",
	},
	9: {
		Title: "Strict TDD",
		Keys: []HelpKey{
			{"ENTER", "Load state / toggle mode"},
			{"↑↓", "Toggle enable/disable"},
			{"R", "Refresh"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Strict TDD mode requires tests to pass before code can be committed. When enabled, sdd-apply will enforce test passing before accepting changes.",
	},
	10: {
		Title: "Review Timeline",
		Keys: []HelpKey{
			{"ENTER", "Load lineages / view detail"},
			{"R", "Refresh lineage list"},
			{"↑↓", "Navigate lineages"},
			{"ESC", "Back to list / main menu"},
		},
		Paragraph: "Browse review lineages stored in the review transaction store. Each lineage represents a complete review lifecycle with events, findings, and corrections.",
	},
	11: {
		Title: "Sessions",
		Keys: []HelpKey{
			{"ENTER", "Load session list"},
			{"R", "Refresh"},
			{"↑↓", "Navigate sessions"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Browse BigMem sessions. Each session groups observations from a single coding conversation.",
	},
}

// GetHelp returns the help content for a screen ID.
func GetHelp(screenID int) HelpContent {
	if h, ok := helpData[screenID]; ok {
		return h
	}
	return HelpContent{Title: "Unknown Screen", Keys: []HelpKey{{"ESC", "Go back"}}}
}

// HelpOverlay renders the help as a styled string.
func HelpOverlay(screenID int) string {
	h := GetHelp(screenID)
	var b []byte
	b = append(b, styles.Title.Render("Help: "+h.Title)...)
	b = append(b, '\n', '\n')
	if h.Paragraph != "" {
		b = append(b, styles.StatusInfo.Render(h.Paragraph)...)
		b = append(b, '\n', '\n')
	}
	b = append(b, styles.Section.Render("Keyboard Shortcuts")...)
	b = append(b, '\n', '\n')
	for _, k := range h.Keys {
		b = append(b, "  "...)
		b = append(b, styles.MenuItemKey.Render(k.Key)...)
		b = append(b, "  — "...)
		b = append(b, k.Desc...)
		b = append(b, '\n')
	}
	b = append(b, '\n')
	b = append(b, styles.Help.Render("Press ? or ESC to close help")...)
	return styles.AppStyle.Render(string(b))
}
