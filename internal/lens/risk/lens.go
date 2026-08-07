package risk

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/lens/gitdiff"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

// RiskLens is a LensPlugin that analyzes git diffs to assess the risk
// level of code changes. It inspects file paths, extensions, and diff
// metadata for signals such as auth changes, shell scripts, security
// modifications, and executable mode changes.
type RiskLens struct{}

// ID returns "risk" — the unique identifier for this lens.
func (l *RiskLens) ID() string { return "risk" }

// Name returns "Risk Assessment" — the human-readable name.
func (l *RiskLens) Name() string { return "Risk Assessment" }

// Version returns "1.0.0" — the current version.
func (l *RiskLens) Version() string { return "1.0.0" }

// Policies returns the list of policies associated with this lens.
// Currently this lens defines no enforceable policies.
func (l *RiskLens) Policies() []plugin.Policy { return []plugin.Policy{} }

// Analyze runs the risk lens analysis against the given subject.
// It executes git diff commands in the subject's repository, parses
// the output, classifies files by risk signal, assigns an overall
// risk level, and returns a LensResult with per-file findings.
func (l *RiskLens) Analyze(ctx context.Context, subject model.ReviewSubject) (*plugin.LensResult, error) {
	files, err := gitdiff.GetDiffStat(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("risk lens: %w", err)
	}

	execMode, err := gitdiff.DetectGitModeChanges(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("risk lens: %w", err)
	}

	signals := classifyFiles(files, execMode)
	level := assignRiskLevel(files, signals)
	findings := buildFindings(files, signals, level)

	return &plugin.LensResult{
		LensID:   l.ID(),
		Findings: findings,
	}, nil
}

// ---- File Classification ----

// classifyFile inspects a single DiffFile's path and extension for
// known risk signal patterns. It returns all matching RiskSignals.
//
// Auth signal: path contains "auth", "token", "credential", "secret",
//
//	"password", or ".env"
//
// Shell signal: extension is .sh, .bash, .zsh, or path starts with
//
//	".github/workflows/"
//
// Security signal: path contains "security", "crypto", "encrypt",
//
//	"cert", or "key"
func classifyFile(file DiffFile) []RiskSignal {
	var signals []RiskSignal
	lowerPath := strings.ToLower(file.Path)

	// Auth signal — sensitive credential paths
	authKeywords := []string{"auth", "token", "credential", "secret", "password", ".env"}
	for _, kw := range authKeywords {
		if strings.Contains(lowerPath, kw) {
			signals = append(signals, SignalAuth)
			break
		}
	}

	// Shell signal — script or workflow files
	shellExts := []string{".sh", ".bash", ".zsh"}
	for _, ext := range shellExts {
		if strings.HasSuffix(lowerPath, ext) {
			signals = append(signals, SignalShell)
			break
		}
	}
	if strings.HasPrefix(lowerPath, ".github/workflows/") {
		signals = append(signals, SignalShell)
	}

	// Security signal — cryptographic or security-sensitive paths
	secKeywords := []string{"security", "crypto", "encrypt", "cert", "key"}
	for _, kw := range secKeywords {
		if strings.Contains(lowerPath, kw) {
			signals = append(signals, SignalSecurity)
			break
		}
	}

	return signals
}

// classifyFiles aggregates risk signals across all changed files and
// optionally includes the executable mode signal. The returned slice
// is sorted for deterministic output.
func classifyFiles(files []DiffFile, hasExecMode bool) []RiskSignal {
	signalSet := make(map[RiskSignal]bool)

	if hasExecMode {
		signalSet[SignalExecMode] = true
	}

	for _, file := range files {
		for _, sig := range classifyFile(file) {
			signalSet[sig] = true
		}
	}

	signals := make([]RiskSignal, 0, len(signalSet))
	for sig := range signalSet {
		signals = append(signals, sig)
	}
	sort.Slice(signals, func(i, j int) bool {
		return signals[i] < signals[j]
	})
	return signals
}

// ---- Risk Level Assignment ----

// assignRiskLevel determines the overall risk level based on detected
// signals and total changed lines.
//
// Rules:
//   - HIGH: any RiskSignal detected
//   - MEDIUM: > 100 changed lines and no signals
//   - LOW: ≤ 100 changed lines and no signals
func assignRiskLevel(files []DiffFile, signals []RiskSignal) RiskLevel {
	if len(signals) > 0 {
		return RiskHigh
	}

	totalLines := 0
	for _, f := range files {
		totalLines += f.Additions + f.Deletions
	}

	if totalLines > 100 {
		return RiskMedium
	}

	return RiskLow
}

// ---- Findings Builder ----

// signalSeverity maps each RiskSignal to its finding severity level.
var signalSeverity = map[RiskSignal]string{
	SignalAuth:     "high",
	SignalShell:    "medium",
	SignalSecurity: "high",
	SignalExecMode: "medium",
}

// levelSeverity maps each RiskLevel to its finding severity level.
var levelSeverity = map[RiskLevel]string{
	RiskLow:    "info",
	RiskMedium: "warning",
	RiskHigh:   "critical",
}

// buildFindings constructs a slice of plugin.Finding from the analysis
// results. It produces an overview finding with the aggregate risk level,
// plus per-file findings for each file that triggered a risk signal.
func buildFindings(files []DiffFile, signals []RiskSignal, level RiskLevel) []plugin.Finding {
	var findings []plugin.Finding

	// Total lines across all files
	totalLines := 0
	for _, f := range files {
		totalLines += f.Additions + f.Deletions
	}

	// Overview finding with aggregate risk assessment
	signalStrs := make([]string, len(signals))
	for i, s := range signals {
		signalStrs[i] = string(s)
	}

	findings = append(findings, plugin.Finding{
		ID:       "risk-overview",
		Severity: levelSeverity[level],
		Message:  fmt.Sprintf("Risk assessment: %s — %d files, %d lines changed, signals: [%s]", level, len(files), totalLines, strings.Join(signalStrs, ", ")),
	})

	// Per-file findings for each signal detected in that file
	findingIdx := 0
	for _, file := range files {
		fileSignals := classifyFile(file)
		for _, sig := range fileSignals {
			findingIdx++
			findings = append(findings, plugin.Finding{
				ID:       fmt.Sprintf("risk-%s-%d", sig, findingIdx),
				Severity: signalSeverity[sig],
				Message:  fmt.Sprintf("Risk signal: %s — file %s", sig, file.Path),
				File:     file.Path,
			})
		}
	}

	// Executable mode finding (not tied to a specific file in the stat output)
	if hasSignal(signals, SignalExecMode) {
		findings = append(findings, plugin.Finding{
			ID:       "risk-exec-mode",
			Severity: signalSeverity[SignalExecMode],
			Message:  "Executable mode change detected in diff — files added or modified with execute permissions",
		})
	}

	return findings
}

// hasSignal reports whether a RiskSignal is present in the given slice.
func hasSignal(signals []RiskSignal, target RiskSignal) bool {
	for _, s := range signals {
		if s == target {
			return true
		}
	}
	return false
}
