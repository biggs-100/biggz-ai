package tui

import (
	"testing"
)

// TestNextPrev walks the full wizard order forward and back.
func TestNextPrev(t *testing.T) {
	wantOrder := []WizardStage{
		StageWelcome,
		StageDetection,
		StageAgents,
		StagePersona,
		StagePreset,
		StageDepTree,
		StageSkillPicker,
		StageReview,
		StageInstalling,
		StageComplete,
	}

	if len(linearRoutes) != len(wantOrder) {
		t.Fatalf("linearRoutes has %d stages, want %d", len(linearRoutes), len(wantOrder))
	}

	// Forward walk from Welcome.
	cur := StageWelcome
	for i, want := range wantOrder {
		if cur != want {
			t.Fatalf("forward step %d: got stage %d, want %d", i, cur, want)
		}
		if i == len(wantOrder)-1 {
			break
		}
		next, ok := NextStage(cur)
		if !ok {
			t.Fatalf("NextStage(%d): ok=false, want true", cur)
		}
		cur = next
	}

	// Past-the-end: no forward from Complete.
	if next, ok := NextStage(StageComplete); ok {
		t.Fatalf("NextStage(Complete) = (%d, true), want ok=false", next)
	}

	// Backward walk from Complete.
	cur = StageComplete
	for i := len(wantOrder) - 1; i >= 0; i-- {
		if cur != wantOrder[i] {
			t.Fatalf("backward step %d: got stage %d, want %d", i, cur, wantOrder[i])
		}
		if i == 0 {
			break
		}
		prev, ok := PrevStage(cur)
		if !ok {
			t.Fatalf("PrevStage(%d): ok=false, want true", cur)
		}
		cur = prev
	}

	// Past-the-start: no back from Welcome.
	if prev, ok := PrevStage(StageWelcome); ok {
		t.Fatalf("PrevStage(Welcome) = (%d, true), want ok=false", prev)
	}
}

// TestRouterRejectsJumps asserts only adjacent moves are possible:
// no jump helper exists, and unknown stages are rejected.
func TestRouterRejectsJumps(t *testing.T) {
	// Detection must resolve forward to Agents (REQ-WIZ-003 ordered routing).
	next, ok := NextStage(StageDetection)
	if !ok || next != StageAgents {
		t.Fatalf("NextStage(Detection) = (%d, %v), want (StageAgents, true)", next, ok)
	}

	// Unknown stages are rejected, not clamped into the route.
	for _, bogus := range []WizardStage{WizardStage(-1), WizardStage(99)} {
		if _, ok := NextStage(bogus); ok {
			t.Fatalf("NextStage(%d): ok=true, want false (jump rejected)", bogus)
		}
		if _, ok := PrevStage(bogus); ok {
			t.Fatalf("PrevStage(%d): ok=true, want false (jump rejected)", bogus)
		}
	}

	// Every stage moves at most one hop per call.
	for stage := range linearRoutes {
		if next, ok := NextStage(stage); ok {
			if next != stage+1 {
				t.Fatalf("NextStage(%d) = %d, want adjacent %d", stage, next, stage+1)
			}
		}
		if prev, ok := PrevStage(stage); ok {
			if prev != stage-1 {
				t.Fatalf("PrevStage(%d) = %d, want adjacent %d", stage, prev, stage-1)
			}
		}
	}
}

// TestLegacyInstallFallback covers the BIGGZ_LEGACY_INSTALL=1 gate.
func TestLegacyInstallFallback(t *testing.T) {
	t.Setenv("BIGGZ_LEGACY_INSTALL", "1")
	if !LegacyInstall() {
		t.Fatal("LegacyInstall() = false with BIGGZ_LEGACY_INSTALL=1, want true")
	}

	t.Setenv("BIGGZ_LEGACY_INSTALL", "")
	if LegacyInstall() {
		t.Fatal("LegacyInstall() = true with flag unset, want false")
	}
}
