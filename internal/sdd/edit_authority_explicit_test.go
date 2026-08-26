package sdd

import "testing"

func TestHasExplicitEditIntent(t *testing.T) {
	tests := []struct {
		prompt string
		want   bool
	}{
		{"apply the fix to internal/sdd/status.go", true},
		{"please apply to internal/review/store.go with the burn fix", true},
		{"investigate the status bug", false},
		{"explore the codebase for sdd status", false},
		{"check the filecoord lock", false},
		{"look into the research divergence", false},
		{"fix it if possible", false},
		{"maybe update the task", false},
		{"consider updating the design", false},
		{"when ready apply the change", false},
	}
	for _, tc := range tests {
		got := HasExplicitEditIntent(tc.prompt)
		if got != tc.want {
			t.Errorf("HasExplicitEditIntent(%q) = %v, want %v", tc.prompt, got, tc.want)
		}
	}
}
