// Package main: run and compare commands.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cmdRun executes journeys against one binary.
func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	binary := fs.String("binary", "", "product binary to drive")
	out := fs.String("out", "-", "results file (default stdout)")
	only := fs.String("only", "", "comma-separated journey ids to run")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *binary == "" {
		fmt.Fprintln(os.Stderr, "error: --binary is required")
		return 2
	}
	// Journeys run with a temp working dir, so resolve once up front.
	abs, err := filepath.Abs(*binary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving --binary: %v\n", err)
		return 2
	}
	*binary = abs
	var selected []journey
	if *only == "" {
		selected = journeys
	} else {
		want := map[string]bool{}
		for _, id := range strings.Split(*only, ",") {
			want[strings.TrimSpace(id)] = true
		}
		for _, j := range journeys {
			if want[j.id] {
				selected = append(selected, j)
			}
		}
		if len(selected) == 0 {
			fmt.Fprintln(os.Stderr, "error: --only matched no journeys")
			return 2
		}
	}
	rep := RunReport{Binary: *binary}
	failed := 0
	for _, j := range selected {
		res := j.run(*binary)
		rep.Journeys = append(rep.Journeys, res)
		status := "pass"
		if !res.Pass {
			status = "FAIL"
			failed++
		}
		fmt.Printf("%s\t%s\t%s\n", status, res.ID, res.Detail)
	}
	if err := writeReport(rep, *out); err != nil {
		fmt.Fprintf(os.Stderr, "error writing report: %v\n", err)
		return 1
	}
	if failed > 0 {
		return 1
	}
	return 0
}

// cmdCompare diffs two run reports journey by journey.
func cmdCompare(args []string) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	before := fs.String("before", "", "baseline results file")
	after := fs.String("after", "", "candidate results file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *before == "" || *after == "" {
		fmt.Fprintln(os.Stderr, "error: --before and --after are required")
		return 2
	}
	brep, err := loadReport(*before)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading before: %v\n", err)
		return 1
	}
	arep, err := loadReport(*after)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading after: %v\n", err)
		return 1
	}
	byID := map[string]JourneyResult{}
	for _, j := range brep.Journeys {
		byID[j.ID] = j
	}
	regressed := 0
	fmt.Printf("%-38s %-10s %-10s %s\n", "journey", "before", "after", "verdict")
	for _, a := range arep.Journeys {
		b, ok := byID[a.ID]
		bs, as := verdict(ok && b.Pass), verdict(a.Pass)
		v := "same"
		if ok && b.Pass && !a.Pass {
			v = "REGRESSION"
			regressed++
		} else if ok && !b.Pass && a.Pass {
			v = "improved"
		} else if !ok {
			v = "new"
		}
		fmt.Printf("%-38s %-10s %-10s %s\n", a.ID, bs, as, v)
	}
	if regressed > 0 {
		return 1
	}
	return 0
}

func verdict(pass bool) string {
	if pass {
		return "pass"
	}
	return "fail"
}
