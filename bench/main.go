// Command biggz-ceremony-bench measures workflow friction by driving a
// biggz-ai binary as a black-box subprocess. Deterministic and offline:
// no model is ever called. Every journey runs in a fresh temp directory
// with its own HOME and never touches real config.
//
//	bench run --binary /path/to/biggz --out results-after.json
//	bench run --binary /path/to/old-biggz --out results-before.json
//	bench compare --before results-before.json --after results-after.json
//
// `run` exits non-zero when any journey fails. `compare` exits non-zero
// when any journey regressed (pass -> fail).
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Observation is everything the bench can see about one product invocation:
// the process boundary and nothing more, so it works against any build,
// including old releases.
type Observation struct {
	Args     []string `json:"args"`
	ExitCode int      `json:"exit_code"`
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
}

// JourneyResult is the verdict of one journey run.
type JourneyResult struct {
	ID     string        `json:"id"`
	Pass   bool          `json:"pass"`
	Detail string        `json:"detail"`
	Steps  []Observation `json:"steps,omitempty"`
}

// RunReport is the full output of `run`.
type RunReport struct {
	Binary   string          `json:"binary"`
	Journeys []JourneyResult `json:"journeys"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "compare":
		os.Exit(cmdCompare(os.Args[2:]))
	case "list":
		for _, j := range journeys {
			fmt.Printf("%s\t%s\n", j.id, j.desc)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  bench run --binary PATH [--out FILE] [--only j01,j02]")
	fmt.Fprintln(os.Stderr, "  bench compare --before FILE --after FILE")
	fmt.Fprintln(os.Stderr, "  bench list")
}

// writeReport marshals the report as indented JSON to stdout or a file.
func writeReport(rep RunReport, out string) error {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if out == "" || out == "-" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(out, data, 0644)
}

func loadReport(path string) (RunReport, error) {
	var rep RunReport
	data, err := os.ReadFile(path)
	if err != nil {
		return rep, err
	}
	err = json.Unmarshal(data, &rep)
	return rep, err
}
