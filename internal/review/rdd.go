// Package review — RDD (Review-Driven Development) kill-switch.
package review

import (
	"bytes"
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

	// ErrRDDModeCorrupt reports a persisted kill-switch record that exists but
	// cannot be read as a mode.
	ErrRDDModeCorrupt = errors.New("review mode file is corrupt")

	// ErrRDDModeRevisionMismatch is the sentinel for compare-and-set failures.
	ErrRDDModeRevisionMismatch = errors.New("rdd mode revision mismatch")

	// ErrRDDModePartiallyApplied is the sentinel for half-applied mirror writes.
	ErrRDDModePartiallyApplied = errors.New("rdd mode partially applied")
)

// RDDModeUnreadableError names the exact file holding an unreadable mode value
// and the command that overwrites it.
type RDDModeUnreadableError struct {
	Scope string // "global", "clone", or "worktree"
	File  string
	Cause error
}

func (err *RDDModeUnreadableError) Error() string {
	repair := "biggz rdd enable"
	if err.Scope == "clone" || err.Scope == "worktree" {
		repair = fmt.Sprintf("biggz rdd disable --scope=%s", err.Scope)
	}
	return fmt.Sprintf(
		"the %s review mode value in %s is not a mode this product can read: %v; an unreadable switch is not a disabled switch, so review delivery stays managed until the value is overwritten: run %s",
		err.Scope, err.File, err.Cause, repair)
}

func (err *RDDModeUnreadableError) Unwrap() error { return ErrRDDModeCorrupt }

// RDDModePartialApplyError is returned when the relocated publish succeeds but
// the pre-relocation mirror publish fails while the mirror is reachable.
type RDDModePartialApplyError struct {
	RelocatedPath string
	MirrorPath    string
	Cause         error
}

func (e *RDDModePartialApplyError) Error() string {
	return fmt.Sprintf("rdd mode partially applied: relocated %s succeeded but mirror %s failed: %v", e.RelocatedPath, e.MirrorPath, e.Cause)
}

func (e *RDDModePartialApplyError) Unwrap() error { return ErrRDDModePartiallyApplied }

func (e *RDDModePartialApplyError) Is(target error) bool {
	return target == ErrRDDModePartiallyApplied
}

type RDDMode string

const (
	RDDModeUnset    RDDMode = ""
	RDDModeEnabled  RDDMode = "enabled"
	RDDModeDisabled RDDMode = "disabled"
)

// RDDModeReach reports whether a clone-local write reached the pre-relocation
// mirror.
type RDDModeReach string

const (
	ReachUnreported RDDModeReach = ""
	ReachMachine    RDDModeReach = "machine"
	ReachThisBuild  RDDModeReach = "this_build"
)

// RDDModeSource identifies the scope that contributed the effective mode.
type RDDModeSource string

const (
	SourceDefault    RDDModeSource = "default"
	SourceGlobal     RDDModeSource = "global"
	SourceClone      RDDModeSource = "clone"       // alias, backward compat
	SourceCloneLocal RDDModeSource = "clone_local" // canonical
	SourceWorktree   RDDModeSource = "worktree"
)

// RDDModeStatus is the write result for a clone-local generation.
type RDDModeStatus struct {
	Reach      RDDModeReach `json:"reach"`
	Revision   string       `json:"revision"`
	Generation int64        `json:"generation"`
	RecordedAt string       `json:"recorded_at"`
}

type RDDStatusReport struct {
	Schema        string       `json:"schema"`
	EffectiveMode RDDMode      `json:"effective_mode"`
	GlobalMode    RDDMode      `json:"global_mode"`
	CloneMode     RDDMode      `json:"clone_mode"`
	WorktreeMode  RDDMode      `json:"worktree_mode"`
	Source        string       `json:"source"`
	Revision      string       `json:"revision"`
	Reach         RDDModeReach `json:"reach"`
	RecordedAt    *time.Time   `json:"recorded_at"`
	WorktreeCount int          `json:"worktree_count,omitempty"`
}

const rddStatusSchema = "biggz-ai.rdd-status/v1"
const rddStateFile = "rdd-mode.json"
const rddGenerationsDir = "biggz/rdd-mode"

// RDDModeOverrideDigestDomain is the domain for generation revision hashing.
const RDDModeOverrideDigestDomain = "biggz-ai.rdd-mode-override-digest/v1"

const maxGeneration int64 = 999999999

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
	RDDOperationRead RDDOperation = iota
	RDDOperationStart
	RDDOperationMutate
)

// reviewModeEnableForSource returns the exact runnable continuation for the
// given source.
func reviewModeEnableForSource(source RDDModeSource) string {
	switch source {
	case SourceDefault, SourceGlobal:
		return "biggz rdd enable --scope=global"
	case SourceClone, SourceCloneLocal, SourceWorktree:
		return "biggz rdd enable --scope=global then biggz rdd enable --scope=clone"
	default:
		// Normalize unknown clone alias variants.
		s := string(source)
		if s == "clone" || s == "clone_local" || s == "worktree" {
			return "biggz rdd enable --scope=global then biggz rdd enable --scope=clone"
		}
		return "biggz rdd enable --scope=global"
	}
}

func rddOperationSubject(op RDDOperation) string {
	switch op {
	case RDDOperationStart:
		return "review start"
	case RDDOperationMutate:
		return "review mutation"
	default:
		return "RDD operation"
	}
}

// RDDDisabledError is returned when an RDD operation is blocked.
type RDDDisabledError struct {
	Source    RDDModeSource
	Operation RDDOperation
}

func (e *RDDDisabledError) Error() string {
	cmd := reviewModeEnableForSource(e.Source)
	subj := rddOperationSubject(e.Operation)
	switch e.Operation {
	case RDDOperationStart:
		return fmt.Sprintf("%s blocked by RDD (source: %s). Enable with: %s", subj, e.Source, cmd)
	case RDDOperationMutate:
		return fmt.Sprintf("%s blocked by RDD (source: %s). Enable with: %s; the review is frozen, not discarded; to continue it from where it stopped, run %s", subj, e.Source, cmd, cmd)
	default:
		return fmt.Sprintf("RDD disabled (%v) source %s. Enable with: %s", e.Operation, e.Source, cmd)
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
		src := RDDModeSource(status.Source)
		// Default resolves as global for enable-path wording (opt-in semantics).
		if src == SourceDefault {
			src = SourceGlobal
		}
		return &RDDDisabledError{Source: src, Operation: op}
	}
	return nil
}

// ---------------------------------------------------------------------------
// RDD Status
// ---------------------------------------------------------------------------

// RDDStatus returns the effective RDD mode.
//
// Precedence: worktree > clone > global. Any "off" at any scope wins.
// Revision is the head's Revision CAS token ("" if none/default).
// Reach is always ReachUnreported on reads (no mirror probe).
func RDDStatus(worktreeGitDir, commonGitDir string) (*RDDStatusReport, error) {
	globalMode, globalErr := readGlobalMode()

	// Read worktree and clone generations directly for revision tracking.
	var worktreeGen *rddGeneration
	var worktreeGenErr error
	var cloneGen *rddGeneration
	var cloneGenErr error

	if worktreeGitDir != "" {
		worktreeGen, worktreeGenErr = readLatestGeneration(filepath.Join(worktreeGitDir, rddGenerationsDir))
	}
	if commonGitDir != "" {
		// Avoid double-read when worktree == common (main worktree or non-linked clone).
		if worktreeGitDir == commonGitDir && worktreeGenErr == nil && worktreeGen != nil {
			cloneGen = worktreeGen
			cloneGenErr = worktreeGenErr
		} else if worktreeGitDir == commonGitDir && worktreeGenErr != nil {
			cloneGenErr = worktreeGenErr
		} else {
			cloneGen, cloneGenErr = readLatestGeneration(filepath.Join(commonGitDir, rddGenerationsDir))
		}
	}

	var worktreeMode, cloneMode RDDMode
	var worktreeErr, cloneErr error

	if worktreeGenErr != nil {
		worktreeErr = worktreeGenErr
		worktreeMode = RDDModeUnset
	} else if worktreeGen != nil {
		worktreeMode = worktreeGen.Mode
	} else {
		worktreeMode = RDDModeUnset
	}

	if cloneGenErr != nil {
		cloneErr = cloneGenErr
		cloneMode = RDDModeUnset
	} else if cloneGen != nil {
		cloneMode = cloneGen.Mode
	} else {
		cloneMode = RDDModeUnset
	}

	var recordedAt *time.Time
	var source string
	var revision string
	effective := RDDModeEnabled

	switch {
	case worktreeMode == RDDModeDisabled:
		effective = RDDModeDisabled
		if worktreeGitDir != commonGitDir && commonGitDir != "" {
			source = "worktree"
		} else {
			source = "clone"
		}
		if worktreeGen != nil {
			revision = worktreeGen.Revision
			if t, err := time.Parse(time.RFC3339Nano, worktreeGen.RecordedAt); err == nil {
				recordedAt = &t
			}
		}
	case cloneMode == RDDModeDisabled:
		effective = RDDModeDisabled
		source = "clone"
		if cloneGen != nil {
			revision = cloneGen.Revision
			if t, err := time.Parse(time.RFC3339Nano, cloneGen.RecordedAt); err == nil {
				recordedAt = &t
			}
		}
	case globalMode == RDDModeDisabled:
		effective = RDDModeDisabled
		source = "global"
		recordedAt = parseGlobalRecordedAt()
	default:
		source = "default"
		recordedAt = parseGlobalRecordedAt()
	}

	wtCount := countLinkedWorktrees(commonGitDir)

	report := &RDDStatusReport{
		Schema: rddStatusSchema, EffectiveMode: effective,
		GlobalMode: globalMode, CloneMode: cloneMode,
		WorktreeMode: worktreeMode, Source: source,
		Revision: revision, Reach: ReachUnreported,
		RecordedAt: recordedAt, WorktreeCount: wtCount,
	}

	// Fail closed on any unreadable source: collect every corrupt record so
	// the caller can name them all. Dedupe by exact file.
	seen := make(map[string]struct{}, 3)
	var corrupt []error
	for _, scopeErr := range []error{worktreeErr, cloneErr, globalErr} {
		if scopeErr == nil {
			continue
		}
		if unreadable, ok := scopeErr.(*RDDModeUnreadableError); ok {
			if _, duplicate := seen[unreadable.File]; duplicate {
				continue
			}
			seen[unreadable.File] = struct{}{}
		}
		corrupt = append(corrupt, scopeErr)
	}
	if len(corrupt) > 0 {
		return report, errors.Join(corrupt...)
	}
	return report, nil
}

func parseGlobalRecordedAt() *time.Time {
	if p, err := globalStatePath(); err == nil {
		_, ra, _ := readFile(p)
		if ra != "" {
			if t, err := time.Parse(time.RFC3339Nano, ra); err == nil {
				return &t
			}
		}
	}
	return nil
}

func parseRecordedAt(genDir string) *time.Time {
	gen, _ := readLatestGeneration(genDir)
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
	// Clear clone override (avoid double-clear when same dir)
	if commonGitDir != worktreeGitDir {
		clearGenerations(commonGitDir)
	}
	return RDDStatus(worktreeGitDir, commonGitDir)
}

// RDDDisable disables at the given scope: "worktree", "clone", or "global".
func RDDDisable(worktreeGitDir, commonGitDir, scope string) (*RDDStatusReport, error) {
	return RDDDisableWithRevision(worktreeGitDir, commonGitDir, scope, "")
}

// RDDDisableWithRevision disables with CAS expectedRevision for clone/worktree.
func RDDDisableWithRevision(worktreeGitDir, commonGitDir, scope, expectedRevision string) (*RDDStatusReport, error) {
	switch scope {
	case "worktree":
		if worktreeGitDir == "" {
			return nil, fmt.Errorf("not in a git repository — cannot use --scope=worktree")
		}
		if _, err := SetWorktreeRDDMode(worktreeGitDir, RDDModeDisabled, expectedRevision); err != nil {
			return nil, err
		}
	case "clone":
		if commonGitDir == "" {
			return nil, fmt.Errorf("not in a git repository — cannot use --scope=clone")
		}
		if _, err := SetCloneLocalRDDMode(worktreeGitDir, commonGitDir, RDDModeDisabled, expectedRevision); err != nil {
			return nil, err
		}
	default: // "global"
		if expectedRevision != "" {
			return nil, fmt.Errorf("expected-revision is only supported for --scope=clone and --scope=worktree")
		}
		if err := writeGlobalMode(RDDModeDisabled); err != nil {
			return nil, err
		}
	}
	return RDDStatus(worktreeGitDir, commonGitDir)
}

// plausibleGitDir reports whether path looks like a genuine git directory.
func plausibleGitDir(path string) bool {
	if path == "" {
		return false
	}
	if fi, err := os.Stat(filepath.Join(path, "HEAD")); err != nil || fi.IsDir() {
		return false
	}
	for _, marker := range []string{"objects", "refs"} {
		if fi, err := os.Stat(filepath.Join(path, marker)); err == nil && fi.IsDir() {
			return true
		}
	}
	for _, marker := range []string{"commondir", "gitdir"} {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}
	return false
}

// clearGenerations removes the CAS generation directory for the given git dir.
func clearGenerations(gitDir string) {
	if gitDir != "" && plausibleGitDir(gitDir) {
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

func readGlobalMode() (RDDMode, error) {
	p, err := globalStatePath()
	if err != nil {
		return RDDModeUnset, err
	}
	m, _, err := readFile(p)
	return m, err
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
	genStr := fmt.Sprintf("%010d", gen.Generation)
	payload := writeLengthPrefixed(
		[]byte(gen.Schema),
		[]byte(genStr),
		[]byte(gen.PreviousRevision),
		[]byte(string(gen.Mode)),
		[]byte(gen.RecordedAt),
	)
	return domainHash(RDDModeOverrideDigestDomain, payload)
}

// scanGenerationHead lists the generation directory and reports the highest
// generation slot and its file name without reading or parsing the record.
func scanGenerationHead(dir string) (bestGen int64, bestFile string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return -1, "", nil
		}
		return -1, "", err
	}
	bestGen = -1
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
	return bestGen, bestFile, nil
}

// readLatestGeneration reads and parses the head generation record.
func readLatestGeneration(dir string) (*rddGeneration, error) {
	_, bestFile, err := scanGenerationHead(dir)
	if err != nil {
		return nil, err
	}
	if bestFile == "" {
		return nil, nil
	}
	path := filepath.Join(dir, bestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &RDDModeUnreadableError{Scope: "clone", File: path, Cause: err}
	}
	var gen rddGeneration
	if err := json.Unmarshal(data, &gen); err != nil {
		return nil, &RDDModeUnreadableError{Scope: "clone", File: path, Cause: err}
	}
	if gen.Mode != RDDModeDisabled {
		return nil, &RDDModeUnreadableError{Scope: "clone", File: path, Cause: fmt.Errorf("unexpected mode value %q in an off-only override", gen.Mode)}
	}
	if gen.Revision != computeGenerationRevision(gen) {
		return nil, &RDDModeUnreadableError{Scope: "clone", File: path, Cause: errors.New("content-addressed revision does not match the record")}
	}
	return &gen, nil
}

// VerifyCloneRevision checks that the head generation's revision matches
// the expected value. Used by --expected-revision in the CLI (advisory, not
// authoritative; the authoritative CAS check is inside SetCloneLocalRDDMode under LOCK).
func VerifyCloneRevision(gitDir, expectedRevision string) error {
	gen, err := readLatestGeneration(filepath.Join(gitDir, rddGenerationsDir))
	if err != nil {
		return err
	}
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
// given git directory. Returns (RDDModeUnset, nil) if no generation exists.
func readRDDModeDir(gitDir string) (RDDMode, error) {
	if gitDir == "" {
		return RDDModeUnset, nil
	}
	gen, err := readLatestGeneration(filepath.Join(gitDir, rddGenerationsDir))
	if err != nil {
		return RDDModeUnset, err
	}
	if gen == nil {
		return RDDModeUnset, nil
	}
	return gen.Mode, nil
}

// writeRDDMode writes an off-only override without CAS (legacy wrapper).
func writeRDDMode(gitDir string, m RDDMode) error {
	_, err := writeRDDModeCAS(gitDir, m, "")
	return err
}

// writeRDDModeCAS is the internal CAS-aware writer with LOCK.
// It is used by writeRDDMode (empty expectedRevision). Mirror handling is
// done by SetCloneLocalRDDMode; this helper only publishes the relocated root.
func writeRDDModeCAS(gitDir string, m RDDMode, expectedRevision string) (*RDDModeStatus, error) {
	if m == RDDModeEnabled {
		return nil, ErrRDDModeRepositoryForcedOn
	}
	if !plausibleGitDir(gitDir) {
		return nil, fmt.Errorf(
			"%s is not a git directory (missing HEAD or objects/refs): refusing to write review mode state there; run from inside a repository or use 'biggz rdd disable --scope=global'",
			gitDir)
	}
	genDir := filepath.Join(gitDir, rddGenerationsDir)
	if err := os.MkdirAll(genDir, 0755); err != nil {
		return nil, err
	}

	var result *RDDModeStatus
	var writeErr error

	fl := NewNamedFileLock(genDir, "LOCK")
	if err := fl.AcquireWithTimeout(5 * time.Second); err != nil {
		return nil, fmt.Errorf("file lock acquire: %w", err)
	}
	defer fl.Release()

	// Hold LOCK across scan→read→CAS→compute→publish.
	headGen, _, scanErr := scanGenerationHead(genDir)
	if scanErr != nil {
		return nil, scanErr
	}
	head, headErr := readLatestGeneration(genDir)
	isCorrupt := headErr != nil && errors.Is(headErr, ErrRDDModeCorrupt)

	// CAS validation (only when caller supplies expectedRevision).
	if isCorrupt {
		if expectedRevision != "" {
			return nil, fmt.Errorf("%w: expected %q but head is %q", ErrRDDModeRevisionMismatch, expectedRevision, "")
		}
	} else if head == nil {
		if expectedRevision != "" {
			return nil, fmt.Errorf("%w: expected %q but head is %q", ErrRDDModeRevisionMismatch, expectedRevision, "")
		}
	} else {
		if expectedRevision != "" && expectedRevision != head.Revision {
			return nil, fmt.Errorf("%w: expected %q but head is %q", ErrRDDModeRevisionMismatch, expectedRevision, head.Revision)
		}
	}

	var genNum int64
	var prevRev string
	switch {
	case isCorrupt:
		// Repair: overwrite exact corrupt slot.
		if headGen >= 0 {
			genNum = headGen
		} else {
			genNum = 0
		}
		prevRev = ""
	case head != nil:
		genNum = head.Generation + 1
		prevRev = head.Revision
	default:
		genNum = 0
		prevRev = ""
	}

	if genNum > maxGeneration {
		return nil, fmt.Errorf("rdd generation exhausted: %d exceeds max %d", genNum, maxGeneration)
	}
	if genNum < 0 {
		genNum = 0
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
		return nil, err
	}
	filename := fmt.Sprintf("gen-%010d.json", genNum)
	relocatedPath := filepath.Join(genDir, filename)
	if isCorrupt {
		if err := os.WriteFile(relocatedPath, data, 0644); err != nil {
			return nil, err
		}
		_ = SyncReviewDirectory(filepath.Dir(relocatedPath))
	} else {
		if err := rddPublishImmutable(relocatedPath, data); err != nil {
			return nil, err
		}
	}

	result = &RDDModeStatus{
		Reach:      ReachThisBuild,
		Revision:   gen.Revision,
		Generation: genNum,
		RecordedAt: gen.RecordedAt,
	}
	// Mirror publishing is handled by SetCloneLocalRDDMode which knows
	// commonGitDir. This helper alone reports ThisBuild; the caller upgrades
	// to Machine when mirror succeeds.
	writeErr = nil
	_ = writeErr
	return result, nil
}

// rddPublishImmutable publishes payload at path with O_CREATE|O_EXCL semantics.
// If the file already exists with identical bytes it is idempotent; if it
// exists with different bytes it fails. The file's directory is synced.
func rddPublishImmutable(path string, payload []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			existing, readErr := os.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("publish immutable: %s exists but cannot read: %w", path, readErr)
			}
			if bytes.Equal(existing, payload) {
				return nil
			}
			return fmt.Errorf("publish immutable: %s exists with different content", path)
		}
		return err
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(path)
		}
	}()
	if _, err = f.Write(payload); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	f.Close()
	_ = SyncReviewDirectory(filepath.Dir(path))
	return nil
}

// SetCloneLocalRDDMode writes a clone-local override with CAS, LOCK, and
// pre-relocation mirror. The worktreeGitDir is unused for the clone write
// but kept for API parity with gentle's SetCloneLocalRDDMode.
func SetCloneLocalRDDMode(worktreeGitDir, commonGitDir string, mode RDDMode, expectedRevision string) (*RDDModeStatus, error) {
	_ = worktreeGitDir
	genDir, mirrorDir, err := validateCloneLocalPreconditions(commonGitDir, mode)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(genDir, 0755); err != nil {
		return nil, err
	}
	fl := NewNamedFileLock(genDir, "LOCK")
	if err := fl.AcquireWithTimeout(5 * time.Second); err != nil {
		return nil, fmt.Errorf("file lock acquire: %w", err)
	}
	defer fl.Release()

	headGen, head, isCorrupt, err := readRDDHead(genDir)
	if err != nil {
		return nil, err
	}
	if err := validateRDDCAS(head, isCorrupt, expectedRevision); err != nil {
		return nil, err
	}
	genNum, prevRev, err := computeNextRDDGeneration(headGen, head, isCorrupt)
	if err != nil {
		return nil, err
	}
	gen, data, filename, err := buildRDDGeneration(genNum, prevRev, mode)
	if err != nil {
		return nil, err
	}
	relocatedPath, err := publishRelocatedGeneration(genDir, filename, data, isCorrupt)
	if err != nil {
		return nil, err
	}
	// Relocated first, mirror second (#2882 ordering).
	mirrorReachable := probeMirrorReachable(mirrorDir)
	return publishCloneMirror(mirrorDir, filename, data, relocatedPath, gen, genNum, mirrorReachable)
}

// SetWorktreeRDDMode writes a worktree-private override with CAS and LOCK.
// No mirror is published for worktree scope.
func SetWorktreeRDDMode(worktreeGitDir string, mode RDDMode, expectedRevision string) (*RDDModeStatus, error) {
	if worktreeGitDir == "" {
		return nil, fmt.Errorf("not in a git repository — cannot use --scope=worktree")
	}
	if mode == RDDModeEnabled {
		return nil, ErrRDDModeWorktreeForcedOn
	}
	if !plausibleGitDir(worktreeGitDir) {
		return nil, fmt.Errorf(
			"%s is not a git directory (missing HEAD or objects/refs): refusing to write review mode state there; run from inside a repository or use 'biggz rdd disable --scope=global'",
			worktreeGitDir)
	}
	genDir := filepath.Join(worktreeGitDir, rddGenerationsDir)
	if err := os.MkdirAll(genDir, 0755); err != nil {
		return nil, err
	}

	fl := NewNamedFileLock(genDir, "LOCK")
	if err := fl.AcquireWithTimeout(5 * time.Second); err != nil {
		return nil, fmt.Errorf("file lock acquire: %w", err)
	}
	defer fl.Release()

	headGen, _, scanErr := scanGenerationHead(genDir)
	if scanErr != nil {
		return nil, scanErr
	}
	head, headErr := readLatestGeneration(genDir)
	isCorrupt := headErr != nil && errors.Is(headErr, ErrRDDModeCorrupt)
	if headErr != nil && !isCorrupt {
		return nil, headErr
	}
	if isCorrupt {
		if expectedRevision != "" {
			return nil, fmt.Errorf("%w: expected %q but head is %q", ErrRDDModeRevisionMismatch, expectedRevision, "")
		}
	} else if head == nil {
		if expectedRevision != "" {
			return nil, fmt.Errorf("%w: expected %q but head is %q", ErrRDDModeRevisionMismatch, expectedRevision, "")
		}
	} else {
		if expectedRevision != "" && expectedRevision != head.Revision {
			return nil, fmt.Errorf("%w: expected %q but head is %q", ErrRDDModeRevisionMismatch, expectedRevision, head.Revision)
		}
	}

	var genNum int64
	var prevRev string
	switch {
	case isCorrupt:
		if headGen >= 0 {
			genNum = headGen
		} else {
			genNum = 0
		}
		prevRev = ""
	case head != nil:
		genNum = head.Generation + 1
		prevRev = head.Revision
	default:
		genNum = 0
		prevRev = ""
	}
	if genNum > maxGeneration {
		return nil, fmt.Errorf("rdd generation exhausted: %d exceeds max %d", genNum, maxGeneration)
	}
	if genNum < 0 {
		genNum = 0
	}

	gen := rddGeneration{
		Schema:           rddStatusSchema,
		Generation:       genNum,
		PreviousRevision: prevRev,
		Mode:             mode,
		RecordedAt:       time.Now().UTC().Format(time.RFC3339Nano),
	}
	gen.Revision = computeGenerationRevision(gen)
	data, err := json.MarshalIndent(gen, "", "  ")
	if err != nil {
		return nil, err
	}
	filename := fmt.Sprintf("gen-%010d.json", genNum)
	relocatedPath := filepath.Join(genDir, filename)
	if isCorrupt {
		if err := os.WriteFile(relocatedPath, data, 0644); err != nil {
			return nil, err
		}
		_ = SyncReviewDirectory(filepath.Dir(relocatedPath))
	} else {
		if err := rddPublishImmutable(relocatedPath, data); err != nil {
			return nil, err
		}
	}
	// Worktree has no mirror; report ThisBuild as single-root reach (not Machine).
	return &RDDModeStatus{
		Reach:      ReachThisBuild,
		Revision:   gen.Revision,
		Generation: genNum,
		RecordedAt: gen.RecordedAt,
	}, nil
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

func readFile(path string) (RDDMode, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RDDModeUnset, "", nil
		}
		return RDDModeUnset, "", err
	}
	var s rddState
	if json.Unmarshal(data, &s) != nil {
		return RDDModeUnset, "", &RDDModeUnreadableError{Scope: "global", File: path, Cause: errors.New("the file is not valid review mode JSON")}
	}
	if s.Mode != RDDModeEnabled && s.Mode != RDDModeDisabled {
		return RDDModeUnset, "", &RDDModeUnreadableError{Scope: "global", File: path, Cause: fmt.Errorf("unknown mode value %q", s.Mode)}
	}
	return s.Mode, s.RecordedAt, nil
}

func writeFile(path string, m RDDMode, recordedAt string) error {
	data, _ := json.MarshalIndent(rddState{
		Schema:     rddStatusSchema,
		Mode:       m,
		RecordedAt: recordedAt,
	}, "", "  ")
	return os.WriteFile(path, data, 0644)
}


