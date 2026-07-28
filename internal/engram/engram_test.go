package engram

import (
	"path/filepath"
	"testing"
)

func TestSaveAndGet(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	obs := &Observation{
		Title:   "Test decision",
		Type:    "decision",
		Content: "**What**: Testing\n**Why**: Verify",
		Project: "test",
		Scope:   "project",
	}

	if err := s.Save(obs); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if obs.ID == "" {
		t.Fatal("expected non-empty ID after Save")
	}

	got, err := s.Get(obs.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Title != "Test decision" {
		t.Errorf("Title = %q, want %q", got.Title, "Test decision")
	}
	if got.Type != "decision" {
		t.Errorf("Type = %q, want %q", got.Type, "decision")
	}
}

func TestSave_UpdatesByTopicKey(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	// First save
	obs1 := &Observation{
		Title:    "Original",
		Type:     "architecture",
		Content:  "First version",
		TopicKey: "test/topic",
		Project:  "test",
	}
	s.Save(obs1)

	// Second save with same topic key — should update
	obs2 := &Observation{
		Title:    "Updated",
		Type:     "architecture",
		Content:  "Second version",
		TopicKey: "test/topic",
		Project:  "test",
	}
	s.Save(obs2)

	// Should have same ID
	if obs1.ID != obs2.ID {
		t.Errorf("expected same ID for same topic key: %s vs %s", obs1.ID, obs2.ID)
	}

	got, _ := s.Get(obs2.ID)
	if got.Title != "Updated" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated")
	}
}

func TestSearch(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	s.Save(&Observation{Title: "Auth design", Type: "architecture", Content: "JWT tokens", Project: "biggz", TopicKey: "auth"})
	s.Save(&Observation{Title: "Bug fix", Type: "bugfix", Content: "Fixed NPE in parser", Project: "biggz", TopicKey: "bug/npe"})
	s.Save(&Observation{Title: "Config change", Type: "config", Content: "Set timeout to 30s", Project: "other", TopicKey: "config"})

	// Search by keyword
	results, err := s.Search("auth", SearchOptions{})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 0 {
		// "auth" appears in title and topic_key
	}
	if len(results) < 1 {
		t.Errorf("expected at least 1 result for 'auth', got %d", len(results))
	}

	// Search by type
	results, err = s.Search("", SearchOptions{Type: "bugfix"})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 bugfix, got %d", len(results))
	}

	// Search by project
	results, err = s.Search("", SearchOptions{Project: "other"})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'other', got %d", len(results))
	}
}

func TestDelete(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	obs := &Observation{Title: "To delete", Type: "discovery", Content: "Will be removed", Project: "test"}
	s.Save(obs)

	if err := s.Delete(obs.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err = s.Get(obs.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestOpen_DefaultDir(t *testing.T) {
	// Override HOME for testing
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	s, err := Open("")
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	expected := filepath.Join(home, ".biggz", "engram")
	if s.RootDir() != expected {
		t.Errorf("RootDir = %q, want %q", s.RootDir(), expected)
	}
}
