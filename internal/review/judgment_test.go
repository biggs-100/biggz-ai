package review

import (
	"testing"
)

func TestJudgmentDay_Clean(t *testing.T) {
	jd := NewJudgmentDay(2)

	jd.RecordJudgeVerdict("judge-a", nil)
	jd.RecordJudgeVerdict("judge-b", nil)

	res, err := jd.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if len(res.Corroborated) != 0 {
		t.Errorf("expected 0 corroborated, got %d", len(res.Corroborated))
	}
	if jd.Status != "completed" {
		t.Errorf("expected completed, got %s", jd.Status)
	}
}

func TestJudgmentDay_Corroborated(t *testing.T) {
	jd := NewJudgmentDay(2)

	finding1 := Finding{ID: "F1", Severity: SeverityCritical, Message: "Buffer overflow", File: "main.go", Line: 42}
	finding2 := Finding{ID: "F2", Severity: SeverityWarning, Message: "Unused var", File: "util.go", Line: 10}

	jd.RecordJudgeVerdict("judge-a", []Finding{finding1, finding2})
	jd.RecordJudgeVerdict("judge-b", []Finding{finding1})

	res, err := jd.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if len(res.Corroborated) != 1 {
		t.Errorf("expected 1 corroborated, got %d", len(res.Corroborated))
	}
	if len(res.Refuted) != 0 {
		t.Errorf("expected 0 refuted, got %d", len(res.Refuted))
	}
}

func TestJudgmentDay_FixAndRejudge(t *testing.T) {
	jd := NewJudgmentDay(2)

	finding := Finding{ID: "F1", Severity: SeverityCritical, Message: "Bug", File: "main.go", Line: 1}
	jd.RecordJudgeVerdict("judge-a", []Finding{finding})
	jd.RecordJudgeVerdict("judge-b", []Finding{finding})

	_, err := jd.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if jd.Status != "fixing" {
		t.Errorf("expected fixing, got %s", jd.Status)
	}

	// Apply fix round
	if err := jd.ApplyFixRound(); err != nil {
		t.Fatalf("ApplyFixRound() error: %v", err)
	}
	if jd.Status != "pending" {
		t.Errorf("expected pending after fix round, got %s", jd.Status)
	}
	if jd.FixRounds != 1 {
		t.Errorf("expected FixRounds=1, got %d", jd.FixRounds)
	}
}

func TestJudgmentDay_MaxRounds(t *testing.T) {
	jd := NewJudgmentDay(1) // only 1 round

	finding := Finding{ID: "F1", Severity: SeverityCritical, Message: "Bug", File: "main.go", Line: 1}
	jd.RecordJudgeVerdict("judge-a", []Finding{finding})
	jd.RecordJudgeVerdict("judge-b", []Finding{finding})

	_, err := jd.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	// With 0 rounds done and max 1, should still be fixing
	// After apply, it will reach max
	if jd.Status != "fixing" {
		t.Errorf("expected fixing, got %s", jd.Status)
	}

	jd.ApplyFixRound()
	jd.RecordJudgeVerdict("judge-a", []Finding{finding})
	jd.RecordJudgeVerdict("judge-b", []Finding{finding})
	_, err = jd.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	// Now FixRounds = 1 >= MaxRounds = 1, should be failed
	if jd.Status != "failed" {
		t.Errorf("expected failed after max rounds, got %s", jd.Status)
	}
}

func TestJudgmentDay_Errors(t *testing.T) {
	jd := NewJudgmentDay(2)

	// Can't resolve before both judges
	jd.RecordJudgeVerdict("judge-a", nil)
	_, err := jd.Resolve()
	if err == nil {
		t.Fatal("expected error resolving without judge B")
	}
}
