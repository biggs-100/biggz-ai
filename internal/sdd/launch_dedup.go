// Package sdd — Sub-Agent Launch Deduplication.
//
// Prevents duplicate sub-agent launches within a session by tracking
// (phase, task-fingerprint) pairs. The orchestrator checks before each
// delegation call and records successful launches.
//
// Usage:
//
//	dedup := NewLaunchDedup()
//	if dedup.ShouldBlock("spec", taskDescription) {
//	    // Skip this launch - already launched
//	}
//	dedup.Record("spec", taskDescription)
package sdd

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// LaunchDedup tracks sub-agent launches to prevent duplicates.
// It is session-scoped and should be created once per orchestrator session.
type LaunchDedup struct {
	// launches stores (phase, fingerprint) pairs that have been launched.
	// The key is "phase:fingerprint".
	launches map[string]bool
}

// NewLaunchDedup creates a new deduplication tracker.
func NewLaunchDedup() *LaunchDedup {
	return &LaunchDedup{
		launches: make(map[string]bool),
	}
}

// launchKey creates the map key for a (phase, fingerprint) pair.
func launchKey(phase, fingerprint string) string {
	return phase + ":" + fingerprint
}

// ShouldBlock reports whether a launch with this phase and task description
// has already been recorded. Returns true if the launch should be skipped.
func (d *LaunchDedup) ShouldBlock(phase, taskDescription string) bool {
	fp := ComputeFingerprint(taskDescription)
	return d.launches[launchKey(phase, fp)]
}

// Record marks a (phase, task-fingerprint) pair as launched.
func (d *LaunchDedup) Record(phase, taskDescription string) {
	fp := ComputeFingerprint(taskDescription)
	d.launches[launchKey(phase, fp)] = true
}

// HasLaunch reports whether a specific (phase, fingerprint) pair exists.
func (d *LaunchDedup) HasLaunch(phase, fingerprint string) bool {
	return d.launches[launchKey(phase, fingerprint)]
}

// Count returns the total number of recorded launches.
func (d *LaunchDedup) Count() int {
	return len(d.launches)
}

// Reset clears all recorded launches (for testing or new session).
func (d *LaunchDedup) Reset() {
	d.launches = make(map[string]bool)
}

// RecordByKey records a launch using a pre-computed key ("phase:fingerprint").
// Used for restoring state from persistent storage.
func (d *LaunchDedup) RecordByKey(key string) {
	d.launches[key] = true
}

// ExportState returns all recorded launch keys as a map.
// Used for persisting state to file.
func (d *LaunchDedup) ExportState() map[string]bool {
	state := make(map[string]bool, len(d.launches))
	for k, v := range d.launches {
		state[k] = v
	}
	return state
}

// Phases returns the list of phases that have had launches.
func (d *LaunchDedup) Phases() []string {
	seen := make(map[string]bool)
	var phases []string
	for key := range d.launches {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 && !seen[parts[0]] {
			seen[parts[0]] = true
			phases = append(phases, parts[0])
		}
	}
	return phases
}

// ComputeFingerprint creates a short, stable fingerprint from a task description.
// The fingerprint is a SHA-256 hash truncated to 12 characters, derived from
// a normalized version of the task description.
//
// Normalization:
//   - Trim whitespace
//   - Collapse multiple spaces to single space
//   - Lowercase everything
//   - Extract key artifact references (paths ending in .md, .go, etc.)
//
// This produces a stable fingerprint even if the task description has minor
// formatting differences, while still distinguishing different tasks.
func ComputeFingerprint(taskDescription string) string {
	normalized := normalizeTaskDescription(taskDescription)
	
	// Extract artifact references for fingerprint
	artifacts := extractArtifactRefs(normalized)
	
	// Combine phase-relevant content
	fingerprint := normalized
	if len(artifacts) > 0 {
		// Use artifacts as the primary fingerprint component
		fingerprint = strings.Join(artifacts, "|")
	}
	
	// Hash it
	hash := sha256.Sum256([]byte(fingerprint))
	return hex.EncodeToString(hash[:])[:12] // Truncate to 12 chars
}

// normalizeTaskDescription normalizes a task description for fingerprinting.
func normalizeTaskDescription(s string) string {
	// Trim and lowercase
	s = strings.TrimSpace(strings.ToLower(s))
	
	// Collapse whitespace
	spaceRe := regexp.MustCompile(`\s+`)
	s = spaceRe.ReplaceAllString(s, " ")
	
	return s
}

// artifactRefRe matches file paths that look like artifact references.
var artifactRefRe = regexp.MustCompile(`[a-z0-9_/-]+\.(md|go|yaml|json|ts|js|py)`)

// extractArtifactRefs extracts file path references from the task description.
func extractArtifactRefs(s string) []string {
	matches := artifactRefRe.FindAllString(s, -1)
	// Deduplicate
	seen := make(map[string]bool)
	var refs []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			refs = append(refs, m)
		}
	}
	return refs
}

// LaunchDedupResult is the result of a dedup check.
type LaunchDedupResult struct {
	Blocked bool   `json:"blocked"`
	Phase   string `json:"phase"`
	Message string `json:"message,omitempty"`
}

// CheckAndRecord is a convenience function that checks if a launch should be
// blocked, and if not, records it. Returns the result.
func CheckAndRecord(dedup *LaunchDedup, phase, taskDescription string) LaunchDedupResult {
	if dedup.ShouldBlock(phase, taskDescription) {
		return LaunchDedupResult{
			Blocked: true,
			Phase:   phase,
			Message: "launch already recorded for this (phase, task) pair",
		}
	}
	dedup.Record(phase, taskDescription)
	return LaunchDedupResult{
		Blocked: false,
		Phase:   phase,
	}
}
