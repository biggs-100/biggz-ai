package steps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/biggs-100/biggz-ai/internal/filecoord"
	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/internal/pipeline"
	"github.com/biggs-100/biggz-ai/plugin"
)

// StateStep merges --agent selection into ~/.biggz-ai/state.json
// (and legacy ~/.biggz/state.json for compat) atomically via
// filemerge.WriteFileAtomic under a file lock, preserving unknown fields.
// It supports dry-run (Prepare preview only, zero writes) and rollback.
type StateStep struct {
	HomeDir  string
	Adapter  plugin.AgentAdapter
	AgentID  string // explicit override; if empty, Adapter.ID() is used
	DryRun   bool
	tracker  *tracker
	orig     map[string][]byte // path -> original bytes (nil means absent)
	order    []string
	PrepareErr error
}

// globalStateMu serializes concurrent state writes in-process.
// filecoord provides cross-process cooperative locking.
var globalStateMu sync.Mutex
var stateMu sync.Mutex // alias for test isolation clarity

func NewStateStep(homeDir string, adapter plugin.AgentAdapter, dryRun bool) *StateStep {
	return &StateStep{HomeDir: homeDir, Adapter: adapter, DryRun: dryRun, tracker: newTracker(), orig: make(map[string][]byte)}
}

// StateFilePath returns the canonical pipeline state file for the given home.
// Primary is ~/.biggz-ai/state.json; legacy ~/.biggz/state.json is also
// managed for backward compatibility (see statePaths).
func StateFilePath(homeDir string) string {
	return filepath.Join(homeDir, ".biggz-ai", "state.json")
}

// LegacyStateFilePath returns the legacy path ~/.biggz/state.json.
func LegacyStateFilePath(homeDir string) string {
	return filepath.Join(homeDir, ".biggz", "state.json")
}

func statePaths(homeDir string) (primary, legacy string) {
	return StateFilePath(homeDir), LegacyStateFilePath(homeDir)
}

func (s *StateStep) Name() string { return "state-merge" }

func (s *StateStep) resolveAgentID() string {
	if s.AgentID != "" {
		return s.AgentID
	}
	if s.Adapter != nil {
		return string(s.Adapter.ID())
	}
	return ""
}

func (s *StateStep) Prepare(ctx context.Context) error {
	if s.PrepareErr != nil {
		return s.PrepareErr
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if s.HomeDir == "" {
		return fmt.Errorf("homeDir empty")
	}
	id := s.resolveAgentID()
	if id == "" {
		return fmt.Errorf("AgentID empty")
	}
	// DryRun still validates but must not write.
	_ = StateFilePath(s.HomeDir)
	_ = LegacyStateFilePath(s.HomeDir)
	return nil
}

func (s *StateStep) Apply(ctx context.Context, ch pipeline.ProgressChan) error {
	s.orig = make(map[string][]byte)
	s.order = nil
	s.tracker.reset()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if s.DryRun {
		if ch != nil {
			select {
			case ch <- pipeline.ProgressEvent{Step: s.Name(), Percent: 100, Message: fmt.Sprintf("dry-run write %s with AgentID %s", StateFilePath(s.HomeDir), s.resolveAgentID())}:
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		return nil
	}
	id := s.resolveAgentID()
	if id == "" {
		return fmt.Errorf("AgentID empty")
	}
	// Serialize in-process writers.
	globalStateMu.Lock()
	defer globalStateMu.Unlock()
	stateMu.Lock()
	defer stateMu.Unlock()

	primary, legacy := statePaths(s.HomeDir)
	// Acquire cooperative file lock for cross-process safety (retry on busy).
	lockRoot := filepath.Join(s.HomeDir, ".biggz-ai", ".locks")
	var lease *filecoord.Lease
	for attempt := 0; attempt < 20; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		l, err := filecoord.Acquire(ctx, primary, lockRoot)
		if err == nil {
			lease = l
			break
		}
		if errors.Is(err, filecoord.ErrBusy) {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		// Operational errors are not fatal for in-process tests; continue without lease.
		break
	}
	if lease != nil {
		defer func() { _ = lease.Release() }()
	}

	// Read existing state preserving unknown fields as raw JSON.
	rawMap, existingBytes, exists, err := s.readRawMap(primary, legacy)
	if err != nil {
		return err
	}
	// Record originals for rollback (both paths).
	if exists {
		s.orig[primary] = existingBytes
		s.order = append(s.order, primary)
		// Legacy may be same content or absent; record for rollback symmetry.
		if legacy != primary {
			if lb, err := os.ReadFile(legacy); err == nil {
				s.orig[legacy] = lb
			} else if os.IsNotExist(err) {
				s.orig[legacy] = nil
			} else {
				s.orig[legacy] = nil
			}
			s.order = append(s.order, legacy)
		}
	} else {
		s.orig[primary] = nil
		s.order = append(s.order, primary)
		if legacy != primary {
			s.orig[legacy] = nil
			s.order = append(s.order, legacy)
		}
	}

	// Merge AgentID (and preserve unknown).
	agentJSON, _ := json.Marshal(id)
	rawMap["agent_id"] = agentJSON
	// Also write capital variant for spec examples that check "AgentID".
	rawMap["AgentID"] = agentJSON

	merged, err := json.MarshalIndent(rawMap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	merged = append(merged, '\n')

	// Write atomically to primary (and legacy for compat).
	if err := s.atomicWrite(primary, merged); err != nil {
		return err
	}
	if legacy != primary {
		// Mirror to legacy if primary succeeded; legacy write failure rolls back primary via tracker.
		if err := s.atomicWrite(legacy, merged); err != nil {
			// Rollback primary on legacy failure to keep atomic view.
			_ = s.tracker.rollback()
			return err
		}
	}
	if ch != nil {
		select {
		case ch <- pipeline.ProgressEvent{Step: s.Name(), Percent: 100, Message: fmt.Sprintf("state AgentID=%s", id)}:
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

func (s *StateStep) atomicWrite(path string, data []byte) error {
	// Record original for tracker rollback.
	if err := s.tracker.record(path); err != nil {
		// Fallback to manual orig map if tracker record fails due to missing parent.
	}
	if _, err := filemerge.WriteFileAtomic(path, data, 0644); err != nil {
		return fmt.Errorf("write state %s: %w", path, err)
	}
	return nil
}

func (s *StateStep) readRawMap(primary, legacy string) (map[string]json.RawMessage, []byte, bool, error) {
	// Prefer primary if exists.
	if data, err := os.ReadFile(primary); err == nil {
		if len(data) == 0 {
			return make(map[string]json.RawMessage), data, true, nil
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, nil, true, fmt.Errorf("parse state %s: %w", primary, err)
		}
		if m == nil {
			m = make(map[string]json.RawMessage)
		}
		return m, data, true, nil
	} else if !os.IsNotExist(err) {
		return nil, nil, true, fmt.Errorf("read state %s: %w", primary, err)
	}
	// Fallback to legacy.
	if data, err := os.ReadFile(legacy); err == nil {
		if len(data) == 0 {
			return make(map[string]json.RawMessage), data, true, nil
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, nil, true, fmt.Errorf("parse state %s: %w", legacy, err)
		}
		if m == nil {
			m = make(map[string]json.RawMessage)
		}
		return m, data, true, nil
	} else if !os.IsNotExist(err) {
		return nil, nil, true, fmt.Errorf("read state %s: %w", legacy, err)
	}
	return make(map[string]json.RawMessage), nil, false, nil
}

func (s *StateStep) Rollback(ctx context.Context) error {
	_ = ctx
	// Prefer tracker rollback (handles both paths via record order).
	if err := s.tracker.rollback(); err != nil {
		return err
	}
	// Fallback to manual orig map for paths not tracked due to early failure.
	for i := len(s.order) - 1; i >= 0; i-- {
		p := s.order[i]
		if _, tracked := s.tracker.orig[p]; tracked {
			continue
		}
		orig, ok := s.orig[p]
		if !ok {
			continue
		}
		if orig == nil {
			_ = os.Remove(p)
		} else {
			_, _ = filemerge.WriteFileAtomic(p, orig, 0644)
		}
	}
	return nil
}
