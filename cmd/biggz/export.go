package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func exportRun() int {
	args := os.Args[2:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz export <type> [args...]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Types:")
		fmt.Fprintln(os.Stderr, "  review <id> [--format json|txt]   Export a review report")
		fmt.Fprintln(os.Stderr, "  memory [--format json|txt]        Export BigMem memories")
		fmt.Fprintln(os.Stderr, "  changelog [--since DATE]          Export changelog from git log")
		return 1
	}

	format := "txt"
	for i := 1; i < len(args); i++ {
		if args[i] == "--format" && i+1 < len(args) {
			format = args[i+1]
		}
	}

	switch args[0] {
	case "review":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz export review <id> [--format json|txt]")
			return 1
		}
		return exportReview(args[1], format)

	case "memory":
		return exportMemory(format)

	case "changelog":
		since := ""
		for i := 1; i < len(args); i++ {
			if args[i] == "--since" && i+1 < len(args) {
				since = args[i+1]
			}
		}
		return exportChangelog(since, format)

	default:
		fmt.Fprintf(os.Stderr, "unknown export type: %s\n", args[0])
		return 1
	}
}

func exportReview(id, format string) int {
	// Read the review ledger/compact state from the store
	content := fmt.Sprintf("Review Export: %s\nGenerated: %s\n\n", id, time.Now().UTC().Format(time.RFC3339))
	content += "This is a review report placeholder.\n"
	content += "To export from the compact store, use: biggz recovery export <id>\n"

	if format == "json" {
		data, _ := json.MarshalIndent(map[string]string{
			"id":        id,
			"exported":  time.Now().UTC().Format(time.RFC3339),
			"content":   content,
		}, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Println(content)
	}
	return 0
}

func exportMemory(format string) int {
	content := fmt.Sprintf("BigMem Memory Export\nGenerated: %s\n\n", time.Now().UTC().Format(time.RFC3339))
	content += "Use 'biggz bigmem export <file>' to export memories.\n"

	if format == "json" {
		data, _ := json.MarshalIndent(map[string]string{
			"exported": time.Now().UTC().Format(time.RFC3339),
			"note":     "use 'biggz bigmem export' for full memory export",
		}, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Println(content)
	}
	return 0
}

func exportChangelog(since, format string) int {
	args := []string{"log", "--oneline", "--no-decorate"}
	if since != "" {
		args = append(args, fmt.Sprintf("--since=%s", since))
	}

	output, err := exec.Command("git", args...).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: git log: %v\n", err)
		return 1
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if format == "json" {
		var entries []map[string]string
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, " ", 2)
			entry := map[string]string{"commit": parts[0]}
			if len(parts) > 1 {
				entry["message"] = parts[1]
			}
			entries = append(entries, entry)
		}
		data, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Println("Changelog:")
		fmt.Println("==========")
		for _, line := range lines {
			if line != "" {
				fmt.Println(line)
			}
		}
	}
	return 0
}


