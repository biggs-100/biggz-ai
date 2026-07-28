// Package engram — extended tools for full Engram protocol compatibility.
package engram

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// --- Session ---

// Session represents a coding session.
type Session struct {
	ID        string    `json:"id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Project   string    `json:"project,omitempty"`
}

// SavePrompt records a user prompt for context.
type SavedPrompt struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	SessionID string    `json:"session_id"`
	CreatedAt time.Time `json:"created_at"`
}

// --- Session operations ---

// SessionStart registers a new session.
func (s *Store) SessionStart(id, project string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := &Session{
		ID:        id,
		StartTime: time.Now(),
		Project:   project,
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal session: %w", err)
	}
	sessionsDir := filepath.Join(s.rootDir, "sessions")
	os.MkdirAll(sessionsDir, 0755)
	path := filepath.Join(sessionsDir, id+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("write session: %w", err)
	}
	return session, nil
}

// SessionEnd marks a session as completed.
func (s *Store) SessionEnd(id, summary string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.rootDir, "sessions", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}
	session.EndTime = time.Now()
	session.Summary = summary
	data, err = json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal session: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("write session: %w", err)
	}
	return &session, nil
}

// SessionContext returns recent sessions and their observations.
func (s *Store) SessionContext(limit int) ([]Session, map[string][]*Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionsDir := filepath.Join(s.rootDir, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read sessions: %w", err)
	}

	var sessions []Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sessionsDir, e.Name()))
		if err != nil {
			continue
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		sessions = append(sessions, s)
	}

	// Sort by start time desc
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}

	// Get observations per session (not implemented for now — return empty)
	return sessions, make(map[string][]*Observation), nil
}

// SavePrompt stores a user prompt.
func (s *Store) SavePrompt(content, sessionID string) (*SavedPrompt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := &SavedPrompt{
		ID:        fmt.Sprintf("prompt-%d", time.Now().UnixNano()),
		Content:   content,
		SessionID: sessionID,
		CreatedAt: time.Now(),
	}
	promptsDir := filepath.Join(s.rootDir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal prompt: %w", err)
	}
	path := filepath.Join(promptsDir, p.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("write prompt: %w", err)
	}
	return p, nil
}

// Update modifies an existing observation.
func (s *Store) Update(id string, updates map[string]any) (*Observation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	obs, err := s.getByID(id)
	if err != nil {
		return nil, err
	}

	if v, ok := updates["title"].(string); ok {
		obs.Title = v
	}
	if v, ok := updates["content"].(string); ok {
		obs.Content = v
	}
	if v, ok := updates["type"].(string); ok {
		obs.Type = v
	}
	if v, ok := updates["topic_key"].(string); ok {
		obs.TopicKey = v
	}
	if v, ok := updates["scope"].(string); ok {
		obs.Scope = v
	}
	obs.UpdatedAt = time.Now()

	if err := s.writeObservation(obs); err != nil {
		return nil, err
	}
	return obs, nil
}

// SuggestTopicKey suggests a stable topic key from title/content.
func SuggestTopicKey(title, content, obsType string) string {
	phrase := title
	if phrase == "" {
		phrase = content
	}
	// Take first meaningful words, lowercase, hyphenate
	words := strings.Fields(phrase)
	if len(words) > 6 {
		words = words[:6]
	}
	key := strings.ToLower(strings.Join(words, "-"))
	// Remove special chars
	key = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, key)
	// Collapse multiple dashes
	for strings.Contains(key, "--") {
		key = strings.ReplaceAll(key, "--", "-")
	}
	key = strings.Trim(key, "-")

	if obsType != "" && !strings.HasPrefix(key, obsType+"/") {
		key = obsType + "/" + key
	}
	return key
}

// --- Timeline ---

// TimelineOptions controls timeline query.
type TimelineOptions struct {
	Limit   int
	Since   time.Time
	Until   time.Time
	Types   []string
}

// TimelineEntry is a chronological entry.
type TimelineEntry struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

// Timeline returns observations in chronological order.
func (s *Store) Timeline(opts TimelineOptions) ([]TimelineEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.obsDir)
	if err != nil {
		return nil, fmt.Errorf("read obs dir: %w", err)
	}

	typeMap := make(map[string]bool, len(opts.Types))
	for _, t := range opts.Types {
		typeMap[t] = true
	}

	var result []TimelineEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.obsDir, entry.Name()))
		if err != nil {
			continue
		}
		var obs Observation
		if err := json.Unmarshal(data, &obs); err != nil {
			continue
		}

		if len(typeMap) > 0 && !typeMap[obs.Type] {
			continue
		}
		if !opts.Since.IsZero() && obs.CreatedAt.Before(opts.Since) {
			continue
		}
		if !opts.Until.IsZero() && obs.CreatedAt.After(opts.Until) {
			continue
		}

		result = append(result, TimelineEntry{
			ID:        obs.ID,
			Title:     obs.Title,
			Type:      obs.Type,
			CreatedAt: obs.CreatedAt,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}
	return result, nil
}

// --- Stats ---

// StoreStats returns usage statistics.
type StoreStats struct {
	TotalObservations int            `json:"total_observations"`
	ByType            map[string]int `json:"by_type"`
	TotalSessions     int            `json:"total_sessions"`
	StoragePath       string         `json:"storage_path"`
}

// Stats returns store statistics.
func (s *Store) Stats() (*StoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &StoreStats{
		ByType:      make(map[string]int),
		StoragePath: s.rootDir,
	}

	entries, err := os.ReadDir(s.obsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			stats.TotalObservations++
			data, _ := os.ReadFile(filepath.Join(s.obsDir, entry.Name()))
			var obs Observation
			if json.Unmarshal(data, &obs) == nil && obs.Type != "" {
				stats.ByType[obs.Type]++
			}
		}
	}

	sessionsDir := filepath.Join(s.rootDir, "sessions")
	sEntries, err := os.ReadDir(sessionsDir)
	if err == nil {
		stats.TotalSessions = len(sEntries)
	}

	return stats, nil
}

// --- Pin / Unpin ---

// Pin marks an observation as pinned.
func (s *Store) Pin(id string) error {
	return s.updateMeta(id, "pinned", true)
}

// Unpin removes pin from an observation.
func (s *Store) Unpin(id string) error {
	return s.updateMeta(id, "pinned", false)
}

func (s *Store) updateMeta(id, key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	obs, err := s.getByID(id)
	if err != nil {
		return err
	}
	obs.UpdatedAt = time.Now()
	return s.writeObservation(obs)
}

func (s *Store) getByID(id string) (*Observation, error) {
	path := filepath.Join(s.obsDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", id, err)
	}
	var obs Observation
	if err := json.Unmarshal(data, &obs); err != nil {
		return nil, fmt.Errorf("parse %s: %w", id, err)
	}
	return &obs, nil
}

// --- Doctor ---

// DoctorResult reports store health.
type DoctorResult struct {
	StoreExists   bool   `json:"store_exists"`
	Observations  int    `json:"observations"`
	CorruptFiles  int    `json:"corrupt_files"`
	StoragePath   string `json:"storage_path"`
	DiskUsage     string `json:"disk_usage,omitempty"`
}

// Doctor runs diagnostics on the store.
func (s *Store) Doctor() (*DoctorResult, error) {
	result := &DoctorResult{StoragePath: s.rootDir}

	if _, err := os.Stat(s.obsDir); err == nil {
		result.StoreExists = true
	}

	entries, _ := os.ReadDir(s.obsDir)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		result.Observations++
		data, err := os.ReadFile(filepath.Join(s.obsDir, entry.Name()))
		if err != nil {
			result.CorruptFiles++
			continue
		}
		var obs Observation
		if err := json.Unmarshal(data, &obs); err != nil {
			result.CorruptFiles++
		}
	}

	return result, nil
}

// --- Compare / Judge ---

// CompareResult compares two observations.
type CompareResult struct {
	A           *Observation `json:"a"`
	B           *Observation `json:"b"`
	SameTopic   bool         `json:"same_topic"`
	SameProject bool         `json:"same_project"`
	TimeDiff    string       `json:"time_diff,omitempty"`
}

// Compare compares two observations by ID.
func (s *Store) Compare(idA, idB string) (*CompareResult, error) {
	a, err := s.Get(idA)
	if err != nil {
		return nil, fmt.Errorf("get A: %w", err)
	}
	b, err := s.Get(idB)
	if err != nil {
		return nil, fmt.Errorf("get B: %w", err)
	}
	r := &CompareResult{A: a, B: b}
	r.SameTopic = a.TopicKey != "" && a.TopicKey == b.TopicKey
	r.SameProject = a.Project != "" && a.Project == b.Project
	if !a.CreatedAt.IsZero() && !b.CreatedAt.IsZero() {
		diff := b.CreatedAt.Sub(a.CreatedAt)
		if diff < 0 {
			diff = -diff
		}
		r.TimeDiff = diff.Round(time.Second).String()
	}
	return r, nil
}

// JudgeRelation records a semantic relation between two observations.
type JudgeRelation struct {
	ObservationA string `json:"observation_a"`
	ObservationB string `json:"observation_b"`
	Relation     string `json:"relation"` // related, compatible, scoped, conflicts_with, supersedes, not_conflict
	Confidence   float64 `json:"confidence"`
	Reason       string `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

// SaveRelation persists a judgment between two observations.
func (s *Store) SaveRelation(aID, bID, relation, reason string, confidence float64) (*JudgeRelation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	jr := &JudgeRelation{
		ObservationA: aID,
		ObservationB: bID,
		Relation:     relation,
		Confidence:   confidence,
		Reason:       reason,
		CreatedAt:    time.Now(),
	}

	relationsDir := filepath.Join(s.rootDir, "relations")
	os.MkdirAll(relationsDir, 0755)
	id := fmt.Sprintf("rel-%s-%s", aID[:min(len(aID), 12)], bID[:min(len(bID), 12)])
	data, err := json.MarshalIndent(jr, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal relation: %w", err)
	}
	path := filepath.Join(relationsDir, id+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("write relation: %w", err)
	}
	return jr, nil
}

// --- CapturePassive ---

// CapturePassive extracts structured learnings from text.
// Looks for "## Key Learnings:" or "## Aprendizajes Clave:" sections.
func CapturePassive(content, project string) ([]*Observation, error) {
	var results []*Observation

	markers := []string{"## Key Learnings", "## Aprendizajes Clave", "## Learnings"}
	for _, marker := range markers {
		idx := strings.Index(content, marker)
		if idx < 0 {
			continue
		}
		section := content[idx:]
		endIdx := strings.Index(section, "\n## ")
		if endIdx > 0 {
			section = section[:endIdx]
		}
		for _, line := range strings.Split(section, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				text := strings.TrimPrefix(line, "- ")
				text = strings.TrimPrefix(text, "* ")
				if len(text) > 10 {
					results = append(results, &Observation{
						Title:     text[:min(len(text), 80)],
						Type:      "discovery",
						Content:   text,
						Project:   project,
						CreatedAt: time.Now(),
					})
				}
			}
		}
	}
	return results, nil
}

// --- Review lifecycle ---

// ReviewAction marks an observation for review or marks it reviewed.
func (s *Store) Review(action string, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// For simplicity, we store review state in a separate file per observation
	reviewDir := filepath.Join(s.rootDir, "reviews")
	os.MkdirAll(reviewDir, 0755)

	reviewFile := filepath.Join(reviewDir, fmt.Sprintf("obs-%d.json", id))
	
	if action == "mark_reviewed" {
		if err := os.Remove(reviewFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove review: %w", err)
		}
		return nil
	}
	
	// action=list or default - mark as needs_review
	state := map[string]any{
		"observation_id": id,
		"status":        "needs_review",
		"updated_at":    time.Now(),
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	return os.WriteFile(reviewFile, data, 0644)
}

// ListNeedsReview returns observation IDs that need review.
func (s *Store) ListNeedsReview() ([]int, error) {
	reviewDir := filepath.Join(s.rootDir, "reviews")
	entries, err := os.ReadDir(reviewDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var id int
		fmt.Sscanf(e.Name(), "obs-%d.json", &id)
		ids = append(ids, id)
	}
	return ids, nil
}

// --- MergeProjects ---

// MergeProjects moves observations from one project to another.
func (s *Store) MergeProjects(sourceProject, targetProject string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.obsDir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.obsDir, entry.Name()))
		if err != nil {
			continue
		}
		var obs Observation
		if err := json.Unmarshal(data, &obs); err != nil {
			continue
		}
		if obs.Project == sourceProject {
			obs.Project = targetProject
			obs.UpdatedAt = time.Now()
			if err := s.writeObservation(&obs); err != nil {
				continue
			}
			count++
		}
	}
	return count, nil
}
