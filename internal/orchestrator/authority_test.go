package orchestrator

import (
	"strings"
	"testing"
)

func TestGuardSD(t *testing.T) {
	// RED: GuardSDAgentAuthority(spec,general) → SD Agent Authority
	err := GuardSDAgentAuthority("spec", "general")
	if err == nil || !strings.Contains(err.Error(), "SD Agent Authority") {
		t.Fatalf("expected SD Agent Authority error for spec/general, got %v", err)
	}
	err = GuardSDAgentAuthority("spec", "explore")
	if err == nil || !strings.Contains(err.Error(), "SD Agent Authority") {
		t.Fatalf("expected SD Agent Authority error for spec/explore, got %v", err)
	}
	// allowed: sdd-* for SDD phases
	if err := GuardSDAgentAuthority("spec", "sdd-spec"); err != nil {
		t.Fatalf("expected allow for sdd-spec, got %v", err)
	}
	if err := GuardSDAgentAuthority("apply", "sdd-apply"); err != nil {
		t.Fatalf("expected allow for sdd-apply, got %v", err)
	}
	if err := GuardSDAgentAuthority("verify", "sdd-verify"); err != nil {
		t.Fatalf("expected allow for sdd-verify, got %v", err)
	}
}

func TestGuardSDAgentAuthority_SDPhases(t *testing.T) {
	cases := []struct {
		phase string
		agent string
		wantOK bool
	}{
		{"propose", "general", false},
		{"spec", "general", false},
		{"design", "general", false},
		{"tasks", "general", false},
		{"apply", "general", false},
		{"verify", "general", false},
		{"archive", "general", false},
		{"explore", "explore", false},  // SDD explore via explore must be blocked, use sdd-explore
		{"research", "explore", false},
		{"propose", "sdd-propose", true},
		{"spec", "sdd-spec", true},
		{"design", "sdd-design", true},
		{"tasks", "sdd-tasks", true},
		{"apply", "sdd-apply", true},
		{"verify", "sdd-verify", true},
		{"archive", "sdd-archive", true},
		{"explore", "sdd-explore", true},
		{"research", "sdd-research", true},
		// non-SDD phases allow general
		{"other", "general", true},
		{"fix", "general", true},
		{"", "general", true},
	}
	for _, c := range cases {
		err := GuardSDAgentAuthority(c.phase, c.agent)
		ok := err == nil
		if ok != c.wantOK {
			t.Errorf("GuardSDAgentAuthority(%q,%q) ok=%v want %v err=%v", c.phase, c.agent, ok, c.wantOK, err)
		}
		if !ok && !strings.Contains(err.Error(), "SD Agent Authority") {
			t.Errorf("error for %q/%q must contain SD Agent Authority, got %q", c.phase, c.agent, err.Error())
		}
	}
}

func TestGuardSDAgentAuthority_CaseInsensitive(t *testing.T) {
	if err := GuardSDAgentAuthority("Spec", "General"); err == nil {
		t.Fatal("case-insensitive general should still block")
	}
	if err := GuardSDAgentAuthority("SPEC", "SDD-SPEC"); err != nil {
		t.Fatalf("case-insensitive sdd-spec should allow, got %v", err)
	}
}

func TestShouldSelectSDD_Ladder(t *testing.T) {
	// 12 files, 800 lines, no explicit SDD request → must NOT select SDD (Simple Delegation)
	if ShouldSelectSDD(false, 12, 800) {
		t.Fatal("ShouldSelectSDD(false,12,800) must be false — size alone never selects SDD")
	}
	// Even 50 files without explicit → no SDD
	if ShouldSelectSDD(false, 50, 5000) {
		t.Fatal("ShouldSelectSDD(false,50,5000) must be false")
	}
	// Explicit request selects SDD regardless of size
	if !ShouldSelectSDD(true, 1, 10) {
		t.Fatal("ShouldSelectSDD(true,1,10) must be true")
	}
	if !ShouldSelectSDD(true, 12, 800) {
		t.Fatal("ShouldSelectSDD(true,12,800) must be true")
	}
	// 0 files but explicit still true
	if !ShouldSelectSDD(true, 0, 0) {
		t.Fatal("ShouldSelectSDD(true,0,0) must be true")
	}
}
