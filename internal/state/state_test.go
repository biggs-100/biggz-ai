package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRead_MissingFileReturnsDefault(t *testing.T) {
	s, err := Read(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("Read on missing file: %v", err)
	}
	if s == nil {
		t.Fatal("Read returned nil")
	}
	if s.AgentID != "" || s.Components != nil || s.Skills != nil {
		t.Errorf("expected empty state, got %+v", s)
	}
}

func TestRead_EmptyFileReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	s, err := Read(path)
	if err != nil {
		t.Fatalf("Read on empty file: %v", err)
	}
	if s == nil {
		t.Fatal("Read returned nil")
	}
}

func TestRead_MalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Read(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestReadWriteRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := &InstallState{
		AgentID:     "opencode",
		Components:  []string{"skills", "config", "prompts"},
		Skills:      []string{"code-review"},
		LastSync:    &now,
		PendingSync: false,
	}

	path := filepath.Join(t.TempDir(), "state.json")
	if err := Write(path, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.AgentID != original.AgentID {
		t.Errorf("AgentID = %q, want %q", got.AgentID, original.AgentID)
	}
	if len(got.Components) != len(original.Components) {
		t.Errorf("Components = %v, want %v", got.Components, original.Components)
	}
	if len(got.Skills) != 1 || got.Skills[0] != "code-review" {
		t.Errorf("Skills = %v, want [code-review]", got.Skills)
	}
	if got.LastSync == nil || !got.LastSync.Equal(*original.LastSync) {
		t.Errorf("LastSync = %v, want %v", got.LastSync, original.LastSync)
	}
	if got.PendingSync != false {
		t.Errorf("PendingSync = %v, want false", got.PendingSync)
	}
}

func TestWrite_CreatesParentDir(t *testing.T) {
	deep := filepath.Join(t.TempDir(), "a", "b", "state.json")
	s := &InstallState{AgentID: "claude"}
	if err := Write(deep, s); err != nil {
		t.Fatalf("Write with parent dirs: %v", err)
	}
	if _, err := os.Stat(deep); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestWrite_NilState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nil.json")
	if err := Write(path, nil); err != nil {
		t.Fatalf("Write nil: %v", err)
	}
	data, _ := os.ReadFile(path)
	var s InstallState
	if err := json.Unmarshal(data, &s); err != nil {
		t.Errorf("unmarshal written nil state: %v", err)
	}
}

func TestMerge_OverlayWins(t *testing.T) {
	base := &InstallState{
		AgentID:    "opencode",
		Components: []string{"skills"},
		Skills:     []string{"code-review"},
	}
	overlay := &InstallState{
		AgentID:    "claude",
		Components: []string{"skills", "config"},
	}

	merged := Merge(base, overlay)
	if merged.AgentID != "claude" {
		t.Errorf("AgentID = %q, want claude", merged.AgentID)
	}
	if len(merged.Components) != 2 {
		t.Errorf("Components = %v, want [skills config]", merged.Components)
	}
	// Skills from base preserved (overlay had nil)
	if len(merged.Skills) != 1 || merged.Skills[0] != "code-review" {
		t.Errorf("Skills = %v, want [code-review]", merged.Skills)
	}
}

func TestMerge_NilBase(t *testing.T) {
	overlay := &InstallState{AgentID: "test"}
	merged := Merge(nil, overlay)
	if merged.AgentID != "test" {
		t.Errorf("Merge(nil, overlay) = %+v, want AgentID=test", merged)
	}
}

func TestMerge_NilOverlay(t *testing.T) {
	base := &InstallState{AgentID: "test"}
	merged := Merge(base, nil)
	if merged.AgentID != "test" {
		t.Errorf("Merge(base, nil) = %+v, want AgentID=test", merged)
	}
}

func TestMerge_BothNil(t *testing.T) {
	merged := Merge(nil, nil)
	if merged == nil {
		t.Fatal("Merge(nil, nil) returned nil")
	}
}

func TestMerge_PendingSyncOverlayWins(t *testing.T) {
	base := &InstallState{PendingSync: false}
	overlay := &InstallState{PendingSync: true}
	merged := Merge(base, overlay)
	if !merged.PendingSync {
		t.Error("PendingSync should be true (overlay wins)")
	}
}

func TestMerge_LastSyncOverlayWins(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	base := &InstallState{LastSync: &now}
	overlay := &InstallState{LastSync: &future}
	merged := Merge(base, overlay)
	if merged.LastSync == nil || !merged.LastSync.Equal(future) {
		t.Errorf("LastSync = %v, want %v", merged.LastSync, future)
	}
}

func TestUnknownFieldPreservation(t *testing.T) {
	// Write extra fields via raw JSON
	raw := `{
		"agent_id": "opencode",
		"future_field": "should survive",
		"nested": {"keep": true}
	}`
	path := filepath.Join(t.TempDir(), "future.json")
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if s.AgentID != "opencode" {
		t.Errorf("AgentID = %q, want opencode", s.AgentID)
	}

	// Write back and verify unknown fields survive
	outPath := filepath.Join(t.TempDir(), "roundtrip.json")
	if err := Write(outPath, s); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, _ := os.ReadFile(outPath)
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}

	if result["future_field"] != "should survive" {
		t.Errorf("future_field = %v, want 'should survive'", result["future_field"])
	}
	nested, ok := result["nested"].(map[string]any)
	if !ok || nested["keep"] != true {
		t.Errorf("nested.keep = %v, want true", result["nested"])
	}
}

func TestMerge_UnknownFieldsPreserved(t *testing.T) {
	// Create states with extra fields
	baseJSON := `{"agent_id": "a", "base_only": "from-base"}`
	overlayJSON := `{"agent_id": "b", "overlay_only": "from-overlay"}`

	var base, overlay InstallState
	json.Unmarshal([]byte(baseJSON), &base)
	json.Unmarshal([]byte(overlayJSON), &overlay)

	merged := Merge(&base, &overlay)
	if merged.AgentID != "b" {
		t.Errorf("AgentID = %q, want b", merged.AgentID)
	}
	// Marshal back and check unknown fields
	data, _ := json.Marshal(merged)
	var result map[string]any
	json.Unmarshal(data, &result)

	if result["base_only"] != "from-base" {
		t.Errorf("base_only = %v, want 'from-base'", result["base_only"])
	}
	if result["overlay_only"] != "from-overlay" {
		t.Errorf("overlay_only = %v, want 'from-overlay'", result["overlay_only"])
	}
}
