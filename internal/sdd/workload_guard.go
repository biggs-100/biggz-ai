// Package sdd — Review Workload Guard.
//
// After sdd-tasks completes and before launching sdd-apply, this guard
// inspects the Review Workload Forecast and applies the cached delivery
// strategy. It prevents oversized changes from being applied without
// explicit approval.
//
// The guard evaluates:
//   - Whether chained PRs are recommended
//   - Whether the 400-line budget risk is high
//   - Whether estimated changed lines exceed 400
//   - Whether a decision is needed before apply
//
// Based on the delivery strategy, it returns:
//   - ALLOW: proceed with apply
//   - ASK: stop and ask user for decision
//   - BLOCK: stop and require explicit exception
package sdd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DeliveryStrategy determines how the workload guard handles oversized changes.
type DeliveryStrategy string

const (
	// AskOnRisk stops and asks the user whether to split into chained PRs.
	AskOnRisk DeliveryStrategy = "ask-on-risk"
	// AutoChain automatically splits without asking (requires chain_strategy).
	AutoChain DeliveryStrategy = "auto-chain"
	// SinglePR requires explicit size:exception approval.
	SinglePR DeliveryStrategy = "single-pr"
	// ExceptionOK continues with size:exception recorded.
	ExceptionOK DeliveryStrategy = "exception-ok"
)

// ChainStrategy determines how chained PRs are structured.
type ChainStrategy string

const (
	// StackedToMain each PR merges to main in order.
	StackedToMain ChainStrategy = "stacked-to-main"
	// FeatureBranchChain PRs target the tracker branch, only tracker merges to main.
	FeatureBranchChain ChainStrategy = "feature-branch-chain"
)

// GuardAction is the result of the workload guard evaluation.
type GuardAction string

const (
	// GuardAllow means proceed with apply.
	GuardAllow GuardAction = "allow"
	// GuardAsk means stop and ask user for decision.
	GuardAsk GuardAction = "ask"
	// GuardBlock means stop and require explicit exception.
	GuardBlock GuardAction = "block"
)

// ReviewWorkloadForecast contains the workload analysis from sdd-tasks.
type ReviewWorkloadForecast struct {
	// ChainedPRsRecommended indicates the task forecast recommends chained PRs.
	ChainedPRsRecommended bool `json:"chained_prs_recommended"`
	// BudgetRiskHigh indicates the 400-line budget risk is high.
	BudgetRiskHigh bool `json:"budget_risk_high"`
	// EstimatedLines is the estimated total changed lines.
	EstimatedLines int `json:"estimated_lines"`
	// DecisionNeeded indicates a decision is needed before apply.
	DecisionNeeded bool `json:"decision_needed"`
	// TaskCount is the total number of tasks.
	TaskCount int `json:"task_count"`
	// CompletedTasks is the number of completed tasks.
	CompletedTasks int `json:"completed_tasks"`
}

// WorkloadGuardResult is the outcome of the workload guard evaluation.
type WorkloadGuardResult struct {
	Action     GuardAction     `json:"action"`
	Reason     string          `json:"reason,omitempty"`
	Strategy   DeliveryStrategy `json:"strategy"`
	ChainStrategy ChainStrategy `json:"chain_strategy,omitempty"`
	Forecast   ReviewWorkloadForecast `json:"forecast"`
	// Exception records the size exception if approved.
	Exception *SizeException `json:"exception,omitempty"`
}

// SizeException records that a size exception was approved.
type SizeException struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
	ApprovedBy string `json:"approved_by,omitempty"`
}

// NeedsGuard reports whether the workload forecast triggers the guard.
// The guard is triggered when any of these conditions are true:
//   - ChainedPRsRecommended is true
//   - BudgetRiskHigh is true
//   - EstimatedLines > 400
//   - DecisionNeeded is true
func (f *ReviewWorkloadForecast) NeedsGuard() bool {
	return f.ChainedPRsRecommended ||
		f.BudgetRiskHigh ||
		f.EstimatedLines > 400 ||
		f.DecisionNeeded
}

// WorkloadGuard evaluates the review workload forecast against the delivery
// strategy and returns the appropriate action.
//
// Parameters:
//   - forecast: the workload analysis from sdd-tasks
//   - strategy: the cached delivery strategy
//   - chainStrategy: the cached chain strategy (required for auto-chain)
//   - exception: existing size exception (if any)
//
// Returns a WorkloadGuardResult with the action to take.
func WorkloadGuard(forecast *ReviewWorkloadForecast, strategy DeliveryStrategy, chainStrategy ChainStrategy, exception *SizeException) *WorkloadGuardResult {
	result := &WorkloadGuardResult{
		Strategy:      strategy,
		ChainStrategy: chainStrategy,
		Forecast:      *forecast,
	}

	// If no guard conditions, always allow
	if !forecast.NeedsGuard() {
		result.Action = GuardAllow
		return result
	}

	// Check for existing exception
	if exception != nil && exception.Approved {
		result.Action = GuardAllow
		result.Exception = exception
		result.Reason = "size exception approved"
		return result
	}

	// Apply delivery strategy
	switch strategy {
	case AskOnRisk:
		result.Action = GuardAsk
		result.Reason = buildAskReason(forecast)
		
	case AutoChain:
		if chainStrategy == "" {
			// Need chain strategy
			result.Action = GuardAsk
			result.Reason = "auto-chain requires chain_strategy (stacked-to-main or feature-branch-chain)"
		} else {
			result.Action = GuardAllow
			result.Reason = fmt.Sprintf("auto-chain with %s strategy", chainStrategy)
		}
		
	case SinglePR:
		result.Action = GuardBlock
		result.Reason = "single-pr requires size:exception for changes exceeding 400 lines"
		
	case ExceptionOK:
		result.Action = GuardAllow
		result.Exception = &SizeException{Approved: true, Reason: "exception-ok strategy"}
		result.Reason = "exception-ok: size exception recorded"
		
	default:
		result.Action = GuardBlock
		result.Reason = fmt.Sprintf("invalid delivery_strategy: %q", strategy)
	}

	return result
}

// buildAskReason creates a human-readable reason for the ASK action.
func buildAskReason(f *ReviewWorkloadForecast) string {
	var reasons []string
	if f.ChainedPRsRecommended {
		reasons = append(reasons, "chained PRs recommended")
	}
	if f.BudgetRiskHigh {
		reasons = append(reasons, "400-line budget risk high")
	}
	if f.EstimatedLines > 400 {
		reasons = append(reasons, fmt.Sprintf("estimated %d lines (exceeds 400)", f.EstimatedLines))
	}
	if f.DecisionNeeded {
		reasons = append(reasons, "decision needed before apply")
	}
	return "workload guard triggered: " + strings.Join(reasons, "; ")
}

// ParseForecast attempts to parse a JSON string into a ReviewWorkloadForecast.
func ParseForecast(jsonStr string) (*ReviewWorkloadForecast, error) {
	var forecast ReviewWorkloadForecast
	if err := json.Unmarshal([]byte(jsonStr), &forecast); err != nil {
		return nil, fmt.Errorf("parse forecast: %w", err)
	}
	return &forecast, nil
}

// ForecastFromJSON is a convenience that parses JSON and runs the guard.
func ForecastFromJSON(forecastJSON string, strategy DeliveryStrategy, chainStrategy ChainStrategy) (*WorkloadGuardResult, error) {
	forecast, err := ParseForecast(forecastJSON)
	if err != nil {
		return nil, err
	}
	return WorkloadGuard(forecast, strategy, chainStrategy, nil), nil
}

// LoadForecastFromFile loads a forecast from a JSON file.
func LoadForecastFromFile(path string) (*ReviewWorkloadForecast, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read forecast file: %w", err)
	}
	return ParseForecast(string(data))
}

// SaveForecastToFile saves a forecast to a JSON file.
func SaveForecastToFile(forecast *ReviewWorkloadForecast, path string) error {
	data, err := json.MarshalIndent(forecast, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal forecast: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// WorkloadGuardSummary returns a one-line summary for the orchestrator.
func WorkloadGuardSummary(result *WorkloadGuardResult) string {
	switch result.Action {
	case GuardAllow:
		return fmt.Sprintf("◆ workload guard · ALLOW (%s)", result.Reason)
	case GuardAsk:
		return fmt.Sprintf("◆ workload guard · ASK (%s)", result.Reason)
	case GuardBlock:
		return fmt.Sprintf("◆ workload guard · BLOCK (%s)", result.Reason)
	default:
		return "◆ workload guard · UNKNOWN"
	}
}

// ValidateDeliveryStrategy checks if a strategy string is valid.
func ValidateDeliveryStrategy(s string) bool {
	switch DeliveryStrategy(s) {
	case AskOnRisk, AutoChain, SinglePR, ExceptionOK:
		return true
	default:
		return false
	}
}

// ValidateChainStrategy checks if a chain strategy string is valid.
func ValidateChainStrategy(s string) bool {
	switch ChainStrategy(s) {
	case StackedToMain, FeatureBranchChain:
		return true
	default:
		return false
	}
}
