package git

// exec.go carries the generic surface of the "Git Wrapper — Single Owner"
// requirement: a runner preserving git's raw output, canonical absolute
// resolvers, and the env-aware constructor the frozen inspector migrates to.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Run executes git with args in dir (empty means the process working directory)
// and returns stdout verbatim. On a non-zero exit the error is the
// *exec.ExitError itself, its Stderr field holding git's unmodified stderr:
// internal/sddattempt classifies failures by stderr wording, so neither the raw
// bytes nor the error type may be swallowed, merged, or reformatted.
func Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitErr.Stderr = stderr.Bytes()
		}
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}

// TopLevel returns the absolute, symlink-resolved worktree root of the
// repository selected by repo (empty selects the current directory). The root
// is absolute and passed via -C, never the caller's cwd (threat matrix TM-2).
func TopLevel(ctx context.Context, repo string) (string, error) {
	root, err := absoluteRepo(repo)
	if err != nil {
		return "", err
	}
	out, err := Run(ctx, "", "-C", root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	top := strings.TrimSpace(string(out))
	if top == "" {
		return "", fmt.Errorf("git rev-parse --show-toplevel: empty output for repo %s", root)
	}
	return resolveAgainst(root, top), nil
}

// RevParse returns the trimmed stdout of `git rev-parse` plus args in the
// repository selected by repo, with -C carrying the absolute root (TM-2).
func RevParse(ctx context.Context, repo string, args ...string) (string, error) {
	root, err := absoluteRepo(repo)
	if err != nil {
		return "", err
	}
	out, err := Run(ctx, "", append([]string{"-C", root, "rev-parse"}, args...)...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveGitDirs returns the absolute, symlink-resolved git and common
// directories of the repository selected by repo (empty selects the current
// directory). It absorbs the semantics formerly duplicated in
// internal/review/store.go (canonicalGitDirectory / resolveGitCommonDir /
// resolveGitDir) and internal/sdd/edit_authority.go (gitCommonDirForPath):
// relative output is joined onto the absolute root, symlinks are resolved,
// directories are validated, and the common dir falls back to the git dir.
func ResolveGitDirs(ctx context.Context, repo string) (gitDir, commonDir string, err error) {
	root, err := absoluteRepo(repo)
	if err != nil {
		return "", "", err
	}
	gitDir, err = resolveGitDirOutput(ctx, root, "--git-dir")
	if err != nil {
		return "", "", err
	}
	commonDir, err = resolveGitDirOutput(ctx, root, "--git-common-dir")
	if err != nil {
		commonDir = gitDir
	}
	return gitDir, commonDir, nil
}

// ExecOptions configures a git command built by NewCommand. Repo is the
// absolute repository root passed via -C; an empty Repo omits -C, so a caller
// relying on the process working directory must opt out explicitly (TM-2).
type ExecOptions struct {
	Repo     string
	NoPager  bool
	ExtraEnv []string
	Stdout   io.Writer
	Stderr   io.Writer
}

// NewCommand builds an env-aware git command: --no-pager when requested, -C
// from Repo, and an environment with every inherited GIT_* variable and locale
// override stripped, then LANG=C, LC_ALL=C and opts.ExtraEnv appended. Byte
// caps stay with the caller's writers.
func NewCommand(opts ExecOptions, args ...string) *exec.Cmd {
	argv := make([]string, 0, len(args)+3)
	if opts.NoPager {
		argv = append(argv, "--no-pager")
	}
	if opts.Repo != "" {
		argv = append(argv, "-C", opts.Repo)
	}
	cmd := exec.Command("git", append(argv, args...)...)
	cmd.Env = sanitizedGitEnv(opts.ExtraEnv...)
	cmd.Stdout, cmd.Stderr = opts.Stdout, opts.Stderr
	return cmd
}

// absoluteRepo resolves the repository selector against the caller's working
// directory; empty selects the current directory.
func absoluteRepo(repo string) (string, error) {
	if repo == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve repository root: %w", err)
		}
		return cwd, nil
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", fmt.Errorf("resolve repository root %q: %w", repo, err)
	}
	return abs, nil
}

// resolveGitDirOutput runs one rev-parse directory query against the absolute
// root, rejects malformed output, and canonicalizes the answer.
func resolveGitDirOutput(ctx context.Context, root, flag string) (string, error) {
	out, err := Run(ctx, "", "-C", root, "rev-parse", flag)
	if err != nil {
		return "", err
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" || strings.Contains(raw, "\x00") || strings.HasPrefix(raw, "--") || strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("git rev-parse %s: unexpected output %q", flag, raw)
	}
	resolved, err := symlinkResolvedDir(root, raw)
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", flag, err)
	}
	return resolved, nil
}

// symlinkResolvedDir absolutizes p against base, resolves symlinks, and
// requires an existing directory.
func symlinkResolvedDir(base, p string) (string, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(p))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", resolved)
	}
	return resolved, nil
}

// resolveAgainst absolutizes p against base and resolves symlinks best-effort.
func resolveAgainst(base, p string) string {
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	p = filepath.Clean(p)
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return p
}

// sanitizedGitEnv returns the process environment minus every inherited GIT_*
// variable, LC_ALL and LANG, plus LANG=C, LC_ALL=C and the caller's extras.
func sanitizedGitEnv(extra ...string) []string {
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
