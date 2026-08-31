package sdd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/pathidentity"
)

// Edit-authority detection (#2540 S1): work units carry no structured target
// field, so the only honest signal that a task plan edits another repository
// is the prose itself. Detection is deliberately conservative: it inspects
// only backticked tokens inside markdown checkbox lines, and it flags a
// token only when the token resolves to a real Git repository different from
// the planning repository and outside every authorized edit root. It catches
// the reported scenario (explicit `../sibling/...` and absolute paths); it
// cannot catch pure prose ("update the billing service"), and a context
// reference can raise a false block — acceptable because the consequence is
// an honest blocked status naming its exits, never silent authority.

var backtickedSpan = regexp.MustCompile("`([^`]+)`")

// taskCheckbox matches markdown task-list lines whose tokens are eligible
// for edit-authority detection.
var taskCheckbox = regexp.MustCompile(`^\s*(?:[-*]|\d+[.)])\s+\[([ xX])\]`)

// readOnlyMarkerAfterToken matches a per-token "(read-only)" suffix
// immediately after a backticked token. Case-insensitive, allows leading
// whitespace. Used to exempt a token from both edit-authority and
// topology guards.
var readOnlyMarkerAfterToken = regexp.MustCompile(`(?i)^\s*\(read-only\)`)

var investigativePhrases = []string{"investigate", "explore", "check", "look into"}
var conditionalPhrases = []string{"if possible", "maybe", "consider", "when ready"}

// detectUnauthorizedEditRoots scans tasks text for path-like tokens in
// checkbox lines, resolves each against workspaceRoot to its nearest
// existing ancestor, and reports every resolved Git root that is neither the
// planning repository's own Git root nor inside allowedEditRoots.
// allowedEditRoots is a parameter so persisted per-change grants can extend
// it without touching detection.
func normalizeAllowedRoots(allowedEditRoots []string) []string {
	allowed := make([]string, 0, len(allowedEditRoots))
	for _, root := range allowedEditRoots {
		root = filepath.Clean(root)
		if resolved, err := filepath.EvalSymlinks(root); err == nil {
			root = resolved
		}
		allowed = append(allowed, root)
	}
	return allowed
}

func isUnauthorizedPath(resolved, planningGitRoot string, allowed []string) (string, bool) {
	target := gitRootOf(resolved)
	if target == "" {
		return "", false
	}
	missing := target
	if target == planningGitRoot {
		missing = sameRepositoryEditRoot(resolved)
	}
	if withinAnyRoot(missing, allowed) {
		return "", false
	}
	return missing, true
}

func collectUnauthorizedFromLine(line, workspaceRoot, planningGitRoot string, allowed []string, out map[string]bool) {
	if len(taskCheckbox.FindStringSubmatch(line)) == 0 {
		return
	}
	// Use index-aware extraction to filter per-token read-only suffix
	matches := backtickedSpan.FindAllStringSubmatchIndex(line, -1)
	for _, m := range matches {
		token := line[m[2]:m[3]]
		if strings.ContainsAny(token, " \t") || strings.Contains(token, "://") {
			continue
		}
		if !strings.ContainsRune(token, '/') && !strings.ContainsRune(token, filepath.Separator) {
			continue
		}
		suffix := line[m[1]:]
		if readOnlyMarkerAfterToken.MatchString(suffix) {
			continue
		}
		resolved := token
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(workspaceRoot, resolved)
		}
		resolved = resolveExistingPath(filepath.Clean(resolved))
		if missing, ok := isUnauthorizedPath(resolved, planningGitRoot, allowed); ok {
			out[missing] = true
		}
	}
}

func detectUnauthorizedEditRoots(tasksText string, workspaceRoot string, allowedEditRoots []string) []string {
	planningGitRoot := gitRootOf(resolveExistingPath(workspaceRoot))
	allowed := normalizeAllowedRoots(allowedEditRoots)
	unauthorized := map[string]bool{}
	for _, line := range strings.Split(tasksText, "\n") {
		collectUnauthorizedFromLine(line, workspaceRoot, planningGitRoot, allowed, unauthorized)
	}
	roots := make([]string, 0, len(unauthorized))
	for root := range unauthorized {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

func sameRepositoryEditRoot(path string) string {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return filepath.Dir(path)
	}
	return path
}

// HasExplicitEditIntent reports whether prompt carries explicit permission
// to edit. Only a phrase containing "apply ... to <path>" counts as explicit.
// Investigative phrases (investigate, explore, check, look into) and
// conditional phrases (if possible, maybe, consider, when ready) never
// grant permission and force read-only exploration.
func HasExplicitEditIntent(prompt string) bool {
	lower := strings.ToLower(prompt)
	for _, phrase := range investigativePhrases {
		if strings.Contains(lower, phrase) {
			return false
		}
	}
	for _, phrase := range conditionalPhrases {
		if strings.Contains(lower, phrase) {
			return false
		}
	}
	if !strings.Contains(lower, "apply") {
		return false
	}
	idx := strings.Index(lower, "apply")
	remainder := lower[idx:]
	if !strings.Contains(remainder, "to") {
		return false
	}
	afterTo := remainder[strings.Index(remainder, "to")+2:]
	if strings.Contains(afterTo, "/") || strings.Contains(afterTo, ".") {
		return true
	}
	return false
}

// pathLikeTokens extracts the conservative candidate set from one checkbox
// line: backticked tokens that contain a path separator (which subsumes
// `../` prefixes and absolute paths). Tokens with whitespace are commands or
// prose, and URL-like tokens are references, not filesystem targets.
// It also filters tokens whose suffix matches readOnlyMarkerAfterToken.
func pathLikeTokens(line string) []string {
	var tokens []string
	matches := backtickedSpan.FindAllStringSubmatchIndex(line, -1)
	for _, m := range matches {
		token := line[m[2]:m[3]]
		if strings.ContainsAny(token, " \t") || strings.Contains(token, "://") {
			continue
		}
		if !strings.ContainsRune(token, '/') && !strings.ContainsRune(token, filepath.Separator) {
			continue
		}
		suffix := line[m[1]:]
		if readOnlyMarkerAfterToken.MatchString(suffix) {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

// resolveExistingPath walks up to the nearest existing ancestor (task prose
// routinely names files that do not exist yet, like `../service-a/...`) and
// then evaluates symlinks so root comparisons happen on the paths the
// filesystem knows.
func resolveExistingPath(path string) string {
	current := path
	for {
		if _, err := os.Lstat(current); err == nil {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	if resolved, err := filepath.EvalSymlinks(current); err == nil {
		return resolved
	}
	return current
}

// gitRootOf walks up from path to the nearest directory containing a `.git`
// entry (a directory for ordinary repositories, a file for worktrees). An
// empty result means the path belongs to no repository and can never be an
// unauthorized edit target.
func gitRootOf(path string) string {
	current := path
	if info, err := os.Stat(current); err != nil || !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func withinAnyRoot(target string, roots []string) bool {
	for _, root := range roots {
		if target == root || strings.HasPrefix(target, root+string(filepath.Separator)) {
			return true
		}
		if pathidentity.Contains(root, target) {
			return true
		}
	}
	return false
}

// gitCommonDirForPath returns the git common dir for the given path via
// "git rev-parse --git-common-dir" memoized per Status via memo map.
// Uses exec.Command, not shell, and validates via filepath.EvalSymlinks.
func gitCommonDirForPath(dir string, memo map[string]string) (string, error) {
	if memo != nil {
		if cached, ok := memo[dir]; ok {
			return cached, nil
		}
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", err
	}
	common := strings.TrimSpace(string(out))
	if common == "" {
		return "", fmt.Errorf("empty git common dir")
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(dir, common)
	}
	common = filepath.Clean(common)
	if resolved, err := filepath.EvalSymlinks(common); err == nil {
		common = resolved
	}
	if memo != nil {
		memo[dir] = common
	}
	return common, nil
}

func sameFile(a, b string) (bool, error) {
	ai, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ai, bi), nil
}

// foreignRuntimeTopologyRoots scans checkbox lines for path-like backticked
// tokens (filtered by readOnlyMarkerAfterToken) and reports every resolved
// Git repository whose common dir differs from the planning repository's
// common dir and lies outside allowedEditRoots. Memoized per Status via
// memo map (3 tokens -> 1 rev-parse). Returns sorted unique foreign roots
// (gitRootOf of the resolved path) for blocked status.
func foreignRuntimeTopologyRoots(tasksText, workspaceRoot string, allowed []string, memo map[string]string) []string {
	if memo == nil {
		memo = make(map[string]string)
	}
	allowedNorm := normalizeAllowedRoots(allowed)
	// Planning common dir via resolveExistingPath + gitCommonDir
	planningPath := resolveExistingPath(workspaceRoot)
	planningDir := planningPath
	if info, err := os.Stat(planningPath); err == nil && !info.IsDir() {
		planningDir = filepath.Dir(planningPath)
	}
	planningCommon, err := gitCommonDirForPath(planningDir, memo)
	if err != nil {
		// If planning repo not git or error, fail closed: no topology block (conservative)
		return nil
	}
	foreign := map[string]bool{}
	for _, line := range strings.Split(tasksText, "\n") {
		if len(taskCheckbox.FindStringSubmatch(line)) == 0 {
			continue
		}
		matches := backtickedSpan.FindAllStringSubmatchIndex(line, -1)
		for _, m := range matches {
			token := line[m[2]:m[3]]
			if strings.ContainsAny(token, " \t") || strings.Contains(token, "://") {
				continue
			}
			if !strings.ContainsRune(token, '/') && !strings.ContainsRune(token, filepath.Separator) {
				continue
			}
			suffix := line[m[1]:]
			if readOnlyMarkerAfterToken.MatchString(suffix) {
				continue
			}
			resolved := token
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(workspaceRoot, resolved)
			}
			resolved = resolveExistingPath(filepath.Clean(resolved))
			targetDir := resolved
			if info, err := os.Stat(targetDir); err == nil && !info.IsDir() {
				targetDir = filepath.Dir(targetDir)
			}
			targetCommon, err := gitCommonDirForPath(targetDir, memo)
			if err != nil {
				continue // not a git repo
			}
			same, err := sameFile(planningCommon, targetCommon)
			if err == nil && same {
				continue // same common dir
			}
			// Different common dir: report foreign git root if outside allowed
			foreignRoot := gitRootOf(resolved)
			if foreignRoot == "" {
				foreignRoot = targetDir
			}
			if withinAnyRoot(foreignRoot, allowedNorm) {
				continue
			}
			foreign[foreignRoot] = true
		}
	}
	roots := make([]string, 0, len(foreign))
	for r := range foreign {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	return roots
}

// editAuthorityBlockedReason names each unauthorized root and both exits:
// keep the plan inside the authorized roots, or grant authority for the
// named repositories.
func editAuthorityBlockedReason(roots []string) string {
	quoted := make([]string, 0, len(roots))
	for _, root := range roots {
		quoted = append(quoted, quotePath(root))
	}
	return fmt.Sprintf(
		"blocked(edit_authority_missing): tasks.md targets repositories outside the authorized edit roots: %s; edit tasks.md so every work unit stays inside the authorized edit roots, or grant this change edit authority for those repositories",
		strings.Join(quoted, ", "),
	)
}

// applyEditAuthorityBlock is now authority-free for sdd-status V2.
// Gentle v2.5.0-rc.1 I6: sdd-status never blocks on edit authority;
// the outside-root check is retained for the pre-apply warning only.
// readChange still populates EditAuthorityBlocked/MissingRoots/Consent so
// sdd-apply can warn blocked(edit_authority_missing) with both exits
// (edit tasks.md or grant), but this derivation hook no longer mutates
// ApplyState or blockedReasons, keeping blockedReasons=[] and nextRecommended
// != resolve-blockers for V2.
func applyEditAuthorityBlock(applyState ApplyState, reasons *blockerReasons, tasksText string, workspaceRoot string, allowedEditRoots []string) ApplyState {
	// V2 authority-free: sdd-status never blocks. Detection stays in
	// readChange for sdd-apply guard; this hook is intentionally a no-op.
	return applyState
}
