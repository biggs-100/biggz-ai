// Package state manages the persisted install state for biggz-ai agents.
// It provides JSON-based persistence with forward-compatible unknown field
// preservation: fields the current binary doesn't understand are kept and
// re-serialized without modification.
//
// BigMem config (strict_tdd, etc.) is stored in ~/.biggz/config.json
// and accessed via the BigMemConfig helpers at the bottom of this file.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// InstallState represents the persisted installation state for an agent.
// Unknown JSON fields are preserved across read/write cycles for forward
// compatibility.
type InstallState struct {
	AgentID     string     `json:"agent_id,omitempty"`
	Components  []string   `json:"components,omitempty"`
	Skills      []string   `json:"skills,omitempty"`
	LastSync    *time.Time `json:"last_sync,omitempty"`
	PendingSync bool       `json:"pending_sync"`

	// extra holds unknown JSON fields for round-trip preservation.
	extra map[string]json.RawMessage
}

// Read loads an InstallState from the given file path. If the file does
// not exist, it returns a zero-valued InstallState (no error).
// Malformed JSON returns an error.
func Read(path string) (*InstallState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &InstallState{}, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	if len(data) == 0 {
		return &InstallState{}, nil
	}

	var s InstallState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &s, nil
}

// Write atomically persists the InstallState to the given file path,
// creating parent directories as needed.
func Write(path string, s *InstallState) error {
	if s == nil {
		s = &InstallState{}
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}

	// Atomic write via temp file + rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write state tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename state: %w", err)
	}

	return nil
}

// Merge combines two InstallState values. The overlay takes precedence
// for all known fields. Unknown fields are merged (overlay wins on
// conflict). Neither input is modified.
//
// This method is a pure function — it does not touch the filesystem.
func Merge(base, overlay *InstallState) *InstallState {
	if base == nil {
		base = &InstallState{}
	}
	if overlay == nil {
		overlay = &InstallState{}
	}

	result := &InstallState{}

	// Known fields: overlay wins when non-zero
	if overlay.AgentID != "" {
		result.AgentID = overlay.AgentID
	} else {
		result.AgentID = base.AgentID
	}

	if overlay.Components != nil {
		result.Components = overlay.Components
	} else {
		result.Components = base.Components
	}

	if overlay.Skills != nil {
		result.Skills = overlay.Skills
	} else {
		result.Skills = base.Skills
	}

	if overlay.LastSync != nil {
		result.LastSync = overlay.LastSync
	} else {
		result.LastSync = base.LastSync
	}

	result.PendingSync = overlay.PendingSync

	// Unknown fields: overlay wins on key conflict
	result.extra = make(map[string]json.RawMessage)
	for k, v := range base.extra {
		result.extra[k] = v
	}
	for k, v := range overlay.extra {
		result.extra[k] = v
	}

	return result
}

// UnmarshalJSON implements json.Unmarshaler for round-trip preservation
// of unknown fields.
func (s *InstallState) UnmarshalJSON(data []byte) error {
	// Unmarshal known fields via type alias to avoid recursion
	type alias InstallState
	if err := json.Unmarshal(data, (*alias)(s)); err != nil {
		return err
	}

	// Capture all top-level keys
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	known := map[string]bool{
		"agent_id":     true,
		"components":   true,
		"skills":       true,
		"last_sync":    true,
		"pending_sync": true,
	}

	s.extra = make(map[string]json.RawMessage)
	for k, v := range raw {
		if !known[k] {
			s.extra[k] = v
		}
	}
	return nil
}

// MarshalJSON implements json.Marshaler for round-trip preservation
// of unknown fields.
func (s *InstallState) MarshalJSON() ([]byte, error) {
	type alias InstallState
	data, err := json.MarshalIndent((*alias)(s), "", "  ")
	if err != nil {
		return nil, err
	}

	if len(s.extra) == 0 {
		return data, nil
	}

	// Merge known and unknown fields into a single map
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	for k, v := range s.extra {
		obj[k] = v
	}
	return json.MarshalIndent(obj, "", "  ")
}

// ─── BigMem Config (stored in ~/.biggz/config.json) ─────────────────────────

// bigmemConfigPath returns the path to the biggz config file.
func bigmemConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".biggz", "config.json")
}

// GetBigMemConfig retrieves a boolean value from ~/.biggz/config.json.
func GetBigMemConfig(key string) (bool, error) {
	path := bigmemConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false, nil
	}
	if v, ok := cfg[key].(bool); ok {
		return v, nil
	}
	return false, nil
}

// SetBigMemConfig sets a boolean value in ~/.biggz/config.json.
func SetBigMemConfig(key string, value bool) error {
	path := bigmemConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var cfg map[string]any
	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, &cfg)
	}
	if cfg == nil {
		cfg = make(map[string]any)
	}
	cfg[key] = value

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}
