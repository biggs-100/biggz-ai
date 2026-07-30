// Package bigmem provides a persistent memory store using SQLite.
// Compatible with gentle-ai's engram protocol — 22 MCP tools.
package bigmem

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Observation is a single memory entry.
type Observation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	TopicKey  string    `json:"topic_key,omitempty"`
	Project   string    `json:"project,omitempty"`
	Scope     string    `json:"scope,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store manages observations using SQLite.
type Store struct {
	mu      sync.RWMutex
	db      *sql.DB
	rootDir string
}

// Open creates or opens the SQLite store at the given root directory.
func Open(rootDir string) (*Store, error) {
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home dir: %w", err)
		}
		rootDir = filepath.Join(home, ".biggz", "bigmem")
	}
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	dbPath := filepath.Join(rootDir, "bigmem.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// WAL mode for better concurrency
	db.Exec("PRAGMA journal_mode=WAL")

	// Create tables + FTS5 full-text search
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS observations (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			type TEXT DEFAULT '',
			content TEXT DEFAULT '',
			topic_key TEXT DEFAULT '',
			project TEXT DEFAULT '',
			scope TEXT DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_topic ON observations(topic_key);
		CREATE INDEX IF NOT EXISTS idx_type ON observations(type);
		CREATE INDEX IF NOT EXISTS idx_project ON observations(project);
		CREATE INDEX IF NOT EXISTS idx_updated ON observations(updated_at);
		CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
			title, content, topic_key, content='observations', content_rowid='rowid'
		);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	// Create memory_relations table for conflict surfacing
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_relations (
			id TEXT PRIMARY KEY,
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			relation TEXT NOT NULL DEFAULT 'pending',
			reason TEXT DEFAULT '',
			evidence TEXT DEFAULT '',
			confidence REAL DEFAULT 0.0,
			judgment_status TEXT NOT NULL DEFAULT 'pending',
			marked_by TEXT DEFAULT '',
			session_id TEXT DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (source_id) REFERENCES observations(id),
			FOREIGN KEY (target_id) REFERENCES observations(id)
		);
		CREATE INDEX IF NOT EXISTS idx_rel_source ON memory_relations(source_id, judgment_status);
		CREATE INDEX IF NOT EXISTS idx_rel_target ON memory_relations(target_id, judgment_status);
		CREATE INDEX IF NOT EXISTS idx_rel_status ON memory_relations(judgment_status);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create memory_relations: %w", err)
	}

	// Create sync triggers for FTS5
	db.Exec(`
		CREATE TRIGGER IF NOT EXISTS observations_ai AFTER INSERT ON observations BEGIN
			INSERT INTO observations_fts(rowid, title, content, topic_key)
			VALUES (new.rowid, new.title, new.content, new.topic_key);
		END;
		CREATE TRIGGER IF NOT EXISTS observations_ad AFTER DELETE ON observations BEGIN
			INSERT INTO observations_fts(observations_fts, rowid, title, content, topic_key)
			VALUES ('delete', old.rowid, old.title, old.content, old.topic_key);
		END;
		CREATE TRIGGER IF NOT EXISTS observations_au AFTER UPDATE ON observations BEGIN
			INSERT INTO observations_fts(observations_fts, rowid, title, content, topic_key)
			VALUES ('delete', old.rowid, old.title, old.content, old.topic_key);
			INSERT INTO observations_fts(rowid, title, content, topic_key)
			VALUES (new.rowid, new.title, new.content, new.topic_key);
		END;
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return &Store{db: db, rootDir: rootDir}, nil
}

// Save persists an observation. Upserts by ID or topic_key.
func (s *Store) Save(obs *Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if obs.ID == "" {
		obs.ID = fmt.Sprintf("obs-%d", time.Now().UnixNano())
	}
	now := time.Now().UTC()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	obs.UpdatedAt = now

	// Check for existing by topic_key
	if obs.TopicKey != "" {
		var existingID string
		err := s.db.QueryRow("SELECT id FROM observations WHERE topic_key = ? AND (project = ? OR project = '')",
			obs.TopicKey, obs.Project).Scan(&existingID)
		if err == nil {
			obs.ID = existingID
			// Preserve original created_at
			s.db.QueryRow("SELECT created_at FROM observations WHERE id = ?", existingID).Scan(&obs.CreatedAt)
		}
	}

	_, err := s.db.Exec(`INSERT INTO observations (id, title, type, content, topic_key, project, scope, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title, type=excluded.type, content=excluded.content,
			topic_key=excluded.topic_key, project=excluded.project, scope=excluded.scope,
			updated_at=excluded.updated_at`,
		obs.ID, obs.Title, obs.Type, obs.Content, obs.TopicKey, obs.Project, obs.Scope,
		obs.CreatedAt.Format(time.RFC3339), obs.UpdatedAt.Format(time.RFC3339))
	return err
}

// Get retrieves an observation by ID.
func (s *Store) Get(id string) (*Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	obs := &Observation{}
	var createdAt, updatedAt string
	err := s.db.QueryRow(`SELECT id, title, type, content, topic_key, project, scope, created_at, updated_at
		FROM observations WHERE id = ?`, id).
		Scan(&obs.ID, &obs.Title, &obs.Type, &obs.Content, &obs.TopicKey, &obs.Project, &obs.Scope,
			&createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("not found: %w", err)
	}
	obs.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	obs.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return obs, nil
}

// SearchOptions filter search results.
type SearchOptions struct {
	Project string
	Type    string
	Scope   string
	Limit   int
}

// Search finds observations using FTS5 full-text search.
func (s *Store) Search(query string, opts SearchOptions) ([]*Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var conditions []string
	var args []any

	if query != "" {
		// FTS5 MATCH — wrap each term in quotes, join with OR
		terms := strings.Fields(query)
		for i, t := range terms {
			terms[i] = "\"" + strings.ReplaceAll(t, "\"", "") + "\""
		}
		ftsQuery := strings.Join(terms, " OR ")
		conditions = append(conditions, "o.rowid IN (SELECT rowid FROM observations_fts WHERE observations_fts MATCH ?)")
		args = append(args, ftsQuery)
	}
	if opts.Project != "" {
		conditions = append(conditions, "o.project = ?")
		args = append(args, opts.Project)
	}
	if opts.Type != "" {
		conditions = append(conditions, "o.type = ?")
		args = append(args, opts.Type)
	}
	if opts.Scope != "" {
		conditions = append(conditions, "o.scope = ?")
		args = append(args, opts.Scope)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := 20
	if opts.Limit > 0 {
		limit = opts.Limit
	}

	rows, err := s.db.Query(fmt.Sprintf(
		`SELECT o.id, o.title, o.type, o.content, o.topic_key, o.project, o.scope, o.created_at, o.updated_at
		FROM observations o %s ORDER BY o.updated_at DESC LIMIT ?`, where),
		append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*Observation
	for rows.Next() {
		obs := &Observation{}
		var ca, ua string
		if err := rows.Scan(&obs.ID, &obs.Title, &obs.Type, &obs.Content, &obs.TopicKey,
			&obs.Project, &obs.Scope, &ca, &ua); err != nil {
			continue
		}
		obs.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		obs.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		results = append(results, obs)
	}
	return results, nil
}

// Delete removes an observation by ID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("DELETE FROM observations WHERE id = ?", id)
	return err
}

// Update modifies fields of an existing observation.
func (s *Store) Update(id string, updates map[string]any) (*Observation, error) {
	obs, err := s.Get(id)
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
	return obs, s.Save(obs)
}

// ---------------------------------------------------------------------------
// Conflict Surfacing (memory_relations + candidates)
// ---------------------------------------------------------------------------

// Candidate represents a potentially conflicting observation.
type Candidate struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Type        string  `json:"type"`
	Score       float64 `json:"score"`
	JudgmentID  string  `json:"judgment_id"`
	TopicKey    string  `json:"topic_key,omitempty"`
}

// FindCandidates searches for similar observations and creates pending
// memory_relations entries. Returns candidates for the caller to judge.
func (s *Store) FindCandidates(savedID, title, project, scope string) ([]Candidate, error) {
	if title == "" {
		return nil, nil
	}

	// Build OR-based FTS5 query from title words
	terms := strings.Fields(title)
	if len(terms) == 0 {
		return nil, nil
	}
	for i, t := range terms {
		terms[i] = "\"" + strings.ReplaceAll(t, "\"", "") + "\""
	}
	ftsQuery := strings.Join(terms, " OR ")

	var conditions []string
	var args []any
	conditions = append(conditions, "o.rowid IN (SELECT rowid FROM observations_fts WHERE observations_fts MATCH ?)")
	args = append(args, ftsQuery)
	if project != "" {
		conditions = append(conditions, "o.project = ?")
		args = append(args, project)
	}
	if scope != "" {
		conditions = append(conditions, "o.scope = ?")
		args = append(args, scope)
	}
	// Exclude self
	conditions = append(conditions, "o.id != ?")
	args = append(args, savedID)

	where := "WHERE " + strings.Join(conditions, " AND ")
	limit := 5

	rows, err := s.db.Query(fmt.Sprintf(
		`SELECT o.id, o.title, o.type, o.topic_key
		FROM observations o %s ORDER BY o.updated_at DESC LIMIT ?`, where),
		append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []Candidate
	for rows.Next() {
		c := Candidate{}
		if err := rows.Scan(&c.ID, &c.Title, &c.Type, &c.TopicKey); err != nil {
			continue
		}
		// Create a pending relation
		relID := fmt.Sprintf("rel-%d", time.Now().UnixNano())
		now := time.Now().UTC().Format(time.RFC3339)
		s.db.Exec(`INSERT INTO memory_relations (id, source_id, target_id, relation, judgment_status, session_id, created_at, updated_at)
			VALUES (?, ?, ?, 'pending', 'pending', '', ?, ?)
			ON CONFLICT(id) DO NOTHING`,
			relID, savedID, c.ID, now, now)
		c.JudgmentID = relID
		candidates = append(candidates, c)
	}
	return candidates, nil
}

// JudgeRelation updates a pending memory_relation with a verdict.
func (s *Store) JudgeRelation(judgmentID, relation, reason, evidence string, confidence float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	validRelations := map[string]bool{
		"related": true, "compatible": true, "scoped": true,
		"conflicts_with": true, "supersedes": true, "not_conflict": true,
	}
	if !validRelations[relation] {
		return fmt.Errorf("invalid relation: %s", relation)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE memory_relations SET
		relation=?, judgment_status='judged', reason=?, evidence=?,
		confidence=?, updated_at=?
		WHERE id=? AND judgment_status='pending'`,
		relation, reason, evidence, confidence, now, judgmentID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("judgment %s not found or already judged", judgmentID)
	}
	return nil
}

// RelationCount returns the number of pending judgments.
func (s *Store) RelationCount() int {
	var n int
	s.db.QueryRow("SELECT COUNT(*) FROM memory_relations WHERE judgment_status='pending'").Scan(&n)
	return n
}

// Relation represents a memory_relations row.
type Relation struct {
	ID              string    `json:"id"`
	SourceID        string    `json:"source_id"`
	TargetID        string    `json:"target_id"`
	Relation        string    `json:"relation"`
	JudgmentStatus  string    `json:"judgment_status"`
	Reason          string    `json:"reason,omitempty"`
	Evidence        string    `json:"evidence,omitempty"`
	Confidence      float64   `json:"confidence,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ListRelations returns relations filtered by judgment status (empty = all).
func (s *Store) ListRelations(status string) ([]Relation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := `SELECT id, source_id, target_id, relation, judgment_status,
		COALESCE(reason,''), COALESCE(evidence,''), COALESCE(confidence,0),
		created_at, updated_at FROM memory_relations`
	var args []any
	if status != "" {
		q += " WHERE judgment_status = ?"
		args = append(args, status)
	}
	q += " ORDER BY created_at DESC LIMIT 50"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Relation
	for rows.Next() {
		var r Relation
		var ca, ua string
		if err := rows.Scan(&r.ID, &r.SourceID, &r.TargetID, &r.Relation,
			&r.JudgmentStatus, &r.Reason, &r.Evidence, &r.Confidence, &ca, &ua); err != nil {
			continue
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		r.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		result = append(result, r)
	}
	return result, nil
}

// GetRelation returns a single relation by ID.
func (s *Store) GetRelation(id string) (*Relation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var r Relation
	var ca, ua string
	err := s.db.QueryRow(
		`SELECT id, source_id, target_id, relation, judgment_status,
		 COALESCE(reason,''), COALESCE(evidence,''), COALESCE(confidence,0),
		 created_at, updated_at FROM memory_relations WHERE id = ?`, id).
		Scan(&r.ID, &r.SourceID, &r.TargetID, &r.Relation,
			&r.JudgmentStatus, &r.Reason, &r.Evidence, &r.Confidence, &ca, &ua)
	if err != nil {
		return nil, fmt.Errorf("relation not found: %w", err)
	}
	r.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	r.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &r, nil
}

// ScanConflicts finds all observations that have no relations and creates
// pending candidates for them. Returns total candidates created.
func (s *Store) ScanConflicts(project string, dryRun bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	q := `SELECT id, title, project, scope FROM observations WHERE 1=1`
	var args []any
	if project != "" {
		q += " AND project = ?"
		args = append(args, project)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	total := 0
	for rows.Next() {
		var id, title, proj, scope string
		if err := rows.Scan(&id, &title, &proj, &scope); err != nil {
			continue
		}
		if title == "" {
			continue
		}
		// Check if this obs already has any relations
		var cnt int
		s.db.QueryRow("SELECT COUNT(*) FROM memory_relations WHERE source_id = ? OR target_id = ?", id, id).Scan(&cnt)
		if cnt > 0 {
			continue
		}
		if dryRun {
			total++
			continue
		}
		// Scan for candidates via FTS
		terms := strings.Fields(title)
		if len(terms) == 0 {
			continue
		}
		for i, t := range terms {
			terms[i] = "\"" + strings.ReplaceAll(t, "\"", "") + "\""
		}
		ftsQ := strings.Join(terms, " OR ")

		candRows, err := s.db.Query(
			`SELECT o.id FROM observations o
			 WHERE o.rowid IN (SELECT rowid FROM observations_fts WHERE observations_fts MATCH ?)
			 AND o.id != ? AND o.project = ? LIMIT 5`, ftsQ, id, proj)
		if err != nil {
			continue
		}
		for candRows.Next() {
			var candID string
			candRows.Scan(&candID)
			relID := fmt.Sprintf("rel-%d", time.Now().UnixNano())
			now := time.Now().UTC().Format(time.RFC3339)
			s.db.Exec(`INSERT OR IGNORE INTO memory_relations (id, source_id, target_id, relation, judgment_status, created_at, updated_at)
				VALUES (?, ?, ?, 'pending', 'pending', ?, ?)`, relID, id, candID, now, now)
			total++
		}
		candRows.Close()
	}
	return total, nil
}

// ProjectSummary holds project-level statistics.
type ProjectSummary struct {
	Name         string `json:"name"`
	Observations int    `json:"observations"`
	Sessions     int    `json:"sessions"`
}

// ListProjects returns all projects with observation and session counts.
func (s *Store) ListProjects() ([]ProjectSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`
		SELECT o.project, COUNT(DISTINCT o.id), COUNT(DISTINCT s.id)
		FROM observations o LEFT JOIN sessions s ON s.project = o.project
		WHERE o.project != ''
		GROUP BY o.project
		ORDER BY o.project`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ProjectSummary
	for rows.Next() {
		var p ProjectSummary
		if err := rows.Scan(&p.Name, &p.Observations, &p.Sessions); err != nil {
			continue
		}
		result = append(result, p)
	}
	return result, nil
}

// Close shuts down the store.
func (s *Store) Close() error {
	return s.db.Close()
}

// RootDir returns the store directory.
func (s *Store) RootDir() string { return s.rootDir }
