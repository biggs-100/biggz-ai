// Content-based risk classification — the Phase D1 replacement for the
// lens-count risk proxy.
//
// The tier of a review candidate is derived from the authored change itself
// (paths, changed lines, per-path line summary), mirroring the spirit of
// gentle-ai's SnapshotBuilder classification
// (internal/reviewtransaction/risk.go) adapted to biggz's path-level
// evidence: named risk signals escalate to the 4R review, provably passive
// documentation stays silent, everything else is one consolidated review.
//
// Volume deliberately plays a bounded role (unlike gentle-ai, where volume is
// not a tier input at all): an unusually large change is high-risk on its own
// because it is beyond what one consolidated review can reliably cover.
//
// Tier decision order (first match wins):
//
//  1. Sensitive domain path            → high
//  2. Execution-controlling config     → high
//  3. Documentation-only change        → low
//  4. changed lines > HighRiskChangedLines → high
//  5. Trivial inert change             → low
//  6. Everything else                  → medium
//
// Documentation-only changes outrank the volume rule (step 4): volume never
// escalates provably passive content, mirroring gentle-ai, where a
// five-thousand-line documentation change is still a documentation change.
package review

import (
	"fmt"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
)

const (
	// HighRiskChangedLines mirrors gentle-ai's LargeChangeLines review
	// boundary. In gentle it is a composition boundary, not a tier input; in
	// biggz a change of this size is high-risk on volume alone because a
	// single consolidated review cannot reliably cover it. A change of exactly
	// this many lines is not high on volume (strictly greater).
	HighRiskChangedLines = 400

	// LowRiskTrivialLines is the small-change threshold below which a change
	// touching only inert, non-executable material is a silent low-risk
	// candidate. Larger inert changes are still low when they are pure
	// documentation (see isDocumentationPath).
	LowRiskTrivialLines = 30
)

// RiskTier is the content-based risk classification of a review candidate.
// It is an alias of the consent gate's ReviewRisk so the classifier and the
// consent envelope share one tier type and one set of constants.
type RiskTier = ReviewRisk

// RiskInput is the authored-change evidence behind a tier. Paths are the
// repository-relative changed paths, ChangedLines the total authored text
// line count, and DiffSummary the per-path authored line count (zero for
// binary and empty entries). BaseTree is the resolved base tree the evidence
// was derived against.
type RiskInput struct {
	Paths        []string
	ChangedLines int
	DiffSummary  map[string]int
	BaseTree     string
}

// Sensitive-domain matching list (curated, conservative). A path matches when
// any of its tokens (path split on / \ . - _) equals one of these tokens
// case-insensitively, when it wears a key-material extension, or when its base
// name is a dot-env file. Conservative on purpose: false positives only ever
// escalate to a more careful review, never to silence.
//
//	auth, authentication, authorize, authorization
//	login, logout, session(s)
//	credential(s), secret(s), token(s), key(s), password(s)
//	payment(s), billing
//	crypto, cipher, encryption, encrypt, decrypt
//	security, secure
//	permission(s), privilege(s), rbac
//	sanitize, sanitizer, sanitization
//	sql, injection
var sensitiveRiskTokens = map[string]struct{}{
	"auth": {}, "authentication": {}, "authorize": {}, "authorization": {},
	"login": {}, "logout": {},
	"session": {}, "sessions": {},
	"credential": {}, "credentials": {},
	"secret": {}, "secrets": {},
	"token": {}, "tokens": {},
	"key": {}, "keys": {},
	"password": {}, "passwords": {},
	"payment": {}, "payments": {}, "billing": {},
	"crypto": {}, "cipher": {}, "encryption": {}, "encrypt": {}, "decrypt": {},
	"security": {}, "secure": {},
	"permission": {}, "permissions": {}, "privilege": {}, "privileges": {}, "rbac": {},
	"sanitize": {}, "sanitizer": {}, "sanitization": {},
	"sql": {}, "injection": {},
}

// sensitiveKeyExtensions are the key-material file extensions that escalate a
// path regardless of its tokens.
var sensitiveKeyExtensions = map[string]struct{}{
	".pem": {}, ".p12": {}, ".pfx": {}, ".jks": {}, ".key": {},
}

// Execution-controlling configuration (CI, deployment, permissions): changes
// here control how the project runs or what it is allowed to do, so they are
// high-risk even when small.
func executionConfigPath(paths []string) string {
	for _, p := range paths {
		lower := strings.ToLower(p)
		base := path.Base(lower)
		for _, name := range []string{
			"dockerfile", "jenkinsfile", ".gitlab-ci.yml", "azure-pipelines.yml",
			"bitbucket-pipelines.yml", ".travis.yml", "serverless.yml", "sudoers",
		} {
			if base == name {
				return p
			}
		}
		if strings.HasPrefix(base, "docker-compose") {
			return p
		}
		if strings.HasPrefix(lower, ".github/workflows/") || strings.Contains(lower, "/.github/workflows/") {
			return p
		}
		if strings.HasPrefix(lower, ".circleci/") || strings.Contains(lower, "/.circleci/") {
			return p
		}
		for _, segment := range strings.Split(lower, "/") {
			switch segment {
			case "kubernetes", "k8s", "terraform":
				return p
			}
		}
	}
	return ""
}

// semanticSourceExtensions are the file extensions that carry executable
// logic. Mirror of gentle-ai's semanticSourceExtensions set.
var semanticSourceExtensions = map[string]struct{}{
	".c": {}, ".cc": {}, ".cpp": {}, ".cs": {}, ".go": {}, ".h": {}, ".hpp": {},
	".java": {}, ".js": {}, ".jsx": {}, ".kt": {}, ".kts": {}, ".php": {}, ".py": {},
	".rb": {}, ".rs": {}, ".sh": {}, ".bash": {}, ".zsh": {}, ".ts": {}, ".tsx": {},
}

// configurationExtensions and configurationBasenames name non-executing
// configuration material: it is never low-risk on its own (mirroring
// gentle-ai's TouchesConfiguration), but it is not high-risk either unless it
// controls execution (see executionConfigPath).
var configurationExtensions = map[string]struct{}{
	".json": {}, ".yaml": {}, ".yml": {}, ".toml": {}, ".ini": {}, ".env": {},
	".conf": {}, ".config": {}, ".xml": {},
}

var configurationBasenames = map[string]struct{}{
	"go.mod": {}, "go.sum": {}, "package.json": {}, "package-lock.json": {},
	"pnpm-lock.yaml": {}, "yarn.lock": {},
}

// isDocumentationPath reports whether a path is documentation, readme,
// commentary, or config-documentation material: inert content that cannot
// execute. A path under a docs segment, a doc extension, the README/LICENSE/
// CHANGELOG basename family, or a non-executing project-doc config all count.
func isDocumentationPath(p string) bool {
	lower := strings.ToLower(p)
	base := path.Base(lower)
	for _, segment := range strings.Split(lower, "/") {
		switch segment {
		case "doc", "docs", "documentation":
			return true
		}
	}
	switch path.Ext(lower) {
	case ".md", ".markdown", ".mdown", ".rst", ".adoc", ".txt":
		return true
	}
	for _, prefix := range []string{
		"readme", "license", "licence", "copying", "notice", "changelog",
		"changes", "contributing", "authors", "code_of_conduct", "governance",
	} {
		if strings.HasPrefix(base, prefix) {
			return true
		}
	}
	switch base {
	case ".editorconfig", ".gitignore", ".gitattributes", ".mailmap", ".git-blame-ignore-revs":
		return true
	}
	return false
}

// documentationOnly reports whether every changed path is documentation
// material. An empty path set is vacuously documentation-only: a change with
// no authored paths is the most silent candidate there is.
func documentationOnly(paths []string) bool {
	for _, p := range paths {
		if !isDocumentationPath(p) {
			return false
		}
	}
	return true
}

// triviallyInert reports whether every changed path is provably passive:
// documentation, or an inert non-source, non-config file with authored lines.
// A binary or empty entry (no authored lines, when the summary is provided)
// is never proven passive and fails closed, mirroring gentle-ai's rule that
// only frozen bytes can withdraw a passive nomination.
func triviallyInert(paths []string, diffSummary map[string]int) bool {
	for _, p := range paths {
		if isDocumentationPath(p) {
			continue
		}
		lower := strings.ToLower(p)
		if _, source := semanticSourceExtensions[path.Ext(lower)]; source {
			return false
		}
		if _, config := configurationExtensions[path.Ext(lower)]; config {
			return false
		}
		if _, config := configurationBasenames[lower]; config {
			return false
		}
		if executionConfigPath([]string{p}) != "" {
			return false
		}
		if diffSummary != nil {
			lines, present := diffSummary[p]
			if !present || lines <= 0 {
				return false
			}
		}
	}
	return true
}

// tokenizePath splits a logical path into lower-cased tokens on / \ . - _,
// the same tokenization gentle-ai's hot-path signal matcher uses.
func tokenizePath(logicalPath string) []string {
	return strings.FieldsFunc(strings.ToLower(logicalPath), func(r rune) bool {
		return r == '/' || r == '\\' || r == '.' || r == '-' || r == '_'
	})
}

// sensitiveDomainPath returns the first changed path that touches a sensitive
// domain (auth, security, payments, crypto, secrets, credentials, sessions,
// permissions, sanitization, SQL injection surfaces), or "" when none does.
func sensitiveDomainPath(paths []string) string {
	for _, p := range paths {
		lower := strings.ToLower(p)
		base := path.Base(lower)
		if base == ".env" || strings.HasPrefix(base, ".env.") {
			return p
		}
		if _, hit := sensitiveKeyExtensions[path.Ext(lower)]; hit {
			return p
		}
		for _, token := range tokenizePath(p) {
			if _, hit := sensitiveRiskTokens[token]; hit {
				return p
			}
		}
	}
	return ""
}

// ClassifyRisk selects the review tier from the authored change evidence.
// The decision order is: sensitive domain path, execution-controlling config,
// documentation-only change, oversized change, trivial inert change, and
// finally everything else is medium. Documentation-only changes stay low at
// any size: volume never escalates provably passive content, mirroring
// gentle-ai (a five-thousand-line documentation change is still documentation).
func ClassifyRisk(paths []string, changedLines int, diffSummary map[string]int) RiskTier {
	if sensitiveDomainPath(paths) != "" {
		return RiskHigh
	}
	if executionConfigPath(paths) != "" {
		return RiskHigh
	}
	if documentationOnly(paths) {
		return RiskLow
	}
	if changedLines > HighRiskChangedLines {
		return RiskHigh
	}
	if changedLines < LowRiskTrivialLines && triviallyInert(paths, diffSummary) {
		return RiskLow
	}
	return RiskMedium
}

// PlanLenses resolves the frozen lens selection for a start. Declared lenses
// win when non-empty; otherwise the tier decides: low → no lenses, medium →
// the single focus lens, high → the canonical 4R (risk, readability,
// reliability, resilience).
func PlanLenses(tier RiskTier, declared []string) []string {
	if len(declared) > 0 {
		return append([]string(nil), declared...)
	}
	switch tier {
	case RiskMedium:
		return []string{"risk"}
	case RiskHigh:
		return []string{"risk", "readability", "reliability", "resilience"}
	default:
		return nil
	}
}

// DeriveRiskInput derives the classifier evidence from the subject commit
// against a base tree — the same base derivation as the correction budget
// (DeriveOriginalChangedLines): the explicit baseRef tree when given,
// otherwise the subject commit's parent tree, falling back to git's empty
// tree for a root commit. Legacy subjects without a commit SHA bind to the
// current HEAD.
func DeriveRiskInput(repo, commitSHA, baseRef string) (RiskInput, error) {
	repoArgs := func(args ...string) []string {
		if repo != "" {
			return append([]string{"-C", repo}, args...)
		}
		return args
	}
	target := commitSHA
	if target == "" {
		target = "HEAD"
	}
	candidate, err := gitOutput(exec.Command("git", repoArgs("rev-parse", target+"^{tree}")...))
	if err != nil {
		return RiskInput{}, fmt.Errorf("derive risk input: resolve candidate tree for %s: %w", commitSHA, err)
	}
	base := ""
	if baseRef != "" {
		base, err = gitOutput(exec.Command("git", repoArgs("rev-parse", baseRef+"^{tree}")...))
		if err != nil {
			return RiskInput{}, fmt.Errorf("derive risk input: resolve base tree for %s: %w", baseRef, err)
		}
	} else {
		base, err = gitOutput(exec.Command("git", repoArgs("rev-parse", target+"^^{tree}")...))
		if err != nil {
			base = emptyTreeSHA
		}
	}
	raw, err := gitOutput(exec.Command("git", repoArgs("diff", "--numstat", "--no-renames",
		"--no-ext-diff", "--no-textconv", "--ignore-submodules=none", base, candidate, "--")...))
	if err != nil {
		return RiskInput{}, fmt.Errorf("derive risk input: diff %s vs %s: %w", base, candidate, err)
	}
	summary, err := parseNumstatPerPath(raw)
	if err != nil {
		return RiskInput{}, fmt.Errorf("derive risk input: %w", err)
	}
	paths := make([]string, 0, len(summary))
	total := 0
	for p, lines := range summary {
		paths = append(paths, p)
		total += lines
	}
	sort.Strings(paths)
	return RiskInput{Paths: paths, ChangedLines: total, DiffSummary: summary, BaseTree: base}, nil
}

// parseNumstatPerPath sums additions and deletions per path from `git diff
// --numstat` output. Binary entries (dashes) and empty entries count zero
// lines but still record the path, so the trivial-inert check can fail
// closed on content that is never proven passive.
func parseNumstatPerPath(raw string) (map[string]int, error) {
	summary := make(map[string]int)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			return nil, fmt.Errorf("malformed numstat line %q", line)
		}
		p := strings.Join(fields[2:], "\t")
		if p == "" {
			return nil, fmt.Errorf("malformed numstat line %q", line)
		}
		if fields[0] == "-" || fields[1] == "-" {
			summary[p] = 0
			continue
		}
		additions, addErr := strconv.Atoi(fields[0])
		deletions, delErr := strconv.Atoi(fields[1])
		if addErr != nil || delErr != nil {
			return nil, fmt.Errorf("malformed numstat line %q", line)
		}
		summary[p] = additions + deletions
	}
	return summary, nil
}

// riskEvidence names the classifier evidence behind a tier for the consent
// envelope, so the relayed question can tell a human why a tier was chosen
// without re-deriving review authority.
func riskEvidence(input RiskInput, plannedLenses []string) []string {
	evidence := make([]string, 0, 4)
	if p := sensitiveDomainPath(input.Paths); p != "" {
		evidence = append(evidence, "sensitive path: "+p)
	}
	if input.ChangedLines > HighRiskChangedLines {
		evidence = append(evidence, fmt.Sprintf(
			"changed lines %d exceed the %d-line high-risk boundary", input.ChangedLines, HighRiskChangedLines))
	}
	if p := executionConfigPath(input.Paths); p != "" {
		evidence = append(evidence, "execution-controlling config: "+p)
	}
	if len(plannedLenses) > 0 {
		evidence = append(evidence, "lens plan: "+strings.Join(plannedLenses, ", "))
	}
	return evidence
}
