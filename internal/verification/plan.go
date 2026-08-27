// Package verification implements content-addressed verification plans
// with effect profiles, cost classification, and consent gating.
//
// This is biggz-ai's equivalent of gentle-ai's RAR (Review-After-Review)
// plan system, designed to be simpler while equally powerful.
//
// Key concepts:
//   - Plan: frozen set of verification obligations with content-addressed identity
//   - Effect: cost + mutation + permission profile for a plan
//   - Gate: resolves whether a plan needs human consent to run
//   - Convergence: proves workspace didn't mutate during verification
package verification

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ─── Constants ───────────────────────────────────────────────────────────────

const (
	PlanSchema          = "biggz.verification-plan/v1"
	EffectSchema        = "biggz.verification-effect/v1"
	ConsentSchema       = "biggz.verification-consent/v1"
	AuthorizationSchema = "biggz.verification-authorization/v1"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// VerificationSubject identifies exactly what is being verified.
type VerificationSubject struct {
	Kind             string `json:"kind"`              // "current-changes", "base-diff", "fix-diff"
	Projection       string `json:"projection"`        // "workspace", "staged"
	BaseTree         string `json:"base_tree"`         // git tree OID
	CandidateTree    string `json:"candidate_tree"`    // git tree OID
	PathsDigest      string `json:"paths_digest"`      // SHA-256 of sorted changed paths
	SnapshotIdentity string `json:"snapshot_identity"` // SHA-256 of all snapshot fields
}

// CostClass defines how expensive a verification obligation is.
type CostClass string

const (
	CostQuick    CostClass = "quick"     // runs in seconds
	CostLong     CostClass = "long"      // runs in minutes
	CostVeryLong CostClass = "very_long" // runs in 5+ minutes
	CostUnknown  CostClass = "unknown"   // cannot determine
)

// MutationEffect defines whether a verifier modifies files.
type MutationEffect string

const (
	MutationReadOnly    MutationEffect = "read_only"
	MutationDestructive MutationEffect = "destructive" // modifies files
)

// PermissionEffect defines whether a verifier needs special access.
type PermissionEffect string

const (
	PermissionOrdinary  PermissionEffect = "ordinary"
	PermissionSensitive PermissionEffect = "sensitive" // e.g. network access, secrets
)

// VerificationObligation is a single verifier to run.
type VerificationObligation struct {
	ID          string    `json:"id"`       // e.g. "lens/risk", "lens/reliability"
	Contract    string    `json:"contract"` // e.g. "biggz.functional-proof/v1"
	Cost        CostClass `json:"cost"`
	ReadOnly    bool      `json:"read_only"` // true if verifier never modifies files
	Mandatory   bool      `json:"mandatory"` // true if cannot be skipped
	Description string    `json:"description,omitempty"`
}

// EffectProfile describes the aggregate effect of running a plan.
type EffectProfile struct {
	Schema        string           `json:"schema"`
	Applicable    bool             `json:"applicable"`
	AggregateCost CostClass        `json:"aggregate_cost"`
	Mutation      MutationEffect   `json:"mutation"`
	Permission    PermissionEffect `json:"permission"`
	Digest        string           `json:"digest"` // SHA-256 of all fields
}

// VerificationPlan is a frozen, content-addressed plan.
type VerificationPlan struct {
	Schema       string                   `json:"schema"`
	AuthorityRef string                   `json:"authority_ref"` // SHA-256 of this plan
	Subject      VerificationSubject      `json:"subject"`
	Obligations  []VerificationObligation `json:"obligations"`
	Effects      *EffectProfile           `json:"effects,omitempty"`
	CreatedAt    string                   `json:"created_at"`
}

// PlanGate determines what's needed to execute the plan.
type PlanGate struct {
	Decision        PlanDecision `json:"decision"`
	RequiresConsent bool         `json:"requires_consent"`
	RequiresAuth    bool         `json:"requires_authorization"`
	Reason          string       `json:"reason,omitempty"`
}

// PlanDecision classifies the gate outcome.
type PlanDecision string

const (
	DecisionAutoRun       PlanDecision = "auto_run"       // safe to run unattended
	DecisionNeedsConsent  PlanDecision = "needs_consent"  // expensive, needs human OK
	DecisionNeedsAuth     PlanDecision = "needs_auth"     // destructive/sensitive, needs per-effect OK
	DecisionNotApplicable PlanDecision = "not_applicable" // no work to do
	DecisionEvidenceGap   PlanDecision = "evidence_gap"   // cost unknown, can't decide
)

// FrozenConsent records human approval for an expensive plan.
type FrozenConsent struct {
	Schema      string `json:"schema"`
	PlanRef     string `json:"plan_ref"`    // AuthorityRef of the plan
	PlanDigest  string `json:"plan_digest"` // Digest of the plan
	ConsentedBy string `json:"consented_by"`
	ConsentedAt string `json:"consented_at"`
	Digest      string `json:"digest"` // SHA-256 of all fields
}

// EffectAuthorization records human approval for a sensitive obligation.
type EffectAuthorization struct {
	Schema       string `json:"schema"`
	PlanRef      string `json:"plan_ref"`
	ObligationID string `json:"obligation_id"`
	AuthorizedBy string `json:"authorized_by"`
	AuthorizedAt string `json:"authorized_at"`
	Digest       string `json:"digest"`
}

// ─── Content-addressed helpers ───────────────────────────────────────────────

func contentDigest(v any) string {
	data, _ := json.Marshal(v)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func (s *VerificationSubject) ComputeDigest() string {
	return contentDigest(s)
}

// ─── Plan construction ───────────────────────────────────────────────────────

// NewVerificationSubjectFromSnapshot builds a subject from raw tree data.
func NewVerificationSubjectFromSnapshot(kind, projection, baseTree, candidateTree string, paths []string) VerificationSubject {
	pathsDigest := contentDigest(paths)
	subject := VerificationSubject{
		Kind: kind, Projection: projection,
		BaseTree: baseTree, CandidateTree: candidateTree,
		PathsDigest: pathsDigest,
	}
	subject.SnapshotIdentity = contentDigest(subject)
	return subject
}

// AggregateCost returns the highest cost across all obligations.
func AggregateCost(obligations []VerificationObligation) CostClass {
	ranking := map[CostClass]int{CostQuick: 0, CostLong: 1, CostVeryLong: 2, CostUnknown: 3}
	highest := CostQuick
	for _, o := range obligations {
		if ranking[o.Cost] > ranking[highest] {
			highest = o.Cost
		}
	}
	return highest
}

// BuildEffect computes the effect profile for a set of obligations.
func BuildEffect(obligations []VerificationObligation) *EffectProfile {
	if len(obligations) == 0 {
		return &EffectProfile{
			Schema: EffectSchema, Applicable: false,
			Digest: contentDigest(map[string]any{"schema": EffectSchema, "applicable": false}),
		}
	}

	hasDestructive := false
	hasSensitive := false
	for _, o := range obligations {
		if !o.ReadOnly {
			hasDestructive = true
		}
	}

	mutation := MutationReadOnly
	if hasDestructive {
		mutation = MutationDestructive
	}
	perm := PermissionOrdinary
	if hasSensitive {
		perm = PermissionSensitive
	}

	ep := &EffectProfile{
		Schema: EffectSchema, Applicable: true,
		AggregateCost: AggregateCost(obligations),
		Mutation:      mutation, Permission: perm,
	}
	ep.Digest = contentDigest(ep)
	return ep
}

// NewPlan creates a frozen verification plan.
func NewPlan(subject VerificationSubject, obligations []VerificationObligation) *VerificationPlan {
	effects := BuildEffect(obligations)
	plan := &VerificationPlan{
		Schema:      PlanSchema,
		Subject:     subject,
		Obligations: obligations,
		Effects:     effects,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	plan.AuthorityRef = contentDigest(plan)
	return plan
}

// ─── Plan validation ─────────────────────────────────────────────────────────

// Validate checks the plan is self-consistent.
func (p *VerificationPlan) Validate() error {
	if p.Schema != PlanSchema {
		return fmt.Errorf("invalid plan schema: %s", p.Schema)
	}
	if p.AuthorityRef == "" {
		return fmt.Errorf("authority_ref is required")
	}
	if p.Subject.SnapshotIdentity == "" {
		return fmt.Errorf("subject is incomplete")
	}
	// Recompute digest and compare
	computed := contentDigest(&VerificationPlan{
		Schema: p.Schema, Subject: p.Subject,
		Obligations: p.Obligations, Effects: p.Effects,
		CreatedAt: p.CreatedAt,
	})
	if computed != p.AuthorityRef {
		return fmt.Errorf("authority_ref mismatch: content has been modified")
	}
	return nil
}

// ─── Gate resolution ────────────────────────────────────────────────────────

// ResolveGate determines what's needed to execute this plan.
func (p *VerificationPlan) ResolveGate() PlanGate {
	if p.Effects == nil || !p.Effects.Applicable {
		return PlanGate{Decision: DecisionNotApplicable, Reason: "no applicable obligations"}
	}

	if p.Effects.AggregateCost == CostUnknown {
		return PlanGate{
			Decision: DecisionEvidenceGap,
			Reason:   "cannot determine cost for one or more obligations",
		}
	}

	isDestructive := p.Effects.Mutation == MutationDestructive
	isSensitive := p.Effects.Permission == PermissionSensitive
	isExpensive := p.Effects.AggregateCost == CostLong || p.Effects.AggregateCost == CostVeryLong

	if isDestructive || isSensitive {
		return PlanGate{
			Decision:        DecisionNeedsAuth,
			RequiresAuth:    true,
			RequiresConsent: isExpensive,
			Reason:          planAuthReason(p.Effects),
		}
	}

	if isExpensive {
		return PlanGate{
			Decision:        DecisionNeedsConsent,
			RequiresConsent: true,
			Reason:          fmt.Sprintf("plan is %s (needs human approval)", p.Effects.AggregateCost),
		}
	}

	return PlanGate{Decision: DecisionAutoRun, Reason: "quick + read-only + ordinary"}
}

func planAuthReason(ep *EffectProfile) string {
	var parts []string
	if ep.Mutation == MutationDestructive {
		parts = append(parts, "destructive (modifies files)")
	}
	if ep.Permission == PermissionSensitive {
		parts = append(parts, "sensitive permissions")
	}
	if ep.AggregateCost == CostLong || ep.AggregateCost == CostVeryLong {
		parts = append(parts, fmt.Sprintf("cost: %s", ep.AggregateCost))
	}
	return strings.Join(parts, " + ")
}

// ─── Consent & Authorization ────────────────────────────────────────────────

// NewConsent creates a human consent record for an expensive plan.
func NewConsent(planRef, planDigest, consentedBy string) *FrozenConsent {
	c := &FrozenConsent{
		Schema: ConsentSchema, PlanRef: planRef,
		PlanDigest: planDigest, ConsentedBy: consentedBy,
		ConsentedAt: time.Now().UTC().Format(time.RFC3339),
	}
	c.Digest = contentDigest(c)
	return c
}

// NewAuthorization creates human authorization for a sensitive obligation.
func NewAuthorization(planRef, obligationID, authorizedBy string) *EffectAuthorization {
	a := &EffectAuthorization{
		Schema: AuthorizationSchema, PlanRef: planRef,
		ObligationID: obligationID, AuthorizedBy: authorizedBy,
		AuthorizedAt: time.Now().UTC().Format(time.RFC3339),
	}
	a.Digest = contentDigest(a)
	return a
}

// Validate checks consent integrity.
func (c *FrozenConsent) Validate() error {
	if c.Schema != ConsentSchema {
		return fmt.Errorf("invalid consent schema: %s", c.Schema)
	}
	if c.PlanRef == "" {
		return fmt.Errorf("plan_ref is required")
	}
	if c.ConsentedBy == "" {
		return fmt.Errorf("consented_by is required")
	}
	computed := contentDigest(&FrozenConsent{
		Schema: c.Schema, PlanRef: c.PlanRef,
		PlanDigest: c.PlanDigest, ConsentedBy: c.ConsentedBy,
		ConsentedAt: c.ConsentedAt,
	})
	if computed != c.Digest {
		return fmt.Errorf("consent digest mismatch: content has been modified")
	}
	return nil
}

// ─── Convergence ─────────────────────────────────────────────────────────────

// ConvergenceResult describes the outcome of a post-verification convergence check.
type ConvergenceResult struct {
	Passed       bool   `json:"passed"`
	ExpectedHash string `json:"expected_hash,omitempty"`
	ActualHash   string `json:"actual_hash,omitempty"`
	Message      string `json:"message,omitempty"`
}

// CheckConvergence verifies the workspace didn't mutate during verification.
// The expected subject was captured before verification; the live subject
// is captured after. If they match, convergence is proven.
func CheckConvergence(expected, live VerificationSubject) *ConvergenceResult {
	expectedHash := expected.ComputeDigest()
	liveHash := live.ComputeDigest()

	if expectedHash == liveHash {
		return &ConvergenceResult{Passed: true}
	}

	return &ConvergenceResult{
		Passed:       false,
		ExpectedHash: expectedHash[:12],
		ActualHash:   liveHash[:12],
		Message:      fmt.Sprintf("workspace mutated during verification: expected %s, got %s", expectedHash[:12], liveHash[:12]),
	}
}

// ─── Plan store (in-memory) ──────────────────────────────────────────────────

// PlanStore holds published plans for the current session.
type PlanStore struct {
	plans    map[string]*VerificationPlan
	consents map[string]*FrozenConsent
	auths    map[string]*EffectAuthorization
}

func NewPlanStore() *PlanStore {
	return &PlanStore{
		plans:    make(map[string]*VerificationPlan),
		consents: make(map[string]*FrozenConsent),
		auths:    make(map[string]*EffectAuthorization),
	}
}

// Publish stores a plan by its content-addressed AuthorityRef.
func (ps *PlanStore) Publish(plan *VerificationPlan) error {
	if err := plan.Validate(); err != nil {
		return fmt.Errorf("cannot publish invalid plan: %w", err)
	}
	if _, exists := ps.plans[plan.AuthorityRef]; exists {
		return nil // idempotent
	}
	ps.plans[plan.AuthorityRef] = plan
	return nil
}

// Get retrieves a plan by its AuthorityRef.
func (ps *PlanStore) Get(authorityRef string) (*VerificationPlan, bool) {
	p, ok := ps.plans[authorityRef]
	return p, ok
}

// RecordConsent stores a consent for a plan.
func (ps *PlanStore) RecordConsent(consent *FrozenConsent) error {
	if err := consent.Validate(); err != nil {
		return err
	}
	ps.consents[consent.PlanRef] = consent
	return nil
}

// HasConsent checks if a plan has been consented.
func (ps *PlanStore) HasConsent(planRef string) bool {
	_, ok := ps.consents[planRef]
	return ok
}
