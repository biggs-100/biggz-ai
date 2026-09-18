package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSubagentRuntimeMarker(t *testing.T, home string) {
	t.Helper()
	marker := SubagentRuntimeMarkerPath(home)
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatalf("mkdir marker dir: %v", err)
	}
	if err := os.WriteFile(marker, []byte("export default function biggzSubagentRuntime(pi) {}\n"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
}

func writePiSettingsPackages(t *testing.T, home, packagesJSON string) {
	t.Helper()
	path := subagentRuntimeSettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir settings dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"packages":`+packagesJSON+`}`), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}

func TestSubagentRuntimeMarkerPath_HonorsPICodingAgentDir(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	want := filepath.Join(home, ".pi", "agent", "extensions", SubagentRuntimeTargetName)
	if got := SubagentRuntimeMarkerPath(home); got != want {
		t.Fatalf("marker path = %q, want %q", got, want)
	}

	override := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", override)
	wantOverride := filepath.Join(override, "extensions", SubagentRuntimeTargetName)
	if got := SubagentRuntimeMarkerPath(home); got != wantOverride {
		t.Fatalf("override marker path = %q, want %q", got, wantOverride)
	}
}

// Capability ready when the runtime marker is deployed and j0k3r is absent
// from settings packages — a missing settings.json proves nothing installed.
func TestSubagentRuntimeCapability_MarkerReady(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	writeSubagentRuntimeMarker(t, home)
	if got := SubagentRuntimeCapability(home); got != BackgroundCapabilityReady {
		t.Fatalf("capability = %q, want %q (marker-only, no settings.json)", got, BackgroundCapabilityReady)
	}
	writePiSettingsPackages(t, home, `["npm:@heyhuynhgiabuu/pi-pretty"]`)
	if got := SubagentRuntimeCapability(home); got != BackgroundCapabilityReady {
		t.Fatalf("capability = %q, want %q (marker + j0k3r-free packages)", got, BackgroundCapabilityReady)
	}
}

func TestSubagentRuntimeCapability_AbsentWithoutMarker(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	writePiSettingsPackages(t, home, `["npm:@heyhuynhgiabuu/pi-pretty"]`)
	if got := SubagentRuntimeCapability(home); got != BackgroundCapabilityAbsent {
		t.Fatalf("capability = %q, want %q without marker", got, BackgroundCapabilityAbsent)
	}
}

func TestSubagentRuntimeCapability_J0k3rAloneNotReady(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	writePiSettingsPackages(t, home, `["npm:pi-subagents-j0k3r@1.6.1"]`)
	if got := SubagentRuntimeCapability(home); got != BackgroundCapabilityAbsent {
		t.Fatalf("capability = %q, want %q (j0k3r alone, no marker)", got, BackgroundCapabilityAbsent)
	}
}

// Marker plus j0k3r in settings packages is dual registration: `ready` would lie.
func TestSubagentRuntimeCapability_MarkerWithJ0k3rNotReady(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	writeSubagentRuntimeMarker(t, home)
	writePiSettingsPackages(t, home, `["npm:pi-subagents-j0k3r@1.6.1","npm:@heyhuynhgiabuu/pi-pretty"]`)
	if got := SubagentRuntimeCapability(home); got != BackgroundCapabilityAbsent {
		t.Fatalf("capability = %q, want %q (marker + j0k3r must not be ready)", got, BackgroundCapabilityAbsent)
	}
}

func TestSubagentRuntimeCapability_CorruptSettingsNotReady(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	writeSubagentRuntimeMarker(t, home)
	path := subagentRuntimeSettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SubagentRuntimeCapability(home); got != BackgroundCapabilityAbsent {
		t.Fatalf("capability = %q, want %q (corrupt settings cannot prove absence)", got, BackgroundCapabilityAbsent)
	}
}

func TestSubagentRuntimeCapability_PICodingAgentDirOverride(t *testing.T) {
	override := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", override)
	home := t.TempDir() // no marker here
	marker := filepath.Join(override, "extensions", SubagentRuntimeTargetName)
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("export default function f(pi) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SubagentRuntimeCapability(home); got != BackgroundCapabilityReady {
		t.Fatalf("capability = %q, want %q (PI_CODING_AGENT_DIR marker)", got, BackgroundCapabilityReady)
	}
}

func TestResolveBackgroundSubagentsCapability_DelegatesToMarkerOwner(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	if got := ResolveBackgroundSubagentsCapability(home); got != BackgroundCapabilityAbsent {
		t.Fatalf("capability = %q, want %q", got, BackgroundCapabilityAbsent)
	}
	writeSubagentRuntimeMarker(t, home)
	if got := ResolveBackgroundSubagentsCapability(home); got != BackgroundCapabilityReady {
		t.Fatalf("capability = %q, want %q", got, BackgroundCapabilityReady)
	}
}

// Delta runtime — "Disabled reporting when policy off": policy `off` plus
// capability `absent` MUST render `policy: off`, `capability: absent` and the
// `disabled/unmanaged` notice, typed as a warning.
func TestRenderBackgroundSubagentsReport_Disabled(t *testing.T) {
	r := BackgroundSubagentsResolution{Policy: BackgroundPolicyOff, Source: BackgroundSourceDefault}
	report := RenderBackgroundSubagentsReport(r, BackgroundCapabilityAbsent, nil)
	for _, token := range []string{"policy: off", "capability: absent", "disabled/unmanaged"} {
		if !strings.Contains(report.Message, token) {
			t.Fatalf("disabled report missing %q:\n%s", token, report.Message)
		}
	}
	if report.Type != "warning" {
		t.Fatalf("disabled report type = %q, want warning", report.Type)
	}
	statusLine := "background subagents: off (decided by built-in default; capability: absent)"
	if !strings.Contains(report.Message, statusLine) {
		t.Fatalf("status line drifted, want %q in:\n%s", statusLine, report.Message)
	}
}

// The notice is driven by policy `off` OR capability absent, and never renders
// while background delegation is usable.
func TestRenderBackgroundSubagentsReport_DisabledStateMatrix(t *testing.T) {
	project := BackgroundSubagentsResolution{Policy: BackgroundPolicyOn, Source: BackgroundSourceProject, ProjectFile: "/x/.biggz/background-subagents.json"}
	cases := []struct {
		name       string
		res        BackgroundSubagentsResolution
		capability string
		wantNotice bool
		wantType   string
	}{
		{"policy off + capability absent is disabled and unmanaged", BackgroundSubagentsResolution{Policy: BackgroundPolicyOff, Source: BackgroundSourceDefault}, BackgroundCapabilityAbsent, true, "warning"},
		{"policy off + capability ready is disabled", BackgroundSubagentsResolution{Policy: BackgroundPolicyOff, Source: BackgroundSourceGlobal, GlobalFile: "/g/background-subagents.json"}, BackgroundCapabilityReady, true, "warning"},
		{"policy on + capability absent is unmanaged", project, BackgroundCapabilityAbsent, true, "warning"},
		{"policy on + capability ready stays informational", project, BackgroundCapabilityReady, false, "info"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := RenderBackgroundSubagentsReport(tc.res, tc.capability, nil)
			if got := strings.Contains(report.Message, "disabled/unmanaged"); got != tc.wantNotice {
				t.Fatalf("disabled/unmanaged notice = %v, want %v:\n%s", got, tc.wantNotice, report.Message)
			}
			if report.Type != tc.wantType {
				t.Fatalf("report type = %q, want %q:\n%s", report.Type, tc.wantType, report.Message)
			}
			if tc.wantNotice {
				want := "policy: " + tc.res.Policy.String() + ", capability: " + tc.capability
				if !strings.Contains(report.Message, want) {
					t.Fatalf("notice missing %q:\n%s", want, report.Message)
				}
			}
		})
	}
}
