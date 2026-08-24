package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/biggs-100/biggz-ai/internal/sdd"
)

func TestSddStatusWatchWithJSONErrors(t *testing.T) {
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	if err := os.MkdirAll(filepath.Join(planning, "openspec", "changes"), 0755); err != nil {
		t.Fatalf("mkdir openspec: %v", err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{"watch and json long flags", []string{"--cwd", planning, "--json", "--watch"}},
		{"watch json reverse order", []string{"--cwd", planning, "--watch", "--json"}},
		{"short watch with json", []string{"--cwd", planning, "-w", "--json"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, stderr := runSDDStatusCLIArgs(t, tt.args...)
			if code == 0 {
				t.Fatalf("expected error for --watch with --json, got code 0")
			}
			if !strings.Contains(stderr, "cannot use --watch with --json") {
				t.Errorf("stderr = %q, want containing %q", stderr, "cannot use --watch with --json")
			}
		})
	}
}

func TestStatusWatchShouldError(t *testing.T) {
	tests := []struct {
		name     string
		emitJSON bool
		watch    bool
		want     bool
	}{
		{"neither", false, false, false},
		{"only watch", false, true, false},
		{"only json", true, false, false},
		{"both true", true, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusWatchShouldError(tt.emitJSON, tt.watch)
			if got != tt.want {
				t.Errorf("statusWatchShouldError(%v, %v) = %v, want %v", tt.emitJSON, tt.watch, got, tt.want)
			}
		})
	}
}

func TestParseSddStatusArgsWatchFlag(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantWatch bool
		wantJSON  bool
		wantErr   bool
	}{
		{"long watch", []string{"--watch"}, true, false, false},
		{"short watch", []string{"-w"}, true, false, false},
		{"watch with json", []string{"--watch", "--json"}, true, true, false},
		{"watch with cwd", []string{"--watch", "--cwd", "/tmp"}, true, false, false},
		{"unknown flag", []string{"--watch", "--unknown"}, true, false, true},
		{"help flag", []string{"--help"}, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, emitJSON, _, watch, hasHelp, errMsg := parseSddStatusArgs(tt.args)
			if tt.wantErr && errMsg == "" {
				t.Errorf("parseSddStatusArgs(%v) expected error, got none", tt.args)
			}
			if !tt.wantErr && errMsg != "" {
				t.Errorf("parseSddStatusArgs(%v) unexpected error: %q", tt.args, errMsg)
			}
			if tt.name == "help flag" {
				if !hasHelp {
					t.Errorf("expected hasHelp true for %v", tt.args)
				}
				return
			}
			if watch != tt.wantWatch {
				t.Errorf("watch = %v, want %v", watch, tt.wantWatch)
			}
			if emitJSON != tt.wantJSON {
				t.Errorf("emitJSON = %v, want %v", emitJSON, tt.wantJSON)
			}
		})
	}
}

func TestRenderStatusOnce(t *testing.T) {
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	if err := os.MkdirAll(planning, 0755); err != nil {
		t.Fatalf("mkdir planning: %v", err)
	}
	// No openspec yet — render should error
	if _, err := renderStatusOnce(filepath.Join(planning, "openspec"), sdd.StatusOptions{}); err == nil {
		t.Errorf("expected error for missing openspec, got nil")
	}

	// Seed a complete change
	seedCompleteChange(t, planning, "render-change")
	openspecRoot := filepath.Join(planning, "openspec")

	// Without instructions
	out, err := renderStatusOnce(openspecRoot, sdd.StatusOptions{})
	if err != nil {
		t.Fatalf("renderStatusOnce failed: %v", err)
	}
	if !strings.Contains(out, "render-change") {
		t.Errorf("render output missing change name, got %q", out)
	}
	if !strings.Contains(out, "Active changes") && !strings.Contains(out, "Recent archived") {
		t.Errorf("render output missing expected header, got %q", out)
	}

	// With instructions — still renders (FormatStatus ignores instructions, but StatusWithOptions includes them)
	out2, err := renderStatusOnce(openspecRoot, sdd.StatusOptions{IncludeInstructions: true})
	if err != nil {
		t.Fatalf("renderStatusOnce with instructions failed: %v", err)
	}
	if !strings.Contains(out2, "render-change") {
		t.Errorf("render with instructions missing change name, got %q", out2)
	}
}

func TestSddStatusWatchHelpText(t *testing.T) {
	code, _, stderr := runSDDStatusCLIArgs(t, "--help")
	if code != 0 {
		t.Fatalf("help exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr, "--watch") {
		t.Errorf("help text missing --watch, got %q", stderr)
	}
	if !strings.Contains(stderr, "refresh every 2s") {
		t.Errorf("help text missing refresh phrase, got %q", stderr)
	}
	// Also check -h
	code, _, stderr = runSDDStatusCLIArgs(t, "-h")
	if code != 0 {
		t.Fatalf("help -h exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr, "-w") {
		t.Errorf("help text missing -w, got %q", stderr)
	}
}

func TestSddStatusWatchLoopRendersHeader(t *testing.T) {
	origInterval := sddStatusWatchInterval
	origIterations := sddStatusWatchIterations
	sddStatusWatchInterval = 10 * time.Millisecond
	sddStatusWatchIterations = 1
	defer func() {
		sddStatusWatchInterval = origInterval
		sddStatusWatchIterations = origIterations
	}()

	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	if err := os.MkdirAll(planning, 0755); err != nil {
		t.Fatalf("mkdir planning: %v", err)
	}
	seedCompleteChange(t, planning, "watch-loop-change")

	code, stdout, stderr := runSDDStatusCLIArgs(t, "--cwd", planning, "--watch")
	if code != 0 {
		t.Fatalf("watch loop exit code = %d (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, "biggz sdd-status --watch") {
		t.Errorf("stdout missing watch header, got %q", stdout)
	}
	if !strings.Contains(stdout, "refresh every 2s") {
		t.Errorf("stdout missing refresh phrase, got %q", stdout)
	}
	if !strings.Contains(stdout, "Ctrl+C to exit") {
		t.Errorf("stdout missing Ctrl+C phrase, got %q", stdout)
	}
	if !strings.Contains(stdout, "watch-loop-change") {
		t.Errorf("stdout missing change name, got %q", stdout)
	}
	// ANSI clear should be present
	if !strings.Contains(stdout, "\033[H\033[2J") && !strings.Contains(stdout, "\033c") {
		t.Errorf("stdout missing ANSI clear, got %q", stdout)
	}
}

func TestSddStatusWatchAllowsInstructions(t *testing.T) {
	origInterval := sddStatusWatchInterval
	origIterations := sddStatusWatchIterations
	sddStatusWatchInterval = 10 * time.Millisecond
	sddStatusWatchIterations = 1
	defer func() {
		sddStatusWatchInterval = origInterval
		sddStatusWatchIterations = origIterations
	}()

	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	if err := os.MkdirAll(planning, 0755); err != nil {
		t.Fatalf("mkdir planning: %v", err)
	}
	seedCompleteChange(t, planning, "watch-instr-change")

	code, stdout, stderr := runSDDStatusCLIArgs(t, "--cwd", planning, "--watch", "--instructions")
	if code != 0 {
		t.Fatalf("watch with instructions exit code = %d (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, "biggz sdd-status --watch") {
		t.Errorf("stdout missing watch header, got %q", stdout)
	}
	if !strings.Contains(stdout, "watch-instr-change") {
		t.Errorf("stdout missing change name with instructions, got %q", stdout)
	}
}
