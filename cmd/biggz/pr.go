package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// prCreate handles "biggz pr create" — auto-generates a branch and PR from
// completed SDD apply work.
func prCreate() int {
	args := os.Args[2:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz pr create <change-name> [--title T] [--body B] [--label L]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Auto-generates a branch and PR from completed SDD apply work.")
		fmt.Fprintln(os.Stderr, "Reads the apply-progress to determine what was changed.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Flags:")
		fmt.Fprintln(os.Stderr, "  --title T     PR title (default: derived from change name)")
		fmt.Fprintln(os.Stderr, "  --body B      PR body file or text")
		fmt.Fprintln(os.Stderr, "  --label L     Comma-separated labels (default: auto-detected)")
		fmt.Fprintln(os.Stderr, "  --dry-run     Print what would be done without doing it")
		fmt.Fprintln(os.Stderr, "  --no-issue    Skip issue reference check")
		return 1
	}

	if args[0] != "create" {
		fmt.Fprintf(os.Stderr, "unknown: pr %s\n", args[0])
		return 1
	}

	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: biggz pr create <change-name>")
		return 1
	}

	changeName := args[1]
	dryRun := false
	title := ""
	body := ""
	labels := ""
	noIssue := false

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			dryRun = true
		case "--no-issue":
			noIssue = true
		case "--title":
			if i+1 < len(args) {
				title = args[i+1]; i++
			}
		case "--body":
			if i+1 < len(args) {
				body = args[i+1]; i++
			}
		case "--label":
			if i+1 < len(args) {
				labels = args[i+1]; i++
			}
		}
	}

	// Detect project root
	projectRoot, _ := os.Getwd()
	if gitRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		projectRoot = strings.TrimSpace(string(gitRoot))
	}

	// Derive branch type from change name
	branchType := detectBranchType(changeName)
	branchName := fmt.Sprintf("%s/%s", branchType, strings.ToLower(strings.ReplaceAll(changeName, " ", "-")))

	// Derive title
	if title == "" {
		title = fmt.Sprintf("%s: %s", branchType, changeName)
	}

	// Get changed files since last commit or from the apply-progress
	changedFiles := getChangedFiles(projectRoot)

	// Build PR body
	if body == "" {
		body = buildPRBody(changeName, changedFiles)
	}

	if dryRun {
		fmt.Println("=== PR Dry Run ===")
		fmt.Printf("Branch: %s\n", branchName)
		fmt.Printf("Title:  %s\n", title)
		if !noIssue {
			fmt.Println("Issue:  REQUIRED (Closes #N in body)")
		}
		fmt.Printf("Labels: %s\n", labels)
		fmt.Println("Body:")
		fmt.Println(body)
		fmt.Println("\nFiles to include:")
		for _, f := range changedFiles {
			fmt.Printf("  %s\n", f)
		}
		return 0
	}

	// Check for uncommitted changes
	status, _ := exec.Command("git", "status", "--porcelain").Output()
	if strings.TrimSpace(string(status)) == "" {
		fmt.Println("No uncommitted changes. Nothing to PR.")
		return 0
	}

	// Create and switch to branch
	if err := runCmd("git", "checkout", "-b", branchName); err != nil {
		fmt.Fprintf(os.Stderr, "error: create branch: %v\n", err)
		return 1
	}

	// Stage all changes
	if err := runCmd("git", "add", "-A"); err != nil {
		fmt.Fprintf(os.Stderr, "error: stage: %v\n", err)
		return 1
	}

	// Commit with conventional message
	commitMsg := fmt.Sprintf("%s: %s", branchType, strings.ToLower(changeName))
	if err := runCmd("git", "commit", "-m", commitMsg); err != nil {
		fmt.Fprintf(os.Stderr, "error: commit: %v\n", err)
		return 1
	}

	// Push
	if err := runCmd("git", "push", "-u", "origin", branchName); err != nil {
		fmt.Fprintf(os.Stderr, "error: push: %v\n", err)
		return 1
	}

	// Create PR via gh
	prArgs := []string{"pr", "create", "--title", title, "--body", body}
	if labels != "" {
		for _, l := range strings.Split(labels, ",") {
			prArgs = append(prArgs, "--label", strings.TrimSpace(l))
		}
	}
	prURL, err := exec.Command("gh", prArgs...).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: create PR: %v\n", err)
		return 1
	}
	fmt.Printf("PR created: %s", string(prURL))
	return 0
}

func detectBranchType(name string) string {
	name = strings.ToLower(name)
	for _, prefix := range []string{"fix", "bug", "hotfix"} {
		if strings.HasPrefix(name, prefix) {
			return "fix"
		}
	}
	for _, prefix := range []string{"docs", "doc", "readme"} {
		if strings.HasPrefix(name, prefix) {
			return "docs"
		}
	}
	for _, prefix := range []string{"refactor", "cleanup"} {
		if strings.HasPrefix(name, prefix) {
			return "refactor"
		}
	}
	for _, prefix := range []string{"chore", "ci", "build", "test"} {
		if strings.HasPrefix(name, prefix) {
			return "chore"
		}
	}
	return "feat"
}

func getChangedFiles(projectRoot string) []string {
	out, _ := exec.Command("git", "diff", "--name-only", "--cached").Output()
	staged := strings.Split(strings.TrimSpace(string(out)), "\n")

	out, _ = exec.Command("git", "diff", "--name-only").Output()
	unstaged := strings.Split(strings.TrimSpace(string(out)), "\n")

	out, _ = exec.Command("git", "ls-files", "--others", "--exclude-standard").Output()
	untracked := strings.Split(strings.TrimSpace(string(out)), "\n")

	var all []string
	for _, f := range staged {
		if f != "" { all = append(all, f) }
	}
	for _, f := range unstaged {
		if f != "" { all = append(all, f) }
	}
	for _, f := range untracked {
		if f != "" { all = append(all, f) }
	}
	return all
}

func buildPRBody(changeName string, files []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Summary\n\n%s\n\n", changeName))
	b.WriteString("## Changes\n\n| File | Description |\n|------|------------|\n")
	for _, f := range files {
		b.WriteString(fmt.Sprintf("| `%s` | |\n", f))
	}
	b.WriteString("\n## Test Plan\n\n")
	b.WriteString("- [ ] `go build ./...`\n")
	b.WriteString("- [ ] `go test ./...`\n")
	b.WriteString("- [ ] `go vet ./...`\n")
	b.WriteString("\nCloses #\n")
	return b.String()
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
