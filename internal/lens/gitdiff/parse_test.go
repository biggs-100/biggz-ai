package gitdiff

import (
	"testing"
)

// ---- ParseDiffStat ----

func TestParseDiffStat_Basic(t *testing.T) {
	output := `main.go | 10 ++++++++---
README.md | 2 +-
`
	files := ParseDiffStat(output)

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// main.go: 10 total, 8 plus chars, 3 minus chars → 10*8/11 = 7 additions, 3 deletions
	if files[0].Path != "main.go" {
		t.Errorf("files[0].Path = %q, want %q", files[0].Path, "main.go")
	}
	if files[0].Additions != 7 {
		t.Errorf("files[0].Additions = %d, want %d", files[0].Additions, 7)
	}
	if files[0].Deletions != 3 {
		t.Errorf("files[0].Deletions = %d, want %d", files[0].Deletions, 3)
	}

	// README.md: 2 total, 1 plus, 1 minus → 1 addition, 1 deletion
	if files[1].Path != "README.md" {
		t.Errorf("files[1].Path = %q, want %q", files[1].Path, "README.md")
	}
	if files[1].Additions != 1 {
		t.Errorf("files[1].Additions = %d, want %d", files[1].Additions, 1)
	}
	if files[1].Deletions != 1 {
		t.Errorf("files[1].Deletions = %d, want %d", files[1].Deletions, 1)
	}
}

func TestParseDiffStat_AdditionsOnly(t *testing.T) {
	output := `newfile.go | 15 +++++++++++++++
`
	files := ParseDiffStat(output)

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "newfile.go" {
		t.Errorf("Path = %q, want %q", files[0].Path, "newfile.go")
	}
	if files[0].Additions != 15 {
		t.Errorf("Additions = %d, want %d", files[0].Additions, 15)
	}
	if files[0].Deletions != 0 {
		t.Errorf("Deletions = %d, want %d", files[0].Deletions, 0)
	}
}

func TestParseDiffStat_Empty(t *testing.T) {
	files := ParseDiffStat("")
	if len(files) != 0 {
		t.Errorf("expected 0 files for empty input, got %d", len(files))
	}
}

func TestParseDiffStat_NoMatchLine(t *testing.T) {
	output := ` 1 file changed, 10 insertions(+), 3 deletions(-)
`
	files := ParseDiffStat(output)
	if len(files) != 0 {
		t.Errorf("expected 0 files for summary line, got %d", len(files))
	}
}

func TestParseDiffStat_SpacesInPath(t *testing.T) {
	output := `my project/main.go | 5 +++++
`
	files := ParseDiffStat(output)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "my project/main.go" {
		t.Errorf("Path = %q, want %q", files[0].Path, "my project/main.go")
	}
}

// ---- DetectModeChanges ----

func TestDetectModeChanges_Executable(t *testing.T) {
	raw := `:100644 100755 abc123 abc456 M	scripts/deploy.sh
:100644 100644 def789 def012 M	README.md
`
	if !DetectModeChanges(raw) {
		t.Error("expected true for mode change to 100755")
	}
}

func TestDetectModeChanges_NoExecutableChange(t *testing.T) {
	raw := `:100644 100644 abc123 abc456 M	README.md
`
	if DetectModeChanges(raw) {
		t.Error("expected false when no mode change to 100755")
	}
}

func TestDetectModeChanges_Empty(t *testing.T) {
	if DetectModeChanges("") {
		t.Error("expected false for empty input")
	}
}

func TestParseDiffStat_MultipleNoDeletions(t *testing.T) {
	output := `a.go | 3 +++
b.go | 5 +++++
`
	files := ParseDiffStat(output)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Additions != 3 || files[0].Deletions != 0 {
		t.Errorf("a.go: additions=%d deletions=%d, want 3, 0", files[0].Additions, files[0].Deletions)
	}
	if files[1].Additions != 5 || files[1].Deletions != 0 {
		t.Errorf("b.go: additions=%d deletions=%d, want 5, 0", files[1].Additions, files[1].Deletions)
	}
}
