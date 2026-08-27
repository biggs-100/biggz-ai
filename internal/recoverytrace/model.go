// Package recoverytrace provides release recovery auditing — it tracks what
// happens to every file across releases, determines if invariants are preserved,
// and produces a verifiable ledger that documents every disposition decision.
package recoverytrace

// Disposition is what the recovery decided to do with one path.
type Disposition string

const (
	DispositionKeep       Disposition = "KEEP"
	DispositionTransplant Disposition = "TRANSPLANT"
	DispositionRewrite    Disposition = "REWRITE"
	DispositionDelete     Disposition = "DELETE"
	DispositionRegenerate Disposition = "REGENERATE"
	DispositionDefer      Disposition = "DEFER"
)

// IsValid returns true for known dispositions.
func (d Disposition) IsValid() bool {
	switch d {
	case DispositionKeep, DispositionTransplant, DispositionRewrite,
		DispositionDelete, DispositionRegenerate, DispositionDefer:
		return true
	}
	return false
}

// SystemicContext names the owning system an invariant belongs to.
type SystemicContext string

const (
	ContextHCR SystemicContext = "HCR" // Host, install, command runtime
	ContextMMI SystemicContext = "MMI" // Managed mutation integrity
	ContextACI SystemicContext = "ACI" // Agent capability and instruction
	ContextMCA SystemicContext = "MCA" // Model catalog and assignment
	ContextRAR SystemicContext = "RAR" // Review authority and receipts
	ContextEPD SystemicContext = "EPD" // Evidence, policy, diagnostics
	ContextDSR SystemicContext = "DSR" // Desired-state resource reconciliation
	ContextSDD SystemicContext = "SDD" // SDD lifecycle and artifacts
	ContextPAD SystemicContext = "PAD" // Product admission and delivery
)

// IsValid returns true for known systemic contexts.
func (s SystemicContext) IsValid() bool {
	switch s {
	case ContextHCR, ContextMMI, ContextACI, ContextMCA,
		ContextRAR, ContextEPD, ContextDSR, ContextSDD, ContextPAD:
		return true
	}
	return false
}

// PublicationRef records whether a path exists on one authoritative ref.
type PublicationRef struct {
	Ref     string `json:"ref"`
	Present bool   `json:"present"`
}

// ReleaseClass is where one backlog item stands once the exact released SHA is known.
type ReleaseClass string

const (
	ReleaseClose             ReleaseClass = "close"
	ReleaseSuperseded        ReleaseClass = "superseded"
	ReleasePartiallyCovered  ReleaseClass = "partially_covered"
	ReleaseStillValid        ReleaseClass = "still_valid"
	ReleaseNeedsReproduction ReleaseClass = "needs_reproduction"
)

// IsValid returns true for known release classes.
func (r ReleaseClass) IsValid() bool {
	switch r {
	case ReleaseClose, ReleaseSuperseded, ReleasePartiallyCovered,
		ReleaseStillValid, ReleaseNeedsReproduction:
		return true
	}
	return false
}

// BacklogItem is one imported audit item.
type BacklogItem struct {
	Kind        string `json:"kind"`
	Number      int    `json:"number"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// BacklogEntry pairs a backlog item with its release classification.
type BacklogEntry struct {
	BacklogItem
	Release ReleaseClass `json:"release,omitempty"`
}

// Reconciliation is the counted shape of the recovered backlog.
type Reconciliation struct {
	Issues         int `json:"issues"`
	PullRequests   int `json:"pullRequests"`
	CollisionPRs   int `json:"collisionPrs"`
	Overlaps       int `json:"overlaps"`
	Decompositions int `json:"decompositions"`
}

// Row is the recovery decision for exactly one path.
type Row struct {
	Path                string           `json:"path"`
	Disposition         Disposition      `json:"disposition"`
	Context             SystemicContext  `json:"context,omitempty"`
	Invariant           string           `json:"invariant,omitempty"`
	Proof               []string         `json:"proof,omitempty"`
	Contributor         string           `json:"contributor"`
	Publication         []PublicationRef `json:"publication,omitempty"`
	EarlyDeviation      bool             `json:"earlyDeviation,omitempty"`
	DestinationPath     string           `json:"destinationPath,omitempty"`
	DestinationProof    []string         `json:"destinationProof,omitempty"`
	NoRetainedInvariant bool             `json:"noRetainedInvariant,omitempty"`
	CreatedAt           string           `json:"createdAt,omitempty"`
}

// Ledgers carries the reconciliation totals alongside the items and rows.
type Ledgers struct {
	Reconciliation Reconciliation `json:"reconciliation"`
	Backlog        []BacklogEntry `json:"backlog"`
	Rows           []Row          `json:"rows"`
}
