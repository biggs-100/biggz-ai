package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/codegraph"
)

// codegraphRun handles `biggz codegraph` subcommands.
// Supported shapes:
//   - `biggz codegraph init --cwd <project-root>` — validates the root and delegates to `codegraph init <root>`
//   - `biggz codegraph guidance` — prints the CodeGraph agent guidance markdown
func codegraphRun() int {
	args := os.Args[2:]

	if len(args) == 1 && args[0] == "guidance" {
		fmt.Print(codegraph.GuidanceMarkdown())
		if !strings.HasSuffix(codegraph.GuidanceMarkdown(), "\n") {
			fmt.Println()
		}
		return 0
	}

	if len(args) != 3 || args[0] != "init" || args[1] != "--cwd" || strings.TrimSpace(args[2]) == "" {
		fmt.Fprintln(os.Stderr, "usage: biggz codegraph init --cwd <project-root>")
		fmt.Fprintln(os.Stderr, "       biggz codegraph guidance")
		return 1
	}

	root, err := resolveCodeGraphRoot(args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Delegate to the upstream CodeGraph binary. The verb exists only to
	// validate the root before initialization; intelligence queries use the
	// upstream CLI directly (codegraph status/query/explore/etc.).
	cmd := exec.Command("codegraph", "init", root)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Surface the underlying error with context, including a hint when the
		// binary is not on PATH.
		if execErr, ok := err.(*exec.Error); ok && os.IsNotExist(execErr.Err) {
			fmt.Fprintln(os.Stderr, "error: codegraph binary not found on PATH — install https://github.com/Gentleman-Programming/codegraph")
		}
		fmt.Fprintf(os.Stderr, "error: initialize CodeGraph at %q: %v\n", root, err)
		return 1
	}

	fmt.Printf("CodeGraph initialized: %s\n", root)
	return 0
}

// resolveCodeGraphRoot canonicalizes the candidate and validates it as a safe
// CodeGraph project root. Adapted from gentle-ai's canonicalCodeGraphProjectRoot.
func resolveCodeGraphRoot(candidate string) (string, error) {
	canonicalCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve CodeGraph root %q: %w", candidate, err)
	}
	canonicalCandidate, err = filepath.Abs(canonicalCandidate)
	if err != nil {
		return "", fmt.Errorf("resolve CodeGraph root %q: %w", candidate, err)
	}
	if isUnsafeCodeGraphRoot(canonicalCandidate) {
		return "", fmt.Errorf("unsafe CodeGraph root %q", candidate)
	}
	gitRoot, err := codegraphGitTopLevel(canonicalCandidate)
	if err != nil {
		return "", fmt.Errorf("CodeGraph root %q is not a recognized project: %w", candidate, err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(strings.TrimSpace(gitRoot))
	if err != nil {
		return "", fmt.Errorf("resolve project root %q: %w", gitRoot, err)
	}
	canonicalRoot, err = filepath.Abs(canonicalRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root %q: %w", gitRoot, err)
	}
	if canonicalRoot != canonicalCandidate || isUnsafeCodeGraphRoot(canonicalRoot) {
		return "", fmt.Errorf("unsafe CodeGraph root %q", candidate)
	}
	return canonicalRoot, nil
}

var (
	codegraphHomeDir = os.UserHomeDir
	codegraphTempDir = os.TempDir
)

func isUnsafeCodeGraphRoot(root string) bool {
	if volume := filepath.VolumeName(root); root == volume+string(filepath.Separator) {
		return true
	}
	home := canonicalRestrictedPath(codegraphHomeDirFunc())
	if home != "" && root == home {
		return true
	}
	temp := canonicalRestrictedPath(codegraphTempDir())
	if temp != "" && pathIsWithin(temp, root) {
		return true
	}
	return false
}

func canonicalRestrictedPath(path string) string {
	if path == "" {
		return ""
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		canonical = path
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return ""
	}
	return canonical
}

func codegraphHomeDirFunc() string {
	home, err := codegraphHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func pathIsWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func codegraphGitTopLevel(path string) (string, error) {
	output, err := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
