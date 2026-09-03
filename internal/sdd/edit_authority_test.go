package sdd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/sddattempt"
)

// redirectTestHome points HOME (and USERPROFILE on Windows) at a temp dir so
// the machine-scoped fallback ledger and the legacy migration source stay
// inside test state and never touch real user state.
func redirectTestHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

// initEditAuthorityGitRepo turns dir into a Git repository. Only the
// planning repository needs to be resolvable by git rev-parse; sibling
// service repositories only need a `.git` for root detection.
func initEditAuthorityGitRepo(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v (%s)", dir, err, out)
	}
}

// realPath resolves a path the way grant normalization and detection do.
func realPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// seedChange writes a minimal change (proposal + tasks) into a planning
// workspace and returns the change's own directory.
func seedChange(t *testing.T, planning, name, tasks string) string {
	t.Helper()
	changeRoot := filepath.Join(planning, "openspec", "changes", name)
	if err := os.MkdirAll(changeRoot, 0755); err != nil {
		t.Fatalf("mkdir change root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeRoot, "proposal.md"), []byte("# Proposal\n"), 0644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeRoot, "tasks.md"), []byte(tasks), 0644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}
	return changeRoot
}

// TestDetectUnauthorizedEditRootsHonorsAllowedRoots pins the seam persisted
// per-change grants plug into: allowed roots extend the parameter, detection
// needs no rework. It also pins the nearest-existing-ancestor rule (task
// prose names files that do not exist yet) and that non-path prose stays
// invisible.
func TestDetectUnauthorizedEditRootsHonorsAllowedRoots(t *testing.T) {
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	serviceA := filepath.Join(workspace, "service-a")
	initEditAuthorityGitRepo(t, planning)
	initEditAuthorityGitRepo(t, serviceA)
	tasks := strings.Join([]string{
		"- [ ] 1.1 Update `../service-a/does/not/exist/yet.go` for the rollout",
		"- [ ] 1.2 Update the billing service and run `go test ./...`",
		"- [ ] 1.3 Read https://example.com/service-b/docs first",
		"",
	}, "\n")

	wantA := realPath(t, serviceA)
	flagged := detectUnauthorizedEditRoots(tasks, planning, []string{planning})
	if len(flagged) != 1 || flagged[0] != wantA {
		t.Fatalf("detectUnauthorizedEditRoots() = %v, want exactly [%s]", flagged, wantA)
	}

	granted := detectUnauthorizedEditRoots(tasks, planning, []string{planning, serviceA})
	if len(granted) != 0 {
		t.Fatalf("granting %s must clear the block without detector rework, got %v", serviceA, granted)
	}
}

// TestSingleRepoTasksKeepStatusByteIdentical pins the no-false-positive
// contract: for an ordinary single-repository change, detection matches
// nothing, no marker is minted, no ledger is created, and the serialized
// status carries no edit-authority footprint.
func TestSingleRepoTasksKeepStatusByteIdentical(t *testing.T) {
	redirectTestHome(t)
	planning := t.TempDir()
	initEditAuthorityGitRepo(t, planning)
	changeRoot := seedChange(t, planning, "single-repo-change", strings.Join([]string{
		"- [ ] 1.1 Update `internal/auth/login.go` with the new claim check",
		"- [x] 1.2 Extend `openspec/changes/single-repo-change/tasks.md` after review",
		"- [ ] 1.3 Run `go test ./...` and fix regressions",
		"",
	}, "\n"))

	active, _, err := Status(filepath.Join(planning, "openspec"))
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("active changes = %d, want 1", len(active))
	}
	cs := active[0]
	if cs.EditAuthorityBlocked || cs.GrantedRoots != nil || cs.MissingRoots != nil || cs.Consent != nil {
		t.Fatalf("single-repo change gained an edit-authority footprint: %+v", cs)
	}

	encoded, err := json.Marshal(cs)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"granted_roots", "edit_authority_blocked", "missing_roots", "consent", "blocked(edit_authority_missing)"} {
		if strings.Contains(string(encoded), needle) {
			t.Fatalf("single-repo status serializes an edit-authority footprint (%q): %s", needle, encoded)
		}
	}

	// Zero filesystem footprint: no marker minted, no ledger created.
	if _, err := os.Stat(filepath.Join(changeRoot, changeInstanceMarkerFile)); !os.IsNotExist(err) {
		t.Fatalf("single-repo status minted a change-instance marker: %v", err)
	}
	if _, err := os.Stat(filepath.Join(planning, ".git", "biggz", "sdd-runtime", "v1", "single-repo-change")); !os.IsNotExist(err) {
		t.Fatalf("single-repo status created a runtime ledger: %v", err)
	}
}

// TestBlockedStatusCarriesConsentEnvelope is the end-to-end fixture at the
// status layer: when a multi-repository change reports
// blocked(edit_authority_missing), the change status carries the typed
// consent envelope whose granted choice names the EXACT runnable grant
// invocation — including the change-instance token that the status layer
// itself derives, persists in the change's own directory (so it dies with
// archive and a recreated change mints a fresh one), and will later use to
// project the granted roots back.
func TestBlockedStatusCarriesConsentEnvelope(t *testing.T) {
	redirectTestHome(t)
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	serviceA := filepath.Join(workspace, "service-a")
	serviceB := filepath.Join(workspace, "service-b")
	initEditAuthorityGitRepo(t, planning)
	initEditAuthorityGitRepo(t, serviceA)
	initEditAuthorityGitRepo(t, serviceB)
	changeRoot := seedChange(t, planning, "multi-repo-rollout", strings.Join([]string{
		"- [ ] 1.1 Update `../service-a/internal/api/handler.go` to accept the new header",
		"- [ ] 1.2 Update `../service-b/internal/worker/consume.go` to forward the header",
		"",
	}, "\n"))

	openspecRoot := filepath.Join(planning, "openspec")
	active, _, err := Status(openspecRoot)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("active changes = %d, want 1", len(active))
	}
	cs := active[0]
	if !cs.EditAuthorityBlocked {
		t.Fatal("multi-repo change did not report the edit-authority block")
	}
	if cs.Consent == nil {
		t.Fatal("blocked status carries no consent envelope")
	}
	if err := cs.Consent.Validate(); err != nil {
		t.Fatalf("emitted consent envelope fails its own contract validation: %v", err)
	}

	wantA := realPath(t, serviceA)
	wantB := realPath(t, serviceB)
	if !reflect.DeepEqual(cs.MissingRoots, []string{wantA, wantB}) {
		t.Fatalf("consent.MissingRoots = %v, want [%s %s]", cs.MissingRoots, wantA, wantB)
	}

	// The derivation rule: the embedded change-instance token IS the marker
	// persisted in the change's own directory, so the grant the human
	// consents to binds exactly the identity a later status read projects.
	marker, err := os.ReadFile(filepath.Join(changeRoot, changeInstanceMarkerFile))
	if err != nil {
		t.Fatalf("blocked status persisted no change-instance marker: %v", err)
	}
	token := strings.TrimSpace(string(marker))
	granted := cs.Consent.Choices[0].Invocation
	if token == "" || !strings.Contains(granted, " --change-instance "+token) {
		t.Fatalf("granted invocation %q does not carry the persisted instance token %q", granted, token)
	}

	// Stability within the change lifecycle: a second status resolves the
	// SAME token and renders the same invocation, so a retried conversation
	// never mints a rival identity for one change instance and the
	// deterministic request-id keeps replays idempotent.
	active2, _, err := Status(openspecRoot)
	if err != nil {
		t.Fatalf("Status() #2 error: %v", err)
	}
	again := active2[0]
	marker2, err := os.ReadFile(filepath.Join(changeRoot, changeInstanceMarkerFile))
	if err != nil {
		t.Fatalf("read marker after second status: %v", err)
	}
	if token2 := strings.TrimSpace(string(marker2)); token2 != token {
		t.Fatalf("second status minted a rival instance token %q, want %q", token2, token)
	}
	if again.Consent == nil || again.Consent.Choices[0].Invocation != granted {
		t.Fatalf("second status rendered a different granted invocation: %q vs %q", again.Consent.Choices[0].Invocation, granted)
	}
}

// TestRecreatedChangeNameDoesNotInheritGrantedRoots proves the instance
// containment through the status wiring: a covering grant clears the block
// for THIS change instance, but recreating the change under the same name (a
// fresh change directory, hence a fresh minted marker) projects none of the
// old grants — the recreated change is blocked again with a new instance
// token and planning-only edit authority.
func TestRecreatedChangeNameDoesNotInheritGrantedRoots(t *testing.T) {
	redirectTestHome(t)
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	serviceA := filepath.Join(workspace, "service-a")
	initEditAuthorityGitRepo(t, planning)
	initEditAuthorityGitRepo(t, serviceA)
	tasks := strings.Join([]string{
		"- [ ] 1.1 Update `../service-a/internal/api/handler.go` to accept the new header",
		"",
	}, "\n")
	changeRoot := seedChange(t, planning, "multi-repo-rollout", tasks)
	openspecRoot := filepath.Join(planning, "openspec")
	wantA := realPath(t, serviceA)

	// Blocked status mints the marker; grant against exactly that identity.
	active, _, err := Status(openspecRoot)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if !active[0].EditAuthorityBlocked {
		t.Fatal("first status must report the edit-authority block")
	}
	token, err := readChangeInstanceMarker(changeRoot)
	if err != nil || token == "" {
		t.Fatalf("blocked status persisted no instance marker: %q, %v", token, err)
	}

	// The covering grant: roots = the missing sibling root, instance = the
	// persisted token.
	if _, err := sddattempt.Grant(sddattempt.GrantParams{
		ChangeName: "multi-repo-rollout", RepoRoot: planning,
		Roots: []string{wantA}, Reason: "maintainer authorized the sibling repository",
		Actor: "maintainer", RequestID: "grant-recreation-fixture", ChangeInstance: token,
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	// The covering grant clears detection: the block is gone and the granted
	// roots project for this instance.
	granted, _, err := Status(openspecRoot)
	if err != nil {
		t.Fatalf("post-grant Status() error: %v", err)
	}
	if granted[0].EditAuthorityBlocked || granted[0].Consent != nil {
		t.Fatalf("post-grant status still blocked: %+v", granted[0])
	}
	if !reflect.DeepEqual(granted[0].GrantedRoots, []string{wantA}) {
		t.Fatalf("GrantedRoots = %v, want [%s]", granted[0].GrantedRoots, wantA)
	}

	// Archive moves the change directory away; a recreated change under the
	// same name starts from a fresh directory. The old grants must not
	// resurrect: fresh marker, planning-only authority, blocked again.
	if err := os.RemoveAll(changeRoot); err != nil {
		t.Fatal(err)
	}
	seedChange(t, planning, "multi-repo-rollout", tasks)
	recreated, _, err := Status(openspecRoot)
	if err != nil {
		t.Fatalf("recreated Status() error: %v", err)
	}
	if !recreated[0].EditAuthorityBlocked {
		t.Fatalf("recreated change inherited the archived grant: %+v", recreated[0])
	}
	if len(recreated[0].GrantedRoots) != 0 {
		t.Fatalf("recreated change projected old granted roots: %v", recreated[0].GrantedRoots)
	}
	fresh, err := readChangeInstanceMarker(changeRoot)
	if err != nil || fresh == "" || fresh == token {
		t.Fatalf("recreated change did not mint a fresh instance token: %q vs %q (%v)", fresh, token, err)
	}
}

// TestV2OutsideRootsNeverRecommendsApply is the P0-1 end-to-end pin:
// tasks.md targets a repository outside the authorized edit roots while
// planning is complete, so the internal document reports NextRecommended
// apply with EditAuthorityBlocked set (the topology guard stays quiet
// because the planning workspace itself is not a Git repository, exactly
// the divergence the V2 filter used to hide). The V2 projection must not
// recommend apply, while display stays authority-free.
func TestV2OutsideRootsNeverRecommendsApply(t *testing.T) {
	redirectTestHome(t)
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	serviceA := filepath.Join(workspace, "service-a")
	if err := os.MkdirAll(planning, 0755); err != nil {
		t.Fatalf("mkdir planning: %v", err)
	}
	initEditAuthorityGitRepo(t, serviceA)
	changeRoot := filepath.Join(planning, "openspec", "changes", "multi-repo-rollout")
	for rel, content := range map[string]string{
		"proposal.md":        "# Proposal\n",
		"specs/core/spec.md": "### Requirement: Capability\n#### Scenario: Works\n",
		"design.md":          "# Design\n",
		"tasks.md":           "- [x] 1.1 Agree the header contract\n- [ ] 1.2 Update `../service-a/internal/api/handler.go` to accept the new header\n",
	} {
		path := filepath.Join(changeRoot, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	active, _, err := Status(filepath.Join(planning, "openspec"))
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("active changes = %d, want 1", len(active))
	}
	cs := active[0]
	if !cs.EditAuthorityBlocked {
		t.Fatal("multi-repo change did not report the edit-authority block")
	}
	if cs.NextRecommended != "apply" {
		t.Fatalf("internal NextRecommended = %q, want apply (the hole under test)", cs.NextRecommended)
	}

	projected, err := ProjectStatusV2(cs)
	if err != nil {
		t.Fatalf("ProjectStatusV2 error = %v", err)
	}
	if projected.NextRecommended == "apply" {
		t.Fatalf("V2 nextRecommended = apply for tasks.md outside the authorized roots, want != apply")
	}
	for _, r := range projected.BlockedReasons {
		if strings.Contains(r, "edit_authority_missing") {
			t.Fatalf("V2 blockedReasons leaked authority: %q", r)
		}
	}
	encoded, err := json.Marshal(projected.BlockedReasons)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "[]" {
		t.Fatalf("V2 blockedReasons payload = %s, want [] (authority-free display)", encoded)
	}
}
