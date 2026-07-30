// Package review — RDD (Review-Driven Development) kill-switch.
package review

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrRDDModeRepositoryForcedOn = errors.New("cannot force RDD on via clone override. Clone overrides are off-only. To enable RDD: run 'biggz rdd enable' (defaults to global scope)")
	ErrRDDModeWorktreeForcedOn   = errors.New("cannot force RDD on via worktree override. Worktree overrides are off-only")
)

type RDDMode string

const (
	RDDModeUnset    RDDMode = ""
	RDDModeEnabled  RDDMode = "enabled"
	RDDModeDisabled RDDMode = "disabled"
)

type RDDStatusReport struct {
	Schema        string     `json:"schema"`
	EffectiveMode RDDMode    `json:"effective_mode"`
	GlobalMode    RDDMode    `json:"global_mode"`
	CloneMode     RDDMode    `json:"clone_mode"`
	WorktreeMode  RDDMode    `json:"worktree_mode"`
	Source        string     `json:"source"`
	RecordedAt    *time.Time `json:"recorded_at"`
	WorktreeCount int        `json:"worktree_count,omitempty"`
}

const rddStatusSchema = "biggz-ai.rdd-status/v1"
const rddStateFile = "rdd-mode.json"
const rddGenerationsDir = "biggz/rdd-mode"

type rddState struct {
	Schema     string  `json:"schema"`
	Mode       RDDMode `json:"mode"`
	RecordedAt string  `json:"recorded_at"`
}

// ---------------------------------------------------------------------------
// RDDOperation
// ---------------------------------------------------------------------------

type RDDOperation int

const (
	RDDOperationRead   RDDOperation = iota
	RDDOperationStart
	RDDOperationMutate
)

// RDDDisabledError is returned when an RDD operation is blocked.
type RDDDisabledError struct {
	Source    string
	Operation RDDOperation
}

func (e *RDDDisabledError) Error() string {
	switch e.Operation {
	case RDDOperationStart:
		return "review start blocked by RDD. Enable with: biggz rdd enable"
	case RDDOperationMutate:
		return "review mutation blocked by RDD. Enable with: biggz rdd enable"
	default:
		return fmt.Sprintf("RDD disabled (%v)", e.Operation)
	}
}

// AuthorizeRDDOperation checks whether the given operation is allowed for the
// RDD state. Read operations always pass; Start and Mutate are blocked when
// the effective RDD mode is disabled.
func AuthorizeRDDOperation(op RDDOperation, worktreeGitDir, commonGitDir string) error {
	if op == RDDOperationRead {
		return nil
	}
	status, err := RDDStatus(worktreeGitDir, commonGitDir)
	if err != nil {
		return err
	}
	if status.EffectiveMode == RDDModeDisabled {
		return &RDDDisabledError{Source: status.Source, Operation: op}
	}
	return nil
}

// ---------------------------------------------------------------------------
// RDD Status
// ---------------------------------------------------------------------------

// RDDStatus returns the effective RDD mode.
//
// Parameters:
//   - worktreeGitDir: result of `git rev-parse --git-dir` (per-worktree).
//     Can be empty outside a git repo.
//   - commonGitDir: result of `git rev-parse --git-common-dir` (shared).
//     For a non-worktree repo both dirs are the same; for a linked worktree
//     they differ and the worktree dir is private to that worktree.
//
// Precedence: worktree > clone > global. Any "off" at any scope wins.
func RDDStatus(worktreeGitDir, commonGitDir string) (*RDDStatusReport, error) {
	globalMode := readGlobalMode()
	worktreeMode := readRDDModeDir(worktreeGitDir)
	cloneMode := readRDDModeDir(commonGitDir)

	var recordedAt *time.Time
	var source string
	effective := RDDModeEnabled

	switch {
	case worktreeMode == RDDModeDisabled:
		effective = RDDModeDisabled
		if worktreeGitDir != commonGitDir && commonGitDir != "" {
			source = "worktree"
		} else {
			// Same dir means this is the main worktree (or non-linked clone).
			// The effective scope is "clone" since worktree == clone here.
			source = "clone"
		}
		recordedAt = parseRecordedAt(filepath.Join(worktreeGitDir, rddGenerationsDir))
	case cloneMode == RDDModeDisabled:
		effective = RDDModeDisabled
		source = "clone"
		recordedAt = parseRecordedAt(filepath.Join(commonGitDir, rddGenerationsDir))
	case globalMode == RDDModeDisabled:
		effective = RDDModeDisabled
		source = "global"
		recordedAt = parseGlobalRecordedAt()
	default:
		source = "default"
		recordedAt = parseGlobalRecordedAt()
	}

	wtCount := countLinkedWorktrees(commonGitDir)

	return &RDDStatusReport{
		Schema: rddStatusSchema, EffectiveMode: effective,
		GlobalMode: globalMode, CloneMode: cloneMode,
		WorktreeMode: worktreeMode, Source: source,
		RecordedAt: recordedAt, WorktreeCount: wtCount,
	}, nil
}

func parseGlobalRecordedAt() *time.Time {
	if p, err := globalStatePath(); err == nil {
		_, ra := readFile(p)
		if ra != "" {
			if t, err := time.Parse(time.RFC3339Nano, ra); err == nil {
				return &t
			}
		}
	}
	return nil
}

func parseRecordedAt(genDir string) *time.Time {
	gen := readLatestGeneration(genDir)
	if gen != nil && gen.RecordedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, gen.RecordedAt); err == nil {
			return &t
		}
	}
	return nil
}

// RDDEnable enables globally and clears all scope overrides.
func RDDEnable(worktreeGitDir, commonGitDir string) (*RDDStatusReport, error) {
	if err := writeGlobalMode(RDDModeEnabled); err != nil {
		return nil, err
	}
	// Clear worktree override
	clearGenerations(worktreeGitDir)
	// Clear clone override
	clearGenerations(commonGitDir)
	return RDDStatus(worktreeGitDir, commonGitDir)
}

// RDDDisable disables at the given scope: "worktree", "clone", or "global".
func RDDDisable(worktreeGitDir, commonGitDir, scope string) (*RDDStatusReport, error) {
	switch scope {
	case "worktree":
		if worktreeGitDir == "" {
			return nil, fmt.Errorf("not in a git repository — cannot use --scope=worktree")
		}
		if err := writeRDDMode(worktreeGitDir, RDDModeDisabled); err != nil {
			return nil, err
		}
	case "clone":
		if commonGitDir == "" {
			return nil, fmt.Errorf("not in a git repository — cannot use --scope=clone")
		}
		if err := writeRDDMode(commonGitDir, RDDModeDisabled); err != nil {
			return nil, err
		}
	default: // "global"
		if err := writeGlobalMode(RDDModeDisabled); err != nil {
			return nil, err
		}
	}
	return RDDStatus(worktreeGitDir, commonGitDir)
}

// clearGenerations removes the CAS generation directory for the given git dir.
func clearGenerations(gitDir string) {
	if gitDir != "" {
		os.RemoveAll(filepath.Join(gitDir, rddGenerationsDir))
	}
}

func globalStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".biggz")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, rddStateFile), nil
}

func readGlobalMode() RDDMode {
	p, err := globalStatePath()
	if err != nil {
		return RDDModeUnset
	}
	m, _ := readFile(p)
	return m
}

func writeGlobalMode(m RDDMode) error {
	p, err := globalStatePath()
	if err != nil {
		return err
	}
	return writeFile(p, m, time.Now().UTC().Format(time.RFC3339Nano))
}

// ---------------------------------------------------------------------------
// CAS Generation System (clone-local override)
// ---------------------------------------------------------------------------
//
// Clone-local RDD mode is stored as append-only generation files under
// <git-dir>/biggz/rdd-mode/gen-NNNNNNNNNN.json. Each generation records
// its schema, generation number, previous revision (for audit trail), the
// mode value, and a content-addressed revision hash (SHA-256 of all fields
// except revision).

type rddGeneration struct {
	Schema           string  `json:"schema"`
	Generation       int64   `json:"generation"`
	PreviousRevision string  `json:"previous_revision"`
	Mode             RDDMode `json:"mode"`
	RecordedAt       string  `json:"recorded_at"`
	Revision         string  `json:"revision"`
}

type rddGenerationNoRev struct {
	Schema           string  `json:"schema"`
	Generation       int64   `json:"generation"`
	PreviousRevision string  `json:"previous_revision"`
	Mode             RDDMode `json:"mode"`
	RecordedAt       string  `json:"recorded_at"`
}

func computeGenerationRevision(gen rddGeneration) string {
	data, _ := json.Marshal(rddGenerationNoRev{
		Schema:           gen.Schema,
		Generation:       gen.Generation,
		PreviousRevision: gen.PreviousRevision,
		Mode:             gen.Mode,
		RecordedAt:       gen.RecordedAt,
	})
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func readLatestGeneration(dir string) *rddGeneration {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var bestGen int64 = -1
	var bestFile string
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "gen-") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var gen int64
		if n, err := fmt.Sscanf(e.Name(), "gen-%d.json", &gen); err != nil || n != 1 {
			continue
		}
		if gen > bestGen {
			bestGen = gen
			bestFile = e.Name()
		}
	}
	if bestFile == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, bestFile))
	if err != nil {
		return nil
	}
	var gen rddGeneration
	if err := json.Unmarshal(data, &gen); err != nil {
		return nil
	}
	return &gen
}

// VerifyCloneRevision checks that the head generation's revision matches
// the expected value. Used by --expected-revision in the CLI.
func VerifyCloneRevision(gitDir, expectedRevision string) error {
	gen := readLatestGeneration(filepath.Join(gitDir, rddGenerationsDir))
	if gen == nil {
		return fmt.Errorf("no clone mode generation found")
	}
	if gen.Revision != expectedRevision {
		return fmt.Errorf("expected revision %s does not match head revision %s",
			expectedRevision, gen.Revision)
	}
	return nil
}

// readRDDModeDir reads the mode from the CAS generation directory under the
// given git directory. Returns RDDModeUnset if no generation exists.
func readRDDModeDir(gitDir string) RDDMode {
	if gitDir == "" {
		return RDDModeUnset
	}
	gen := readLatestGeneration(filepath.Join(gitDir, rddGenerationsDir))
	if gen == nil {
		return RDDModeUnset
	}
	return gen.Mode
}

// writeRDDMode writes an off-only override to the CAS generation directory
// under the given git directory (clone or worktree scope).
func writeRDDMode(gitDir string, m RDDMode) error {
	if m == RDDModeEnabled {
		return ErrRDDModeRepositoryForcedOn
	}
	genDir := filepath.Join(gitDir, rddGenerationsDir)
	if err := os.MkdirAll(genDir, 0755); err != nil {
		return err
	}
	prev := readLatestGeneration(genDir)
	var prevRev string
	var genNum int64
	if prev != nil {
		prevRev = prev.Revision
		genNum = prev.Generation + 1
	}
	gen := rddGeneration{
		Schema:           rddStatusSchema,
		Generation:       genNum,
		PreviousRevision: prevRev,
		Mode:             m,
		RecordedAt:       time.Now().UTC().Format(time.RFC3339Nano),
	}
	gen.Revision = computeGenerationRevision(gen)
	data, err := json.MarshalIndent(gen, "", "  ")
	if err != nil {
		return err
	}
	filename := fmt.Sprintf("gen-%010d.json", genNum)
	return os.WriteFile(filepath.Join(genDir, filename), data, 0644)
}

// countLinkedWorktrees returns the number of linked worktrees for the given
// git common directory. Returns 0 if not a git repo or an error occurs.
func countLinkedWorktrees(commonGitDir string) int {
	if commonGitDir == "" {
		return 0
	}
	wtDir := filepath.Join(commonGitDir, "worktrees")
	entries, err := os.ReadDir(wtDir)
	if err != nil {
		return 0
	}
	return len(entries)
}

// ---------------------------------------------------------------------------
// Legacy file I/O (global mode only)
// ---------------------------------------------------------------------------

func readFile(path string) (RDDMode, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RDDModeUnset, ""
	}
	var s rddState
	if json.Unmarshal(data, &s) != nil {
		return RDDModeUnset, ""
	}
	if s.Mode != RDDModeEnabled && s.Mode != RDDModeDisabled {
		return RDDModeUnset, ""
	}
	return s.Mode, s.RecordedAt
}

func writeFile(path string, m RDDMode, recordedAt string) error {
	data, _ := json.MarshalIndent(rddState{
		Schema:     rddStatusSchema,
		Mode:       m,
		RecordedAt: recordedAt,
	}, "", "  ")
	return os.WriteFile(path, data, 0644)
}
