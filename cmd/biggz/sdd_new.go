package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func sddNewRun() int {
	args := os.Args[2:]
	if len(args) >= 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(os.Stderr, "Usage: biggz sdd-new [change-name]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Interactive wizard to create a new SDD change.")
		fmt.Fprintln(os.Stderr, "Scaffolds change directory and proposal document.")
		return 1
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("=== SDD New Change Wizard ===\n")

	// Step 1: Change name
	changeName := ""
	if len(args) > 0 {
		changeName = args[0]
	} else {
		fmt.Print("Change name (short, kebab-case): ")
		name, _ := reader.ReadString('\n')
		changeName = strings.TrimSpace(strings.ToLower(strings.ReplaceAll(name, " ", "-")))
	}
	if changeName == "" {
		fmt.Fprintln(os.Stderr, "error: change name is required")
		return 1
	}
	fmt.Printf("  → %s\n\n", changeName)

	// Step 2: Description
	fmt.Print("Brief description (what does this change do?): ")
	desc, _ := reader.ReadString('\n')
	desc = strings.TrimSpace(desc)
	if desc == "" {
		desc = changeName
	}
	fmt.Printf("  → %s\n\n", desc)

	// Step 3: Change type
	fmt.Println("Change type:")
	types := []struct {
		id   string
		desc string
	}{
		{"feature", "New feature or enhancement"},
		{"bugfix", "Bug fix"},
		{"refactor", "Code refactoring"},
		{"docs", "Documentation"},
		{"chore", "Maintenance, CI, tooling"},
		{"performance", "Performance improvement"},
	}
	for i, t := range types {
		fmt.Printf("  %d. %s — %s\n", i+1, t.id, t.desc)
	}
	fmt.Printf("Select [1-%d] (default: 1): ", len(types))
	typeIdx := 1
	typeStr, _ := reader.ReadString('\n')
	typeStr = strings.TrimSpace(typeStr)
	if typeStr != "" {
		fmt.Sscanf(typeStr, "%d", &typeIdx)
	}
	if typeIdx < 1 || typeIdx > len(types) {
		typeIdx = 1
	}
	changeType := types[typeIdx-1].id
	fmt.Printf("  → %s\n\n", changeType)

	// Step 4: Domain template
	fmt.Println("Domain template (optional — pre-fills spec format):")
	templates := []struct {
		id   string
		desc string
	}{
		{"none", "No template (start from scratch)"},
		{"api-endpoint", "API endpoint"},
		{"cli-command", "CLI command"},
		{"bug-fix", "Bug fix"},
		{"refactor", "Refactor"},
		{"database-migration", "Database migration"},
	}
	for i, t := range templates {
		fmt.Printf("  %d. %s — %s\n", i+1, t.id, t.desc)
	}
	fmt.Printf("Select [1-%d] (default: 1): ", len(templates))
	tmplIdx := 1
	tmplStr, _ := reader.ReadString('\n')
	tmplStr = strings.TrimSpace(tmplStr)
	if tmplStr != "" {
		fmt.Sscanf(tmplStr, "%d", &tmplIdx)
	}
	if tmplIdx < 1 || tmplIdx > len(templates) {
		tmplIdx = 1
	}
	templateID := templates[tmplIdx-1].id
	fmt.Printf("  → %s\n\n", templateID)

	// Step 5: Confirm
	fmt.Println("=== Summary ===")
	fmt.Printf("  Name:        %s\n", changeName)
	fmt.Printf("  Description: %s\n", desc)
	fmt.Printf("  Type:        %s\n", changeType)
	fmt.Printf("  Template:    %s\n", templateID)
	fmt.Print("\nCreate change? [Y/n]: ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm == "n" || confirm == "no" {
		fmt.Println("Cancelled.")
		return 0
	}

	// Scaffold
	projectRoot := detectProjectRoot()
	changeDir := filepath.Join(projectRoot, "openspec", "changes", changeName)

	// Create directories
	dirs := []string{
		changeDir,
		filepath.Join(changeDir, "specs"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error: create %s: %v\n", d, err)
			return 1
		}
	}

	// Write _meta.yaml
	metaContent := fmt.Sprintf(`change_name: %s
description: %s
type: %s
created_at: %s
status: draft
`, changeName, desc, changeType, time.Now().UTC().Format(time.RFC3339))
	os.WriteFile(filepath.Join(changeDir, "_meta.yaml"), []byte(metaContent), 0644)

	// Write proposal.md
	proposalContent := fmt.Sprintf(`# Proposal: %s

## Intent

%s

## Scope

**In scope:**
-

**Out of scope:**
-

## Approach

[Describe the technical approach]

## Success Criteria

- [ ]

## Rollback Plan

[How to revert if something goes wrong]
`, changeName, desc)
	os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte(proposalContent), 0644)

	// Copy template if selected
	if templateID != "none" {
		tmplSrc := filepath.Join(projectRoot, "openspec", "templates", templateID+".md")
		tmplDst := filepath.Join(changeDir, "specs", templateID+".md")
		if data, err := os.ReadFile(tmplSrc); err == nil {
			os.WriteFile(tmplDst, data, 0644)
		}
	}

	fmt.Printf("\n✓ Change %q created at:\n", changeName)
	fmt.Printf("  %s\n", changeDir)
	fmt.Printf("  %s\n", filepath.Join(changeDir, "proposal.md"))
	fmt.Printf("\nNext: run sdd-propose or edit proposal.md\n")
	return 0
}

func detectProjectRoot() string {
	projectRoot, _ := os.Getwd()
	if gitRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		projectRoot = strings.TrimSpace(string(gitRoot))
	}
	return projectRoot
}
