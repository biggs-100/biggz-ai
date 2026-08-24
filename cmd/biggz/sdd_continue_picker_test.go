package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/sdd"
)

func TestSddContinue_NoActiveChanges(t *testing.T) {
	workspace := t.TempDir()
	openspecRoot := filepath.Join(workspace, "openspec")
	if err := os.MkdirAll(filepath.Join(openspecRoot, "changes", "archive"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(old)

	var stdout, stderr bytes.Buffer
	code := runSddContinue([]string{}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "No active changes") {
		t.Errorf("stdout = %q, want containing 'No active changes'", stdout.String())
	}
	if !strings.Contains(stdout.String(), "biggz sdd-new") {
		t.Errorf("stdout = %q, want hint to sdd-new", stdout.String())
	}
}

func TestSddContinue_SingleAutoSelect(t *testing.T) {
	workspace := t.TempDir()
	openspecRoot := filepath.Join(workspace, "openspec")
	changeDir := filepath.Join(openspecRoot, "changes", "single-change")
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Proposal\n"), 0644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(old)

	var stdout, stderr bytes.Buffer
	code := runSddContinue([]string{}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Auto-selected: single-change") {
		t.Errorf("stdout = %q, want Auto-selected", out)
	}
	if !strings.Contains(out, "[next:") {
		t.Errorf("stdout = %q, want [next:", out)
	}
	if !strings.Contains(out, "Change: single-change") {
		t.Errorf("stdout = %q, want Change line after auto-select", out)
	}
	if !strings.Contains(out, "Next phase:") {
		t.Errorf("stdout = %q, want Next phase", out)
	}
}

func TestSddContinue_MultipleListNonTTY(t *testing.T) {
	workspace := t.TempDir()
	openspecRoot := filepath.Join(workspace, "openspec")
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(openspecRoot, "changes", name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# P\n"), 0644); err != nil {
			t.Fatalf("write proposal %s: %v", name, err)
		}
	}
	old, _ := os.Getwd()
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(old)

	var stdout, stderr bytes.Buffer
	// Non-TTY stdin (bytes.Buffer is not *os.File CharDevice)
	code := runSddContinue([]string{}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 for non-TTY with multiple; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Select a change to continue:") {
		t.Errorf("stdout = %q, want picker header", out)
	}
	if !strings.Contains(out, "1) alpha") {
		t.Errorf("stdout = %q, want '1) alpha'", out)
	}
	if !strings.Contains(out, "2) beta") {
		t.Errorf("stdout = %q, want '2) beta'", out)
	}
	if !strings.Contains(out, "[next:") {
		t.Errorf("stdout = %q, want [next:", out)
	}
	if !strings.Contains(out, "tasks") {
		t.Errorf("stdout = %q, want tasks count", out)
	}
	if !strings.Contains(stderr.String(), "biggz sdd-continue <change>") {
		t.Errorf("stderr = %q, want hint for non-TTY", stderr.String())
	}
}

func TestSddContinue_HelpMentionsPicker(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runSddContinue([]string{"--help"}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	help := stderr.String()
	if !strings.Contains(help, "Usage: biggz sdd-continue") {
		t.Errorf("help = %q, want Usage", help)
	}
	if !strings.Contains(strings.ToLower(help), "picker") {
		t.Errorf("help = %q, want mentioning picker", help)
	}
	if !strings.Contains(help, "[change]") {
		t.Errorf("help = %q, want [change] optional", help)
	}
}

func TestSddContinue_WithArg(t *testing.T) {
	workspace := t.TempDir()
	openspecRoot := filepath.Join(workspace, "openspec")
	changeDir := filepath.Join(openspecRoot, "changes", "my-change")
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# P\n"), 0644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(old)

	var stdout, stderr bytes.Buffer
	code := runSddContinue([]string{"my-change"}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Change: my-change") {
		t.Errorf("stdout = %q, want Change line", stdout.String())
	}
}

func TestSddContinue_PromptPicker_ValidSelection(t *testing.T) {
	active := []sdd.ChangeStatus{
		{Name: "alpha", NextRecommended: "spec", TaskProgress: sdd.TaskProgress{Total: 2, Completed: 1}},
		{Name: "beta", NextRecommended: "design", TaskProgress: sdd.TaskProgress{Total: 3, Completed: 0}},
	}
	in := strings.NewReader("2\n")
	var out bytes.Buffer
	name, err := promptChangePickerWithIO(active, in, &out)
	if err != nil {
		t.Fatalf("prompt error: %v", err)
	}
	if name != "beta" {
		t.Errorf("selected = %q, want beta", name)
	}
	if !strings.Contains(out.String(), "Enter number") {
		t.Errorf("prompt output = %q, want prompt", out.String())
	}
}

func TestSddContinue_PromptPicker_InvalidThenValid(t *testing.T) {
	active := []sdd.ChangeStatus{
		{Name: "alpha", NextRecommended: "spec"},
		{Name: "beta", NextRecommended: "design"},
	}
	in := strings.NewReader("5\n1\n")
	var out bytes.Buffer
	name, err := promptChangePickerWithIO(active, in, &out)
	if err != nil {
		t.Fatalf("prompt error: %v", err)
	}
	if name != "alpha" {
		t.Errorf("selected = %q, want alpha", name)
	}
	if !strings.Contains(out.String(), "Invalid selection") {
		t.Errorf("output = %q, want Invalid selection message", out.String())
	}
}

func TestSddContinue_PromptPicker_EmptyInputThenValid(t *testing.T) {
	active := []sdd.ChangeStatus{
		{Name: "a", NextRecommended: "propose"},
		{Name: "b", NextRecommended: "spec"},
	}
	in := strings.NewReader("\n\n2\n")
	var out bytes.Buffer
	name, err := promptChangePickerWithIO(active, in, &out)
	if err != nil {
		t.Fatalf("prompt error: %v", err)
	}
	if name != "b" {
		t.Errorf("selected = %q, want b", name)
	}
}

func TestSddContinue_MultipleTTYPrompt(t *testing.T) {
	workspace := t.TempDir()
	openspecRoot := filepath.Join(workspace, "openspec")
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(openspecRoot, "changes", name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# P\n"), 0644); err != nil {
			t.Fatalf("write proposal %s: %v", name, err)
		}
	}
	oldWd, _ := os.Getwd()
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	orig := isTerminalFunc
	isTerminalFunc = func(io.Reader) bool { return true }
	defer func() { isTerminalFunc = orig }()

	in := strings.NewReader("2\n")
	var stdout, stderr bytes.Buffer
	code := runSddContinue([]string{}, in, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Select a change to continue:") {
		t.Errorf("stdout = %q, want picker header", out)
	}
	if !strings.Contains(out, "Enter number") {
		t.Errorf("stdout = %q, want prompt", out)
	}
	if !strings.Contains(out, "Change: beta") {
		t.Errorf("stdout = %q, want selected beta after prompt", out)
	}
}

// Table-driven test for phase label.
func TestSddContinuePhaseLabel(t *testing.T) {
	tests := []struct {
		name string
		cs   sdd.ChangeStatus
		want string
	}{
		{
			name: "nextRecommended proposal",
			cs:   sdd.ChangeStatus{NextRecommended: "propose"},
			want: "next: propose",
		},
		{
			name: "done",
			cs:   sdd.ChangeStatus{NextRecommended: "done"},
			want: "done",
		},
		{
			name: "resolve-blockers with reasons",
			cs:   sdd.ChangeStatus{NextRecommended: "resolve-blockers", BlockedReasons: []string{"a blocker"}},
			want: "resolve-blockers: a blocker",
		},
		{
			name: "fallback no proposal",
			cs:   sdd.ChangeStatus{HasProposal: false},
			want: "explore/proposal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sddContinuePhaseLabel(tt.cs)
			if got != tt.want {
				t.Errorf("phaseLabel = %q, want %q", got, tt.want)
			}
		})
	}
}
