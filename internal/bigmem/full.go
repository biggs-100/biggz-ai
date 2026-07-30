// Package bigmem — extended SQLite-based tools.
package bigmem

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Session represents a coding session.
type Session struct {
	ID        string    `json:"id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Project   string    `json:"project,omitempty"`
}

// SavedPrompt records a user prompt.
type SavedPrompt struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	SessionID string    `json:"session_id"`
	CreatedAt time.Time `json:"created_at"`
}

// SessionStart registers a new session.
func (s *Store) SessionStart(id, project string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := &Session{ID: id, StartTime: time.Now(), Project: project}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS sessions
		(id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT)`)
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec("INSERT INTO sessions (id, start_time, project) VALUES (?, ?, ?)",
		id, session.StartTime.Format(time.RFC3339), project)
	return session, err
}

// SessionEnd marks a session as completed.
func (s *Store) SessionEnd(id, summary string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("UPDATE sessions SET end_time = ?, summary = ? WHERE id = ?",
		time.Now().UTC().Format(time.RFC3339), summary, id)
	if err != nil {
		return nil, err
	}
	return &Session{ID: id, Summary: summary}, nil
}

// SessionContext returns recent sessions.
func (s *Store) SessionContext(limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 5
	}
	// Ensure sessions table exists
	s.db.Exec(`CREATE TABLE IF NOT EXISTS sessions
		(id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT)`)

	rows, err := s.db.Query("SELECT id, start_time, end_time, summary, project FROM sessions ORDER BY start_time DESC LIMIT ?", limit)
	if err != nil {
		return nil, fmt.Errorf("session query: %w", err)
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var sess Session
		var st, et, summary sql.NullString
		if err := rows.Scan(&sess.ID, &st, &et, &summary, &sess.Project); err != nil {
			continue
		}
		if st.Valid {
			sess.StartTime, _ = time.Parse(time.RFC3339, st.String)
		}
		if et.Valid {
			sess.EndTime, _ = time.Parse(time.RFC3339, et.String)
		}
		if summary.Valid {
			sess.Summary = summary.String
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

// SavePrompt stores a user prompt.
func (s *Store) SavePrompt(content, sessionID string) (*SavedPrompt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Exec(`CREATE TABLE IF NOT EXISTS prompts
		(id TEXT PRIMARY KEY, content TEXT, session_id TEXT, created_at TEXT)`)
	p := &SavedPrompt{
		ID:        fmt.Sprintf("prompt-%d", time.Now().UnixNano()),
		Content:   content,
		SessionID: sessionID,
		CreatedAt: time.Now(),
	}
	_, err := s.db.Exec("INSERT INTO prompts (id, content, session_id, created_at) VALUES (?, ?, ?, ?)",
		p.ID, p.Content, p.SessionID, p.CreatedAt.Format(time.RFC3339))
	return p, err
}

// SuggestTopicKey suggests a stable topic key from title/content.
func SuggestTopicKey(title, content, obsType string) string {
	phrase := title
	if phrase == "" {
		phrase = content
	}
	words := strings.Fields(phrase)
	if len(words) > 6 {
		words = words[:6]
	}
	key := strings.ToLower(strings.Join(words, "-"))
	key = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, key)
	for strings.Contains(key, "--") {
		key = strings.ReplaceAll(key, "--", "-")
	}
	key = strings.Trim(key, "-")
	if obsType != "" && !strings.HasPrefix(key, obsType+"/") {
		key = obsType + "/" + key
	}
	return key
}

// TimelineOptions controls timeline queries.
type TimelineOptions struct {
	Limit   int
	FocusID string // center observation ID for before/after view
	Before  int    // max entries before focus
	After   int    // max entries after focus
}

// TimelineEntry is a chronological entry with display hints.
type TimelineEntry struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	IsFocus   bool      `json:"is_focus,omitempty"`
	IsBefore  bool      `json:"is_before,omitempty"`
}

// Timeline returns observations in chronological order.
func (s *Store) Timeline(opts TimelineOptions) ([]TimelineEntry, error) {
	var rows *sql.Rows
	var err error
	var result []TimelineEntry

	if opts.FocusID != "" {
		// Get created_at of focus observation
		var focusTime string
		err := s.db.QueryRow("SELECT created_at FROM observations WHERE id = ?", opts.FocusID).Scan(&focusTime)
		if err != nil {
			return nil, fmt.Errorf("focus not found: %w", err)
		}

		// Query: observations before focus (same project, older)
		beforeLimit := 5
		if opts.Before > 0 {
			beforeLimit = opts.Before
		}
		beforeRows, err := s.db.Query(
			`SELECT id, title, type, created_at FROM observations
			 WHERE created_at < ? AND id != ?
			 ORDER BY created_at DESC LIMIT ?`, focusTime, opts.FocusID, beforeLimit)
		if err == nil {
			defer beforeRows.Close()
			for beforeRows.Next() {
				var e TimelineEntry
				var ca string
				if err := beforeRows.Scan(&e.ID, &e.Title, &e.Type, &ca); err != nil {
					continue
				}
				e.CreatedAt, _ = time.Parse(time.RFC3339, ca)
				e.IsBefore = true
				result = append(result, e)
			}
		}

		// Add focus observation
		var focusEntry TimelineEntry
		var ca string
		if err := s.db.QueryRow("SELECT id, title, type, created_at FROM observations WHERE id = ?", opts.FocusID).
			Scan(&focusEntry.ID, &focusEntry.Title, &focusEntry.Type, &ca); err == nil {
			focusEntry.CreatedAt, _ = time.Parse(time.RFC3339, ca)
			focusEntry.IsFocus = true
			result = append(result, focusEntry)
		}

		// Query: observations after focus
		afterLimit := 5
		if opts.After > 0 {
			afterLimit = opts.After
		}
		afterRows, err := s.db.Query(
			`SELECT id, title, type, created_at FROM observations
			 WHERE created_at > ? AND id != ?
			 ORDER BY created_at ASC LIMIT ?`, focusTime, opts.FocusID, afterLimit)
		if err == nil {
			defer afterRows.Close()
			for afterRows.Next() {
				var e TimelineEntry
				var ca string
				if err := afterRows.Scan(&e.ID, &e.Title, &e.Type, &ca); err != nil {
					continue
				}
				e.CreatedAt, _ = time.Parse(time.RFC3339, ca)
				result = append(result, e)
			}
		}

		return result, nil
	}

	// No focus: return most recent
	limit := 20
	if opts.Limit > 0 {
		limit = opts.Limit
	}
	rows, err = s.db.Query("SELECT id, title, type, created_at FROM observations ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e TimelineEntry
		var ca string
		if err := rows.Scan(&e.ID, &e.Title, &e.Type, &ca); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		result = append(result, e)
	}
	return result, nil
}

// StoreStats returns usage statistics.
type StoreStats struct {
	TotalObservations int            `json:"total_observations"`
	ByType            map[string]int `json:"by_type"`
	TotalSessions     int            `json:"total_sessions"`
	StoragePath       string         `json:"storage_path"`
}

// Stats returns store statistics.
func (s *Store) Stats() (*StoreStats, error) {
	stats := &StoreStats{ByType: make(map[string]int), StoragePath: s.rootDir}
	s.db.QueryRow("SELECT COUNT(*) FROM observations").Scan(&stats.TotalObservations)
	rows, _ := s.db.Query("SELECT type, COUNT(*) FROM observations WHERE type != '' GROUP BY type")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var c int
			rows.Scan(&t, &c)
			stats.ByType[t] = c
		}
	}
	s.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&stats.TotalSessions)
	return stats, nil
}

// Pin marks an observation as pinned (updates updated_at).
func (s *Store) Pin(id string) error {
	_, err := s.db.Exec("UPDATE observations SET updated_at = ? WHERE id = ?",
		time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// Unpin removes pin.
func (s *Store) Unpin(id string) error { return s.Pin(id) }

// DoctorResult reports store health.
type DoctorResult struct {
	StoreExists  bool   `json:"store_exists"`
	Observations int    `json:"observations"`
	CorruptFiles int    `json:"corrupt_files"`
	StoragePath  string `json:"storage_path"`
	Corrupt      bool   `json:"corrupt"`
}

// Doctor runs diagnostics on the store, including PRAGMA integrity_check.
func (s *Store) Doctor() (*DoctorResult, error) {
	r := &DoctorResult{StoragePath: s.rootDir, StoreExists: true}
	s.db.QueryRow("SELECT COUNT(*) FROM observations").Scan(&r.Observations)

	// Run PRAGMA integrity_check to detect database corruption.
	rows, err := s.db.Query("PRAGMA integrity_check")
	if err != nil {
		// If we can't even run the check, treat it as corrupt.
		r.Corrupt = true
		return r, nil
	}
	defer rows.Close()

	var messages []string
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err != nil {
			continue
		}
		if msg != "ok" {
			messages = append(messages, msg)
		}
	}

	r.Corrupt = len(messages) > 0

	return r, nil
}

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

// JudgeRelation records a relation between two observations.
type JudgeRelation struct {
	ObservationA string  `json:"observation_a"`
	ObservationB string  `json:"observation_b"`
	Relation     string  `json:"relation"`
	Confidence   float64 `json:"confidence"`
	Reason       string  `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

// SaveRelation persists a judgment.
func (s *Store) SaveRelation(aID, bID, relation, reason string, confidence float64) (*JudgeRelation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Exec(`CREATE TABLE IF NOT EXISTS relations
		(id TEXT PRIMARY KEY, obs_a TEXT, obs_b TEXT, relation TEXT, confidence REAL, reason TEXT, created_at TEXT)`)
	jr := &JudgeRelation{
		ObservationA: aID, ObservationB: bID,
		Relation: relation, Confidence: confidence, Reason: reason,
		CreatedAt: time.Now(),
	}
	id := fmt.Sprintf("rel-%d", jr.CreatedAt.UnixNano())
	_, err := s.db.Exec("INSERT INTO relations (id, obs_a, obs_b, relation, confidence, reason, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, aID, bID, relation, confidence, reason, jr.CreatedAt.Format(time.RFC3339))
	return jr, err
}

// CapturePassive extracts learnings from text.
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
						Title: text[:min(len(text), 80)], Type: "discovery",
						Content: text, Project: project, CreatedAt: time.Now(),
					})
				}
			}
		}
	}
	return results, nil
}

// MergeProjects moves observations from one project to another.
func (s *Store) MergeProjects(sourceProject, targetProject string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec("UPDATE observations SET project = ?, updated_at = ? WHERE project = ?",
		targetProject, time.Now().UTC().Format(time.RFC3339), sourceProject)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// Review marks an observation for review or marks it reviewed.
func (s *Store) Review(action string, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Exec(`CREATE TABLE IF NOT EXISTS reviews (observation_id INTEGER PRIMARY KEY, status TEXT, updated_at TEXT)`)
	if action == "mark_reviewed" {
		_, err := s.db.Exec("DELETE FROM reviews WHERE observation_id = ?", id)
		return err
	}
	_, err := s.db.Exec("INSERT OR REPLACE INTO reviews (observation_id, status, updated_at) VALUES (?, 'needs_review', ?)",
		id, time.Now().UTC().Format(time.RFC3339))
	return err
}

// ListNeedsReview returns observation IDs that need review.
func (s *Store) ListNeedsReview() ([]int, error) {
	rows, err := s.db.Query("SELECT observation_id FROM reviews WHERE status = 'needs_review'")
	if err != nil {
		return nil, nil
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}

// ---------------------------------------------------------------------------
// Sync: export/import observations as JSON chunks
// ---------------------------------------------------------------------------

// SyncStatus returns sync metadata.
type SyncStatus struct {
	ExportDir string `json:"export_dir"`
	ChunkCount int   `json:"chunk_count"`
	ObsCount  int   `json:"obs_count"`
}

// SyncExport exports all observations (optionally filtered by project) as
// newline-delimited JSON chunks into <projectRoot>/.engram/.
func (s *Store) SyncExport(project, projectRoot string) error {
	dir := filepath.Join(projectRoot, ".engram")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// Create a .gitignore that allows ndjson files through even if .biggz/ is
	// in a parent .gitignore
	gitIgnorePath := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gitIgnorePath); os.IsNotExist(err) {
		os.WriteFile(gitIgnorePath, []byte("# Sync chunks — safe to commit\n*\n!.gitignore\n*.ndjson\n"), 0644)
	}

	s.mu.RLock()
	q := "SELECT id, title, type, content, topic_key, project, scope, created_at, updated_at FROM observations"
	var args []any
	if project != "" {
		q += " WHERE project = ?"
		args = append(args, project)
	}
	rows, err := s.db.Query(q, args...)
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	defer rows.Close()

	name := fmt.Sprintf("sync-%s.ndjson", time.Now().UTC().Format("20060102T150405"))
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	count := 0
	for rows.Next() {
		obs := &Observation{}
		var ca, ua string
		if err := rows.Scan(&obs.ID, &obs.Title, &obs.Type, &obs.Content,
			&obs.TopicKey, &obs.Project, &obs.Scope, &ca, &ua); err != nil {
			continue
		}
		obs.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		obs.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		if err := enc.Encode(obs); err != nil {
			return err
		}
		count++
	}
	if count == 0 {
		os.Remove(filepath.Join(dir, name))
		return fmt.Errorf("no observations to export")
	}
	return nil
}

// SyncImport reads all .ndjson chunks from <projectRoot>/.engram/ and
// imports any observation whose ID does not already exist in the store.
func (s *Store) SyncImport(projectRoot string) (int, error) {
	dir := filepath.Join(projectRoot, ".engram")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("sync dir: %w", err)
	}

	total := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ndjson") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(string(data)))
		for dec.More() {
			var obs Observation
			if err := dec.Decode(&obs); err != nil {
				break
			}
			// Only import if ID doesn't exist
			var existing int
			s.db.QueryRow("SELECT COUNT(*) FROM observations WHERE id = ?", obs.ID).Scan(&existing)
			if existing > 0 {
				continue
			}
			if obs.ID == "" {
				obs.ID = fmt.Sprintf("obs-%d", time.Now().UnixNano())
			}
			if obs.CreatedAt.IsZero() {
				obs.CreatedAt = time.Now()
			}
			obs.UpdatedAt = time.Now()
			_, err := s.db.Exec(
				`INSERT INTO observations (id, title, type, content, topic_key, project, scope, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
				 ON CONFLICT(id) DO NOTHING`,
				obs.ID, obs.Title, obs.Type, obs.Content, obs.TopicKey,
				obs.Project, obs.Scope,
				obs.CreatedAt.Format(time.RFC3339), obs.UpdatedAt.Format(time.RFC3339))
			if err == nil {
				total++
			}
		}
	}
	return total, nil
}

// SyncStatus returns the number of chunks and observations in the sync dir.
func (s *Store) SyncStatus(projectRoot string) (*SyncStatus, error) {
	dir := filepath.Join(projectRoot, ".engram")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return &SyncStatus{ExportDir: dir, ChunkCount: 0, ObsCount: 0}, nil
	}

	chunks := 0
	totalObs := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ndjson") {
			continue
		}
		chunks++
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		lines := strings.Count(string(data), "\n")
		totalObs += lines
	}
	return &SyncStatus{ExportDir: dir, ChunkCount: chunks, ObsCount: totalObs}, nil
}
