package main

import (
	"encoding/json"
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
//   - `biggz codegraph report <change> [--cwd] [--json] [--md]` — full change-intent graph
func codegraphRun() int {
	args := os.Args[2:]

	// Top-level help for codegraph verb
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		printCodegraphHelp()
		return 0
	}

	if len(args) == 1 && args[0] == "guidance" {
		fmt.Print(codegraph.GuidanceMarkdown())
		if !strings.HasSuffix(codegraph.GuidanceMarkdown(), "\n") {
			fmt.Println()
		}
		return 0
	}

	// Route report before init validation so report help is reachable
	if len(args) >= 1 && args[0] == "report" {
		return reportRun(args[1:])
	}

	if len(args) != 3 || args[0] != "init" || args[1] != "--cwd" || strings.TrimSpace(args[2]) == "" {
		printCodegraphHelp()
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

func printCodegraphHelp() {
	fmt.Fprintln(os.Stderr, "usage: biggz codegraph <command> [args...]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  init --cwd <project-root>            Initialize CodeGraph index for a project")
	fmt.Fprintln(os.Stderr, "  guidance                             Print CodeGraph agent guidance")
	fmt.Fprintln(os.Stderr, "  report <change> [--cwd <path>] [--json <path>] [--md <path>]")
	fmt.Fprintln(os.Stderr, "                                         Generate change-intent graph (JSON + Markdown)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Flags for report:")
	fmt.Fprintln(os.Stderr, "  --cwd <path>   Project root (default .)")
	fmt.Fprintln(os.Stderr, "  --json <path>  JSON output path (default openspec/changes/<change>/codegraph.json)")
	fmt.Fprintln(os.Stderr, "  --md <path>    Markdown output path (default openspec/changes/<change>/codegraph.md)")
}

// resolveReportRoot canonicalizes cwd for report via Abs+EvalSymlinks, rejecting traversal via clean resolution.
func resolveReportRoot(candidate string) (string, error) {
	if strings.TrimSpace(candidate) == "" {
		candidate = "."
	}
	canonical, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		canonical = candidate
	}
	abs, err := filepath.Abs(canonical)
	if err != nil {
		return "", fmt.Errorf("resolve report root %q: %w", candidate, err)
	}
	// Reject obvious unsafe traversal if resolved path contains ".." after cleaning (should not happen after Abs)
	cleaned := filepath.Clean(abs)
	if cleaned != abs {
		abs = cleaned
	}
	// Validate directory exists?
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("resolve report root %q: %w", candidate, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("resolve report root %q: not a directory", candidate)
	}
	return abs, nil
}

func reportRun(args []string) int {
	// Handle help
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			fmt.Fprintln(os.Stderr, "usage: biggz codegraph report <change> [--cwd <path>] [--json <path>] [--md <path>]")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Generate change-intent graph from SDD artifacts + Go scan.")
			fmt.Fprintln(os.Stderr, "  <change>       Change name under openspec/changes/<change> (proposal.md required)")
			fmt.Fprintln(os.Stderr, "  --cwd <path>   Project root (default .)")
			fmt.Fprintln(os.Stderr, "  --json <path>  JSON output path (default openspec/changes/<change>/codegraph.json)")
			fmt.Fprintln(os.Stderr, "  --md <path>    Markdown output path (default openspec/changes/<change>/codegraph.md)")
			return 0
		}
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(os.Stderr, "usage: biggz codegraph report <change> [--cwd <path>] [--json <path>] [--md <path>]")
		fmt.Fprintln(os.Stderr, "error: <change> is required")
		return 1
	}
	change := strings.TrimSpace(args[0])
	if change == "" {
		fmt.Fprintln(os.Stderr, "usage: biggz codegraph report <change> [--cwd <path>] [--json <path>] [--md <path>]")
		fmt.Fprintln(os.Stderr, "error: <change> is required")
		return 1
	}
	// Parse flags after change
	cwd := "."
	jsonPath := ""
	mdPath := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--cwd":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
				fmt.Fprintln(os.Stderr, "error: --cwd requires a value")
				return 1
			}
			cwd = args[i+1]
			i++
		case "--json":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
				fmt.Fprintln(os.Stderr, "error: --json requires a value")
				return 1
			}
			jsonPath = args[i+1]
			i++
		case "--md":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
				fmt.Fprintln(os.Stderr, "error: --md requires a value")
				return 1
			}
			mdPath = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "--cwd=") {
				cwd = strings.TrimPrefix(args[i], "--cwd=")
			} else if strings.HasPrefix(args[i], "--json=") {
				jsonPath = strings.TrimPrefix(args[i], "--json=")
			} else if strings.HasPrefix(args[i], "--md=") {
				mdPath = strings.TrimPrefix(args[i], "--md=")
			} else {
				fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", args[i])
				fmt.Fprintln(os.Stderr, "usage: biggz codegraph report <change> [--cwd <path>] [--json <path>] [--md <path>]")
				return 1
			}
		}
	}
	root, err := resolveReportRoot(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	report, err := codegraph.Generate(change, root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	// Resolve default output paths if not provided
	if strings.TrimSpace(jsonPath) == "" {
		jsonPath = filepath.Join(root, "openspec", "changes", change, "codegraph.json")
	}
	if strings.TrimSpace(mdPath) == "" {
		mdPath = filepath.Join(root, "openspec", "changes", change, "codegraph.md")
	}
	if err := codegraph.Emit(report, jsonPath, mdPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	// Dual output: emit JSON to stdout as well
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Println(string(data))
	return 0
}
