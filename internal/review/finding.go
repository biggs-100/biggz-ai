package review

// FindingSeverity represents the impact level of a finding.
type FindingSeverity string

const (
	SeverityCritical FindingSeverity = "critical"
	SeverityWarning  FindingSeverity = "warning"
	SeverityInfo     FindingSeverity = "info"
)

// Classification describes how a finding relates to the change being reviewed.
type Classification string

const (
	ClassIntroduced         Classification = "introduced"          // new issue created by this change
	ClassPreExisting        Classification = "pre_existing"        // issue existed before this change
	ClassBehaviorActivated  Classification = "behavior_activated"  // latent behavior exposed by change
	ClassWorsened           Classification = "worsened"            // pre-existing issue made worse
)

// Finding represents a single observation from a lens analysis.
type Finding struct {
	ID             string         `json:"id"`
	Severity       FindingSeverity `json:"severity"`
	Message        string         `json:"message"`
	File           string         `json:"file,omitempty"`
	Line           int            `json:"line,omitempty"`
	LensID         string         `json:"lens_id"`
	Classification Classification `json:"classification,omitempty"`
	EvidenceRef    int            `json:"evidence_ref,omitempty"` // position in evidence chain
}
