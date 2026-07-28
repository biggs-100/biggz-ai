package model

import (
	"testing"
)

// TestSchemaVersion_NewReviewState verifies that NewReviewState sets
// SchemaVersion to CurrentSchemaVersion.
func TestSchemaVersion_NewReviewState(t *testing.T) {
	subject := ReviewSubject{
		Repository: "test/repo",
		CommitSHA:  "abc123",
	}
	state := NewReviewState(subject)
	if state.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", state.SchemaVersion, CurrentSchemaVersion)
	}
}

// TestSchemaVersion_Matching verifies that a state created with the current
// schema version requires no migration — the versions match.
func TestSchemaVersion_Matching(t *testing.T) {
	subject := ReviewSubject{
		Repository: "test/repo",
		CommitSHA:  "abc123",
	}
	state := NewReviewState(subject)

	// SchemaVersion is "1.0", which should match CurrentSchemaVersion.
	if state.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("state SchemaVersion (%q) does not match CurrentSchemaVersion (%q)",
			state.SchemaVersion, CurrentSchemaVersion)
	}
}

// TestSchemaVersion_Mismatch verifies that a state serialized with an older
// schema version reports a mismatch when compared to CurrentSchemaVersion.
func TestSchemaVersion_Mismatch(t *testing.T) {
	// Simulate a deserialized state with an older schema version.
	oldVersion := "0.9"
	state := ReviewState{
		SchemaVersion: oldVersion,
	}

	// CurrentSchemaVersion must be different from oldVersion for the
	// mismatch test to be meaningful.
	if CurrentSchemaVersion == oldVersion {
		t.Skip("CurrentSchemaVersion equals oldVersion — cannot test mismatch")
	}

	if state.SchemaVersion == CurrentSchemaVersion {
		t.Errorf("expected schema version mismatch: state has %q, current is %q",
			state.SchemaVersion, CurrentSchemaVersion)
	}
}
