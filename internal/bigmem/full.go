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

// ─── Session ─────────────────────────────────────────────────────────────────

// Session represents a coding session with directory tracking.
type Session struct {
	ID        string    `json:"id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Project   string    `json:"project,omitempty"`
	Directory string    `json:"directory,omitempty"`
}

// SessionStart registers a new session.
func (s *Store) SessionStart(id, project string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := &Session{ID: id, StartTime: time.Now(), Project: project}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS sessions
		(id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT)`)
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

// SessionContext returns recent sessions, newest first.
func (s *Store) SessionContext(limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 5
	}
	s.db.Exec(`CREATE TABLE IF NOT EXISTS sessions
		(id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT)`)

	rows, err := s.db.Query(
		"SELECT id, start_time, end_time, summary, project, directory FROM sessions ORDER BY start_time DESC LIMIT ?", limit)
	if err != nil {
		return nil, fmt.Errorf("session query: %w", err)
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var sess Session
		var st, et, summary, dir sql.NullString
		if err := rows.Scan(&sess.ID, &st, &et, &summary, &sess.Project, &dir); err != nil {
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
		if dir.Valid {
			sess.Directory = dir.String
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

// ─── Prompts ─────────────────────────────────────────────────────────────────

// SavedPrompt records a user prompt.
type SavedPrompt struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	SessionID string    `json:"session_id"`
	CreatedAt time.Time `json:"created_at"`
}

// SavePrompt persists a user prompt.
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

// SearchPrompts searches prompts using FTS5.
func (s *Store) SearchPrompts(query, project string, limit int) ([]SavedPrompt, error) {
	if limit <= 0 {
		limit = 10
	}
	// Ensure FTS table exists
	s.db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS prompts_fts USING fts5(
		content, project, content='prompts', content_rowid='rowid'
	)`)

	terms := strings.Fields(query)
	for i, t := range terms {
		terms[i] = "\"" + strings.ReplaceAll(t, "\"", "") + "\""
	}
	ftsQuery := strings.Join(terms, " AND ")

	sqlQ := `SELECT p.id, p.content, p.session_id, p.created_at
		FROM prompts_fts fts JOIN prompts p ON p.rowid = fts.rowid
		WHERE prompts_fts MATCH ?`
	args := []any{ftsQuery}

	if project != "" {
		sqlQ += " AND prompts_fts.project MATCH ?"
		args = append(args, project)
	}
	sqlQ += " ORDER BY rank LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(sqlQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SavedPrompt
	for rows.Next() {
		var p SavedPrompt
		var ca string
		if err := rows.Scan(&p.ID, &p.Content, &p.SessionID, &ca); err != nil {
			continue
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		results = append(results, p)
	}
	return results, nil
}

// ListPromptsBySession returns all prompts for a given session.
func (s *Store) ListPromptsBySession(sessionID string) ([]SavedPrompt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.db.Exec(`CREATE TABLE IF NOT EXISTS prompts
		(id TEXT PRIMARY KEY, content TEXT, session_id TEXT, created_at TEXT)`)
	rows, err := s.db.Query(
		"SELECT id, content, session_id, created_at FROM prompts WHERE session_id = ? ORDER BY created_at ASC", sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SavedPrompt
	for rows.Next() {
		var p SavedPrompt
		var ca string
		if err := rows.Scan(&p.ID, &p.Content, &p.SessionID, &ca); err != nil {
			continue
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		result = append(result, p)
	}
	return result, nil
}

// ─── Suggest Topic Key ───────────────────────────────────────────────────────

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

// ─── Timeline ────────────────────────────────────────────────────────────────

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
		var focusTime string
		err := s.db.QueryRow("SELECT created_at FROM observations WHERE id = ?", opts.FocusID).Scan(&focusTime)
		if err != nil {
			return nil, fmt.Errorf("focus not found: %w", err)
		}

		beforeLimit := 5
		if opts.Before > 0 {
			beforeLimit = opts.Before
		}
		beforeRows, err := s.db.Query(
			`SELECT id, title, type, created_at FROM observations
			 WHERE created_at < ? AND id != ? AND deleted_at IS NULL
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

		var focusEntry TimelineEntry
		var ca string
		if err := s.db.QueryRow("SELECT id, title, type, created_at FROM observations WHERE id = ?", opts.FocusID).
			Scan(&focusEntry.ID, &focusEntry.Title, &focusEntry.Type, &ca); err == nil {
			focusEntry.CreatedAt, _ = time.Parse(time.RFC3339, ca)
			focusEntry.IsFocus = true
			result = append(result, focusEntry)
		}

		afterLimit := 5
		if opts.After > 0 {
			afterLimit = opts.After
		}
		afterRows, err := s.db.Query(
			`SELECT id, title, type, created_at FROM observations
			 WHERE created_at > ? AND id != ? AND deleted_at IS NULL
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

	limit := 20
	if opts.Limit > 0 {
		limit = opts.Limit
	}
	rows, err = s.db.Query(
		"SELECT id, title, type, created_at FROM observations WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT ?", limit)
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

// ─── Stats ───────────────────────────────────────────────────────────────────

// StoreStats returns usage statistics.
type StoreStats struct {
	TotalObservations int            `json:"total_observations"`
	ByType            map[string]int `json:"by_type"`
	TotalSessions     int            `json:"total_sessions"`
	TotalPrompts      int            `json:"total_prompts"`
	StoragePath       string         `json:"storage_path"`
}

// Stats returns store statistics.
func (s *Store) Stats() (*StoreStats, error) {
	stats := &StoreStats{ByType: make(map[string]int), StoragePath: s.rootDir}
	s.db.QueryRow("SELECT COUNT(*) FROM observations WHERE deleted_at IS NULL").Scan(&stats.TotalObservations)
	rows, _ := s.db.Query("SELECT type, COUNT(*) FROM observations WHERE type != '' AND type != 'deleted' AND deleted_at IS NULL GROUP BY type")
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
	s.db.QueryRow("SELECT COUNT(*) FROM prompts").Scan(&stats.TotalPrompts)
	return stats, nil
}

// ─── Pin ─────────────────────────────────────────────────────────────────────

// Pin marks an observation as pinned (sets pinned=1).
func (s *Store) Pin(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("UPDATE observations SET pinned=1, updated_at=? WHERE id=?",
		time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// Unpin removes pin (sets pinned=0).
func (s *Store) Unpin(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("UPDATE observations SET pinned=0, updated_at=? WHERE id=?",
		time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// ─── Doctor ──────────────────────────────────────────────────────────────────

// DoctorResult reports store health.
type DoctorResult struct {
	StoreExists  bool   `json:"store_exists"`
	Observations int    `json:"observations"`
	CorruptFiles int    `json:"corrupt_files"`
	StoragePath  string `json:"storage_path"`
	Corrupt      bool   `json:"corrupt"`
}

// Doctor runs diagnostics including PRAGMA integrity_check.
func (s *Store) Doctor() (*DoctorResult, error) {
	r := &DoctorResult{StoragePath: s.rootDir, StoreExists: true}
	s.db.QueryRow("SELECT COUNT(*) FROM observations").Scan(&r.Observations)

	rows, err := s.db.Query("PRAGMA integrity_check")
	if err != nil {
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

// ─── Compare ─────────────────────────────────────────────────────────────────

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

// ─── Relations (simple) ──────────────────────────────────────────────────────

// JudgeRelation records a relation between two observations.
type JudgeRelation struct {
	ObservationA string    `json:"observation_a"`
	ObservationB string    `json:"observation_b"`
	Relation     string    `json:"relation"`
	Confidence   float64   `json:"confidence"`
	Reason       string    `json:"reason"`
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

// ─── Passive Capture ─────────────────────────────────────────────────────────

// CapturePassive extracts learnings from text (## Key Learnings section).
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

// ─── Review / Lifecycle ──────────────────────────────────────────────────────

// Review marks an observation for review or marks it reviewed.
// Uses the reviews table keyed by observation_id (TEXT matching observation IDs).
func (s *Store) Review(action string, obsID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Exec(`CREATE TABLE IF NOT EXISTS reviews
		(observation_id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'needs_review', updated_at TEXT NOT NULL)`)

	if action == "mark_reviewed" {
		_, err := s.db.Exec("DELETE FROM reviews WHERE observation_id = ?", obsID)
		return err
	}
	_, err := s.db.Exec("INSERT OR REPLACE INTO reviews (observation_id, status, updated_at) VALUES (?, 'needs_review', ?)",
		obsID, time.Now().UTC().Format(time.RFC3339))
	return err
}

// ListNeedsReview returns observation IDs that need review.
func (s *Store) ListNeedsReview() ([]string, error) {
	rows, err := s.db.Query("SELECT observation_id FROM reviews WHERE status = 'needs_review'")
	if err != nil {
		return nil, nil
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}

// ─── Sync: export/import ─────────────────────────────────────────────────────

// SyncStatus returns sync metadata.
type SyncStatus struct {
	ExportDir  string `json:"export_dir"`
	ChunkCount int    `json:"chunk_count"`
	ObsCount   int    `json:"obs_count"`
}

// SyncExport exports observations as NDJSON chunks into <projectRoot>/.bigmem/.
func (s *Store) SyncExport(project, projectRoot string) error {
	dir := filepath.Join(projectRoot, ".bigmem")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	gitIgnorePath := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gitIgnorePath); os.IsNotExist(err) {
		os.WriteFile(gitIgnorePath, []byte("# Sync chunks — safe to commit\n*\n!.gitignore\n*.ndjson\n"), 0644)
	}

	s.mu.RLock()
	q := `SELECT id, title, type, content, session_id, tool_name, topic_key, project, scope,
		normalized_hash, revision_count, duplicate_count, last_seen_at, review_after,
		pinned, created_at, updated_at, deleted_at FROM observations WHERE deleted_at IS NULL`
	var args []any
	if project != "" {
		q += " AND project = ?"
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
		var ra, da, lsa sql.NullString
		var pinnedInt int
		if err := rows.Scan(&obs.ID, &obs.Title, &obs.Type, &obs.Content,
			&obs.SessionID, &obs.ToolName, &obs.TopicKey, &obs.Project, &obs.Scope,
			&obs.NormalizedHash, &obs.RevisionCount, &obs.DuplicateCount,
			&lsa, &ra, &pinnedInt, &ca, &ua, &da); err != nil {
			continue
		}
		obs.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		obs.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		if lsa.Valid {
			obs.LastSeenAt = &lsa.String
		}
		if ra.Valid {
			obs.ReviewAfter = &ra.String
		}
		if da.Valid {
			obs.DeletedAt = &da.String
		}
		obs.Pinned = pinnedInt != 0
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

// SyncImport reads NDJSON chunks from <projectRoot>/.bigmem/ and imports.
func (s *Store) SyncImport(projectRoot string) (int, error) {
	dir := filepath.Join(projectRoot, ".bigmem")
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
				`INSERT INTO observations (id, title, type, content, session_id, tool_name,
				 topic_key, project, scope, normalized_hash, revision_count, duplicate_count,
				 last_seen_at, review_after, pinned, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?, ?, ?, ?)
				 ON CONFLICT(id) DO NOTHING`,
				obs.ID, obs.Title, obs.Type, obs.Content, obs.SessionID, obs.ToolName,
				obs.TopicKey, obs.Project, obs.Scope, obs.NormalizedHash,
				nil, nil, boolToInt(obs.Pinned), // last_seen_at, review_after = nil
				obs.CreatedAt.Format(time.RFC3339), obs.UpdatedAt.Format(time.RFC3339))
			if err == nil {
				total++
			}
		}
	}
	return total, nil
}

// SyncStatus returns chunk/observation counts in the sync dir.
func (s *Store) SyncStatus(projectRoot string) (*SyncStatus, error) {
	dir := filepath.Join(projectRoot, ".bigmem")
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
