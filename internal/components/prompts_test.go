package components

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

// TestExtractModelSection verifies that extractModelSection correctly extracts
// the capable and small sections from a skill file with dual markers.
func TestExtractModelSection(t *testing.T) {
	data, err := assets.FS.ReadFile("skills/sdd-apply/SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(sdd-apply/SKILL.md) error = %v", err)
	}
	content := string(data)

	capable := extractModelSection(content, "capable")
	small := extractModelSection(content, "small")

	if capable == content {
		t.Fatal("capable extraction returned full content, expected filtered section")
	}
	if small == content {
		t.Fatal("small extraction returned full content, expected filtered section")
	}
	if capable == small {
		t.Fatal("capable and small sections are identical, expected different content")
	}

	// Capable should contain the full verbose workflow (e.g., Strict TDD Hard Gate)
	if !strings.Contains(capable, "Strict TDD") {
		t.Error("capable section missing expected 'Strict TDD' content")
	}
	// Small should contain the condensed 9-step checklist marker
	if !strings.Contains(small, "max 3 files") {
		t.Error("small section missing expected 'max 3 files' constraint")
	}
	// Capable should NOT contain small's condensed marker (or vice versa check)
	if strings.Contains(capable, "max 3 files") {
		t.Error("capable section should not contain small-only 'max 3 files'")
	}

	// Small vs capable length: small should be significantly shorter (~70L vs 270L)
	if len(small) >= len(capable) {
		t.Errorf("small len %d should be < capable len %d", len(small), len(capable))
	}
}

// TestExtractModelSectionFallback verifies that content without markers is returned unchanged.
func TestExtractModelSectionFallback(t *testing.T) {
	plain := "# Plain content\nNo markers here."
	if got := extractModelSection(plain, "capable"); got != plain {
		t.Fatalf("fallback: got %q, want original plain content", got)
	}
	if got := extractModelSection(plain, "small"); got != plain {
		t.Fatalf("fallback small: got %q, want original", got)
	}
}

// TestExtractModelSectionGoldenCapableNotEqualSmall is the golden check that
// sdd-apply capable != small as required by the spec.
func TestExtractModelSectionGoldenCapableNotEqualSmall(t *testing.T) {
	skillNames := []string{"sdd-apply", "sdd-verify", "sdd-spec", "sdd-design", "sdd-tasks", "sdd-archive", "sdd-explore", "sdd-init", "sdd-propose", "sdd-onboard"}
	for _, name := range skillNames {
		data, err := assets.FS.ReadFile("skills/" + name + "/SKILL.md")
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", name, err)
		}
		content := string(data)
		capable := extractModelSection(content, "capable")
		small := extractModelSection(content, "small")
		if capable == small {
			t.Errorf("%s: capable == small, expected distinct tiered content", name)
		}
		if len(small) == 0 {
			t.Errorf("%s: small section empty", name)
		}
		if len(capable) == 0 {
			t.Errorf("%s: capable section empty", name)
		}
		// Small should be 60-80 lines (approx 2000-5000 chars) and capable 150+ lines
		smallLines := strings.Count(small, "\n")
		capableLines := strings.Count(capable, "\n")
		if smallLines > 120 {
			t.Errorf("%s: small too long %d lines, want <=120", name, smallLines)
		}
		if capableLines < 50 {
			t.Errorf("%s: capable too short %d lines, want >=50", name, capableLines)
		}
	}
}

// TestSharedPromptDir verifies the expected directory path is returned.
func TestSharedPromptDir(t *testing.T) {
	want := filepath.FromSlash("/home/testuser/.config/opencode/prompts/sdd")
	got := SharedPromptDir(filepath.FromSlash("/home/testuser"))
	if got != want {
		t.Fatalf("SharedPromptDir(%q) = %q, want %q", "/home/testuser", got, want)
	}
}

func TestSharedPromptDirUsesXDGConfigHome(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	want := filepath.Join(xdg, "opencode", "prompts", "sdd")
	if got := SharedPromptDir(home); got != want {
		t.Fatalf("SharedPromptDir() = %q, want %q", got, want)
	}
}

// TestWriteSharedPromptFilesCreates10Files verifies that WriteSharedPromptFiles
// creates exactly the 10 expected prompt files under {homeDir}/.config/opencode/prompts/sdd/.
func TestWriteSharedPromptFilesCreates10Files(t *testing.T) {
	home := t.TempDir()
	changed, err := WriteSharedPromptFiles(home, nil)
	if err != nil {
		t.Fatalf("WriteSharedPromptFiles() error = %v", err)
	}
	if !changed {
		t.Fatal("WriteSharedPromptFiles() first call changed = false, want true")
	}
	expected := []string{
		"sdd-init.md",
		"sdd-explore.md",
		"sdd-propose.md",
		"sdd-spec.md",
		"sdd-design.md",
		"sdd-tasks.md",
		"sdd-apply.md",
		"sdd-verify.md",
		"sdd-archive.md",
		"sdd-onboard.md",
	}
	promptDir := SharedPromptDir(home)
	for _, fileName := range expected {
		path := filepath.Join(promptDir, fileName)
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Errorf("prompt file %q not found: %v", path, statErr)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("prompt file %q is empty", path)
		}
	}
}

// TestWriteSharedPromptFilesIdempotent verifies that calling twice returns changed=false on second call.
func TestWriteSharedPromptFilesIdempotent(t *testing.T) {
	home := t.TempDir()
	first, err := WriteSharedPromptFiles(home, nil)
	if err != nil {
		t.Fatalf("WriteSharedPromptFiles() first error = %v", err)
	}
	if !first {
		t.Fatal("WriteSharedPromptFiles() first call changed = false, want true")
	}
	second, err := WriteSharedPromptFiles(home, nil)
	if err != nil {
		t.Fatalf("WriteSharedPromptFiles() second error = %v", err)
	}
	if second {
		t.Fatal("WriteSharedPromptFiles() second call changed = true, want false (idempotent)")
	}
}

// TestWriteSharedPromptFilesWithCapabilities verifies content differs based on capability.
func TestWriteSharedPromptFilesWithCapabilities(t *testing.T) {
	home := t.TempDir()
	capableMap := map[string]string{"sdd-apply": "capable"}
	if _, err := WriteSharedPromptFiles(home, capableMap); err != nil {
		t.Fatalf("WriteSharedPromptFiles(capable) error = %v", err)
	}
	capablePath := filepath.Join(SharedPromptDir(home), "sdd-apply.md")
	capableContent, err := os.ReadFile(capablePath)
	if err != nil {
		t.Fatalf("ReadFile(capable) error = %v", err)
	}
	smallMap := map[string]string{"sdd-apply": "small"}
	if _, err := WriteSharedPromptFiles(home, smallMap); err != nil {
		t.Fatalf("WriteSharedPromptFiles(small) error = %v", err)
	}
	smallPath := filepath.Join(SharedPromptDir(home), "sdd-apply.md")
	smallContent, err := os.ReadFile(smallPath)
	if err != nil {
		t.Fatalf("ReadFile(small) error = %v", err)
	}
	if string(capableContent) == string(smallContent) {
		t.Fatal("sdd-apply.md content should differ between 'capable' and 'small' sections")
	}
	if !strings.Contains(string(smallContent), "max 3 files") {
		t.Error("small section should contain 'max 3 files'")
	}
	if strings.Contains(string(capableContent), "max 3 files") {
		t.Error("capable section should NOT contain 'max 3 files'")
	}
}

// TestWriteSharedPromptFilesLanguageContract verifies language contract is injected into every prompt.
func TestWriteSharedPromptFilesLanguageContract(t *testing.T) {
	phases := []string{
		"sdd-init.md",
		"sdd-explore.md",
		"sdd-propose.md",
		"sdd-spec.md",
		"sdd-design.md",
		"sdd-tasks.md",
		"sdd-apply.md",
		"sdd-verify.md",
		"sdd-archive.md",
		"sdd-onboard.md",
	}
	for _, capability := range []string{"capable", "small"} {
		t.Run(capability, func(t *testing.T) {
			home := t.TempDir()
			phaseCapabilities := make(map[string]string, len(phases))
			for _, fileName := range phases {
				phase := strings.TrimSuffix(fileName, ".md")
				phaseCapabilities[phase] = capability
			}
			if _, err := WriteSharedPromptFiles(home, phaseCapabilities); err != nil {
				t.Fatalf("WriteSharedPromptFiles(%s) error = %v", capability, err)
			}
			for _, fileName := range phases {
				path := filepath.Join(SharedPromptDir(home), fileName)
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("ReadFile(%q) error = %v", path, err)
				}
				text := string(content)
				for _, required := range []string{
					"Generated technical artifacts default to English",
				} {
					if !strings.Contains(text, required) {
						t.Fatalf("%s/%s missing language contract %q", capability, fileName, required)
					}
				}
			}
		})
	}
}

// TestSDDVerifySmallJSON verifies that sdd-verify small section uses the required JSON format.
func TestSDDVerifySmallJSON(t *testing.T) {
	data, err := assets.FS.ReadFile("skills/sdd-verify/SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(sdd-verify) error = %v", err)
	}
	small := extractModelSection(string(data), "small")
	if !strings.Contains(small, `"status"`) || !strings.Contains(small, `"pass|fail"`) {
		t.Error("sdd-verify small section missing JSON status pass|fail")
	}
	if !strings.Contains(small, `"checks"`) || !strings.Contains(small, `"criterion"`) {
		t.Error("sdd-verify small section missing checks/criterion JSON structure")
	}
}
