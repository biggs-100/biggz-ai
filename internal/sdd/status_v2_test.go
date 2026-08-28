package sdd

import (
	"encoding/json"
	"strings"
	"testing"
)

const freshV2RerunInstruction = "Start a fresh implementation state and rerun `biggz sdd-status --contract biggz-ai.sdd-status/v2`."

func TestSDDStatusV2CleanBreak(t *testing.T) {
	t.Run("v2 is the sole default and v1 is refused read-only", func(t *testing.T) {
		if StatusSchemaVersion != 2 {
			t.Fatalf("StatusSchemaVersion = %d, want 2", StatusSchemaVersion)
		}
		if StatusSchemaName != "biggz-ai.sdd-status" {
			t.Fatalf("StatusSchemaName = %q, want biggz-ai.sdd-status", StatusSchemaName)
		}
		if StatusContractV2 != "biggz-ai.sdd-status/v2" {
			t.Fatalf("StatusContractV2 = %q, want biggz-ai.sdd-status/v2", StatusContractV2)
		}

		defaultArgs, err := ParseCommandArgs([]string{})
		if err != nil {
			t.Fatalf("ParseCommandArgs(default) error = %v", err)
		}
		if defaultArgs.Contract != "biggz-ai.sdd-status/v2" {
			t.Fatalf("default contract = %q, want biggz-ai.sdd-status/v2", defaultArgs.Contract)
		}

		v2Args, err := ParseCommandArgs([]string{"--contract", "biggz-ai.sdd-status/v2"})
		if err != nil {
			t.Fatalf("ParseCommandArgs(v2) error = %v", err)
		}
		if v2Args.Contract != "biggz-ai.sdd-status/v2" {
			t.Fatalf("v2 contract = %q, want biggz-ai.sdd-status/v2", v2Args.Contract)
		}

		_, err = ParseCommandArgs([]string{"--contract", "biggz-ai.sdd-status/v1"})
		if err == nil || err.Error() != "unsupported sdd-status contract \"biggz-ai.sdd-status/v1\". "+freshV2RerunInstruction {
			t.Fatalf("v1 refusal = %v, want one fresh-v2 rerun instruction", err)
		}
		_, err = ParseCommandArgs([]string{"--contract=biggz-ai.sdd-status/v1"})
		if err == nil || !strings.Contains(err.Error(), freshV2RerunInstruction) {
			t.Fatalf("v1 refusal via --contract= form = %v, want fresh instruction", err)
		}
		afterRefusalDefaultArgs, err := ParseCommandArgs([]string{})
		if err != nil {
			t.Fatalf("ParseCommandArgs(default after v1 refusal) error = %v", err)
		}
		if afterRefusalDefaultArgs.Contract != defaultArgs.Contract {
			t.Fatalf("v1 refusal changed default: before=%#v after=%#v", defaultArgs, afterRefusalDefaultArgs)
		}
	})

	t.Run("projection has the exact v2 authority-free key sets", func(t *testing.T) {
		// Build a minimal derived status
		cs := ChangeStatus{
			SchemaName:    StatusSchemaName,
			SchemaVersion: StatusSchemaVersion,
			Name:          "test-change",
			ChangeRoot:    "/repo/openspec/changes/test-change",
			PlanningHome:  PlanningHome{Mode: "repo-local", Path: "/repo/openspec"},
			ArtifactStore: ArtifactStoreOpenSpec,
			ArtifactPaths: ArtifactPaths{
				Proposal:      []string{"/repo/openspec/changes/test-change/proposal.md"},
				Specs:         []string{"/repo/openspec/changes/test-change/specs/core/spec.md"},
				Design:        []string{"/repo/openspec/changes/test-change/design.md"},
				Tasks:         []string{"/repo/openspec/changes/test-change/tasks.md"},
				ApplyProgress: []string{},
				VerifyReport:  []string{},
			},
			ContextFiles: ArtifactPaths{
				Proposal:      []string{"/repo/openspec/changes/test-change/proposal.md"},
				Specs:         []string{"/repo/openspec/changes/test-change/specs/core/spec.md"},
				Design:        []string{"/repo/openspec/changes/test-change/design.md"},
				Tasks:         []string{"/repo/openspec/changes/test-change/tasks.md"},
				ApplyProgress: []string{},
				VerifyReport:  []string{},
			},
			Artifacts: map[string]ArtifactState{
				"proposal": ArtifactDone, "specs": ArtifactDone, "design": ArtifactDone,
				"tasks": ArtifactDone, "applyProgress": ArtifactMissing, "verifyReport": ArtifactMissing,
			},
			TaskProgress: TaskProgress{Total: 1, Completed: 0, Pending: 1},
			Dependencies: Dependencies{
				Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyAllDone,
				Tasks: DependencyAllDone, Apply: DependencyReady, Verify: DependencyBlocked, Archive: DependencyBlocked,
			},
			ApplyState:       ApplyReady,
			ActionContext:    ActionContext{Mode: "repo-local", WorkspaceRoot: "/repo", AllowedEditRoots: []string{"/repo"}},
			Relationships:    Relationships{},
			RemediationState: RemediationState{},
			NextRecommended:  "apply",
			BlockedReasons:   []string{},
		}
		projected, err := ProjectStatusV2(cs)
		if err != nil {
			t.Fatalf("ProjectStatusV2 error = %v", err)
		}
		payload, err := json.Marshal(projected)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(payload, &document); err != nil {
			t.Fatal(err)
		}
		expectedKeys := []string{
			"schemaName", "schemaVersion", "changeName", "artifactStore", "planningHome", "changeRoot",
			"artifactPaths", "contextFiles", "artifacts", "taskProgress", "dependencies", "applyState",
			"actionContext", "relationships", "remediationState", "nextRecommended", "blockedReasons",
		}
		for _, k := range expectedKeys {
			if _, ok := document[k]; !ok {
				t.Fatalf("missing expected key %q in projection: %s", k, payload)
			}
		}
		for _, forbidden := range []string{"reviewGate", "reviewTransaction", "reVerify", "runtimeStatus", "reviewPolicy", "reviewLedger", "reviewReceipt", "reviewBundle", "reviewContext", "reviewState", "lineageId", "generation", "fixBatch", "correctionBudget"} {
			if strings.Contains(string(payload), forbidden) {
				t.Fatalf("v2 projection retained authority key %q: %s", forbidden, payload)
			}
		}
		// Check nested keys
		var ap map[string]json.RawMessage
		if err := json.Unmarshal(document["artifactPaths"], &ap); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"proposal", "specs", "design", "tasks", "applyProgress", "verifyReport"} {
			if _, ok := ap[k]; !ok {
				t.Fatalf("artifactPaths missing %q", k)
			}
		}
		var rs map[string]json.RawMessage
		if err := json.Unmarshal(document["remediationState"], &rs); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"required", "complete", "failedEvidenceRevision", "reason"} {
			if _, ok := rs[k]; !ok {
				t.Fatalf("remediationState missing %q", k)
			}
		}
		if document["schemaName"] == nil || document["schemaVersion"] == nil {
			t.Fatal("schema identity missing")
		}
		// Explicitly ensure no extra keys beyond allowlist (allowing optional reviewOffer, consent, phaseInstructions)
		allowedSet := map[string]bool{}
		for _, k := range expectedKeys {
			allowedSet[k] = true
		}
		allowedSet["reviewOffer"] = true
		allowedSet["consent"] = true
		allowedSet["phaseInstructions"] = true
		for k := range document {
			if !allowedSet[k] {
				t.Fatalf("unexpected key %q in v2 projection", k)
			}
		}
	})

	t.Run("unknown contract is rejected", func(t *testing.T) {
		_, err := ParseCommandArgs([]string{"--contract", "biggz-ai.sdd-status/v3"})
		if err == nil || !strings.Contains(err.Error(), "unsupported sdd-status contract") {
			t.Fatalf("unknown contract should be rejected, got %v", err)
		}
	})
}

func TestV2AuthorityFree(t *testing.T) {
	t.Run("projection omits authority keys and clears blockedReasons", func(t *testing.T) {
		cs := ChangeStatus{
			SchemaName:    StatusSchemaName,
			SchemaVersion: StatusSchemaVersion,
			Name:          "test",
			ChangeRoot:    "/repo/openspec/changes/test",
			PlanningHome:  PlanningHome{Mode: "repo-local", Path: "/repo/openspec"},
			ArtifactStore: ArtifactStoreOpenSpec,
			ArtifactPaths: ArtifactPaths{
				Proposal:      []string{"/repo/openspec/changes/test/proposal.md"},
				Specs:         []string{"/repo/openspec/changes/test/specs/core/spec.md"},
				Design:        []string{"/repo/openspec/changes/test/design.md"},
				Tasks:         []string{"/repo/openspec/changes/test/tasks.md"},
				ApplyProgress: []string{},
				VerifyReport:  []string{},
			},
			ContextFiles: ArtifactPaths{
				Proposal:      []string{"/repo/openspec/changes/test/proposal.md"},
				Specs:         []string{"/repo/openspec/changes/test/specs/core/spec.md"},
				Design:        []string{"/repo/openspec/changes/test/design.md"},
				Tasks:         []string{"/repo/openspec/changes/test/tasks.md"},
				ApplyProgress: []string{},
				VerifyReport:  []string{},
			},
			Artifacts: map[string]ArtifactState{
				"proposal": ArtifactDone, "specs": ArtifactDone, "design": ArtifactDone,
				"tasks": ArtifactDone, "applyProgress": ArtifactMissing, "verifyReport": ArtifactMissing,
			},
			TaskProgress: TaskProgress{Total: 1, Completed: 0, Pending: 1},
			Dependencies: Dependencies{
				Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyAllDone,
				Tasks: DependencyAllDone, Apply: DependencyReady, Verify: DependencyBlocked, Archive: DependencyBlocked,
			},
			ApplyState:       ApplyReady,
			ActionContext:    ActionContext{Mode: "repo-local", WorkspaceRoot: "/repo", AllowedEditRoots: []string{"/repo"}},
			Relationships:    Relationships{},
			RemediationState: RemediationState{},
			BlockedReasons:   []string{"blocked(edit_authority_missing): tasks.md targets repositories outside the authorized edit roots: \"/other\"; edit tasks.md so every work unit stays inside the authorized edit roots, or grant this change edit authority for those repositories"},
			NextRecommended:  "resolve-blockers",
			GrantedRoots:     []string{"/other"},
			EditAuthorityBlocked: true,
			MissingRoots:     []string{"/other"},
		}
		projected, err := ProjectStatusV2(cs)
		if err != nil {
			t.Fatalf("ProjectStatusV2 error = %v", err)
		}
		payload, err := json.Marshal(projected)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"granted_roots", "edit_authority_blocked", "missing_roots"} {
			if strings.Contains(string(payload), forbidden) {
				t.Fatalf("v2 projection must NOT emit %q: %s", forbidden, payload)
			}
		}
		if len(projected.BlockedReasons) != 0 {
			t.Fatalf("blockedReasons = %v, want [] (authority-free)", projected.BlockedReasons)
		}
		if projected.NextRecommended == "resolve-blockers" {
			t.Fatalf("nextRecommended = %q, want != resolve-blockers (authority-free)", projected.NextRecommended)
		}
	})
	t.Run("v2 allowlist keys only", func(t *testing.T) {
		cs := ChangeStatus{
			SchemaName:    StatusSchemaName,
			SchemaVersion: StatusSchemaVersion,
			Name:          "test",
			ChangeRoot:    "/repo/openspec/changes/test",
			PlanningHome:  PlanningHome{Mode: "repo-local", Path: "/repo/openspec"},
			ArtifactStore: ArtifactStoreOpenSpec,
			ArtifactPaths: ArtifactPaths{
				Proposal:      []string{"/repo/openspec/changes/test/proposal.md"},
				Specs:         []string{"/repo/openspec/changes/test/specs/core/spec.md"},
				Design:        []string{"/repo/openspec/changes/test/design.md"},
				Tasks:         []string{"/repo/openspec/changes/test/tasks.md"},
				ApplyProgress: []string{},
				VerifyReport:  []string{},
			},
			ContextFiles: ArtifactPaths{
				Proposal:      []string{"/repo/openspec/changes/test/proposal.md"},
				Specs:         []string{"/repo/openspec/changes/test/specs/core/spec.md"},
				Design:        []string{"/repo/openspec/changes/test/design.md"},
				Tasks:         []string{"/repo/openspec/changes/test/tasks.md"},
				ApplyProgress: []string{},
				VerifyReport:  []string{},
			},
			Artifacts: map[string]ArtifactState{
				"proposal": ArtifactDone, "specs": ArtifactDone, "design": ArtifactDone,
				"tasks": ArtifactDone, "applyProgress": ArtifactMissing, "verifyReport": ArtifactMissing,
			},
			TaskProgress: TaskProgress{Total: 1, Completed: 0, Pending: 1},
			Dependencies: Dependencies{
				Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyAllDone,
				Tasks: DependencyAllDone, Apply: DependencyReady, Verify: DependencyBlocked, Archive: DependencyBlocked,
			},
			ApplyState:       ApplyReady,
			ActionContext:    ActionContext{Mode: "repo-local", WorkspaceRoot: "/repo", AllowedEditRoots: []string{"/repo"}},
			Relationships:    Relationships{},
			RemediationState: RemediationState{},
			NextRecommended:  "apply",
			BlockedReasons:   []string{},
		}
		projected, err := ProjectStatusV2(cs)
		if err != nil {
			t.Fatalf("ProjectStatusV2 error = %v", err)
		}
		payload, err := json.Marshal(projected)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(payload, &doc); err != nil {
			t.Fatal(err)
		}
		allowed := map[string]bool{
			"schemaName": true, "schemaVersion": true, "changeName": true, "artifactStore": true, "planningHome": true, "changeRoot": true,
			"artifactPaths": true, "contextFiles": true, "artifacts": true, "taskProgress": true, "dependencies": true, "applyState": true,
			"actionContext": true, "relationships": true, "remediationState": true, "nextRecommended": true, "blockedReasons": true,
			"reviewOffer": true, "consent": true, "phaseInstructions": true,
		}
		for k := range doc {
			if !allowed[k] {
				t.Fatalf("unexpected key %q in v2 projection (allowlist only)", k)
			}
		}
	})
}

func TestV1Refused(t *testing.T) {
	_, err := ParseCommandArgs([]string{"--contract", "biggz-ai.sdd-status/v1"})
	if err == nil || !strings.Contains(err.Error(), "unsupported sdd-status contract") {
		t.Fatalf("v1 refusal = %v, want unsupported", err)
	}
	if !strings.Contains(err.Error(), freshV2RerunInstruction) {
		t.Fatalf("v1 refusal missing rerun instruction: %v", err)
	}
	_, err = ParseCommandArgs([]string{"--contract=biggz-ai.sdd-status/v1"})
	if err == nil || !strings.Contains(err.Error(), freshV2RerunInstruction) {
		t.Fatalf("v1 refusal via = form = %v, want fresh instruction", err)
	}
}

func TestProjectStatusV2RejectsUnsupportedValues(t *testing.T) {
	base := ChangeStatus{
		SchemaName:    StatusSchemaName,
		SchemaVersion: StatusSchemaVersion,
		Name:          "test",
		ChangeRoot:    "/repo/openspec/changes/test",
		PlanningHome:  PlanningHome{Mode: "repo-local", Path: "/repo/openspec"},
		ArtifactStore: ArtifactStoreOpenSpec,
		ArtifactPaths: ArtifactPaths{
			Proposal:      []string{"/repo/openspec/changes/test/proposal.md"},
			Specs:         []string{"/repo/openspec/changes/test/specs/core/spec.md"},
			Design:        []string{"/repo/openspec/changes/test/design.md"},
			Tasks:         []string{"/repo/openspec/changes/test/tasks.md"},
			ApplyProgress: []string{},
			VerifyReport:  []string{},
		},
		ContextFiles: ArtifactPaths{
			Proposal:      []string{"/repo/openspec/changes/test/proposal.md"},
			Specs:         []string{"/repo/openspec/changes/test/specs/core/spec.md"},
			Design:        []string{"/repo/openspec/changes/test/design.md"},
			Tasks:         []string{"/repo/openspec/changes/test/tasks.md"},
			ApplyProgress: []string{},
			VerifyReport:  []string{},
		},
		Artifacts: map[string]ArtifactState{
			"proposal": ArtifactDone, "specs": ArtifactDone, "design": ArtifactDone,
			"tasks": ArtifactDone, "applyProgress": ArtifactMissing, "verifyReport": ArtifactMissing,
		},
		TaskProgress: TaskProgress{Total: 1, Completed: 0, Pending: 1},
		Dependencies: Dependencies{
			Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyAllDone,
			Tasks: DependencyAllDone, Apply: DependencyReady, Verify: DependencyBlocked, Archive: DependencyBlocked,
		},
		ApplyState:       ApplyReady,
		ActionContext:    ActionContext{Mode: "repo-local", WorkspaceRoot: "/repo", AllowedEditRoots: []string{"/repo"}},
		Relationships:    Relationships{},
		RemediationState: RemediationState{},
		NextRecommended:  "apply",
		BlockedReasons:   []string{},
	}

	tests := []struct {
		name   string
		mutate func(*ChangeStatus)
		want   string
	}{
		{
			name:   "unknown next action",
			mutate: func(cs *ChangeStatus) { cs.NextRecommended = "working" },
			want:   `unsupported SDD v2 next action "working"`,
		},
		{
			name:   "unknown artifact store",
			mutate: func(cs *ChangeStatus) { cs.ArtifactStore = "workrun" },
			want:   `unsupported SDD v2 artifact store "workrun"`,
		},
		{
			name:   "unknown artifact state",
			mutate: func(cs *ChangeStatus) { cs.Artifacts["proposal"] = "checking" },
			want:   `unsupported SDD v2 artifact "proposal" state "checking"`,
		},
		{
			name:   "unsupported identity version",
			mutate: func(cs *ChangeStatus) { cs.SchemaVersion = 1 },
			want:   `unsupported SDD status identity`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := base
			tt.mutate(&cs)
			_, err := ProjectStatusV2(cs)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ProjectStatusV2() error = %v, want %q", err, tt.want)
			}
		})
	}
}
