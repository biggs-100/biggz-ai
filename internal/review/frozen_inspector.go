package review

// Frozen-tree inspector: the single read-only boundary that derives reviewer
// evidence from immutable Git trees. It never reads the index or the working
// tree.
//
// Isolation model (adapted from gentle-ai's frozen candidate inspector):
//
//   - The repository is resolved exactly once (`rev-parse --show-toplevel`)
//     and every later invocation runs `git -C <resolved root>`, so the
//     caller's cwd can never select a different repository.
//   - The base and candidate are TREE object names resolved once against the
//     real repository (refs are only resolvable there); every later read
//     uses those frozen names, so staged or unstaged edits cannot
//     participate.
//   - Every tree read runs inside a throwaway GIT_DIR whose
//     GIT_OBJECT_DIRECTORY is the real repository's object store, with
//     system/global config and every attribute source neutralized
//     (GIT_CONFIG_NOSYSTEM, empty GIT_CONFIG_SYSTEM/GLOBAL, GIT_ATTR_NOSYSTEM,
//     an empty core.attributesFile, and no worktree or index of its own), so
//     neither the repository's nor the machine's .gitattributes can move the
//     classification; the isolated view's empty refs also hide replace refs.
//   - Per-path patches use the frozen patch flags and never `--text`: a
//     binary blob stays classified binary and never spills its bytes, while
//     a documentation-like path (even an executable one) still materializes
//     its patch.
//   - Reads are byte-capped; exceeding a cap is a typed refusal
//     (FrozenByteCapRefusal), never a silent truncation.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const (
	// MaxFrozenPathPatchBytes bounds one per-path patch read.
	MaxFrozenPathPatchBytes = 4 << 20
	// MaxFrozenTaskPatchBytes bounds the whole derived hunk set across every
	// changed path (the shared artifact bound).
	MaxFrozenTaskPatchBytes = ArtifactResultLimit
	// frozenGitReadLimit bounds manifest and rev-parse reads, whose size
	// follows the candidate's path count rather than its content.
	frozenGitReadLimit = 4 << 20
	// frozenGitStderrLimit bounds the captured stderr of one invocation.
	frozenGitStderrLimit = 64 << 10
)

// FrozenInspectionRefusal is the typed refusal for frozen-inspection input
// the inspector cannot resolve: the repository selection, the object store,
// or the candidate/base trees. It never guesses and never falls back to the
// index, the working tree, or a different repository.
type FrozenInspectionRefusal struct {
	Stage  string
	Detail string
	err    error
}

// Error implements error.
func (r *FrozenInspectionRefusal) Error() string {
	if r.err != nil {
		return fmt.Sprintf("frozen inspector: %s: %s: %v", r.Stage, r.Detail, r.err)
	}
	return fmt.Sprintf("frozen inspector: %s: %s", r.Stage, r.Detail)
}

// Unwrap exposes the underlying cause for errors.Is/errors.As.
func (r *FrozenInspectionRefusal) Unwrap() error { return r.err }

// FrozenByteCapRefusal is the typed refusal emitted when a frozen-tree read
// exceeds its byte cap. A refused read emits no bytes at all: this is never
// a silent truncation.
type FrozenByteCapRefusal struct {
	Scope  string // "path", "task", or "read"
	Path   string // set for a per-path refusal
	Cap    int
	Actual int
}

// Error implements error.
func (r *FrozenByteCapRefusal) Error() string {
	switch r.Scope {
	case "path":
		return fmt.Sprintf("frozen inspector: patch for %q is %d bytes, exceeding the %d-byte per-path cap (refused whole; never truncated)",
			r.Path, r.Actual, r.Cap)
	case "read":
		return fmt.Sprintf("frozen inspector: git output is %d bytes, exceeding the %d-byte read cap (refused whole; never truncated)",
			r.Actual, r.Cap)
	default:
		return fmt.Sprintf("frozen inspector: hunks total %d bytes, exceeding the %d-byte task cap (refused whole; never truncated)",
			r.Actual, r.Cap)
	}
}

// refuse builds a typed inspection refusal.
func refuse(stage, detail string, err error) error {
	return &FrozenInspectionRefusal{Stage: stage, Detail: detail, err: err}
}

// FrozenInspector is a read-only view over one candidate's frozen trees.
type FrozenInspector struct {
	repoRoot       string
	gitDir         string
	env            []string
	attributesFile string
	baseTree       string
	candidateTree  string
	paths          []string
	pathSet        map[string]struct{}
	closed         bool
}

// OpenFrozenInspector resolves the repository once, resolves the frozen
// base/candidate trees of the candidate commit against it, derives the
// changed-path manifest from the trees, and installs the isolated Git view.
// repo may be empty (the current directory is discovered); commitSHA may be
// empty (HEAD). baseRef is the explicit base boundary; the candidate
// commit's parent tree is the default, with Git's empty tree for a root
// commit. Callers own the returned inspector and must Close it.
func OpenFrozenInspector(repo, commitSHA, baseRef string) (*FrozenInspector, error) {
	repoRoot, err := resolveFrozenRepoRoot(repo)
	if err != nil {
		return nil, err
	}
	baseTree, candidateTree, err := resolveFrozenTrees(repoRoot, commitSHA, baseRef)
	if err != nil {
		return nil, err
	}
	gitDir, env, attributesFile, err := setupFrozenGitView(repoRoot)
	if err != nil {
		return nil, err
	}
	inspector := &FrozenInspector{
		repoRoot: repoRoot, gitDir: gitDir, env: env, attributesFile: attributesFile,
		baseTree: baseTree, candidateTree: candidateTree,
	}
	if err := inspector.loadManifest(); err != nil {
		return nil, errors.Join(err, inspector.Close())
	}
	return inspector, nil
}

// BaseTree returns the frozen base tree object name.
func (i *FrozenInspector) BaseTree() string { return i.baseTree }

// CandidateTree returns the frozen candidate tree object name.
func (i *FrozenInspector) CandidateTree() string { return i.candidateTree }

// Paths returns a copy of the changed-path manifest in canonical order.
func (i *FrozenInspector) Paths() []string { return slices.Clone(i.paths) }

// Close removes the throwaway Git view. It is idempotent.
func (i *FrozenInspector) Close() error {
	if i == nil || i.closed {
		return nil
	}
	i.closed = true
	return os.RemoveAll(i.gitDir)
}

// PatchForPath returns the frozen patch bytes for one changed path: the
// exact diff between the base and candidate tree objects, scoped by a
// literal pathspec, read through the isolated view. The patch never forces
// --text, so a binary path renders Git's "binary files differ" summary (it
// names the path and proves the change) without spilling blob bytes.
func (i *FrozenInspector) PatchForPath(path string) ([]byte, error) {
	if err := i.usable(); err != nil {
		return nil, err
	}
	if _, ok := i.pathSet[path]; !ok {
		return nil, refuse("patch", fmt.Sprintf("path %q is not in the frozen changed-path manifest", path), nil)
	}
	return i.readPatch(path, MaxFrozenPathPatchBytes)
}

// Hunks returns the frozen patch bytes for every changed path, capped in
// total: exceeding the task cap refuses whole instead of truncating.
func (i *FrozenInspector) Hunks() (map[string][]byte, error) {
	return i.hunksBounded(MaxFrozenTaskPatchBytes, MaxFrozenPathPatchBytes)
}

// hunksBounded is the testable cap seam behind Hunks.
func (i *FrozenInspector) hunksBounded(taskCap, pathCap int) (map[string][]byte, error) {
	if err := i.usable(); err != nil {
		return nil, err
	}
	hunks := make(map[string][]byte, len(i.paths))
	total := 0
	for _, path := range i.paths {
		payload, err := i.readPatch(path, pathCap)
		if err != nil {
			return nil, err
		}
		total += len(payload)
		if total > taskCap {
			return nil, &FrozenByteCapRefusal{Scope: "task", Cap: taskCap, Actual: total}
		}
		hunks[path] = payload
	}
	return hunks, nil
}

// usable refuses reads on a closed inspector.
func (i *FrozenInspector) usable() error {
	if i == nil || i.closed {
		return refuse("patch", "the frozen inspector is closed", nil)
	}
	return nil
}

// readPatch reads one per-path patch under an explicit byte cap.
func (i *FrozenInspector) readPatch(path string, cap int) ([]byte, error) {
	payload, total, err := i.gitLimited(cap, i.patchArgs(path)...)
	if err != nil {
		return nil, err
	}
	if total > cap {
		return nil, &FrozenByteCapRefusal{Scope: "path", Path: path, Cap: cap, Actual: total}
	}
	return payload, nil
}

// patchArgs is the mandated per-path patch invocation. Renames stay disabled
// (the reference default) so every manifest entry renders as its own
// add/delete/modify patch; --text is deliberately absent.
func (i *FrozenInspector) patchArgs(path string) []string {
	return []string{
		"-c", "color.ui=false",
		"-c", "core.attributesFile=" + i.attributesFile,
		"-c", "diff.external=",
		"diff", "--patch", "--full-index", "--no-color", "--no-ext-diff", "--no-textconv",
		"--no-renames", "--diff-algorithm=myers", "--no-indent-heuristic", "--unified=3",
		"--ignore-submodules=none", i.baseTree, i.candidateTree, "--", ":(literal)" + path,
	}
}

// loadManifest derives the changed-path manifest from the frozen trees only,
// through the isolated view, in canonical (sorted) order.
func (i *FrozenInspector) loadManifest() error {
	raw, err := i.gitSmall("diff", "--raw", "-z", "--full-index", "--no-renames",
		"--no-ext-diff", "--no-textconv", "--ignore-submodules=none",
		i.baseTree, i.candidateTree, "--")
	if err != nil {
		return refuse("manifest", "derive the changed-path manifest from the frozen trees", err)
	}
	entries, err := parseRawManifest(raw)
	if err != nil {
		return refuse("manifest", "parse the changed-path manifest", err)
	}
	i.paths = make([]string, 0, len(entries))
	i.pathSet = make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		i.paths = append(i.paths, entry.Path)
		i.pathSet[entry.Path] = struct{}{}
	}
	slices.Sort(i.paths)
	return nil
}

// gitSmall runs one small git read under the shared read cap.
func (i *FrozenInspector) gitSmall(args ...string) ([]byte, error) {
	payload, total, err := i.gitLimited(frozenGitReadLimit, args...)
	if err != nil {
		return nil, err
	}
	if total > frozenGitReadLimit {
		return nil, &FrozenByteCapRefusal{Scope: "read", Cap: frozenGitReadLimit, Actual: total}
	}
	return payload, nil
}

// gitLimited runs one git invocation inside the isolated view: rooted at the
// resolved repository (never the caller's cwd), with the sanitized isolated
// environment, capturing stdout up to the limit while counting the total so
// an over-limit read can refuse with its true size.
func (i *FrozenInspector) gitLimited(limit int, args ...string) ([]byte, int, error) {
	command := exec.Command("git", append([]string{"--no-pager", "-C", i.repoRoot}, args...)...)
	command.Env = i.env
	stdout := &frozenOutput{limit: limit}
	stderr := &frozenOutput{limit: frozenGitStderrLimit}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return nil, stdout.total, fmt.Errorf("frozen inspector: git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.buffer.String()))
	}
	return stdout.buffer.Bytes(), stdout.total, nil
}

// frozenOutput captures a stream up to a byte limit while counting the total.
type frozenOutput struct {
	buffer bytes.Buffer
	limit  int
	total  int
}

// Write implements io.Writer.
func (o *frozenOutput) Write(p []byte) (int, error) {
	o.total += len(p)
	if room := o.limit - o.buffer.Len(); room > 0 {
		if len(p) <= room {
			o.buffer.Write(p)
		} else {
			o.buffer.Write(p[:room])
		}
	}
	return len(p), nil
}

// ---------------------------------------------------------------------------
// Repository resolution and the isolated view
// ---------------------------------------------------------------------------

// resolveFrozenRepoRoot resolves the repository exactly once.
func resolveFrozenRepoRoot(repo string) (string, error) {
	root, err := frozenGitOutput(frozenGitEnvironment(), repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", refuse("repository", fmt.Sprintf("cannot resolve %q to a Git work tree", frozenRepoLabel(repo)), err)
	}
	if strings.TrimSpace(root) == "" {
		return "", refuse("repository", fmt.Sprintf("%q resolved to an empty work tree root", frozenRepoLabel(repo)), nil)
	}
	return filepath.Clean(strings.TrimSpace(root)), nil
}

// frozenRepoLabel names the repository selection in refusals.
func frozenRepoLabel(repo string) string {
	if repo == "" {
		return "."
	}
	return repo
}

// resolveFrozenTrees resolves the candidate and base TREE object names once,
// against the real repository. The default base is the candidate commit's
// parent tree; a root commit falls back to Git's empty tree.
func resolveFrozenTrees(repoRoot, commitSHA, baseRef string) (string, string, error) {
	env := frozenGitEnvironment()
	target := commitSHA
	if target == "" {
		target = "HEAD"
	}
	candidate, err := frozenRevParseTree(env, repoRoot, target)
	if err != nil {
		return "", "", refuse("candidate tree", fmt.Sprintf("resolve %s^{tree} in %q", target, repoRoot), err)
	}
	if baseRef != "" {
		base, err := frozenRevParseTree(env, repoRoot, baseRef)
		if err != nil {
			return "", "", refuse("base tree", fmt.Sprintf("resolve %s^{tree} in %q", baseRef, repoRoot), err)
		}
		return base, candidate, nil
	}
	base, err := frozenRevParseTree(env, repoRoot, target+"^")
	if err != nil {
		// A root commit has no parent: its base is Git's empty tree.
		return emptyTreeSHA, candidate, nil
	}
	return base, candidate, nil
}

// frozenRevParseTree resolves one revision to its tree object name.
func frozenRevParseTree(env []string, repoRoot, revision string) (string, error) {
	name, err := frozenGitOutput(env, repoRoot, "rev-parse", "--verify", "--quiet", revision+"^{tree}")
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", fmt.Errorf("revision %q resolved to an empty object name", revision)
	}
	return name, nil
}

// setupFrozenGitView creates the throwaway Git view every tree read runs
// inside and returns its directory, its environment, and the neutral
// attributes file every patch invocation points at.
func setupFrozenGitView(repoRoot string) (string, []string, string, error) {
	commonDir, err := frozenGitCommonDir(repoRoot)
	if err != nil {
		return "", nil, "", err
	}
	objectsDir := filepath.Join(commonDir, "objects")
	if info, statErr := os.Stat(objectsDir); statErr != nil || !info.IsDir() {
		return "", nil, "", refuse("object store", fmt.Sprintf("%q is not a directory", objectsDir), statErr)
	}
	gitDir, err := os.MkdirTemp("", "biggz-frozen-git-*")
	if err != nil {
		return "", nil, "", refuse("isolated view", "create the throwaway Git directory", err)
	}
	env, attributesFile, err := prepareFrozenGitView(gitDir, objectsDir, frozenObjectFormat(repoRoot))
	if err != nil {
		return "", nil, "", errors.Join(err, os.RemoveAll(gitDir))
	}
	return gitDir, env, attributesFile, nil
}

// frozenGitCommonDir resolves the repository's common Git directory once.
func frozenGitCommonDir(repoRoot string) (string, error) {
	common, err := frozenGitOutput(frozenGitEnvironment(), repoRoot, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", refuse("object store", "resolve the repository's common Git directory", err)
	}
	if common == "" || strings.HasPrefix(common, "--") {
		return "", refuse("object store", fmt.Sprintf("the common Git directory %q is unusable", common), nil)
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(repoRoot, common)
	}
	return filepath.Clean(common), nil
}

// prepareFrozenGitView writes the isolated view's skeleton and neutral files
// and returns the isolated environment plus the attributes file path.
func prepareFrozenGitView(gitDir, objectsDir, objectFormat string) ([]string, string, error) {
	if err := writeFrozenGitSkeleton(gitDir, objectFormat); err != nil {
		return nil, "", err
	}
	neutral, err := createFrozenNeutralFiles(gitDir)
	if err != nil {
		return nil, "", err
	}
	env := frozenGitEnvironment(
		"GIT_DIR="+gitDir,
		"GIT_OBJECT_DIRECTORY="+objectsDir,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_SYSTEM="+neutral[0],
		"GIT_CONFIG_GLOBAL="+neutral[1],
		"GIT_CONFIG_COUNT=0",
		"GIT_ATTR_NOSYSTEM=1",
	)
	return env, neutral[2], nil
}

// writeFrozenGitSkeleton writes the empty bare view: its own objects/refs
// directories, an unborn HEAD, and a config that records the repository's
// object format so object names keep their meaning.
func writeFrozenGitSkeleton(gitDir, objectFormat string) error {
	for _, dir := range []string{filepath.Join(gitDir, "objects"), filepath.Join(gitDir, "refs")} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			return fmt.Errorf("frozen inspector: create isolated view: %w", err)
		}
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/frozen-context\n"), 0o600); err != nil {
		return fmt.Errorf("frozen inspector: write isolated view HEAD: %w", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(frozenGitConfig(objectFormat)), 0o600); err != nil {
		return fmt.Errorf("frozen inspector: write isolated view config: %w", err)
	}
	return nil
}

// frozenGitConfig renders the isolated view's config: a bare repository (so
// no worktree or index participates in a read) with the repository's object
// format extension when it is not sha1.
func frozenGitConfig(objectFormat string) string {
	if objectFormat == "sha256" {
		return "[core]\n\trepositoryFormatVersion = 1\n\tbare = true\n[extensions]\n\tobjectFormat = sha256\n"
	}
	return "[core]\n\trepositoryFormatVersion = 0\n\tbare = true\n"
}

// createFrozenNeutralFiles creates the empty system, global, and attributes
// files that neutralize every remaining config/attribute source.
func createFrozenNeutralFiles(gitDir string) ([]string, error) {
	files := make([]string, 0, 3)
	for _, name := range []string{"system", "global", "attributes"} {
		path := filepath.Join(gitDir, "neutral-"+name)
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			return nil, fmt.Errorf("frozen inspector: create the neutral %s file: %w", name, err)
		}
		files = append(files, path)
	}
	return files, nil
}

// frozenObjectFormat reports the repository's object hash algorithm. git >=
// 2.38 answers directly; older git echoes the unknown flag back (recognized
// by its leading "--") and degrades to the extensions.objectformat config
// entry, defaulting to sha1 — the only format that existed before it.
func frozenObjectFormat(repoRoot string) string {
	format, err := frozenGitOutput(frozenGitEnvironment(), repoRoot, "rev-parse", "--show-object-format")
	if err == nil {
		switch format {
		case "sha1", "sha256":
			return format
		}
	}
	fallback, err := frozenGitOutput(frozenGitEnvironment(), repoRoot, "config", "--get", "extensions.objectformat")
	if err != nil {
		return "sha1"
	}
	if strings.EqualFold(strings.TrimSpace(fallback), "sha256") {
		return "sha256"
	}
	return "sha1"
}

// frozenGitEnvironment returns the process environment minus every inherited
// GIT_* variable (repository selection, index, worktree, object store,
// config, attributes, tracing) and locale overrides, plus the caller's own
// extras, so no export of the invoking process can leak another repository
// or an alternate index into a read.
func frozenGitEnvironment(extra ...string) []string {
	env := make([]string, 0, len(os.Environ())+len(extra)+2)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "GIT_") || upper == "LC_ALL" || upper == "LANG" {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "LANG=C", "LC_ALL=C")
	return append(env, extra...)
}

// frozenGitOutput runs one git command with -C repo (when given) and returns
// its trimmed stdout.
func frozenGitOutput(env []string, repo string, args ...string) (string, error) {
	command := frozenGitCommand(env, repo, args...)
	out, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// frozenGitCommand builds one git command rooted at repo (when given).
func frozenGitCommand(env []string, repo string, args ...string) *exec.Cmd {
	var command *exec.Cmd
	if repo != "" {
		command = exec.Command("git", append([]string{"-C", repo}, args...)...)
	} else {
		command = exec.Command("git", args...)
	}
	command.Env = env
	return command
}
