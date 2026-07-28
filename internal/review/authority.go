package review

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/biggz-ai/biggz/model"
)

// AuthorityVerifier validates that a review receipt matches the repository state.
type AuthorityVerifier struct {
	RepoRoot string
}

// AuthorityResult describes the outcome of an authority check.
type AuthorityResult struct {
	Valid      bool   `json:"valid"`
	Reason     string `json:"reason"`
	ReviewID   string `json:"review_id,omitempty"`
	MerkleRoot string `json:"merkle_root,omitempty"`
	GitCommit  string `json:"git_commit,omitempty"`
	GitBranch  string `json:"git_branch,omitempty"`
}

// Verify checks that the receipt is valid for the given review state.
func (av *AuthorityVerifier) Verify(receipt *Receipt, state *model.ReviewState) *AuthorityResult {
	if receipt == nil {
		return &AuthorityResult{Valid: false, Reason: "no receipt provided"}
	}
	if state == nil {
		return &AuthorityResult{Valid: false, Reason: "no review state provided"}
	}
	if receipt.ReviewID != state.ID {
		return &AuthorityResult{Valid: false, Reason: fmt.Sprintf("receipt ReviewID %q does not match state %q", receipt.ReviewID, state.ID)}
	}
	if receipt.MerkleRoot != state.MerkleRoot {
		return &AuthorityResult{Valid: false, Reason: "receipt MerkleRoot does not match state"}
	}

	result := &AuthorityResult{
		Valid:      true,
		ReviewID:   state.ID,
		MerkleRoot: state.MerkleRoot,
		Reason:     "receipt is valid",
	}

	if av.RepoRoot != "" {
		result.GitCommit, result.GitBranch = av.gitState()
	}
	return result
}

func (av *AuthorityVerifier) gitState() (commit, branch string) {
	c, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err == nil {
		commit = strings.TrimSpace(string(c))
	}
	b, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err == nil {
		branch = strings.TrimSpace(string(b))
	}
	return
}
