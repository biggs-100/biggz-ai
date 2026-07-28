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

	// Create tables
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

// Search finds observations matching the query.
func (s *Store) Search(query string, opts SearchOptions) ([]*Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var conditions []string
	var args []any

	if query != "" {
		q := "%" + strings.ToLower(query) + "%"
		conditions = append(conditions, "(LOWER(title) LIKE ? OR LOWER(content) LIKE ? OR LOWER(topic_key) LIKE ?)")
		args = append(args, q, q, q)
	}
	if opts.Project != "" {
		conditions = append(conditions, "project = ?")
		args = append(args, opts.Project)
	}
	if opts.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, opts.Type)
	}
	if opts.Scope != "" {
		conditions = append(conditions, "scope = ?")
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
		`SELECT id, title, type, content, topic_key, project, scope, created_at, updated_at
		FROM observations %s ORDER BY updated_at DESC LIMIT ?`, where),
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

// Close shuts down the store.
func (s *Store) Close() error {
	return s.db.Close()
}

// RootDir returns the store directory.
func (s *Store) RootDir() string { return s.rootDir }
