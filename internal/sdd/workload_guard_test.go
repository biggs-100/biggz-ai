package sdd

import (
	"testing"
)

func TestReviewWorkloadForecast_NeedsGuard(t *testing.T) {
	tests := []struct {
		name     string
		forecast ReviewWorkloadForecast
		expected bool
	}{
		{
			name:     "no guard conditions",
			forecast: ReviewWorkloadForecast{EstimatedLines: 100},
			expected: false,
		},
		{
			name:     "chained PRs recommended",
			forecast: ReviewWorkloadForecast{ChainedPRsRecommended: true, EstimatedLines: 100},
			expected: true,
		},
		{
			name:     "budget risk high",
			forecast: ReviewWorkloadForecast{BudgetRiskHigh: true, EstimatedLines: 100},
			expected: true,
		},
		{
			name:     "estimated lines exceed 400",
			forecast: ReviewWorkloadForecast{EstimatedLines: 500},
			expected: true,
		},
		{
			name:     "decision needed",
			forecast: ReviewWorkloadForecast{DecisionNeeded: true, EstimatedLines: 100},
			expected: true,
		},
		{
			name:     "multiple conditions",
			forecast: ReviewWorkloadForecast{ChainedPRsRecommended: true, BudgetRiskHigh: true, EstimatedLines: 600},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.forecast.NeedsGuard(); got != tt.expected {
				t.Errorf("NeedsGuard() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWorkloadGuard_NoGuardNeeded(t *testing.T) {
	forecast := &ReviewWorkloadForecast{EstimatedLines: 100}
	result := WorkloadGuard(forecast, AskOnRisk, "", nil)
	if result.Action != GuardAllow {
		t.Errorf("expected ALLOW, got %v", result.Action)
	}
}

func TestWorkloadGuard_AskOnRisk(t *testing.T) {
	forecast := &ReviewWorkloadForecast{EstimatedLines: 500, ChainedPRsRecommended: true}
	result := WorkloadGuard(forecast, AskOnRisk, "", nil)
	if result.Action != GuardAsk {
		t.Errorf("expected ASK, got %v", result.Action)
	}
	if result.Reason == "" {
		t.Error("expected reason")
	}
}

func TestWorkloadGuard_AutoChain_WithStrategy(t *testing.T) {
	forecast := &ReviewWorkloadForecast{EstimatedLines: 500, ChainedPRsRecommended: true}
	result := WorkloadGuard(forecast, AutoChain, StackedToMain, nil)
	if result.Action != GuardAllow {
		t.Errorf("expected ALLOW, got %v", result.Action)
	}
	if result.ChainStrategy != StackedToMain {
		t.Errorf("expected StackedToMain, got %v", result.ChainStrategy)
	}
}

func TestWorkloadGuard_AutoChain_WithoutStrategy(t *testing.T) {
	forecast := &ReviewWorkloadForecast{EstimatedLines: 500, ChainedPRsRecommended: true}
	result := WorkloadGuard(forecast, AutoChain, "", nil)
	if result.Action != GuardAsk {
		t.Errorf("expected ASK (needs chain strategy), got %v", result.Action)
	}
}

func TestWorkloadGuard_SinglePR(t *testing.T) {
	forecast := &ReviewWorkloadForecast{EstimatedLines: 500, ChainedPRsRecommended: true}
	result := WorkloadGuard(forecast, SinglePR, "", nil)
	if result.Action != GuardBlock {
		t.Errorf("expected BLOCK, got %v", result.Action)
	}
}

func TestWorkloadGuard_ExceptionOK(t *testing.T) {
	forecast := &ReviewWorkloadForecast{EstimatedLines: 500, ChainedPRsRecommended: true}
	result := WorkloadGuard(forecast, ExceptionOK, "", nil)
	if result.Action != GuardAllow {
		t.Errorf("expected ALLOW, got %v", result.Action)
	}
	if result.Exception == nil || !result.Exception.Approved {
		t.Error("expected exception to be approved")
	}
}

func TestWorkloadGuard_ExistingException(t *testing.T) {
	forecast := &ReviewWorkloadForecast{EstimatedLines: 500, ChainedPRsRecommended: true}
	exception := &SizeException{Approved: true, Reason: "previously approved"}
	result := WorkloadGuard(forecast, SinglePR, "", exception)
	if result.Action != GuardAllow {
		t.Errorf("expected ALLOW (exception exists), got %v", result.Action)
	}
}

func TestWorkloadGuard_InvalidStrategy(t *testing.T) {
	forecast := &ReviewWorkloadForecast{EstimatedLines: 500, ChainedPRsRecommended: true}
	result := WorkloadGuard(forecast, "invalid", "", nil)
	if result.Action != GuardBlock {
		t.Errorf("expected BLOCK for invalid strategy, got %v", result.Action)
	}
}

func TestParseForecast(t *testing.T) {
	json := `{"chained_prs_recommended":true,"budget_risk_high":false,"estimated_lines":450,"decision_needed":true}`
	forecast, err := ParseForecast(json)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !forecast.ChainedPRsRecommended {
		t.Error("expected ChainedPRsRecommended to be true")
	}
	if forecast.EstimatedLines != 450 {
		t.Errorf("expected 450 lines, got %d", forecast.EstimatedLines)
	}
}

func TestParseForecast_Invalid(t *testing.T) {
	_, err := ParseForecast("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidateDeliveryStrategy(t *testing.T) {
	tests := []struct {
		strategy string
		valid    bool
	}{
		{"ask-on-risk", true},
		{"auto-chain", true},
		{"single-pr", true},
		{"exception-ok", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			if got := ValidateDeliveryStrategy(tt.strategy); got != tt.valid {
				t.Errorf("ValidateDeliveryStrategy(%q) = %v, want %v", tt.strategy, got, tt.valid)
			}
		})
	}
}

func TestValidateChainStrategy(t *testing.T) {
	tests := []struct {
		strategy string
		valid    bool
	}{
		{"stacked-to-main", true},
		{"feature-branch-chain", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			if got := ValidateChainStrategy(tt.strategy); got != tt.valid {
				t.Errorf("ValidateChainStrategy(%q) = %v, want %v", tt.strategy, got, tt.valid)
			}
		})
	}
}

func TestWorkloadGuardSummary(t *testing.T) {
	tests := []struct {
		name   string
		result *WorkloadGuardResult
		contains string
	}{
		{
			name: "allow",
			result: &WorkloadGuardResult{Action: GuardAllow, Reason: "test reason"},
			contains: "ALLOW",
		},
		{
			name: "ask",
			result: &WorkloadGuardResult{Action: GuardAsk, Reason: "test reason"},
			contains: "ASK",
		},
		{
			name: "block",
			result: &WorkloadGuardResult{Action: GuardBlock, Reason: "test reason"},
			contains: "BLOCK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := WorkloadGuardSummary(tt.result)
			if summary == "" {
				t.Error("expected non-empty summary")
			}
		})
	}
}
