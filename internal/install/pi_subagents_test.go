package install_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/biggs-100/biggz-ai/internal/assets"
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
	// sdd-apply should have edit/bash tools (not read-only) and no task/mcp (pi has no such tools; BigMem via allowlisted biggz_mem_* tools)
	if !strings.Contains(content, "- edit") || !strings.Contains(content, "- bash") {
		t.Errorf("sdd-apply.md should have edit/bash tools, got %q", content[:500])
	}
	if !strings.Contains(content, "- read") || !strings.Contains(content, "- write") {
		t.Errorf("sdd-apply.md should have read/write tools, got %q", content[:500])
	}
	if !strings.Contains(content, "- biggz_mem_save") || !strings.Contains(content, "- biggz_mem_search") {
		t.Errorf("sdd-apply.md should have BigMem tools (biggz_mem_save/search), got %q", content[:800])
	}
	if !strings.Contains(content, "- biggz_mem_get_observation") || !strings.Contains(content, "- biggz_mem_context") {
		t.Errorf("sdd-apply.md should have BigMem tools (get_observation/context), got %q", content[:800])
	}
	if strings.Contains(content, "- task") || strings.Contains(content, "- mcp") {
		t.Errorf("sdd-apply.md must NOT contain unavailable tools task/mcp, got %q", content[:800])
	}
	// Ensure allowlist includes base tools plus BigMem tools
	for _, want := range []string{"  - read\n", "  - edit\n", "  - bash\n", "  - write\n", "  - biggz_mem_save\n", "  - biggz_mem_search\n", "  - biggz_mem_get_observation\n", "  - biggz_mem_context\n", "  - mem_save\n"} {
		if !strings.Contains(content, want) {
			t.Errorf("sdd-apply.md missing expected tool %q in frontmatter, got %q", strings.TrimSpace(want), content[:800])
		}
	}

	// sdd-research should be read-only (grep/find/ls) plus BigMem and no task/mcp
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
	if !strings.Contains(rcontent, "- read") || !strings.Contains(rcontent, "- ls") {
		t.Errorf("sdd-research.md should have read/ls tools, got %q", rcontent[:800])
	}
	if !strings.Contains(rcontent, "- biggz_mem_save") || !strings.Contains(rcontent, "- biggz_mem_search") {
		t.Errorf("sdd-research.md should have BigMem tools (biggz_mem_save/search), got %q", rcontent[:800])
	}
	if strings.Contains(rcontent, "- task") || strings.Contains(rcontent, "- mcp") {
		t.Errorf("sdd-research.md must NOT contain unavailable tools task/mcp, got %q", rcontent[:800])
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
	if !strings.Contains(econtent, "- read") || !strings.Contains(econtent, "- grep") {
		t.Errorf("sdd-explore.md should have read/grep tools, got %q", econtent[:800])
	}
	if !strings.Contains(econtent, "- biggz_mem_save") || !strings.Contains(econtent, "- biggz_mem_search") {
		t.Errorf("sdd-explore.md should have BigMem tools (biggz_mem_save/search), got %q", econtent[:800])
	}
	if strings.Contains(econtent, "- task") || strings.Contains(econtent, "- mcp") {
		t.Errorf("sdd-explore.md must NOT contain unavailable tools task/mcp, got %q", econtent[:800])
	}
	// Ensure BigMem protocol block is still injected (5b1d257) after allowlist fix
	for _, p := range []string{content, rcontent, econtent} {
		if !strings.Contains(p, "biggz:bigmem-protocol") {
			t.Errorf("pi agent should contain <!-- biggz:bigmem-protocol --> block, got %q", p[:800])
		}
	}

	// web_* must be in sdd-research only (REQ-INST-002/005)
	if !strings.Contains(rcontent, "- web_search") || !strings.Contains(rcontent, "- web_fetch") {
		t.Errorf("sdd-research.md should have web_search/web_fetch, got %q", rcontent[:1000])
	}
	if strings.Contains(content, "web_search") || strings.Contains(content, "web_fetch") {
		t.Errorf("sdd-apply.md must NOT contain web_*, got %q", content[:800])
	}
	if strings.Contains(econtent, "web_search") || strings.Contains(econtent, "web_fetch") {
		t.Errorf("sdd-explore.md must NOT contain web_*, got %q", econtent[:800])
	}

	// branch-pr (non-sdd) should NOT be deployed
	branchPath := filepath.Join(home, ".pi", "agent", "agents", "branch-pr.md")
	if _, err := os.Stat(branchPath); err == nil {
		t.Errorf("branch-pr.md should not be deployed to pi agents dir")
	}
}

func TestOverlayWebToolsGating(t *testing.T) {
	data, err := fs.ReadFile(assets.FS, "opencode/sdd-overlay-multi.json")
	if err != nil {
		t.Fatalf("read overlay via FS: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"web_search": true`) || !strings.Contains(s, `"web_fetch": true`) {
		t.Errorf("overlay sdd-research should contain web_search/web_fetch")
	}
	count := strings.Count(s, "web_search")
	if count != 1 {
		t.Errorf("web_search appears %d times, want 1 (sdd-research only)", count)
	}
	skill, err := fs.ReadFile(assets.FS, "skills/sdd-research/SKILL.md")
	if err != nil {
		t.Fatalf("read SKILL.md via FS: %v", err)
	}
	sk := string(skill)
	for _, need := range []string{"open-web", "TAVILY_API_KEY", "BIGGZ_DDG_FALLBACK", "BIGGZ_WEB_FETCH_HEADLESS"} {
		if !strings.Contains(sk, need) {
			t.Errorf("SKILL.md missing gating doc %q", need)
		}
	}
}

func TestWebSearchJS_CapsAndGuards(t *testing.T) {
	data, err := fs.ReadFile(assets.FS, "pi/biggz-web-search.js")
	if err != nil {
		t.Fatalf("read web search js: %v", err)
	}
	s := string(data)
	for _, need := range []string{"BLOCKED_SCHEMES", "isPrivateIP", "FETCH_TIMEOUT_MS", "ONE_MB", "parseRetryAfter", "resolveProviderOrder", "publisherFor", "chrome124", "safari17", "FetchBlocked", "BIGGZ_DDG_FALLBACK", "BIGGZ_WEB_FETCH_HEADLESS"} {
		if !strings.Contains(s, need) {
			t.Errorf("biggz-web-search.js missing %q", need)
		}
	}
	if !strings.Contains(s, "10_000") && !strings.Contains(s, "10000") {
		t.Errorf("missing 10s timeout cap")
	}
	if !strings.Contains(s, "1MB") && !strings.Contains(s, "1024 * 1024") {
		t.Errorf("missing 1MB cap marker")
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
