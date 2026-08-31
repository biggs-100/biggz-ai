package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadOnlyMarker verifies per-token exemption.
func TestReadOnlyMarker(t *testing.T) {
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	other := filepath.Join(workspace, "other")
	initEditAuthorityGitRepo(t, planning)
	initEditAuthorityGitRepo(t, other)
	// Create a file structure for other
	os.MkdirAll(filepath.Join(other, "docs"), 0755)
	os.WriteFile(filepath.Join(other, "docs", "api.md"), []byte("# api"), 0644)
	os.MkdirAll(filepath.Join(other, "src"), 0755)
	os.WriteFile(filepath.Join(other, "src", "main.go"), []byte("package main"), 0644)

	// Token with (read-only) should be exempt
	tasksReadOnly := strings.Join([]string{
		"- [ ] 1.1 Edit `../other/docs/api.md` (read-only) for docs",
		"",
	}, "\n")
	missing := detectUnauthorizedEditRoots(tasksReadOnly, planning, []string{planning})
	if len(missing) != 0 {
		t.Fatalf("read-only token should be exempt, got %v", missing)
	}
	// Without marker should be blocked
	tasksBlocked := strings.Join([]string{
		"- [ ] 1.1 Edit `../other/src/main.go` for code",
		"",
	}, "\n")
	missing2 := detectUnauthorizedEditRoots(tasksBlocked, planning, []string{planning})
	if len(missing2) == 0 {
		t.Fatal("non-read-only foreign path should be blocked")
	}
	// Mix: first read-only exempt, second blocked
	tasksMixed := strings.Join([]string{
		"- [ ] 1.1 Edit `../other/docs/api.md` (read-only) and `../other/src/main.go`",
		"",
	}, "\n")
	missing3 := detectUnauthorizedEditRoots(tasksMixed, planning, []string{planning})
	if len(missing3) != 1 {
		t.Fatalf("mixed should have 1 blocked, got %v", missing3)
	}
	// Case-insensitive
	tasksCase := strings.Join([]string{
		"- [ ] 1.1 Edit `../other/docs/api.md` (READ-ONLY)",
		"",
	}, "\n")
	missing4 := detectUnauthorizedEditRoots(tasksCase, planning, []string{planning})
	if len(missing4) != 0 {
		t.Fatalf("case-insensitive read-only should be exempt, got %v", missing4)
	}
	// Also test foreignRuntimeTopologyRoots respects read-only
	allowed := []string{planning}
	memo := make(map[string]string)
	foreignReadOnly := foreignRuntimeTopologyRoots(tasksReadOnly, planning, allowed, memo)
	if len(foreignReadOnly) != 0 {
		t.Fatalf("topology read-only should be exempt, got %v", foreignReadOnly)
	}
	foreignBlocked := foreignRuntimeTopologyRoots(tasksBlocked, planning, allowed, make(map[string]string))
	if len(foreignBlocked) == 0 {
		t.Fatal("topology should block non-read-only foreign")
	}
}

// TestTopologyBlocksApplyNotSpec verifies foreign blocks apply not spec.
func TestTopologyBlocksApplyNotSpec(t *testing.T) {
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	foreign := filepath.Join(workspace, "foreign-clone")
	initEditAuthorityGitRepo(t, planning)
	initEditAuthorityGitRepo(t, foreign)
	os.MkdirAll(filepath.Join(foreign, "src"), 0755)
	os.WriteFile(filepath.Join(foreign, "src", "file.go"), []byte("package main"), 0644)

	// Create change with foreign path in tasks, and all planning artifacts done -> apply ready, should be blocked
	changeRoot := seedChange(t, planning, "topology-change", strings.Join([]string{
		"- [ ] 1.1 Update `../foreign-clone/src/file.go` for rollout",
		"",
	}, "\n"))
	// Add proposal/design/specs to make coreReady true
	os.WriteFile(filepath.Join(changeRoot, "design.md"), []byte("# Design\n"), 0644)
	os.MkdirAll(filepath.Join(changeRoot, "specs", "sdd"), 0755)
	os.WriteFile(filepath.Join(changeRoot, "specs", "sdd", "spec.md"), []byte("# Spec\n"), 0644)
	// Status should be blocked with cross_common_dir_runtime_target
	active, _, err := Status(filepath.Join(planning, "openspec"))
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	var cs ChangeStatus
	for _, c := range active {
		if c.Name == "topology-change" {
			cs = c
			break
		}
	}
	if cs.ApplyState != ApplyBlocked {
		t.Fatalf("expected ApplyBlocked due to topology, got %v", cs.ApplyState)
	}
	found := false
	for _, r := range cs.BlockedReasons {
		if strings.Contains(r, "cross_common_dir_runtime_target") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("BlockedReasons should contain cross_common_dir_runtime_target, got %v", cs.BlockedReasons)
	}
	if cs.NextRecommended != "resolve-blockers" {
		t.Fatalf("NextRecommended %q want resolve-blockers", cs.NextRecommended)
	}
	// Now test spec phase not blocked: make design missing => coreReady false => not blocked for topology
	os.Remove(filepath.Join(changeRoot, "design.md"))
	active2, _, _ := Status(filepath.Join(planning, "openspec"))
	for _, c := range active2 {
		if c.Name == "topology-change" {
			// Should be next: design, not resolve-blockers
			if c.NextRecommended == "resolve-blockers" {
				t.Fatalf("spec phase should not be blocked by topology, got resolve-blockers with %v", c.BlockedReasons)
			}
			break
		}
	}
}

// TestTopologyThreat verifies injection, symlink, memo.
func TestTopologyThreat(t *testing.T) {
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	initEditAuthorityGitRepo(t, planning)
	// Memo test: 3 tokens from same repo should result in 1 rev-parse per repo (memoized)
	foreign := filepath.Join(workspace, "foreign-memo")
	initEditAuthorityGitRepo(t, foreign)
	tasks := strings.Join([]string{
		"- [ ] 1.1 `../foreign-memo/a.go`",
		"- [ ] 1.2 `../foreign-memo/b.go`",
		"- [ ] 1.3 `../foreign-memo/c.go`",
		"",
	}, "\n")
	memo := make(map[string]string)
	foreignRoots := foreignRuntimeTopologyRoots(tasks, planning, []string{planning}, memo)
	if len(foreignRoots) != 1 {
		t.Fatalf("expected 1 foreign root, got %v", foreignRoots)
	}
	if len(memo) != 2 {
		t.Fatalf("memo should have 2 entries (planning+foreign) 3 tokens -> 1 rev-parse per repo, got %d: %v", len(memo), memo)
	}
	// Symlink EvalSymlinks check (skip if privilege not held)
	realSibling := filepath.Join(workspace, "real-sibling")
	initEditAuthorityGitRepo(t, realSibling)
	link := filepath.Join(workspace, "link-sibling")
	if err := os.Symlink(realSibling, link); err == nil {
		resolved := resolveExistingPath(filepath.Join(link, "file.go"))
		if !strings.Contains(resolved, "real-sibling") {
			t.Fatalf("resolveExistingPath should eval symlink, got %q", resolved)
		}
	} else {
		t.Logf("symlink not available, skipping symlink check: %v", err)
	}
	// Injection: path with a/b should be handled via exec.Command not shell (no shell expansion)
	// Verify that a token like "../foreign-memo/a;b" does not cause shell injection
	injectionTasks := strings.Join([]string{
		"- [ ] 1.1 `../foreign-memo/a; rm -rf /`",
		"",
	}, "\n")
	memo2 := make(map[string]string)
	// Should not panic or execute shell; just return maybe foreign root or empty, but not crash
	_ = foreignRuntimeTopologyRoots(injectionTasks, planning, []string{planning}, memo2)
}

// TestResolveExistingPathEvalSymlinks verifies EvalSymlinks.
func TestResolveExistingPathEvalSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	os.MkdirAll(real, 0755)
	os.WriteFile(filepath.Join(real, "file.txt"), []byte("hi"), 0644)
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink: %v", err)
	}
	p := filepath.Join(link, "file.txt")
	resolved := resolveExistingPath(p)
	if resolved != filepath.Join(real, "file.txt") && resolved != real {
		// resolveExistingPath walks to existing ancestor and evals symlink, so for file path it should eval to real/file.txt
		// But our function evals symlink on the existing ancestor (link -> real), then returns resolved
		if !strings.Contains(resolved, "real") {
			t.Fatalf("resolveExistingPath symlink not evaluated: %q", resolved)
		}
	}
}
