package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func validateCloneLocalPreconditions(commonGitDir string, mode RDDMode) (string, string, error) {
	if commonGitDir == "" {
		return "", "", fmt.Errorf("not in a git repository — cannot use --scope=clone")
	}
	if mode == RDDModeEnabled {
		return "", "", ErrRDDModeRepositoryForcedOn
	}
	if !plausibleGitDir(commonGitDir) {
		return "", "", fmt.Errorf(
			"%s is not a git directory (missing HEAD or objects/refs): refusing to write review mode state there; run from inside a repository or use 'biggz rdd disable --scope=global'",
			commonGitDir)
	}
	genDir := filepath.Join(commonGitDir, rddGenerationsDir)
	mirrorDir := filepath.Join(commonGitDir, "gentle-ai", "rdd-mode")
	return genDir, mirrorDir, nil
}

func readRDDHead(genDir string) (int64, *rddGeneration, bool, error) {
	headGen, _, scanErr := scanGenerationHead(genDir)
	if scanErr != nil {
		return -1, nil, false, scanErr
	}
	head, headErr := readLatestGeneration(genDir)
	isCorrupt := headErr != nil && errors.Is(headErr, ErrRDDModeCorrupt)
	if headErr != nil && !isCorrupt {
		return headGen, nil, false, headErr
	}
	return headGen, head, isCorrupt, nil
}

func validateRDDCAS(head *rddGeneration, isCorrupt bool, expectedRevision string) error {
	if isCorrupt {
		if expectedRevision != "" {
			return fmt.Errorf("%w: expected %q but head is %q", ErrRDDModeRevisionMismatch, expectedRevision, "")
		}
		return nil
	}
	if head == nil {
		if expectedRevision != "" {
			return fmt.Errorf("%w: expected %q but head is %q", ErrRDDModeRevisionMismatch, expectedRevision, "")
		}
		return nil
	}
	if expectedRevision != "" && expectedRevision != head.Revision {
		return fmt.Errorf("%w: expected %q but head is %q", ErrRDDModeRevisionMismatch, expectedRevision, head.Revision)
	}
	return nil
}

func computeNextRDDGeneration(headGen int64, head *rddGeneration, isCorrupt bool) (int64, string, error) {
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
		return 0, "", fmt.Errorf("rdd generation exhausted: %d exceeds max %d", genNum, maxGeneration)
	}
	if genNum < 0 {
		genNum = 0
	}
	return genNum, prevRev, nil
}

func buildRDDGeneration(genNum int64, prevRev string, mode RDDMode) (rddGeneration, []byte, string, error) {
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
		return rddGeneration{}, nil, "", err
	}
	filename := fmt.Sprintf("gen-%010d.json", genNum)
	return gen, data, filename, nil
}

func publishRelocatedGeneration(genDir, filename string, data []byte, isCorrupt bool) (string, error) {
	relocatedPath := filepath.Join(genDir, filename)
	if isCorrupt {
		if err := os.WriteFile(relocatedPath, data, 0644); err != nil {
			return "", err
		}
		_ = SyncReviewDirectory(filepath.Dir(relocatedPath))
		return relocatedPath, nil
	}
	if err := rddPublishImmutable(relocatedPath, data); err != nil {
		return "", err
	}
	return relocatedPath, nil
}

func probeMirrorReachable(mirrorDir string) bool {
	if info, statErr := os.Stat(mirrorDir); statErr == nil && info.IsDir() {
		return true
	}
	if mkErr := os.MkdirAll(mirrorDir, 0755); mkErr == nil {
		if info2, err2 := os.Stat(mirrorDir); err2 == nil && info2.IsDir() {
			return true
		}
	}
	return false
}

func publishCloneMirror(mirrorDir, filename string, data []byte, relocatedPath string, gen rddGeneration, genNum int64, mirrorReachable bool) (*RDDModeStatus, error) {
	if !mirrorReachable {
		return &RDDModeStatus{
			Reach:      ReachThisBuild,
			Revision:   gen.Revision,
			Generation: genNum,
			RecordedAt: gen.RecordedAt,
		}, nil
	}
	mirrorPath := filepath.Join(mirrorDir, filename)
	if err := rddPublishImmutable(mirrorPath, data); err != nil {
		return &RDDModeStatus{
			Reach:      ReachThisBuild,
			Revision:   gen.Revision,
			Generation: genNum,
			RecordedAt: gen.RecordedAt,
		}, &RDDModePartialApplyError{
			RelocatedPath: relocatedPath,
			MirrorPath:    mirrorPath,
			Cause:         err,
		}
	}
	return &RDDModeStatus{
		Reach:      ReachMachine,
		Revision:   gen.Revision,
		Generation: genNum,
		RecordedAt: gen.RecordedAt,
	}, nil
}
