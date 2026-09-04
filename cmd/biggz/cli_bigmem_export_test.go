package main

// CLI-level tests for paged export (SDD fix-bigmem-mcp-nplus1, Phase 3):
// export pages past the 50-row Store cap, --limit/--project behave, the JSON
// array shape round-trips through import, and conflicts list output is stable.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
)

func isolateExportHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

func seedExportSaves(t *testing.T, n int, project string) {
	t.Helper()
	for i := 0; i < n; i++ {
		title := fmt.Sprintf("export-note-%s-%04d", project, i)
		body := fmt.Sprintf("export body %s %04d unique-%s-%04d", project, i, project, i)
		code, _, stderr := captureBigmemRun([]string{"save", title, body, "--project", project})
		if code != 0 {
			t.Fatalf("save #%d exit %d stderr=%q", i, code, stderr)
		}
	}
}

func readExportFile(t *testing.T, path string) []*bigmem.Observation {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var obs []*bigmem.Observation
	if err := json.Unmarshal(data, &obs); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return obs
}

func TestExportCompletesBeyond50(t *testing.T) {
	dir := isolateExportHome(t)
	seedExportSaves(t, 70, "exp1")
	out := filepath.Join(dir, "out.json")
	code, _, stderr := captureBigmemRun([]string{"export", out})
	if code != 0 {
		t.Fatalf("export exit %d stderr=%q", code, stderr)
	}
	if got := readExportFile(t, out); len(got) != 70 {
		t.Fatalf("export = %d rows, want 70", len(got))
	}
}

func TestExportLimitFlag(t *testing.T) {
	dir := isolateExportHome(t)
	seedExportSaves(t, 70, "exp1")

	capped := filepath.Join(dir, "capped.json")
	code, _, stderr := captureBigmemRun([]string{"export", capped, "--limit", "60"})
	if code != 0 {
		t.Fatalf("export --limit 60 exit %d stderr=%q", code, stderr)
	}
	if got := readExportFile(t, capped); len(got) != 60 {
		t.Fatalf("export --limit 60 = %d rows, want 60", len(got))
	}

	neg := filepath.Join(dir, "neg.json")
	if code, _, _ := captureBigmemRun([]string{"export", neg, "--limit", "-1"}); code != 1 {
		t.Fatalf("export --limit -1 exit %d, want 1", code)
	}
}

func TestExportProjectFilter(t *testing.T) {
	dir := isolateExportHome(t)
	seedExportSaves(t, 70, "exp1")
	seedExportSaves(t, 15, "exp2")
	out := filepath.Join(dir, "p2.json")
	code, _, stderr := captureBigmemRun([]string{"export", out, "--project", "exp2"})
	if code != 0 {
		t.Fatalf("export --project exit %d stderr=%q", code, stderr)
	}
	got := readExportFile(t, out)
	if len(got) != 15 {
		t.Fatalf("export --project exp2 = %d rows, want 15", len(got))
	}
	for _, o := range got {
		if o.Project != "exp2" {
			t.Fatalf("export leaked project %q", o.Project)
		}
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	dir := isolateExportHome(t)
	seedExportSaves(t, 70, "exp1")
	out := filepath.Join(dir, "out.json")
	if code, _, stderr := captureBigmemRun([]string{"export", out}); code != 0 {
		t.Fatalf("export exit %d stderr=%q", code, stderr)
	}
	// Fresh store: import must re-parse the paged file with zero errors.
	fresh := t.TempDir()
	t.Setenv("HOME", fresh)
	t.Setenv("USERPROFILE", fresh)
	code, stdout, stderr := captureBigmemRun([]string{"import", out})
	if code != 0 {
		t.Fatalf("import exit %d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "Imported 70/70") {
		t.Fatalf("import output = %q, want Imported 70/70", stdout)
	}
}

func TestConflictsListStable(t *testing.T) {
	isolateExportHome(t)
	seedExportSaves(t, 3, "exp1")
	code, first, stderr := captureBigmemRun([]string{"conflicts", "list"})
	if code != 0 {
		t.Fatalf("conflicts list exit %d stderr=%q", code, stderr)
	}
	code, second, stderr := captureBigmemRun([]string{"conflicts", "list"})
	if code != 0 {
		t.Fatalf("conflicts list (rerun) exit %d stderr=%q", code, stderr)
	}
	if first != second {
		t.Fatalf("conflicts list not stable:\n%q\n%q", first, second)
	}
}
