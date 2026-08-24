package tui

import (
	"os"
	"testing"
)

func setNoAnimationEnv(t *testing.T, name string, value *string) {
	t.Helper()
	if value == nil {
		orig, wasSet := os.LookupEnv(name)
		t.Cleanup(func() {
			if wasSet {
				_ = os.Setenv(name, orig)
			} else {
				_ = os.Unsetenv(name)
			}
		})
		_ = os.Unsetenv(name)
		return
	}
	orig, wasSet := os.LookupEnv(name)
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv(name, orig)
		} else {
			_ = os.Unsetenv(name)
		}
	})
	_ = os.Setenv(name, *value)
}

func TestAnimationRequiresExactOne(t *testing.T) {
	one := "1"
	zero := "0"
	empty := ""
	truthy := "true"
	tests := []struct {
		name     string
		biggzVal *string
		gentleVal *string
		want     bool
	}{
		{name: "exact one disables", biggzVal: &one, want: true},
		{name: "unset preserves animation", biggzVal: nil, want: false},
		{name: "empty preserves animation", biggzVal: &empty, want: false},
		{name: "zero preserves animation", biggzVal: &zero, want: false},
		{name: "other value preserves animation", biggzVal: &truthy, want: false},
		{name: "gentle compat disables", gentleVal: &one, want: true},
		{name: "biggz takes precedence", biggzVal: &one, gentleVal: &zero, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setNoAnimationEnv(t, "BIGGZ_NO_ANIMATION", tt.biggzVal)
			setNoAnimationEnv(t, "GENTLE_AI_NO_ANIMATION", tt.gentleVal)
			got := tuiAnimationsDisabled()
			if got != tt.want {
				t.Fatalf("tuiAnimationsDisabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnimationDisabledWithEnv(t *testing.T) {
	one := "1"
	setNoAnimationEnv(t, "BIGGZ_NO_ANIMATION", &one)
	if !tuiAnimationsDisabled() {
		t.Fatal("expected animation disabled with BIGGZ_NO_ANIMATION=1")
	}
	// Ensure other values do not disable.
	zero := "0"
	setNoAnimationEnv(t, "BIGGZ_NO_ANIMATION", &zero)
	// This subtest runs with BIGGZ=0, need to unset gentle as well.
	setNoAnimationEnv(t, "GENTLE_AI_NO_ANIMATION", nil)
	if tuiAnimationsDisabled() {
		t.Fatal("expected animation enabled with BIGGZ_NO_ANIMATION=0")
	}
}
