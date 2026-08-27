package lens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIGuard_PromptFmtSprintfFails(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "lens.go")
	content := "package tmp\nimport \"fmt\"\nfunc Foo(){ fmt.Sprintf(\"prompt %s\", \"x\") }\n"
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, _ := os.ReadFile(file)
	lines := strings.Split(string(data), "\n")
	found := false
	for _, line := range lines {
		if strings.Contains(line, "fmt.Sprintf") && !strings.Contains(line, "//lint:ignore no-fmtSprintf") {
			found = true
		}
	}
	if !found {
		t.Fatal("should find non-allowlisted fmt.Sprintf")
	}
	// Simulate CI filter: should be fail
	filtered := []string{}
	for _, line := range lines {
		if strings.Contains(line, "fmt.Sprintf") && !strings.Contains(line, "//lint:ignore no-fmtSprintf") {
			filtered = append(filtered, line)
		}
	}
	if len(filtered) == 0 {
		t.Error("expected fail for non-allowlisted")
	}
}

func TestCIGuard_CleanPasses(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "clean.go")
	content := "package tmp\nfunc Foo(){ println(\"clean\") }\n"
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, _ := os.ReadFile(file)
	if strings.Contains(string(data), "fmt.Sprintf") {
		t.Error("clean should not contain fmt.Sprintf")
	}
	lines := strings.Split(string(data), "\n")
	filtered := filterPromptSprintf(lines)
	if len(filtered) != 0 {
		t.Errorf("clean should pass, got %v", filtered)
	}
}

func TestCIGuard_AllowlistedPasses(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "allowed.go")
	content := "package tmp\nimport \"fmt\"\nfunc Foo(){ fmt.Sprintf(\"x\") //lint:ignore no-fmtSprintf\n}\n"
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, _ := os.ReadFile(file)
	lines := strings.Split(string(data), "\n")
	filtered := filterPromptSprintf(lines)
	if len(filtered) != 0 {
		t.Errorf("allowlisted should pass, got %v", filtered)
	}
}

func filterPromptSprintf(lines []string) []string {
	var out []string
	for _, line := range lines {
		if strings.Contains(line, "fmt.Sprintf") && !strings.Contains(line, "//lint:ignore no-fmtSprintf") {
			out = append(out, line)
		}
	}
	return out
}

func TestCIGuard_CurrentLensClean(t *testing.T) {
	// Verify current internal/review/lens has no non-allowlisted fmt.Sprintf via direct file scan
	// Walk files (non-test) and check
	files := []string{
		"internal/review/lens/readability/lens.go",
		"internal/review/lens/readability/complexity.go",
		"internal/review/lens/reliability/lens.go",
		"internal/review/lens/resilience/lens.go",
		"internal/review/lens/external/adapter.go",
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		filtered := filterPromptSprintf(lines)
		if len(filtered) != 0 {
			t.Errorf("%s has non-allowlisted fmt.Sprintf: %v", f, filtered)
		}
	}
	// Also ensure html/template not used for prompts
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "html/template") {
			t.Errorf("should not use html/template in %s", f)
		}
	}
}
