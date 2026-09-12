// Package assets_test verifies the embedded OpenCode plugin files keep the
// ported contract shapes from gentle-ai while preserving biggz's deliberate
// divergences (quarantine-to-file persistence).
package assets_test

import (
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

// TestSDDVerifySubjectWriter proves the shipped sdd-verify skill instructs the
// verify phase to write the review subject file the status offer points at
// (design D5): `<changeRoot>/review-subject.json` with the workspace root and
// commit_sha HEAD, written before the RDD gate runs so the offered
// `biggz review start --subject ...` invocation is runnable, while sdd-status
// stays read-only. The assertion reads the embedded asset (never a disk
// mirror), so dropping the instruction from the skill fails here instead of
// surfacing later as an unrunnable offer.
func TestSDDVerifySubjectWriter(t *testing.T) {
	data, err := assets.FS.ReadFile("skills/sdd-verify/SKILL.md")
	if err != nil {
		t.Fatalf("Read(skills/sdd-verify/SKILL.md) error = %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"Before the RDD gate runs, write `<changeRoot>/review-subject.json`",
		`{"repository":"<workspace root>","commit_sha":"HEAD"}`,
		"biggz review start --subject '<changeRoot>/review-subject.json'",
		"`sdd-status` stays read-only",
		"the writer lives in verify, never in status",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("sdd-verify/SKILL.md missing the review-subject writer instruction %q", want)
		}
	}

	// Either model variant may be rendered for the verify phase, so both the
	// <!-- section:model-capable --> and <!-- section:model-small --> variants
	// must carry the instruction; a single occurrence means one rendering lost
	// it and the writer contract silently regressed for that model class.
	if got := strings.Count(source, "Before the RDD gate runs, write `<changeRoot>/review-subject.json`"); got != 2 {
		t.Fatalf("review-subject writer instruction present %d times, want once per model variant (capable + small)", got)
	}
}
