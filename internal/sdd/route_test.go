package sdd

import (
	"slices"
	"testing"
)

func TestEvaluateRoute(t *testing.T) {
	cases := []struct {
		name         string
		in           RouteInput
		wantRoute    RouteDecision
		wantTriggers []string
	}{
		{name: "empty defaults direct", in: RouteInput{}, wantRoute: RouteDirect},
		{
			name:      "one-file mechanical direct",
			in:        RouteInput{EstimatedLines: 10, EstimatedFiles: 1, AcceptanceClear: true, SingleDomain: true, VerificationPlanned: true},
			wantRoute: RouteDirect,
		},
		{
			name:      "quiet-tools precedent carves out scale",
			in:        RouteInput{EstimatedLines: 200, EstimatedFiles: 7, NontrivialFiles: 1, AcceptanceClear: true, SingleDomain: true, VerificationPlanned: true},
			wantRoute: RouteDirect,
		},
		{
			name:      "large but clear stays direct",
			in:        RouteInput{EstimatedLines: 500, EstimatedFiles: 8, NontrivialFiles: 3, AcceptanceClear: true, SingleDomain: true, VerificationPlanned: true},
			wantRoute: RouteDirect,
		},
		{
			name:         "large unclear asks with scale and ambiguity",
			in:           RouteInput{EstimatedLines: 500},
			wantRoute:    RouteAskSDD,
			wantTriggers: []string{"scale-lines", "unclear-acceptance"},
		},
		{
			name:         "five files unclear asks scale-files",
			in:           RouteInput{EstimatedFiles: 5},
			wantRoute:    RouteAskSDD,
			wantTriggers: []string{"scale-files", "unclear-acceptance"},
		},
		{
			name:         "two nontrivial unclear asks",
			in:           RouteInput{NontrivialFiles: 2},
			wantRoute:    RouteAskSDD,
			wantTriggers: []string{"scale-nontrivial"},
		},
		{
			name:      "tiny unclear stays direct",
			in:        RouteInput{EstimatedLines: 20, EstimatedFiles: 1},
			wantRoute: RouteDirect,
		},
		{
			name:         "substantial unclear asks",
			in:           RouteInput{EstimatedLines: 150, EstimatedFiles: 1},
			wantRoute:    RouteAskSDD,
			wantTriggers: []string{"unclear-acceptance"},
		},
		{
			name:         "new domain asks even when clear",
			in:           RouteInput{EstimatedLines: 10, EstimatedFiles: 1, AcceptanceClear: true, SingleDomain: true, VerificationPlanned: true, NewDomain: true},
			wantRoute:    RouteAskSDD,
			wantTriggers: []string{"new-domain"},
		},
		{
			name:         "cross-cutting beats carve-out",
			in:           RouteInput{EstimatedLines: 50, EstimatedFiles: 3, AcceptanceClear: true, SingleDomain: true, VerificationPlanned: true, CrossCutting: true},
			wantRoute:    RouteAskSDD,
			wantTriggers: []string{"cross-cutting"},
		},
		{
			name:         "multiple hard triggers all named",
			in:           RouteInput{NewPublicInterface: true, TouchesGolden: true, CriticalPackages: true},
			wantRoute:    RouteAskSDD,
			wantTriggers: []string{"new-public-interface", "golden-tests", "critical-packages"},
		},
		{
			name:         "ambiguous done asks alone",
			in:           RouteInput{AmbiguousDone: true},
			wantRoute:    RouteAskSDD,
			wantTriggers: []string{"ambiguous-done"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateRoute(tc.in)
			if got.Route != tc.wantRoute {
				t.Fatalf("route = %q, want %q (reason: %s)", got.Route, tc.wantRoute, got.Reason)
			}
			for _, want := range tc.wantTriggers {
				if !slices.Contains(got.Triggers, want) {
					t.Errorf("triggers %v missing %q", got.Triggers, want)
				}
			}
			if tc.wantRoute == RouteAskSDD && got.Reason == "" {
				t.Errorf("ask verdict must carry a reason")
			}
			if tc.wantRoute == RouteDirect && len(got.Triggers) != 0 {
				t.Errorf("direct verdict must not carry triggers, got %v", got.Triggers)
			}
		})
	}
}
