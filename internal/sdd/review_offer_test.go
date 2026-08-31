package sdd

import (
	"os"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
)

// TestReviewOffer emits offer when enabled PASS, nil when disabled/fail, and quoting for spaces.
func TestReviewOffer(t *testing.T) {
	// Helper to force RDD enabled/disabled via global file
	t.Run("enabled_PASS_emits_offer", func(t *testing.T) {
		ws := t.TempDir()
		// Ensure RDD enabled (default ON) – no setup needed, but ensure global enabled
		// We isolate by using t.TempDir as HOME? Our isRDDEnabled uses git dirs, not HOME for global? Actually review reads ~/.biggz/rdd-mode.json via UserHomeDir, which points to real home, not temp. So we rely on existing global enabled state which is enabled.
		// Verify real global is enabled
		status, _ := review.RDDStatus("", "")
		if status != nil && status.EffectiveMode == review.RDDModeDisabled {
			t.Skip("global RDD disabled, cannot test enabled case")
		}
		changeRoot := seedDeriveChange(t, ws, "offer-pass", map[string]string{
			"proposal.md":        "# Proposal\n",
			"specs/core/spec.md": specFixture,
			"design.md":          "# Design\n",
			"tasks.md":           "- [x] T1\n",
			"verify-report.md":   passingReport("1/1", "1/1"),
		})
		cs, err := readChange(changeRoot, "offer-pass", false, ws, false)
		if err != nil {
			t.Fatalf("readChange: %v", err)
		}
		if cs.ReviewOffer == nil || !cs.ReviewOffer.Available {
			t.Fatalf("expected ReviewOffer available, got %+v", cs.ReviewOffer)
		}
		if !strings.Contains(cs.ReviewOffer.Invocation, "biggz review start --lineage") {
			t.Fatalf("invocation missing prefix: %q", cs.ReviewOffer.Invocation)
		}
		// Must contain quoted lineage via pathquote.Quote (double quotes around change-sha)
		if !strings.Contains(cs.ReviewOffer.Invocation, "\"offer-pass-") {
			t.Fatalf("invocation not quoted: %q", cs.ReviewOffer.Invocation)
		}
		// Must not contain persisted lineage binding
		if strings.Contains(cs.ReviewOffer.Invocation, "lineage\":\"") || strings.Contains(cs.ReviewOffer.Invocation, "receipt") {
			t.Fatalf("invocation must not embed persisted lineage/receipt: %q", cs.ReviewOffer.Invocation)
		}
	})

	t.Run("verify_failing_emits_nil", func(t *testing.T) {
		ws := t.TempDir()
		changeRoot := seedDeriveChange(t, ws, "offer-fail", map[string]string{
			"proposal.md":        "# Proposal\n",
			"specs/core/spec.md": specFixture,
			"design.md":          "# Design\n",
			"tasks.md":           "- [x] T1\n",
			"verify-report.md":   failingReport("1/1", "1/1"),
		})
		cs, err := readChange(changeRoot, "offer-fail", false, ws, false)
		if err != nil {
			t.Fatalf("readChange: %v", err)
		}
		if cs.ReviewOffer != nil {
			t.Fatalf("expected nil ReviewOffer for failing verify, got %+v", cs.ReviewOffer)
		}
	})

	t.Run("missing_verify_emits_nil", func(t *testing.T) {
		ws := t.TempDir()
		changeRoot := seedDeriveChange(t, ws, "offer-missing", map[string]string{
			"proposal.md":        "# Proposal\n",
			"specs/core/spec.md": specFixture,
			"design.md":          "# Design\n",
			"tasks.md":           "- [x] T1\n",
		})
		cs, err := readChange(changeRoot, "offer-missing", false, ws, false)
		if err != nil {
			t.Fatalf("readChange: %v", err)
		}
		if cs.ReviewOffer != nil {
			t.Fatalf("expected nil when verify missing, got %+v", cs.ReviewOffer)
		}
	})

	t.Run("blockers_nonzero_emits_nil", func(t *testing.T) {
		ws := t.TempDir()
		// passingReport with blockers via direct string
		blockReport := "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nblockers: 1\ncritical_findings: 0\nrequirements: 1/1\nscenarios: 1/1\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n"
		changeRoot := seedDeriveChange(t, ws, "offer-blockers", map[string]string{
			"proposal.md":        "# Proposal\n",
			"specs/core/spec.md": specFixture,
			"design.md":          "# Design\n",
			"tasks.md":           "- [x] T1\n",
			"verify-report.md":   blockReport,
		})
		cs, err := readChange(changeRoot, "offer-blockers", false, ws, false)
		if err != nil {
			t.Fatalf("readChange: %v", err)
		}
		if cs.ReviewOffer != nil {
			t.Fatalf("expected nil when blockers>0, got %+v", cs.ReviewOffer)
		}
	})
}

func TestReviewOfferQuoting(t *testing.T) {
	ws := t.TempDir()
	// Change name with space – file system allows space on temp dir
	change := "my change"
	changeRoot := seedDeriveChange(t, ws, change, map[string]string{
		"proposal.md":        "# Proposal\n",
		"specs/core/spec.md": specFixture,
		"design.md":          "# Design\n",
		"tasks.md":           "- [x] T1\n",
		"verify-report.md":   passingReport("1/1", "1/1"),
	})
	cs, err := readChange(changeRoot, change, false, ws, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if cs.ReviewOffer == nil {
		t.Fatalf("expected offer for my change")
	}
	// Must contain pathquote.Quote style: "<change>-<sha>" quoted
	expectedPrefix := "\"my change-"
	if !strings.Contains(cs.ReviewOffer.Invocation, expectedPrefix) {
		t.Fatalf("quoting failed, invocation %q does not contain %q", cs.ReviewOffer.Invocation, expectedPrefix)
	}
	// Ensure no persisted lineage leakage
	if strings.Contains(cs.ReviewOffer.Invocation, "binding") {
		t.Fatalf("invocation leaks binding: %q", cs.ReviewOffer.Invocation)
	}
}

func TestReviewOfferDisabledEmitsNil(t *testing.T) {
	// Simulate RDD disabled by setting global disabled via file in temp HOME
	home := t.TempDir()
	// Override HOME for review status – review reads UserHomeDir
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	_ = os.Setenv("HOME", home)
	_ = os.Setenv("USERPROFILE", home)
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("USERPROFILE", oldUserProfile)
	}()
	// Write disabled global
	if _, err := review.RDDStatus("", ""); err == nil {
		// Use review package to disable
		_, _ = review.RDDDisable("", "", "global")
	}
	ws := t.TempDir()
	changeRoot := seedDeriveChange(t, ws, "offer-disabled", map[string]string{
		"proposal.md":        "# Proposal\n",
		"specs/core/spec.md": specFixture,
		"design.md":          "# Design\n",
		"tasks.md":           "- [x] T1\n",
		"verify-report.md":   passingReport("1/1", "1/1"),
	})
	cs, err := readChange(changeRoot, "offer-disabled", false, ws, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if cs.ReviewOffer != nil {
		t.Fatalf("expected nil when RDD disabled, got %+v", cs.ReviewOffer)
	}
	// Re-enable for other tests
	_, _ = review.RDDEnable("", "")
}
