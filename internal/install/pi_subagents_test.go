package install_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/biggs-100/biggz-ai/internal/install"
)

func TestDeployPiSubAgents(t *testing.T) {
	home := t.TempDir()

	// Minimal mock FS with 2 fake sdd skills and one non-sdd skill that should be ignored.
	mockFS := fstest.MapFS{
		"skills/sdd-apply/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: sdd-apply\ndescription: Apply phase test\n---\n\n## Body\nApply body here.\n"),
		},
		"skills/sdd-research/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: sdd-research\ndescription: Research phase test\n---\n\n## Research Body\nResearch content.\n"),
		},
		"skills/sdd-explore/SKILL.md": &fstest.MapFile{
			Data: []byte("<!-- section:model-capable -->\n---\nname: sdd-explore\ndescription: Explore phase test\n---\n\n## Explore\nExplore body.\n<!-- /section:model-capable -->\n<!-- section:model-small -->\n---\nname: sdd-explore\ndescription: small\n---\nsmall body\n<!-- /section:model-small -->\n"),
		},
		"skills/branch-pr/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: branch-pr\ndescription: Not SDD\n---\nbody\n"),
		},
	}

	n, err := install.DeployPiSubAgents(home, mockFS)
	if err != nil {
		t.Fatalf("DeployPiSubAgents: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 pi agents deployed (sdd-apply, sdd-research, sdd-explore), got %d", n)
	}

	// Verify sdd-apply.md exists with frontmatter
	applyPath := filepath.Join(home, ".pi", "agent", "agents", "sdd-apply.md")
	data, err := os.ReadFile(applyPath)
	if err != nil {
		t.Fatalf("sdd-apply.md not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: sdd-apply") {
		t.Errorf("sdd-apply.md frontmatter missing name: got %q", content[:200])
	}
	if !strings.Contains(content, "description:") {
		t.Errorf("sdd-apply.md missing description: %q", content[:200])
	}
	if !strings.Contains(content, "tools:") {
		t.Errorf("sdd-apply.md missing tools: %q", content[:500])
	}
	if !strings.Contains(content, "Apply body here.") {
		t.Errorf("sdd-apply.md missing body: %q", content)
	}
	// sdd-apply should have edit/bash tools (not read-only)
	if !strings.Contains(content, "- edit") || !strings.Contains(content, "- bash") {
		t.Errorf("sdd-apply.md should have edit/bash tools, got %q", content[:500])
	}

	// sdd-research should be read-only (grep/find/ls)
	researchPath := filepath.Join(home, ".pi", "agent", "agents", "sdd-research.md")
	rdata, err := os.ReadFile(researchPath)
	if err != nil {
		t.Fatalf("sdd-research.md not created: %v", err)
	}
	rcontent := string(rdata)
	if !strings.Contains(rcontent, "name: sdd-research") {
		t.Errorf("sdd-research.md frontmatter missing name")
	}
	if !strings.Contains(rcontent, "- grep") || !strings.Contains(rcontent, "- find") {
		t.Errorf("sdd-research.md should have read-only tools grep/find, got %q", rcontent[:500])
	}

	// sdd-explore dual-section handling: should contain capable body, not small body
	explorePath := filepath.Join(home, ".pi", "agent", "agents", "sdd-explore.md")
	edata, err := os.ReadFile(explorePath)
	if err != nil {
		t.Fatalf("sdd-explore.md not created: %v", err)
	}
	econtent := string(edata)
	if !strings.Contains(econtent, "Explore body.") {
		t.Errorf("sdd-explore.md should contain capable body, got %q", econtent)
	}
	if strings.Contains(econtent, "small body") {
		t.Errorf("sdd-explore.md should not contain small model body")
	}

	// branch-pr (non-sdd) should NOT be deployed
	branchPath := filepath.Join(home, ".pi", "agent", "agents", "branch-pr.md")
	if _, err := os.Stat(branchPath); err == nil {
		t.Errorf("branch-pr.md should not be deployed to pi agents dir")
	}
}

func TestDeployPiSubAgents_DryRun(t *testing.T) {
	home := t.TempDir()
	mockFS := fstest.MapFS{
		"skills/sdd-apply/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: sdd-apply\ndescription: Apply\n---\nBody\n"),
		},
		"skills/sdd-spec/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: sdd-spec\ndescription: Spec\n---\nSpec body\n"),
		},
	}

	n, err := install.DeployPiSubAgents(home, mockFS, true)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if n != 2 {
		t.Fatalf("dry-run expected 2, got %d", n)
	}
	// No files should exist
	agentsDir := filepath.Join(home, ".pi", "agent", "agents")
	if _, err := os.Stat(agentsDir); err == nil {
		entries, _ := os.ReadDir(agentsDir)
		if len(entries) > 0 {
			t.Errorf("dry-run should not write files, but found %d entries", len(entries))
		}
	}
}

func TestDeployPiSubAgents_PIEnvOverride(t *testing.T) {
	tmpHome := t.TempDir()
	override := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", override)

	mockFS := fstest.MapFS{
		"skills/sdd-apply/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: sdd-apply\ndescription: Apply\n---\nBody\n"),
		},
	}

	n, err := install.DeployPiSubAgents(tmpHome, mockFS)
	if err != nil {
		t.Fatalf("env override: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1, got %d", n)
	}
	// Should be under override/agents, not tmpHome/.pi
	overridePath := filepath.Join(override, "agents", "sdd-apply.md")
	if _, err := os.Stat(overridePath); err != nil {
		t.Fatalf("expected file at PI_CODING_AGENT_DIR override %q: %v", overridePath, err)
	}
	defaultPath := filepath.Join(tmpHome, ".pi", "agent", "agents", "sdd-apply.md")
	if _, err := os.Stat(defaultPath); err == nil {
		t.Errorf("should not write to default path when PI_CODING_AGENT_DIR is set")
	}
}
