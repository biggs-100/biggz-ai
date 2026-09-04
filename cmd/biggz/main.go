package main

import (
	"fmt"
	"os"

	"github.com/biggs-100/biggz-ai/internal/doctor"
	"github.com/biggs-100/biggz-ai/internal/tui"
)

const nonInteractiveTUIError = "biggz tui requires both stdin and stdout to be terminals"

// isattyFn reports whether fd is a terminal. It is a package-level var for
// test injection, matching the gentle-ai port (55216cfc) and the existing
// isTerminalFunc in cli_sdd.go. The default implementation uses Stat +
// ModeCharDevice, which is sufficient for the dual-stream guard without
// pulling golang.org/x/term.
var isattyFn = func(fd uintptr) bool {
	var file *os.File
	switch fd {
	case os.Stdin.Fd():
		file = os.Stdin
	case os.Stdout.Fd():
		file = os.Stdout
	case os.Stderr.Fd():
		file = os.Stderr
	default:
		file = os.NewFile(fd, "")
		if file == nil {
			return false
		}
	}
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// checkTUIInteractive reports whether the current process can launch the TUI.
// It is extracted for testability; main() prints the error and exits.
func checkTUIInteractive() error {
	if !isattyFn(os.Stdin.Fd()) || !isattyFn(os.Stdout.Fd()) {
		return fmt.Errorf("%s", nonInteractiveTUIError)
	}
	return nil
}

// ---- CLI Entry Point ----

func main() {
	if len(os.Args) > 1 {
		// Check if first arg is a recognized subcommand
		switch os.Args[1] {
		case "install":
			os.Exit(installRun())
		case "uninstall":
			os.Exit(uninstallRun())
		case "sdd-status":
			os.Exit(sddStatusRun())
		case "sdd-apply":
			os.Exit(sddApplyRun())
		case "sdd-verify-validate":
			os.Exit(sddVerifyValidateRun())
		case "sdd-attempt":
			os.Exit(sddAttemptRun())
		case "sdd-continue":
			os.Exit(sddContinueRun())
		case "sdd-new":
			os.Exit(sddNewRun())
		case "sdd-profile":
			os.Exit(sddProfileRun())
		case "sdd-remediate":
			os.Exit(sddRemediateRun())
		case "sdd-gatekeeper":
			os.Exit(sddGatekeeperRun())
		case "sdd-dedup":
			os.Exit(sddDedupRun())
		case "sdd-workload":
			os.Exit(sddWorkloadRun())
		case "bigmem":
			os.Exit(bigmemRun())
		case "recall":
			os.Exit(recallRun())
		case "backup":
			os.Exit(backupRun())
		case "help":
			os.Exit(helpRun())
		case "codegraph":
			os.Exit(codegraphRun())
		case "release":
			os.Exit(releaseRun())
		case "skill-registry":
			os.Exit(skillRegistryRun())
		case "rdd":
			os.Exit(rddRun())
		case "tdd":
			os.Exit(tddRun())
		case "review":
			os.Exit(reviewRun())
		case "doctor":
			os.Exit(doctorRun())
		case "update":
			os.Exit(updateRun())
		case "upgrade":
			os.Exit(upgradeRun())
		case "sync":
			os.Exit(syncRun())
		case "plugin":
			os.Exit(pluginRun())
		case "mcp":
			os.Exit(mcpRun())
		case "pr":
			os.Exit(prCreate())
		case "export":
			os.Exit(exportRun())
		case "hooks":
			os.Exit(hooksRun())
		case "recovery":
			os.Exit(recoveryRun())
		case "version", "--version", "-v":
			v := doctor.BuildVersion
			if v == "" {
				v = "dev"
			}
			fmt.Printf("biggz-ai %s\n", v)
			return
		}
	}

	// If no recognized subcommand, check for --help
	if len(os.Args) > 1 {
		for _, arg := range os.Args[1:] {
			if arg == "--help" || arg == "-h" {
				printHelp()
				return
			}
		}
	}

	// Interactive terminal → TUI launcher (bare-invocation parity with gentle-ai).
	// Port 55216cfc: require BOTH stdin and stdout to be terminals, with an
	// injectable isattyFn for the dual-stream matrix.
	if err := checkTUIInteractive(); err != nil {
		fmt.Fprintln(os.Stderr, nonInteractiveTUIError)
		printHelp()
		os.Exit(1)
	}
	tui.Run()
	return
}
