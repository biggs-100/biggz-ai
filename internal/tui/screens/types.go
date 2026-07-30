package screens

import tea "github.com/charmbracelet/bubbletea"

// NavigateMsg tells the main model to switch screens.
type NavigateMsg struct {
	Screen int
}

// QuitMsg tells the main model to quit.
type QuitMsg struct{}

// MenuItem represents a selectable menu item.
type MenuItem struct {
	Key         string
	Label       string
	Description string
	Screen      int
}

// NavHelper provides common navigation for screens.
type NavHelper struct {
	Items   []MenuItem
	Cursor  int
}

// NewNavHelper creates a nav helper from menu items.
func NewNavHelper(items []MenuItem) NavHelper {
	return NavHelper{Items: items}
}

// Update handles up/down/enter navigation.
func (n *NavHelper) Update(msg tea.KeyMsg) (int, bool, bool) {
	switch msg.String() {
	case "up", "k":
		if n.Cursor > 0 {
			n.Cursor--
		}
	case "down", "j":
		if n.Cursor < len(n.Items)-1 {
			n.Cursor++
		}
	case "enter", " ":
		return n.Items[n.Cursor].Screen, true, false
	case "q":
		return 0, false, true
	}
	return 0, false, false
}
