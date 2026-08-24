package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/doctor"
	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/sdd"
)

// prEvidenceOptions configures evidence injection for PR bodies.
type prEvidenceOptions struct {
	WithEvidence bool
	ChangeFilter string
	TemplatePath string
	Cwd          string
}

// prCreate handles "biggz pr create" — auto-generates a branch and PR from
// completed SDD apply work.
func prCreate() int {
	args := os.Args[2:]
	// Help: bare `biggz pr` or `biggz pr --help`
	if len(args) == 0 {
		printPRHelp()
		return 1
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			printPRHelp()
			return 1
		}
	}

	if args[0] != "create" {
		fmt.Fprintf(os.Stderr, "unknown: pr %s\n", args[0])
		return 1
	}

	// Parse flags and positional change-name.
	// Supports interspersed flags before/after the positional.
	changeName := ""
	dryRun := false
	title := ""
	bodyFlag := ""
	labels := ""
	noIssue := false
	withEvidence := false
	changeFilter := ""
	templatePath := ""

	// Start after "create"
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			dryRun = true
		case "--no-issue":
			noIssue = true
		case "--with-evidence":
			withEvidence = true
		case "--change":
			if i+1 < len(args) {
				changeFilter = args[i+1]
				i++
			}
		case "--template":
			if i+1 < len(args) {
				templatePath = args[i+1]
				i++
			}
		case "--title":
			if i+1 < len(args) {
				title = args[i+1]
				i++
			}
		case "--body":
			if i+1 < len(args) {
				bodyFlag = args[i+1]
				i++
			}
		case "--label":
			if i+1 < len(args) {
				labels = args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(args[i], "--") {
				fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", args[i])
				return 1
			}
			if changeName == "" {
				changeName = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "error: unexpected argument %q\n", args[i])
				return 1
			}
		}
	}

	if changeName == "" {
		fmt.Fprintln(os.Stderr, "Usage: biggz pr create <change-name> [--title T] [--body B] [--label L] [--with-evidence] [--change <name>] [--template <path>]")
		return 1
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
	body := ""
	if bodyFlag != "" {
		body = bodyFlag
	} else {
		body = buildPRBody(changeName, changedFiles)
	}
	if withEvidence {
		cwdForEvidence := projectRoot
		if cwdForEvidence == "" {
			cwdForEvidence, _ = os.Getwd()
		}
		evidence := buildEvidenceBlock(cwdForEvidence, changeFilter)
		if templatePath != "" {
			if data, err := os.ReadFile(templatePath); err == nil {
				tmpl := string(data)
				if strings.Contains(tmpl, "{{evidence}}") {
					body = strings.ReplaceAll(tmpl, "{{evidence}}", evidence)
				} else {
					body = strings.TrimRight(tmpl, "\n") + "\n\n" + evidence
				}
			} else {
				fmt.Fprintf(os.Stderr, "warning: could not read template %q: %v\n", templatePath, err)
				if strings.Contains(body, "{{evidence}}") {
					body = strings.ReplaceAll(body, "{{evidence}}", evidence)
				} else {
					body = body + "\n\n" + evidence
				}
			}
		} else {
			if strings.Contains(body, "{{evidence}}") {
				body = strings.ReplaceAll(body, "{{evidence}}", evidence)
			} else {
				body = body + "\n\n" + evidence
			}
		}
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

func printPRHelp() {
	fmt.Fprintln(os.Stderr, "Usage: biggz pr create <change-name> [--title T] [--body B] [--label L] [--with-evidence] [--change <name>] [--template <path>]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Auto-generates a branch and PR from completed SDD apply work.")
	fmt.Fprintln(os.Stderr, "Reads the apply-progress to determine what was changed.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Flags:")
	fmt.Fprintln(os.Stderr, "  --title T         PR title (default: derived from change name)")
	fmt.Fprintln(os.Stderr, "  --body B          PR body file or text")
	fmt.Fprintln(os.Stderr, "  --label L         Comma-separated labels (default: auto-detected)")
	fmt.Fprintln(os.Stderr, "  --dry-run         Print what would be done without doing it")
	fmt.Fprintln(os.Stderr, "  --no-issue        Skip issue reference check")
	fmt.Fprintln(os.Stderr, "  --with-evidence   Auto-inject SDD evidence chain and review lineage into PR body")
	fmt.Fprintln(os.Stderr, "  --change <name>   Scope evidence to one SDD change (default: all active)")
	fmt.Fprintln(os.Stderr, "  --template <path> Path to custom PR template with {{evidence}} placeholder")
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
		if f != "" {
			all = append(all, f)
		}
	}
	for _, f := range unstaged {
		if f != "" {
			all = append(all, f)
		}
	}
	for _, f := range untracked {
		if f != "" {
			all = append(all, f)
		}
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

// buildPRBodyWithEvidence builds the PR body with optional evidence injection.
// It is the testable entry point for --with-evidence behavior; prCreate delegates
// to it when the flag is set. Cwd in opts controls the workspace root for
// SDD status (defaults to projectRoot/git toplevel via os.Getwd).
func buildPRBodyWithEvidence(changeName string, files []string, opts prEvidenceOptions) string {
	base := buildPRBody(changeName, files)
	if !opts.WithEvidence {
		return base
	}
	cwd := opts.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
		if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
			if t := strings.TrimSpace(string(out)); t != "" {
				cwd = t
			}
		}
	}
	evidence := buildEvidenceBlock(cwd, opts.ChangeFilter)
	if opts.TemplatePath != "" {
		if data, err := os.ReadFile(opts.TemplatePath); err == nil {
			tmpl := string(data)
			if strings.Contains(tmpl, "{{evidence}}") {
				return strings.ReplaceAll(tmpl, "{{evidence}}", evidence)
			}
			return strings.TrimRight(tmpl, "\n") + "\n\n" + evidence
		}
		// template unreadable: fall through to base injection
	}
	if strings.Contains(base, "{{evidence}}") {
		return strings.ReplaceAll(base, "{{evidence}}", evidence)
	}
	return base + "\n\n" + evidence
}

// buildEvidenceBlock assembles the deterministic markdown evidence block:
// SDD changes, review lineage, version, and comparison reference.
func buildEvidenceBlock(cwd string, changeFilter string) string {
	var b strings.Builder
	b.WriteString("## Evidence\n")
	openspecRoot := filepath.Join(cwd, "openspec")
	active, _, err := sdd.StatusWithOptions(openspecRoot, sdd.StatusOptions{})
	if err != nil {
		active = nil
	}
	if changeFilter != "" {
		var filtered []sdd.ChangeStatus
		for _, cs := range active {
			if cs.Name == changeFilter {
				filtered = append(filtered, cs)
			}
		}
		active = filtered
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Name < active[j].Name })
	b.WriteString(fmt.Sprintf("- **SDD Changes**: %d active\n", len(active)))
	for _, cs := range active {
		phase := prPhaseLabel(cs)
		next := cs.NextRecommended
		if next == "" {
			next = "unknown"
		}
		b.WriteString(fmt.Sprintf("  - `%s` — [next: %s] (%d/%d tasks) — %s\n", cs.Name, next, cs.TaskProgress.Completed, cs.TaskProgress.Total, phase))
		if cs.RemediationState.Required {
			b.WriteString(fmt.Sprintf("    - Remediation: %s\n", cs.RemediationState.Reason))
		}
	}
	// Review lineage — best effort; skip gracefully when no store/lineage.
	if lineages, err := review.NewAuthority(cwd).Inventory(); err == nil && len(lineages) > 0 {
		for _, li := range lineages {
			st, err := review.NewAuthority(cwd).Status(li.LineageID)
			if err != nil {
				continue
			}
			state := "unknown"
			if st.NextTransition != nil {
				state = st.NextTransition.Action
				if st.NextTransition.Reason != "" {
					state = fmt.Sprintf("%s (%s)", state, st.NextTransition.Reason)
				}
			} else if st.Receipt != nil {
				state = "finalized"
			} else if st.EventCount > 0 {
				state = "in_review"
			} else if !st.ChainValid {
				state = "invalid"
			}
			receipt := "none"
			if st.Receipt != nil && st.Receipt.BindingHash != "" {
				receipt = shortEvidenceHash(st.Receipt.BindingHash)
			} else if st.HeadHash != "" {
				receipt = shortEvidenceHash(st.HeadHash)
			}
			budget := 0
			if st.Budget != nil {
				budget = st.Budget.CorrectionLines
			}
			lensesStr := "-"
			if len(st.Lenses) > 0 {
				var names []string
				for _, l := range st.Lenses {
					names = append(names, l.Lens)
				}
				lensesStr = strings.Join(names, ", ")
			} else if len(st.LensPlan) > 0 {
				lensesStr = strings.Join(st.LensPlan, ", ")
			}
			b.WriteString(fmt.Sprintf("- **Review**: lineage `%s` — %s — receipt `%s` — budget `%d` — lenses `%s`\n", st.LineageID, state, receipt, budget, lensesStr))
		}
	}
	ver := currentVersion(cwd)
	b.WriteString(fmt.Sprintf("- **Version**: %s\n", ver))
	b.WriteString("- **Comparison**: see `docs/comparison-with-gentle.md`\n")
	return b.String()
}

func prPhaseLabel(cs sdd.ChangeStatus) string {
	if cs.NextRecommended != "" {
		switch cs.NextRecommended {
		case "done":
			return "done"
		case "resolve-blockers":
			if len(cs.BlockedReasons) > 0 {
				return "resolve-blockers: " + strings.Join(cs.BlockedReasons, " ")
			}
			return "resolve-blockers"
		default:
			return "next: " + cs.NextRecommended
		}
	}
	switch {
	case !cs.HasProposal:
		return "explore/proposal"
	case !cs.HasSpecs:
		return "spec"
	case !cs.HasDesign:
		return "design"
	case !cs.HasTasks:
		return "tasks"
	case cs.TasksDone < cs.TasksTotal:
		return fmt.Sprintf("apply (%d/%d tasks)", cs.TasksDone, cs.TasksTotal)
	case !cs.HasVerify:
		return "verify"
	default:
		return "archive-ready"
	}
}

func shortEvidenceHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func currentVersion(cwd string) string {
	if doctor.BuildVersion != "" {
		return doctor.BuildVersion
	}
	repo := cwd
	if repo == "" {
		repo, _ = os.Getwd()
	}
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	if repo != "" {
		cmd.Dir = repo
	}
	if out, err := cmd.Output(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return v
		}
	}
	return "dev"
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
