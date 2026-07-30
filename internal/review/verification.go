package review

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// VerificationDomain identifies the scope of a verification contract.
type VerificationDomain string

const (
	DomainNativeLowRisk            VerificationDomain = "biggz.native-low-risk-verification/v1"
	DomainFunctionalProof          VerificationDomain = "biggz.functional-proof/v1"
	DomainAdversarialReview        VerificationDomain = "biggz.adversarial-review/v1"
	DomainStructuralReadback       VerificationDomain = "biggz.structural-readback/v1"
)

// VerificationContract defines what must be verified for a review to pass.
type VerificationContract struct {
	ID             string             `json:"id"`
	LineageID      string             `json:"lineage_id"`
	Domain         VerificationDomain `json:"domain"`
	Requirements   []string           `json:"requirements"`
	MaxRetries     int                `json:"max_retries"`
	RetryDelay     time.Duration      `json:"retry_delay"`
	StrictMode     bool               `json:"strict_mode"`
	EvidenceHashes []string           `json:"evidence_hashes,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
}

// VerificationResult records the outcome of a verification attempt.
type VerificationResult struct {
	ContractID    string    `json:"contract_id"`
	Attempt       int       `json:"attempt"`
	Passed        bool      `json:"passed"`
	Evidence      string    `json:"evidence,omitempty"`
	Findings      []string  `json:"findings,omitempty"`
	Retryable     bool      `json:"retryable"`
	ExecutedAt    time.Time `json:"executed_at"`
	Duration      string    `json:"duration,omitempty"`
}

// VerificationEngine runs verification contracts with retry logic.
type VerificationEngine struct {
	contracts []VerificationContract
	results   []VerificationResult
	maxRetry  int
}

// NewVerificationEngine creates a verification engine.
func NewVerificationEngine(maxRetry int) *VerificationEngine {
	return &VerificationEngine{
		contracts: make([]VerificationContract, 0),
		results:   make([]VerificationResult, 0),
		maxRetry:  maxRetry,
	}
}

// RegisterContract adds a verification contract.
func (ve *VerificationEngine) RegisterContract(c VerificationContract) {
	ve.contracts = append(ve.contracts, c)
}

// Execute runs a contract with retry. The verifyFn is called for each attempt;
// it should return (true, nil) on pass, (false, nil) on fail, or (false, err)
// on error (which triggers retry).
func (ve *VerificationEngine) Execute(contractID string, verifyFn func() (bool, string, []string, error)) (*VerificationResult, error) {
	contract := ve.findContract(contractID)
	if contract == nil {
		return nil, fmt.Errorf("contract %s not found", contractID)
	}

	maxAttempts := contract.MaxRetries + 1
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var lastResult *VerificationResult
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		start := time.Now()
		passed, evidence, findings, err := verifyFn()
		duration := time.Since(start)

		result := &VerificationResult{
			ContractID: contractID,
			Attempt:    attempt,
			Passed:     passed,
			Evidence:   evidence,
			Findings:   findings,
			Retryable:  err != nil && attempt < maxAttempts,
			ExecutedAt: time.Now().UTC(),
			Duration:   duration.Round(time.Millisecond).String(),
		}

		// If strict mode, only retry on error, not on failure
		if contract.StrictMode && !passed && err == nil {
			result.Retryable = false
		}

		ve.results = append(ve.results, *result)
		lastResult = result

		if passed {
			break
		}
		if !result.Retryable {
			break
		}
	}

	return lastResult, nil
}

func (ve *VerificationEngine) findContract(id string) *VerificationContract {
	for i := range ve.contracts {
		if ve.contracts[i].ID == id {
			return &ve.contracts[i]
		}
	}
	return nil
}

// Results returns all verification results.
func (ve *VerificationEngine) Results() []VerificationResult {
	return ve.results
}

// ContractResults returns verification results for a specific contract.
func (ve *VerificationEngine) ContractResults(contractID string) []VerificationResult {
	var filtered []VerificationResult
	for _, r := range ve.results {
		if r.ContractID == contractID {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// AllPassed returns true if all contracts have at least one passing result.
func (ve *VerificationEngine) AllPassed() bool {
	passed := make(map[string]bool)
	for _, r := range ve.results {
		if r.Passed {
			passed[r.ContractID] = true
		}
	}
	for _, c := range ve.contracts {
		if !passed[c.ID] {
			return false
		}
	}
	return true
}

// StandardVerifier returns a verifyFn for standard contract checks.
// It checks that all requirements produce evidence entries.
type StandardVerifier struct {
	Requirements []string
	HaveEvidence func(req string) bool
}

// Verify executes standard verification.
func (sv *StandardVerifier) Verify(contractID string, engine *VerificationEngine) (*VerificationResult, error) {
	return engine.Execute(contractID, func() (bool, string, []string, error) {
		var missing []string
		for _, req := range sv.Requirements {
			if !sv.HaveEvidence(req) {
				missing = append(missing, req)
			}
		}
		if len(missing) > 0 {
			return false, fmt.Sprintf("missing evidence for: %s", strings.Join(missing, ", ")), missing, nil
		}
		return true, "all requirements verified", nil, nil
	})
}

// VerifyContractFile parses a contract from JSON bytes.
func VerifyContractFile(data []byte) (*VerificationContract, error) {
	var c VerificationContract
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse contract: %w", err)
	}
	if c.ID == "" {
		return nil, fmt.Errorf("contract missing ID")
	}
	if c.Domain == "" {
		return nil, fmt.Errorf("contract missing domain")
	}
	return &c, nil
}
