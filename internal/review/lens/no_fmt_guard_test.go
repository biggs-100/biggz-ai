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

// TestCIGuard_CurrentLensClean is the CI enforcement step for the lens
// fmt.Sprintf contract: every non-test Go file under internal/review/lens must
// either avoid fmt.Sprintf or carry the `//lint:ignore no-fmtSprintf` marker
// on the same line. Paths are resolved from the repository root, and an
// unreadable file or an empty scan is a hard failure — a scan that cannot
// read its targets must never pass.
func TestCIGuard_CurrentLensClean(t *testing.T) {
	root := repoRoot(t)
	files := lensGoFiles(t, root)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("unreadable target %s: %v (an unreadable file fails the guard, never passes it)", f, err)
		}
		if filtered := filterPromptSprintf(strings.Split(string(data), "\n")); len(filtered) != 0 {
			t.Errorf("%s has non-allowlisted fmt.Sprintf: %v", f, filtered)
		}
		if strings.Contains(string(data), "html/template") {
			t.Errorf("should not use html/template in %s", f)
		}
	}
}

// repoRoot resolves the repository root from the package working directory
// (`go test` sets cwd to the package dir), so repository-relative paths are
// read from where they actually live instead of silently failing open.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repository root (go.mod) not found above %s", dir)
		}
		dir = parent
	}
}

// lensGoFiles lists every non-test .go file under internal/review/lens.
// Zero resolved files is a hard failure: a scan with no targets is not a pass.
func lensGoFiles(t *testing.T, root string) []string {
	t.Helper()
	base := filepath.Join(root, "internal", "review", "lens")
	var files []string
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", base, err)
	}
	if len(files) == 0 {
		t.Fatalf("scan resolved zero non-test .go files under %s", base)
	}
	return files
}
