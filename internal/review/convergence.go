package review

import (
	"crypto/sha256"
	"fmt"
	"os/exec"
	"strings"
)

// ─── Verification Convergence ────────────────────────────────────────────────
//
// Verification convergence ensures that the workspace state did not change
// during verification. If a verifier modifies files, the post-verification
// tree will differ from the pre-verification one, and the convergence check
// detects this and returns an error.

// VerificationSubject captures the Git tree state being verified.
type VerificationSubject struct {
	Kind          string `json:"kind"`       // "current-changes", "base-diff", "fix-diff"
	Projection    string `json:"projection"` // "workspace", "staged"
	BaseTree      string `json:"base_tree"`
	CandidateTree string `json:"candidate_tree"`
	TreeHash      string `json:"tree_hash"` // SHA-256 of all tree OIDs
}

// SnapshotVerificationSubject captures the current Git tree state.
func SnapshotVerificationSubject(kind, projection string) (*VerificationSubject, error) {
	subject := &VerificationSubject{
		Kind:       kind,
		Projection: projection,
	}

	// Capture base tree (HEAD)
	baseCmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
	if out, err := baseCmd.Output(); err == nil {
		subject.BaseTree = strings.TrimSpace(string(out))
	}

	// Capture candidate tree (current state)
	var candCmd *exec.Cmd
	if projection == "staged" {
		candCmd = exec.Command("git", "write-tree")
	} else {
		// workspace: staged + unstaged
		candCmd = exec.Command("git", "rev-parse", "HEAD^{tree}")
	}
	if out, err := candCmd.Output(); err == nil {
		subject.CandidateTree = strings.TrimSpace(string(out))
	}

	// Compute tree hash
	h := sha256.New()
	h.Write([]byte(subject.Kind))
	h.Write([]byte(subject.Projection))
	h.Write([]byte(subject.BaseTree))
	h.Write([]byte(subject.CandidateTree))
	subject.TreeHash = fmt.Sprintf("%x", h.Sum(nil))

	return subject, nil
}

// ResnapshotVerificationSubject rebuilds the subject and checks for convergence.
// Returns nil if the subject hasn't changed, or an error describing the mutation.
func (s *VerificationSubject) Resnapshot() error {
	current, err := SnapshotVerificationSubject(s.Kind, s.Projection)
	if err != nil {
		return fmt.Errorf("resnapshot: %w", err)
	}

	if current.TreeHash != s.TreeHash {
		// Detect what changed
		diffCmd := exec.Command("git", "diff", "--name-only", s.BaseTree, current.CandidateTree)
		diffOut, _ := diffCmd.Output()
		changedFiles := strings.TrimSpace(string(diffOut))

		return &VerificationSubjectMutationError{
			ExpectedTreeHash: s.TreeHash,
			ActualTreeHash:   current.TreeHash,
			ChangedFiles:     changedFiles,
			Message:          fmt.Sprintf("workspace mutated during verification: tree hash changed from %s to %s", s.TreeHash[:12], current.TreeHash[:12]),
		}
	}
	return nil
}

// VerificationSubjectMutationError is returned when the workspace changed during verification.
type VerificationSubjectMutationError struct {
	ExpectedTreeHash string
	ActualTreeHash   string
	ChangedFiles     string
	Message          string
}

func (e *VerificationSubjectMutationError) Error() string {
	return e.Message
}
