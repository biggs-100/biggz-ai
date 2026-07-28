package review

import "time"

// Correction represents a single fix applied during a review cycle.
// This is intentionally minimal — budget, retry limits, and policy rules
// are evaluated externally by PolicyEvaluator, not embedded in the model.
type Correction struct {
	ID           string    `json:"id"`
	Files        []string  `json:"files"`
	LinesChanged int       `json:"lines_changed"`
	Reason       string    `json:"reason"`
	BeforeHash   string    `json:"before_hash"`  // SHA of evidence chain before correction
	AfterHash    string    `json:"after_hash"`   // SHA of evidence chain after correction
	CreatedAt    time.Time `json:"created_at"`
}
