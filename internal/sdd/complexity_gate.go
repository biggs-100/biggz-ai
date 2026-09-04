package sdd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/review/lens/readability"
)

// ComplexityGateResult is the verdict of the diff-aware complexity gate.
// Only functions intersecting the measured diff can block: pre-existing
// violations elsewhere are structurally grandfathered.
type ComplexityGateResult struct {
	// Passed is false when the diff introduces threshold offenders in
	// critical packages.
	Passed bool
	// Blocking lists new critical-package offenders (fail the gate).
	Blocking []readability.Offender
	// Warnings lists non-blocking findings: test-file offenders and
	// engine warnings. Non-critical packages pass silent (the R2 lens
	// still reports them at review).
	Warnings []string
	// FilesScanned counts per-file diff sections measured.
	FilesScanned int
}

// GateDiffComplexity runs the complexity gate over a unified diff for
// repoRoot. Worktree file content is read from disk; only hunk-overlapping
// functions in critical packages block.
func GateDiffComplexity(repoRoot, diff string) ComplexityGateResult {
	res := ComplexityGateResult{Passed: true}
	hunks := readability.SplitFileDiffs(diff)
	res.FilesScanned = len(hunks)
	if len(hunks) == 0 {
		return res
	}
	offs, warnings := readability.OffendersForFileDiffs(repoRoot, hunks)
	res.Warnings = append(res.Warnings, warnings...)
	for _, o := range offs {
		switch {
		case isGateTestFile(o.File):
			res.Warnings = append(res.Warnings, fmt.Sprintf("test offender (informational): %s:%s cyclo=%d cog=%d", o.File, o.Function, o.Cyclomatic, o.Cognitive))
		case isGateCriticalPackage(o.File):
			res.Blocking = append(res.Blocking, o)
		default:
			res.Warnings = append(res.Warnings, fmt.Sprintf("non-critical offender (warning only): %s:%s cyclo=%d cog=%d", o.File, o.Function, o.Cyclomatic, o.Cognitive))
		}
	}
	if len(res.Blocking) > 0 {
		res.Passed = false
	}
	return res
}

// GateWorkingTreeComplexity runs the gate over uncommitted worktree changes:
// `git diff HEAD` (staged + unstaged) plus untracked .go files measured
// whole. It returns an error (not a failure) when git is unavailable or the
// directory is not a repo — callers should SKIP the gate in that case.
func GateWorkingTreeComplexity(repoRoot string) (ComplexityGateResult, error) {
	if _, err := gitOut(repoRoot, "rev-parse", "--git-dir"); err != nil {
		return ComplexityGateResult{}, fmt.Errorf("not a git repo: %s", repoRoot)
	}
	out, err := gitOut(repoRoot, "diff", "HEAD", "--")
	if err != nil {
		return ComplexityGateResult{}, fmt.Errorf("git diff HEAD: %w", err)
	}
	var sb strings.Builder
	sb.WriteString(string(out))
	if !strings.HasSuffix(sb.String(), "\n") && sb.Len() > 0 {
		sb.WriteString("\n")
	}
	sb.WriteString(untrackedDiff(repoRoot))
	return GateDiffComplexity(repoRoot, sb.String()), nil
}

// untrackedDiff synthesizes per-file diff sections (one hunk covering the
// whole file) for untracked .go files, so brand-new files are measured.
func untrackedDiff(repoRoot string) string {
	out, err := gitOut(repoRoot, "status", "--porcelain", "--untracked-files=all", "--", "*.go")
	if err != nil {
		return ""
	}
	var sb strings.Builder
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 4 || !strings.HasPrefix(line, "??") {
			continue
		}
		rel := strings.TrimSpace(line[2:])
		// Porcelain quotes paths with special chars: "a/b c.go".
		rel = strings.Trim(rel, `"`)
		if !strings.HasSuffix(strings.ToLower(rel), ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			continue
		}
		n := bytes.Count(data, []byte{'\n'}) + 1
		sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n@@ -0,0 +1,%d @@\n", rel, rel, n))
	}
	return sb.String()
}

func gitOut(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	return cmd.Output()
}

func isGateTestFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), "_test.go")
}

func isGateCriticalPackage(path string) bool {
	for _, pkg := range readability.CriticalPackages {
		if path == pkg || strings.HasPrefix(path, pkg+"/") {
			return true
		}
	}
	return false
}

// FormatGateBlockers renders blocking offenders for gate reasons and reports.
func FormatGateBlockers(blocking []readability.Offender) string {
	parts := make([]string, 0, len(blocking))
	for _, o := range blocking {
		parts = append(parts, fmt.Sprintf("%s:%s cyclo=%d cog=%d", o.File, o.Function, o.Cyclomatic, o.Cognitive))
	}
	return strings.Join(parts, "; ")
}
