package review

import (
	"fmt"
	"time"
)

// JudgmentDay manages an adversarial review with two independent judges.
// Two judges review the same candidate blindly, their findings are compared,
// and a refuter resolves any disagreements. Fix rounds are limited.
type JudgmentDay struct {
	JudgeA     *JudgeVerdict
	JudgeB     *JudgeVerdict
	Refuter    *RefuterVerdict
	FixRounds  int
	MaxRounds  int
	Status     string // pending, judging, refuting, fixing, completed, failed
	CreatedAt  time.Time
	CompletedAt time.Time
}

// JudgeVerdict is the result from one blind judge.
type JudgeVerdict struct {
	JudgeID  string    `json:"judge_id"`
	Findings []Finding `json:"findings"`
	Status   string    `json:"status"` // clean, has_findings, error
	Error    string    `json:"error,omitempty"`
}

// RefuterVerdict resolves disagreements between judges.
type RefuterVerdict struct {
	Corroborated []Finding `json:"corroborated"` // both judges agreed
	Refuted      []string  `json:"refuted"`       // finding IDs that were refuted
	Inconclusive []string  `json:"inconclusive"`  // could not determine
}

// NewJudgmentDay creates a new judgment day review.
func NewJudgmentDay(maxRounds int) *JudgmentDay {
	if maxRounds < 1 {
		maxRounds = 2 // default max fix rounds
	}
	return &JudgmentDay{
		MaxRounds: maxRounds,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
}

// RecordJudgeVerdict records one judge's findings.
func (jd *JudgmentDay) RecordJudgeVerdict(judgeID string, findings []Finding) error {
	if jd.Status != "pending" && jd.Status != "judging" {
		return fmt.Errorf("cannot record judge verdict in status %q", jd.Status)
	}
	jd.Status = "judging"

	verdict := &JudgeVerdict{
		JudgeID:  judgeID,
		Findings: findings,
		Status:   "has_findings",
	}
	if len(findings) == 0 {
		verdict.Status = "clean"
	}

	if jd.JudgeA == nil {
		jd.JudgeA = verdict
	} else if jd.JudgeB == nil {
		jd.JudgeB = verdict
	} else {
		return fmt.Errorf("both judges have already recorded verdicts")
	}
	return nil
}

// Resolve compares both judges' findings and produces a refuter verdict.
// Must be called after both judges have recorded.
func (jd *JudgmentDay) Resolve() (*RefuterVerdict, error) {
	if jd.JudgeA == nil || jd.JudgeB == nil {
		return nil, fmt.Errorf("both judges must record before resolving")
	}
	jd.Status = "refuting"

	// Find corroborated findings (match by ID or content)
	corroborated := jd.findCorroborated()
	refuted := jd.findRefuted()
	inconclusive := jd.findInconclusive()

	jd.Refuter = &RefuterVerdict{
		Corroborated: corroborated,
		Refuted:      refuted,
		Inconclusive: inconclusive,
	}

	// If corroborated findings exist and rounds remain, status = fixing
	if len(corroborated) > 0 && jd.FixRounds < jd.MaxRounds {
		jd.Status = "fixing"
	} else if len(corroborated) > 0 {
		jd.Status = "failed" // max rounds reached with issues
	} else {
		jd.Status = "completed" // no issues found
	}

	jd.CompletedAt = time.Now()
	return jd.Refuter, nil
}

// ApplyFixRound increments the fix counter and resets for re-review.
func (jd *JudgmentDay) ApplyFixRound() error {
	if jd.Status != "fixing" {
		return fmt.Errorf("can only apply fix rounds in 'fixing' status, got %q", jd.Status)
	}
	jd.FixRounds++
	jd.JudgeA = nil
	jd.JudgeB = nil
	jd.Refuter = nil
	jd.Status = "pending"
	return nil
}

func (jd *JudgmentDay) findCorroborated() []Finding {
	a := jd.JudgeA.Findings
	b := jd.JudgeB.Findings

	// Build a map of judge B's findings by ID
	bMap := make(map[string]Finding)
	for _, f := range b {
		bMap[f.ID] = f
	}

	var corroborated []Finding
	for _, fa := range a {
		if fb, ok := bMap[fa.ID]; ok {
			// Both judges found the same issue
			if fb.Severity == fa.Severity {
				corroborated = append(corroborated, fa)
			}
		}
	}
	return corroborated
}

func (jd *JudgmentDay) findRefuted() []string {
	a := jd.JudgeA.Findings
	b := jd.JudgeB.Findings

	aMap := make(map[string]bool)
	for _, f := range a {
		aMap[f.ID] = true
	}

	var refuted []string
	for _, fb := range b {
		if !aMap[fb.ID] {
			// judge A didn't find this — refuted
			refuted = append(refuted, fb.ID)
		}
	}
	return refuted
}

func (jd *JudgmentDay) findInconclusive() []string {
	a := jd.JudgeA.Findings
	b := jd.JudgeB.Findings

	bMap := make(map[string]Finding)
	for _, f := range b {
		bMap[f.ID] = f
	}

	var inconclusive []string
	for _, fa := range a {
		if fb, ok := bMap[fa.ID]; ok {
			if fb.Severity != fa.Severity {
				inconclusive = append(inconclusive, fa.ID)
			}
		}
	}
	return inconclusive
}
