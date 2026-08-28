package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Negotiated contract envelope (Phase D2)
// ---------------------------------------------------------------------------

// contractJSON renders a contract envelope to raw JSON and asserts the
// top-level carries EXACTLY schema/lineage/next_transition: contract mode
// returns only the envelope, no raw status fields.
func contractJSON(t *testing.T, env *ContractEnvelope) map[string]json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	want := []string{"schema", "lineage", "next_transition"}
	if len(raw) != len(want) {
		t.Fatalf("envelope keys = %v, want exactly %v (no raw status fields)", keysOf(raw), want)
	}
	for _, key := range want {
		if _, ok := raw[key]; !ok {
			t.Fatalf("envelope missing key %q", key)
		}
	}
	return raw
}

func keysOf(raw map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	return keys
}

func TestContractEnvelope_CollectInputsExact(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "ct-collect", []string{"risk", "readability", "reliability"}, "")

	captureLens(t, repo, "ct-collect", head, "risk", 0)
	captureLens(t, repo, "ct-collect", head, "reliability", 2)

	store, err := Open(repo, "ct-collect")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	env, err := NewAuthority(repo).BuildNextTransition("ct-collect")
	if err != nil {
		t.Fatalf("BuildNextTransition: %v", err)
	}
	raw := contractJSON(t, env)

	if string(raw["schema"]) != `"biggz-ai.review-integration/v1"` {
		t.Errorf("schema = %s", raw["schema"])
	}
	if string(raw["lineage"]) != `"ct-collect"` {
		t.Errorf("lineage = %s", raw["lineage"])
	}

	var nt NextTransitionEnvelope
	if err := json.Unmarshal(raw["next_transition"], &nt); err != nil {
		t.Fatalf("next_transition: %v", err)
	}
	if nt.Type != "collect" {
		t.Fatalf("type = %q, want collect", nt.Type)
	}
	if nt.Operation != "" || len(nt.Arguments) != 0 || nt.ReasonCode != "" {
		t.Errorf("collect envelope must not carry execute/stop fields: %+v", nt)
	}
	if len(nt.Inputs) != 1 {
		t.Fatalf("inputs = %v, want exactly the capture input", keysOfMap(nt.Inputs))
	}
	input, ok := nt.Inputs["capture"]
	if !ok {
		t.Fatalf("inputs keys = %v, want capture", keysOfMap(nt.Inputs))
	}
	if input.Lineage != "ct-collect" {
		t.Errorf("capture lineage = %q", input.Lineage)
	}
	if input.Target != head {
		t.Errorf("capture target = %q, want genesis commit %q", input.Target, head)
	}
	if input.Lens != "readability" {
		t.Errorf("capture lens = %q, want readability (first missing slot in canonical order)", input.Lens)
	}
	if input.Order != 0 {
		t.Errorf("capture order = %d, want 0", input.Order)
	}
	if input.ExpectedRevision != chain.HeadHash {
		t.Errorf("capture expected_revision = %q, want current head %q", input.ExpectedRevision, chain.HeadHash)
	}
	if input.RepositoryContext == nil {
		t.Fatal("capture repository_context missing")
	}
	if input.RepositoryContext.Repository != repo {
		t.Errorf("repository_context.repository = %q, want the review subject repository %q", input.RepositoryContext.Repository, repo)
	}
	if want := filepath.Base(repo); input.RepositoryContext.Project != want {
		t.Errorf("repository_context.project = %q, want %q", input.RepositoryContext.Project, want)
	}

	// The envelope must never carry the subject hash: the plugin derives it
	// via capture-result --preflight before the real capture.
	payload, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(payload), "subject_hash") {
		t.Error("envelope must not contain subject_hash (derived via preflight)")
	}
}

func keysOfMap(m map[string]ContractCaptureInput) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func TestContractEnvelope_ExecuteFinalize(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "ct-finalize", []string{"risk", "readability"}, "")
	captureLensClean(t, repo, "ct-finalize", head, "risk", 0)
	captureLensClean(t, repo, "ct-finalize", head, "readability", 1)

	env, err := NewAuthority(repo).BuildNextTransition("ct-finalize")
	if err != nil {
		t.Fatalf("BuildNextTransition: %v", err)
	}
	raw := contractJSON(t, env)
	var nt NextTransitionEnvelope
	if err := json.Unmarshal(raw["next_transition"], &nt); err != nil {
		t.Fatalf("next_transition: %v", err)
	}
	if nt.Type != "execute" || nt.Operation != "finalize" {
		t.Fatalf("transition = %+v, want execute finalize", nt)
	}
	if len(nt.Arguments) != 1 || nt.Arguments[0] != "ct-finalize" {
		t.Errorf("arguments = %v, want [ct-finalize]", nt.Arguments)
	}
	if nt.Inputs != nil || nt.ReasonCode != "" {
		t.Errorf("execute envelope must not carry collect/stop fields: %+v", nt)
	}
}

func TestContractEnvelope_ExecuteResumeCorrection(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "ct-correction", []string{"risk"}, "")

	// A deterministic blocking finding finalizes into an unresolved receipt;
	// the resume + new capture after finalize leaves it unresolved: correction.
	captureLens(t, repo, "ct-correction", head, "risk", 0)
	if _, err := Finalize(repo, "ct-correction"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if err := resumeLineage(t, repo, "ct-correction"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	captureLens(t, repo, "ct-correction", head, "readability", 1)

	env, err := NewAuthority(repo).BuildNextTransition("ct-correction")
	if err != nil {
		t.Fatalf("BuildNextTransition: %v", err)
	}
	raw := contractJSON(t, env)
	var nt NextTransitionEnvelope
	if err := json.Unmarshal(raw["next_transition"], &nt); err != nil {
		t.Fatalf("next_transition: %v", err)
	}
	if nt.Type != "execute" || nt.Operation != "resume" {
		t.Fatalf("transition = %+v, want execute resume", nt)
	}
	// 5 original changed lines -> budget min(200, ceil(5/2)) = 3: the contract
	// offers the max allowed forecast; the orchestrator may run a lower one.
	want := []string{"ct-correction", "--correction-lines", "3"}
	if strings.Join(nt.Arguments, " ") != strings.Join(want, " ") {
		t.Errorf("arguments = %v, want %v", nt.Arguments, want)
	}
}

func TestContractEnvelope_StopReadyForGates(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "ct-gate", []string{"risk"}, "")
	captureLensClean(t, repo, "ct-gate", head, "risk", 0)
	if _, err := Finalize(repo, "ct-gate"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	env, err := NewAuthority(repo).BuildNextTransition("ct-gate")
	if err != nil {
		t.Fatalf("BuildNextTransition: %v", err)
	}
	raw := contractJSON(t, env)
	var nt NextTransitionEnvelope
	if err := json.Unmarshal(raw["next_transition"], &nt); err != nil {
		t.Fatalf("next_transition: %v", err)
	}
	if nt.Type != "stop" || nt.ReasonCode != "ready_for_gates" {
		t.Fatalf("transition = %+v, want stop ready_for_gates", nt)
	}
	if nt.Inputs != nil || nt.Operation != "" {
		t.Errorf("stop envelope must not carry collect/execute fields: %+v", nt)
	}
}

func TestContractEnvelope_StopChainInvalid(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "ct-invalid", []string{"risk"}, "")

	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	genesis := chain.GenesisHash
	path := filepath.Join(store.Dir, "v1", "events", genesis)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = filepath.Join(store.Dir, genesis)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read genesis: %v", err)
	}
	if err := os.WriteFile(path, append(data, ' '), 0644); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	env, err := NewAuthority(repo).BuildNextTransition("ct-invalid")
	if err != nil {
		t.Fatalf("BuildNextTransition: %v", err)
	}
	if env.NextTransition.Type != "stop" || env.NextTransition.ReasonCode != "chain_invalid" {
		t.Fatalf("transition = %+v, want stop chain_invalid", env.NextTransition)
	}
}

func TestContractEnvelope_StopTerminalState(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "ct-terminated", []string{"risk"}, "")
	if _, err := Invalidate(repo, "ct-terminated", "policy violation"); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}

	env, err := NewAuthority(repo).BuildNextTransition("ct-terminated")
	if err != nil {
		t.Fatalf("BuildNextTransition: %v", err)
	}
	if env.NextTransition.Type != "stop" || env.NextTransition.ReasonCode != "invalidated" {
		t.Fatalf("transition = %+v, want stop invalidated", env.NextTransition)
	}
}

func TestContractEnvelope_StopRDDDisabled(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "ct-rdd", []string{"risk"}, "")

	gitDir, err := gitIn(repo, "rev-parse", "--git-dir")
	if err != nil {
		t.Fatalf("git dir: %v", err)
	}
	gitDir = filepath.Join(repo, gitDir)
	if _, err := RDDDisable(gitDir, gitDir, "clone"); err != nil {
		t.Fatalf("RDDDisable: %v", err)
	}

	env, err := NewAuthority(repo).BuildNextTransition("ct-rdd")
	if err != nil {
		t.Fatalf("BuildNextTransition: %v", err)
	}
	if env.NextTransition.Type != "stop" || env.NextTransition.ReasonCode != "rdd_disabled" {
		t.Fatalf("transition = %+v, want stop rdd_disabled", env.NextTransition)
	}
}

func TestContractEnvelope_EmptyLineageErrors(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	env, err := NewAuthority(repo).BuildNextTransition("ct-empty")
	if err == nil {
		t.Fatalf("BuildNextTransition = %+v, want error for an empty lineage", env)
	}
	if !strings.Contains(err.Error(), "no next transition") {
		t.Errorf("error should say there is nothing to route, got: %v", err)
	}
}

func TestContractEnvelope_RepositoryContextContractShapeAccepted(t *testing.T) {
	// The exact JSON the contract envelope emits ({repository, project}) must
	// decode and validate as a capture repository context.
	context, err := DecodeRepositoryContext([]byte(`{"repository":"/tmp/acme/repo","project":"repo"}`))
	if err != nil {
		t.Fatalf("DecodeRepositoryContext: %v", err)
	}
	if context.Repo != "/tmp/acme/repo" {
		t.Errorf("repo = %q, want the resolved alias", context.Repo)
	}
	if err := context.Validate(CaptureBinding{}); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := DecodeRepositoryContext([]byte(`{"repository":"/tmp/acme/repo","project":"other"}`)); err != nil {
		t.Fatalf("decoder must not validate the project echo: %v", err)
	}
	mismatch, err := DecodeRepositoryContext([]byte(`{"repository":"/tmp/acme/repo","project":"other"}`))
	if err != nil {
		t.Fatalf("DecodeRepositoryContext: %v", err)
	}
	if err := mismatch.Validate(CaptureBinding{}); err == nil || !strings.Contains(err.Error(), "project") {
		t.Errorf("Validate should reject a project/repository mismatch, got: %v", err)
	}
	// Unknown keys stay rejected.
	if _, err := DecodeRepositoryContext([]byte(`{"repository":"/tmp/repo","unknown":true}`)); err == nil {
		t.Fatal("expected rejection of unknown context key")
	}
}

// ---------------------------------------------------------------------------
// Consent envelope follow-up invocations
// ---------------------------------------------------------------------------

func TestConsentEnvelope_WithFollowUpInvocations(t *testing.T) {
	decision, err := EvaluateStartConsent(subject(), "ct-lineage", highInput, []string{"risk"}, "relay", false)
	if err != nil {
		t.Fatalf("EvaluateStartConsent: %v", err)
	}
	if decision.Envelope == nil {
		t.Fatal("relay envelope missing")
	}

	// Without a base the choices stay invocation-free (byte-for-byte non-contract).
	for _, choice := range decision.Envelope.Choices {
		if choice.Invocation != "" {
			t.Errorf("choice %q must carry no invocation without contract mode, got %q", choice.ID, choice.Invocation)
		}
	}

	base := "biggz review start --subject C:\\tmp\\subject.json --lineage ct-lineage --lenses risk"
	decision.Envelope.WithFollowUpInvocations(base)
	wantGranted := base + " --consent granted"
	wantDeclined := base + " --consent declined"
	got := map[string]string{}
	for _, choice := range decision.Envelope.Choices {
		got[choice.ID] = choice.Invocation
	}
	if got["granted"] != wantGranted {
		t.Errorf("granted invocation = %q, want %q", got["granted"], wantGranted)
	}
	if got["declined"] != wantDeclined {
		t.Errorf("declined invocation = %q, want %q", got["declined"], wantDeclined)
	}

	// The schema stays the existing consent schema; only the choices extend.
	if decision.Envelope.Schema != ConsentModeSchema {
		t.Errorf("schema = %q, want %q", decision.Envelope.Schema, ConsentModeSchema)
	}
}
