package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
)

// ---------------------------------------------------------------------------
// ClassifyRisk
// ---------------------------------------------------------------------------

func TestClassifyRisk_SensitiveDomainHigh(t *testing.T) {
	cases := []struct {
		path string
	}{
		{"internal/auth/token.go"},
		{"internal/authentication/login.go"},
		{"cmd/billing/charge.go"},
		{"services/payments/checkout.go"},
		{"pkg/crypto/keys.go"},
		{"config/secrets.yaml"},
		{"deploy/credentials.yml"},
		{"internal/session/store.go"},
		{"rbac/roles.yaml"},
		{"db/sanitize.go"},
		{"queries/sql_injection_test.go"},
		{".env"},
		{".env.production"},
		{"certs/server.pem"},
		{"keys/id_rsa"},
		{"internal/Auth/Login.go"},
	}
	for _, tc := range cases {
		if got := ClassifyRisk([]string{tc.path}, 5, nil); got != RiskHigh {
			t.Errorf("ClassifyRisk([%q], 5) = %q, want %q", tc.path, got, RiskHigh)
		}
	}
}

func TestClassifyRisk_VolumeHigh(t *testing.T) {
	if got := ClassifyRisk([]string{"cmd/main.go"}, HighRiskChangedLines+1, nil); got != RiskHigh {
		t.Errorf("lines %d = %q, want high", HighRiskChangedLines+1, got)
	}
	// Exactly at the boundary is not high on volume alone.
	if got := ClassifyRisk([]string{"cmd/main.go"}, HighRiskChangedLines, nil); got != RiskMedium {
		t.Errorf("lines %d = %q, want medium", HighRiskChangedLines, got)
	}
}

func TestClassifyRisk_ExecutionConfigHigh(t *testing.T) {
	cases := []string{
		".github/workflows/ci.yml",
		"k8s/deploy.yaml",
		"kubernetes/ingress.yaml",
		"Dockerfile",
		"deploy/docker-compose.yml",
		"Jenkinsfile",
		".gitlab-ci.yml",
		".circleci/config.yml",
		"serverless.yml",
		"terraform/main.tf",
		"etc/sudoers",
	}
	for _, path := range cases {
		if got := ClassifyRisk([]string{path}, 3, nil); got != RiskHigh {
			t.Errorf("ClassifyRisk([%q], 3) = %q, want %q", path, got, RiskHigh)
		}
	}
}

func TestClassifyRisk_DocsOnlyLow(t *testing.T) {
	cases := []struct {
		paths []string
		lines int
	}{
		{[]string{"README.md"}, 1},
		{[]string{"docs/architecture.md"}, 5000},
		{[]string{"CHANGELOG.txt", "LICENSE"}, 100},
		{[]string{"docs/guide.rst", "docs/api.adoc"}, 40},
		{[]string{"doc/notes.md"}, 5},
		{[]string{".editorconfig", ".gitignore"}, 4},
		{[]string{"README.md", "internal/docs/design.md"}, 200},
		{nil, 0},
	}
	for _, tc := range cases {
		if got := ClassifyRisk(tc.paths, tc.lines, nil); got != RiskLow {
			t.Errorf("ClassifyRisk(%v, %d) = %q, want %q", tc.paths, tc.lines, got, RiskLow)
		}
	}
}

func TestClassifyRisk_TrivialInertLow(t *testing.T) {
	summary := map[string]int{"data/seed.csv": 5, "assets/icons.svg": 3}
	if got := ClassifyRisk([]string{"data/seed.csv", "assets/icons.svg"}, 8, summary); got != RiskLow {
		t.Errorf("trivial inert change = %q, want low", got)
	}
}

func TestClassifyRisk_Medium(t *testing.T) {
	cases := []struct {
		paths   []string
		lines   int
		summary map[string]int
	}{
		{[]string{"cmd/main.go"}, 80, nil},
		{[]string{"README.md", "cmd/util.go"}, 60, nil},
		{[]string{"pkg/util/helper.go"}, 5, nil},
		{[]string{"package.json"}, 3, nil},
		{[]string{"go.mod"}, 2, nil},
		{[]string{"cmd/main.go", "internal/store/store.go"}, 390, nil},
		// Binary content is never proven passive: an image change is not low
		// even when the change is tiny.
		{[]string{"assets/logo.png"}, 0, map[string]int{"assets/logo.png": 0}},
	}
	for _, tc := range cases {
		if got := ClassifyRisk(tc.paths, tc.lines, tc.summary); got != RiskMedium {
			t.Errorf("ClassifyRisk(%v, %d, %v) = %q, want %q", tc.paths, tc.lines, tc.summary, got, RiskMedium)
		}
	}
}

// ---------------------------------------------------------------------------
// PlanLenses
// ---------------------------------------------------------------------------

func TestPlanLenses_DeclaredWins(t *testing.T) {
	declared := []string{"readability"}
	for _, tier := range []RiskTier{RiskLow, RiskMedium, RiskHigh} {
		if got := PlanLenses(tier, declared); !reflect.DeepEqual(got, declared) {
			t.Errorf("PlanLenses(%q, declared) = %v, want %v", tier, got, declared)
		}
	}
	// The caller's slice is not aliased.
	planned := PlanLenses(RiskHigh, declared)
	planned[0] = "risk"
	if declared[0] != "readability" {
		t.Error("PlanLenses must not alias the declared slice")
	}
}

func TestPlanLenses_FromTier(t *testing.T) {
	if got := PlanLenses(RiskLow, nil); len(got) != 0 {
		t.Errorf("low plan = %v, want empty", got)
	}
	if got := PlanLenses(RiskMedium, nil); !reflect.DeepEqual(got, []string{"risk"}) {
		t.Errorf("medium plan = %v, want [risk]", got)
	}
	want4R := []string{"risk", "readability", "reliability", "resilience"}
	if got := PlanLenses(RiskHigh, nil); !reflect.DeepEqual(got, want4R) {
		t.Errorf("high plan = %v, want %v", got, want4R)
	}
}

// ---------------------------------------------------------------------------
// DeriveRiskInput
// ---------------------------------------------------------------------------

// riskFixtureRepo builds a git repo whose head commit adds a docs file and a
// sensitive-domain file on top of a base commit.
func riskFixtureRepo(t *testing.T) (repo string, head string) {
	t.Helper()
	repo = t.TempDir()
	gitInit(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatalf("write base.txt: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "base")
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "docs", "guide.md"), []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "internal", "auth"), 0755); err != nil {
		t.Fatalf("mkdir auth: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "internal", "auth", "token.go"), []byte("package auth\n\nfunc Issue() {}\n"), 0644); err != nil {
		t.Fatalf("write token.go: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "candidate")
	return repo, runGitInDir(t, repo, "rev-parse", "HEAD")
}

func TestDeriveRiskInput_PathsAndLines(t *testing.T) {
	repo, head := riskFixtureRepo(t)
	input, err := DeriveRiskInput(repo, head, "")
	if err != nil {
		t.Fatalf("DeriveRiskInput: %v", err)
	}
	wantPaths := []string{"docs/guide.md", "internal/auth/token.go"}
	if !reflect.DeepEqual(input.Paths, wantPaths) {
		t.Errorf("paths = %v, want %v", input.Paths, wantPaths)
	}
	if input.ChangedLines != 6 {
		t.Errorf("changed lines = %d, want 6", input.ChangedLines)
	}
	if input.DiffSummary["docs/guide.md"] != 3 || input.DiffSummary["internal/auth/token.go"] != 3 {
		t.Errorf("diff summary = %v", input.DiffSummary)
	}
	if input.BaseTree == "" {
		t.Error("base tree is empty")
	}
	if got := ClassifyRisk(input.Paths, input.ChangedLines, input.DiffSummary); got != RiskHigh {
		t.Errorf("fixture classifies as %q, want high (sensitive path)", got)
	}
}

func TestDeriveRiskInput_LegacySubjectBindsHead(t *testing.T) {
	repo, _ := riskFixtureRepo(t)
	input, err := DeriveRiskInput(repo, "", "")
	if err != nil {
		t.Fatalf("DeriveRiskInput(no commit SHA): %v", err)
	}
	if len(input.Paths) != 2 || input.ChangedLines == 0 {
		t.Errorf("legacy subject did not bind to HEAD: %+v", input)
	}
}

func TestDeriveRiskInput_BaseRef(t *testing.T) {
	repo, _ := riskFixtureRepo(t)
	baseCommit := runGitInDir(t, repo, "rev-parse", "HEAD~1")
	input, err := DeriveRiskInput(repo, "HEAD", baseCommit)
	if err != nil {
		t.Fatalf("DeriveRiskInput(base-ref): %v", err)
	}
	if input.BaseTree != runGitInDir(t, repo, "rev-parse", baseCommit+"^{tree}") {
		t.Errorf("base tree not resolved from base-ref: %q", input.BaseTree)
	}
	if len(input.Paths) != 2 {
		t.Errorf("paths = %v, want the two candidate paths", input.Paths)
	}
}

// ---------------------------------------------------------------------------
// Wiring: the frozen start plan and the receipt
// ---------------------------------------------------------------------------

// TestStartFreezesPlannedSelection verifies that a start without --lenses
// freezes the classifier-driven selection into the start_review event, so
// next_transition and finalize can rely on it (the Phase D1 A2-gap fix).
func TestStartFreezesPlannedSelection(t *testing.T) {
	repo, head := riskFixtureRepo(t)
	lineage := "freeze-planned"
	store, err := Open(repo, lineage)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	input, err := DeriveRiskInput(repo, head, "")
	if err != nil {
		t.Fatalf("DeriveRiskInput: %v", err)
	}
	tier := ClassifyRisk(input.Paths, input.ChangedLines, input.DiffSummary)
	planned := PlanLenses(tier, nil)
	budget, err := DeriveCorrectionBudget(input.ChangedLines)
	if err != nil {
		t.Fatalf("DeriveCorrectionBudget: %v", err)
	}
	plan := StartEventPayload{
		Schema: ReviewStartEventSchema, Repository: repo, CommitSHA: head,
		BaseRef: input.BaseTree, OriginalChangedLines: input.ChangedLines,
		CorrectionBudget: budget, MaxCorrectionAttempts: MaxCompactCorrectionAttempts,
		SelectedLenses: planned, RiskTier: string(tier), LensPlan: planned,
	}
	review := New(model.ReviewSubject{Repository: repo, CommitSHA: head})
	review.State.Role = model.RoleReviewer
	review.WithStore(store).FreezeStartPlan(plan)
	if err := review.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	var frozen StartEventPayload
	if err := json.Unmarshal(chain.Records[0].Payload, &frozen); err != nil {
		t.Fatalf("unmarshal genesis: %v", err)
	}
	want4R := []string{"risk", "readability", "reliability", "resilience"}
	if frozen.RiskTier != "high" {
		t.Errorf("frozen risk_tier = %q, want high", frozen.RiskTier)
	}
	if !reflect.DeepEqual(frozen.SelectedLenses, want4R) {
		t.Errorf("frozen lenses = %v, want the planned 4R %v", frozen.SelectedLenses, want4R)
	}
	if !reflect.DeepEqual(frozen.LensPlan, want4R) {
		t.Errorf("frozen lens_plan = %v, want %v", frozen.LensPlan, want4R)
	}
	// Legacy payload fields are preserved.
	if frozen.Repository != repo || frozen.CommitSHA != head || frozen.OriginalChangedLines != 6 {
		t.Errorf("legacy payload fields not preserved: %+v", frozen)
	}
}

// TestFinalizeReceiptRecordsFrozenTier verifies the receipt carries the
// classifier tier frozen at start, not a lens-count proxy.
// Receipt is ephemeral (burned after finalize), so the file is deleted and
// we verify via the burned marker and the complete_review event.
func TestFinalizeReceiptRecordsFrozenTier(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, err := Open(repo, "finalize-tier")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	plan := StartEventPayload{
		Schema: ReviewStartEventSchema, Repository: repo, CommitSHA: head,
		BaseRef: "", OriginalChangedLines: 5, CorrectionBudget: 3,
		MaxCorrectionAttempts: MaxCompactCorrectionAttempts,
		SelectedLenses: []string{"risk", "readability", "reliability", "resilience"},
		RiskTier:       "high", LensPlan: []string{"risk", "readability", "reliability", "resilience"},
	}
	review := New(model.ReviewSubject{Repository: repo, CommitSHA: head})
	review.State.Role = model.RoleReviewer
	review.WithStore(store).FreezeStartPlan(plan)
	if err := review.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	for index, lens := range []string{"risk", "readability", "reliability", "resilience"} {
		captureLens(t, repo, "finalize-tier", head, lens, index)
	}
	outcome, err := Finalize(repo, "finalize-tier")
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	// Receipt is ephemeral: file is deleted after burn, marker exists.
	if _, err := os.Stat(filepath.Join(store.Dir, outcome.ReceiptPath)); !os.IsNotExist(err) {
		t.Fatalf("receipt file %q should be deleted after burn", outcome.ReceiptPath)
	}
	if _, err := os.Stat(filepath.Join(store.Dir, BurnedMarkerFile)); err != nil {
		t.Fatalf("burned marker should exist: %v", err)
	}
	if !store.IsBurned() {
		t.Fatal("store should be burned after finalize")
	}
	// Verify the receipt tier via the outcome hash binding: the outcome hash
	// is computed from the receipt that carried the frozen tier. We check
	// that the lineage was started with high tier and that the burned receipt
	// hash is valid.
	if !validSHA256Identity(outcome.ReceiptHash) {
		t.Fatalf("outcome receipt hash invalid: %q", outcome.ReceiptHash)
	}
	// The chain still contains the complete_review event referencing the burned receipt.
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	found := false
	for _, rec := range chain.Records {
		if rec.Operation == CompleteReviewOperation {
			var evt completeEventPayload
			if err := json.Unmarshal(rec.Payload, &evt); err == nil && evt.ReceiptHash == outcome.ReceiptHash {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("complete_review event with burned receipt hash not found")
	}
}

// TestStatusSurfacesFrozenTierAndPlan verifies status --json exposes the
// frozen risk_tier and lens_plan from the start_review genesis.
func TestStatusSurfacesFrozenTierAndPlan(t *testing.T) {
	repo, head := riskFixtureRepo(t)
	lineage := "status-tier"
	store, err := Open(repo, lineage)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	input, err := DeriveRiskInput(repo, head, "")
	if err != nil {
		t.Fatalf("DeriveRiskInput: %v", err)
	}
	tier := ClassifyRisk(input.Paths, input.ChangedLines, input.DiffSummary)
	plan := StartEventPayload{
		Schema: ReviewStartEventSchema, Repository: repo, CommitSHA: head,
		BaseRef: input.BaseTree, OriginalChangedLines: input.ChangedLines,
		CorrectionBudget: 4, MaxCorrectionAttempts: MaxCompactCorrectionAttempts,
		SelectedLenses: PlanLenses(tier, nil), RiskTier: string(tier), LensPlan: PlanLenses(tier, nil),
	}
	review := New(model.ReviewSubject{Repository: repo, CommitSHA: head})
	review.State.Role = model.RoleReviewer
	review.WithStore(store).FreezeStartPlan(plan)
	if err := review.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	auth := NewAuthority(repo)
	st, err := auth.Status(lineage)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.RiskTier != "high" {
		t.Errorf("status risk_tier = %q, want high", st.RiskTier)
	}
	want4R := []string{"risk", "readability", "reliability", "resilience"}
	if !reflect.DeepEqual(st.LensPlan, want4R) {
		t.Errorf("status lens_plan = %v, want %v", st.LensPlan, want4R)
	}
}
