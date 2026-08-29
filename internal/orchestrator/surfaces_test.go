package orchestrator

import (
	"testing"
)

func TestIsTaskScopedRepositoryRelativePath_Rejects(t *testing.T) {
	rejects := []string{"../x", "/etc/passwd", "~/x", "*.go", "a[0].go", "a b/c", "", ".", "./", "a/../b", "C:\\Windows\\x", "a?b", "a{b}", "a[b"}
	for _, p := range rejects {
		if IsTaskScopedRepositoryRelativePath(p) {
			t.Errorf("expected reject for %q", p)
		}
	}
}

func TestIsTaskScopedRepositoryRelativePath_Accepts(t *testing.T) {
	accepts := []string{"internal/orchestrator/surfaces.go", "./internal/review/hash.go", "docs/skill-style-guide.md", "a/b/c.go"}
	for _, p := range accepts {
		if !IsTaskScopedRepositoryRelativePath(p) {
			t.Errorf("expected accept for %q", p)
		}
	}
	// backslash normalized
	if !IsTaskScopedRepositoryRelativePath("internal\\orchestrator\\surfaces.go") {
		t.Error("backslash should be normalized and accepted")
	}
	// glob only on first segment should reject, deeper glob allowed? first segment is before / so a/b*.go first segment a => no glob => accept
	if !IsTaskScopedRepositoryRelativePath("internal/foo*.go") {
		// first segment internal has no glob, so should accept even though second segment has *
		// this matches gentle's first-segment-only rule
		t.Error("second segment glob should be allowed per first-segment rule")
	}
}

func TestHasTaskScopedAllowedEditSurfaces(t *testing.T) {
	good := "## Allowed edit surfaces\n- `internal/orchestrator/surfaces.go`\n- `docs/skill-style-guide.md`\n"
	if !HasTaskScopedAllowedEditSurfaces(good) {
		t.Fatal("expected good surfaces to pass")
	}
	bad := "## Allowed edit surfaces\n- `../x`\n"
	if HasTaskScopedAllowedEditSurfaces(bad) {
		t.Fatal("expected bad surfaces to fail")
	}
	missing := "no heading here\n- `internal/a.go`\n"
	if HasTaskScopedAllowedEditSurfaces(missing) {
		t.Fatal("missing heading should fail")
	}
}

func TestRejectUnscopedBoundedWriterDispatch(t *testing.T) {
	blocked := RejectUnscopedBoundedWriterDispatch(map[string]any{"agent": "worker", "task": "no surfaces", "context": ""})
	if blocked == nil || blocked.Reason != WRITER_EDIT_SURFACE_REJECTION {
		t.Fatalf("expected block with WRITER rejection, got %v", blocked)
	}
	allowed := RejectUnscopedBoundedWriterDispatch(map[string]any{"agent": "worker", "task": "## Allowed edit surfaces\n- `internal/orchestrator/surfaces.go`\n", "context": ""})
	if allowed != nil {
		t.Fatalf("expected allow when surfaces present, got %v", allowed)
	}
	// non-writer agent should not block even without surfaces
	if r := RejectUnscopedBoundedWriterDispatch(map[string]any{"agent": "researcher", "task": "no surfaces"}); r != nil {
		t.Fatalf("non-writer should not block, got %v", r)
	}
}

func TestShouldEnforceScopedSurfacesViaOrchestrator(t *testing.T) {
	// direct orchestrator path: ensure rejection logic triggers correctly
	blocked := RejectUnscopedBoundedWriterDispatch(map[string]any{"agent": "worker", "task": "x"})
	if blocked == nil {
		t.Fatal("worker without surfaces should block")
	}
	allowed := RejectUnscopedBoundedWriterDispatch(map[string]any{"agent": "worker", "task": "## Allowed edit surfaces\n- `internal/orchestrator/surfaces.go`\n"})
	if allowed != nil {
		t.Fatalf("worker with surfaces should allow, got %v", allowed)
	}
}
