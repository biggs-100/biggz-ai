// Command gitexec is the compiled guard the CI step builds from this repository and
// runs directly:
//
//	go build -o /tmp/gitexec ./tools/gitexec/cmd/gitexec && /tmp/gitexec -root .
//
// The exit status is the verdict. It is 0 only when the fixture self-check passed,
// the scan resolved targets, and every baseline entry matched a live site. A missing
// binary exits 127 through the shell; a build failure never reaches the binary and
// leaves nothing to run. Neither can print a verdict.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/biggs-100/biggz-ai/tools/gitexec"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(argv []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gitexec", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root to scan")
	fixtures := flags.String("fixtures", filepath.Join("tools", "gitexec", "testdata", "fixtures"),
		"checked-in fixture set for the positive control")
	update := flags.Bool("update", false, "refresh baseline line numbers in place; never adds or deletes entries")
	if err := flags.Parse(argv); err != nil {
		return 1
	}
	// 1. Positive control: prove the scanner scanned before trusting a clean tree.
	self, err := gitexec.SelfCheck(*fixtures)
	if err != nil {
		fmt.Fprintf(stderr, "gitexec: %v\n", err)
		return 1
	}
	// 2. Census and classify the tree.
	res, err := (gitexec.Scanner{Root: *root}).Scan()
	if err != nil {
		fmt.Fprintf(stderr, "gitexec: %v\n", err)
		return 1
	}
	// 3. Reconcile against the frozen baseline.
	baselinePath := filepath.Join(*root, ".github", "guard-baseline.txt")
	entries, err := gitexec.LoadBaseline(baselinePath)
	if err != nil {
		fmt.Fprintf(stderr, "gitexec: %v\n", err)
		return 1
	}
	stale, added, refreshed := gitexec.Reconcile(entries, res.Findings)
	for _, f := range added {
		fmt.Fprintln(stdout, f)
	}
	for _, entry := range stale {
		fmt.Fprintf(stderr, "gitexec: stale baseline entry: %s\n", entry)
	}
	if len(added) > 0 || len(stale) > 0 {
		return 1
	}
	if *update {
		if err := os.WriteFile(baselinePath, gitexec.FormatBaseline(refreshed), 0o644); err != nil {
			fmt.Fprintf(stderr, "gitexec: %v\n", err)
			return 1
		}
	}
	debt, boundary := counts(res.Findings)
	fmt.Fprintf(stdout, "gitexec: OK — self-check %d sites, %d .go + %d host targets, %d debt, %d boundary, baseline matched\n",
		len(self.Findings), res.GoTargets, res.HostTargets, debt, boundary)
	return 0
}

// counts splits live sites into the debt and boundary classes.
func counts(findings []gitexec.Finding) (debt, boundary int) {
	for _, f := range findings {
		if f.Rule == gitexec.RuleGoGitSpawn {
			debt++
			continue
		}
		boundary++
	}
	return debt, boundary
}
