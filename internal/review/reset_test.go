package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReset_MissingLineageErrors(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	if _, err := Reset(repo, "no-such-lineage", "bench"); err == nil ||
		!strings.Contains(err.Error(), "no such lineage") {
		t.Fatalf("expected no-such-lineage error, got %v", err)
	}
}

func TestReset_EmptyReasonRefuses(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	if _, err := Reset(repo, "any", ""); err == nil ||
		!strings.Contains(err.Error(), "--reason is required") {
		t.Fatalf("expected reason-required error, got %v", err)
	}
}

func TestReset_MovesLineageToTrashWithRecord(t *testing.T) {
	repo, store, _ := appendTestRecords(t, "reset-me", 3)
	dir := store.Dir

	report, err := Reset(repo, "reset-me", "bench corruption drill")
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("lineage dir must be gone, stat err = %v", err)
	}
	if report.TrashedTo == "" {
		t.Fatalf("report must carry trash destination")
	}
	data, err := os.ReadFile(filepath.Join(report.TrashedTo, "reset-record.json"))
	if err != nil {
		t.Fatalf("read reset record: %v", err)
	}
	var rec ResetRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("decode reset record: %v", err)
	}
	if rec.LineageID != "reset-me" || rec.Reason != "bench corruption drill" {
		t.Fatalf("record mismatch: %+v", rec)
	}
	// Events moved along (nothing deleted).
	entries, err := os.ReadDir(report.TrashedTo)
	if err != nil || len(entries) < 2 {
		t.Fatalf("trash must hold events plus record, got %v %v", entries, err)
	}
}
