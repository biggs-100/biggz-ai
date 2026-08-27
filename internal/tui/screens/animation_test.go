package screens

import (
	"os"
	"testing"
)

func setNoAnimationEnv(t *testing.T, value *string) {
	t.Helper()
	for _, name := range []string{"BIGGZ_NO_ANIMATION", "GENTLE_AI_NO_ANIMATION"} {
		// We manage both, but tests set only one; ensure the other is unset unless explicitly set.
		// This helper only sets BIGGZ; gentle compat is handled separately in compat test.
		_ = name
	}
	const name = "BIGGZ_NO_ANIMATION"
	previous, wasSet := os.LookupEnv(name)
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv(name, previous)
		} else {
			_ = os.Unsetenv(name)
		}
	})
	var err error
	if value == nil {
		err = os.Unsetenv(name)
	} else {
		err = os.Setenv(name, *value)
	}
	if err != nil {
		t.Fatalf("set %s: %v", name, err)
	}
}

func setGentleNoAnimationEnv(t *testing.T, value *string) {
	t.Helper()
	const name = "GENTLE_AI_NO_ANIMATION"
	previous, wasSet := os.LookupEnv(name)
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv(name, previous)
		} else {
			_ = os.Unsetenv(name)
		}
	})
	var err error
	if value == nil {
		err = os.Unsetenv(name)
	} else {
		err = os.Setenv(name, *value)
	}
	if err != nil {
		t.Fatalf("set %s: %v", name, err)
	}
}

func TestAnimationTickRequiresExactOne(t *testing.T) {
	one := "1"
	zero := "0"
	empty := ""
	truthy := "true"
	tests := []struct {
		name    string
		value   *string
		wantNil bool
	}{
		{name: "exact one disables tick", value: &one, wantNil: true},
		{name: "unset preserves tick", value: nil, wantNil: false},
		{name: "empty preserves tick", value: &empty, wantNil: false},
		{name: "zero preserves tick", value: &zero, wantNil: false},
		{name: "other preserves tick", value: &truthy, wantNil: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setNoAnimationEnv(t, tt.value)
			setGentleNoAnimationEnv(t, nil)
			cmd := tickCmd()
			if tt.wantNil && cmd != nil {
				t.Fatal("tickCmd should be nil when animation disabled")
			}
			if !tt.wantNil && cmd == nil {
				t.Fatal("tickCmd should not be nil when animation enabled")
			}
		})
	}
}

func TestAnimationGentleCompat(t *testing.T) {
	one := "1"
	setNoAnimationEnv(t, nil)
	setGentleNoAnimationEnv(t, &one)
	if !tuiAnimationsDisabled() {
		t.Fatal("expected animation disabled via GENTLE_AI_NO_ANIMATION=1")
	}
	if tickCmd() != nil {
		t.Fatal("tickCmd should be nil with gentle compat env")
	}
}

func TestAnimationUpdateKeepsStaticFrame(t *testing.T) {
	one := "1"
	setNoAnimationEnv(t, &one)
	setGentleNoAnimationEnv(t, nil)

	m := NewAgentBuilderScreen()
	m.generating = true
	m.spinner = 3
	updated, cmd := m.Update(abTickMsg{})
	state := updated.(AgentBuilderScreen)
	if state.spinner != 3 {
		t.Fatalf("spinner frame = %d, want 3 (static when disabled)", state.spinner)
	}
	if cmd != nil {
		t.Fatal("tick command should not be rescheduled when animation is disabled")
	}
}

func TestAnimationUpdateAdvancesWhenEnabled(t *testing.T) {
	setNoAnimationEnv(t, nil)
	setGentleNoAnimationEnv(t, nil)

	m := NewAgentBuilderScreen()
	m.generating = true
	m.spinner = 3
	updated, cmd := m.Update(abTickMsg{})
	state := updated.(AgentBuilderScreen)
	if state.spinner != 4 {
		t.Fatalf("spinner frame = %d, want 4", state.spinner)
	}
	if cmd == nil {
		t.Fatal("tick command should be rescheduled when animation enabled")
	}
}
