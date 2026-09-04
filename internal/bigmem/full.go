// Package bigmem — extended SQLite-based tools.
package bigmem

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	projpkg "github.com/biggs-100/biggz-ai/internal/project"
)

// ─── Session ─────────────────────────────────────────────────────────────────

// Session represents a coding session with directory tracking and branching (REQ-B1).
type Session struct {
	ID            string    `json:"id"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time,omitempty"`
	Summary       string    `json:"summary,omitempty"`
	Project       string    `json:"project,omitempty"`
	Directory     string    `json:"directory,omitempty"`
	ParentID      *string   `json:"parent_id,omitempty"`
	LeafID        string    `json:"leaf_id,omitempty"`
	BranchSummary string    `json:"branch_summary,omitempty"`
}

// ─── Implicit session helpers (Engram parity) ───────────────────────────────

func defaultSessionID(project string) string {
	if strings.TrimSpace(project) == "" {
		return "manual-save"
	}
	return "manual-save-" + strings.TrimSpace(project)
}

func currentWorkingDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

// MostRecentActiveSession resolves the active (un-ended) session for a project
// from the persisted sessions table. Mirrors Engram's Store.MostRecentActiveSession.
func (s *Store) MostRecentActiveSession(project string) (string, bool, error) {
	normalized := projpkg.NormalizeProjectName(strings.TrimSpace(project))
	if strings.TrimSpace(project) == "" || normalized == "" {
		return "", false, nil
	}
	// Use normalized value for query; keep case-insensitive via LOWER.
	var id string
	err := s.db.QueryRow(`
		SELECT id
		FROM sessions
		WHERE LOWER(project) = LOWER(?)
		  AND (end_time IS NULL OR end_time = '')
		  AND id NOT LIKE 'manual-save%'
		ORDER BY datetime(start_time) DESC, id DESC
		LIMIT 1
	`, normalized).Scan(&id)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// EnsureImplicitSession ensures a session row exists for sessionID/project.
// It is idempotent: duplicate primary-key errors are ignored. It also creates
// the sessions table if missing and fills directory with cwd when empty.
func (s *Store) EnsureImplicitSession(sessionID, project string) error {
	return s.EnsureImplicitSessionWithCWD(sessionID, project)
}

func (s *Store) EnsureImplicitSessionWithCWD(sessionID, project string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("sessionID required")
	}
	cwd := currentWorkingDirectory()
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS sessions
		(id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT, parent_id TEXT REFERENCES sessions(id) ON DELETE SET NULL, leaf_id TEXT, branch_summary TEXT)`)
	_ = ensureColumns(s.db, "sessions", []columnDef{
		{name: "parent_id", ddl: "TEXT REFERENCES sessions(id) ON DELETE SET NULL"},
		{name: "leaf_id", ddl: "TEXT"},
		{name: "branch_summary", ddl: "TEXT"},
	})
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_parent_id ON sessions(parent_id)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_leaf_id ON sessions(leaf_id)`)
	_, err := s.db.Exec("INSERT INTO sessions (id, start_time, project, directory, parent_id, leaf_id) VALUES (?, ?, ?, ?, NULL, ?)",
		sessionID, time.Now().Format(time.RFC3339), project, cwd, sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(strings.ToLower(err.Error()), "constraint") || strings.Contains(err.Error(), "PRIMARY") {
			if cwd != "" {
				_, _ = s.db.Exec("UPDATE sessions SET directory = ? WHERE id = ? AND (directory IS NULL OR directory = '')", cwd, sessionID)
			}
			return nil
		}
		return err
	}
	return nil
}

func ensureImplicitSessionWithCWD(s *Store, sessionID, project string) error {
	return s.EnsureImplicitSessionWithCWD(sessionID, project)
}

// resolveFallbackSessionID resolves the session a write should attach to when
// caller did not provide explicit session_id (Engram parity).
func resolveFallbackSessionID(s *Store, project string) string {
	if s != nil {
		if id, ok, err := s.MostRecentActiveSession(project); err == nil && ok {
			return id
		}
	}
	return defaultSessionID(project)
}

// SessionStart registers a new session.
func (s *Store) SessionStart(id, project string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := &Session{ID: id, StartTime: time.Now(), Project: project, LeafID: id}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS sessions
		(id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT, parent_id TEXT REFERENCES sessions(id) ON DELETE SET NULL, leaf_id TEXT, branch_summary TEXT)`)
	if err != nil {
		return nil, err
	}
	_ = ensureColumns(s.db, "sessions", []columnDef{
		{name: "parent_id", ddl: "TEXT REFERENCES sessions(id) ON DELETE SET NULL"},
		{name: "leaf_id", ddl: "TEXT"},
		{name: "branch_summary", ddl: "TEXT"},
	})
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_parent_id ON sessions(parent_id)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_leaf_id ON sessions(leaf_id)`)
	_, err = s.db.Exec("INSERT INTO sessions (id, start_time, project, parent_id, leaf_id) VALUES (?, ?, ?, NULL, ?)",
		id, session.StartTime.Format(time.RFC3339), project, id)
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

// parseSessionTime tries multiple layouts for legacy session timestamps.
func parseSessionTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05 -0700", s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	return time.Time{}
}

// SessionContext returns recent sessions, newest first.
func (s *Store) SessionContext(limit int) ([]Session, error) {
	return s.SessionContextCtx(context.Background(), limit)
}

// SessionContextCtx returns recent sessions with ctx + timeout (CTX-2/3/4).
// Holds the logic; SessionContext is a thin Background() wrapper (D2).
// Cancelled ctx fails visibly with wrapped ctx.Err() before touching SQLite.
func (s *Store) SessionContextCtx(ctx context.Context, limit int) ([]Session, error) {
	ctx, cancel := WithTimeout(ctx)
	defer cancel()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("bigmem session context: %w", ctx.Err())
	}
	if limit <= 0 {
		limit = 5
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS sessions
		(id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT, parent_id TEXT REFERENCES sessions(id) ON DELETE SET NULL, leaf_id TEXT, branch_summary TEXT)`); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem session context: %w", ctx.Err())
		}
		// Legacy ignores DDL errors; preserve parity for non-ctx failures.
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT id, start_time, end_time, summary, project, directory, parent_id, leaf_id, branch_summary FROM sessions ORDER BY start_time DESC LIMIT ?", limit)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem session context: %w", ctx.Err())
		}
		return nil, fmt.Errorf("session query: %w", err)
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var sess Session
		var st, et, summary, dir, parentID, leafID, branchSummary sql.NullString
		if err := rows.Scan(&sess.ID, &st, &et, &summary, &sess.Project, &dir, &parentID, &leafID, &branchSummary); err != nil {
			continue
		}
		if st.Valid {
			sess.StartTime = parseSessionTime(st.String)
		}
		if et.Valid {
			sess.EndTime = parseSessionTime(et.String)
		}
		if summary.Valid {
			sess.Summary = summary.String
		}
		if dir.Valid {
			sess.Directory = dir.String
		}
		if parentID.Valid && parentID.String != "" {
			v := parentID.String
			sess.ParentID = &v
		}
		if leafID.Valid {
			sess.LeafID = leafID.String
		}
		if branchSummary.Valid {
			sess.BranchSummary = branchSummary.String
		}
		sessions = append(sessions, sess)
	}
	if err := rows.Err(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem session context: %w", ctx.Err())
		}
		return nil, err
	}
	return sessions, nil
}

// ─── Branching (REQ-B3/B4/B5) ───────────────────────────────────────────────

// CreateBranch creates a branching session. parentID="" creates a root (parent_id NULL, leaf_id=self).
// Validates parent exists when non-empty; branch_summary is optional.
func (s *Store) CreateBranch(parentID, branchSummary string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Ensure table exists with branching schema.
	s.db.Exec(`CREATE TABLE IF NOT EXISTS sessions
		(id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT, parent_id TEXT REFERENCES sessions(id) ON DELETE SET NULL, leaf_id TEXT, branch_summary TEXT)`)
	_ = ensureColumns(s.db, "sessions", []columnDef{
		{name: "parent_id", ddl: "TEXT REFERENCES sessions(id) ON DELETE SET NULL"},
		{name: "leaf_id", ddl: "TEXT"},
		{name: "branch_summary", ddl: "TEXT"},
	})
	if parentID != "" {
		var exists int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", parentID).Scan(&exists); err != nil {
			return nil, err
		}
		if exists == 0 {
			return nil, fmt.Errorf("parent %q not found", parentID)
		}
	}
	id := fmt.Sprintf("sess-%d-%d", time.Now().UnixNano(), atomic.AddInt64(&globalIDSeq, 1))
	now := time.Now().UTC().Format(time.RFC3339)
	var parentVal any
	if parentID == "" {
		parentVal = nil
	} else {
		parentVal = parentID
	}
	if _, err := s.db.Exec(`INSERT INTO sessions (id, start_time, parent_id, leaf_id, branch_summary) VALUES (?, ?, ?, ?, ?)`, id, now, parentVal, id, branchSummary); err != nil {
		return nil, err
	}
	sess := &Session{ID: id, StartTime: parseSessionTime(now), LeafID: id, BranchSummary: branchSummary}
	if parentID != "" {
		v := parentID
		sess.ParentID = &v
	}
	return sess, nil
}

// GetBranch retrieves a session by ID with branching fields.
func (s *Store) GetBranch(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getBranchLocked(id)
}

func (s *Store) getBranchLocked(id string) (*Session, error) {
	var sess Session
	var st, et, summary, project, dir, parentID, leafID, branchSummary sql.NullString
	// Param ? only — SQL injection safe (REQ-B4 threat).
	err := s.db.QueryRow(`SELECT id, start_time, end_time, summary, project, directory, parent_id, leaf_id, branch_summary FROM sessions WHERE id = ?`, id).Scan(&sess.ID, &st, &et, &summary, &project, &dir, &parentID, &leafID, &branchSummary)
	if err != nil {
		return nil, fmt.Errorf("branch %q not found: %w", id, err)
	}
	if st.Valid {
		sess.StartTime = parseSessionTime(st.String)
	}
	if et.Valid {
		sess.EndTime = parseSessionTime(et.String)
	}
	if summary.Valid {
		sess.Summary = summary.String
	}
	if project.Valid {
		sess.Project = project.String
	}
	if dir.Valid {
		sess.Directory = dir.String
	}
	if parentID.Valid && parentID.String != "" {
		v := parentID.String
		sess.ParentID = &v
	}
	if leafID.Valid {
		sess.LeafID = leafID.String
	}
	if branchSummary.Valid {
		sess.BranchSummary = branchSummary.String
	}
	return &sess, nil
}

// ListBranches returns all sessions ordered by start_time DESC.
func (s *Store) ListBranches() ([]Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT id, start_time, end_time, summary, project, directory, parent_id, leaf_id, branch_summary FROM sessions ORDER BY start_time DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var sess Session
		var st, et, summary, project, dir, parentID, leafID, branchSummary sql.NullString
		if err := rows.Scan(&sess.ID, &st, &et, &summary, &project, &dir, &parentID, &leafID, &branchSummary); err != nil {
			continue
		}
		if st.Valid {
			sess.StartTime = parseSessionTime(st.String)
		}
		if et.Valid {
			sess.EndTime = parseSessionTime(et.String)
		}
		if summary.Valid {
			sess.Summary = summary.String
		}
		if project.Valid {
			sess.Project = project.String
		}
		if dir.Valid {
			sess.Directory = dir.String
		}
		if parentID.Valid && parentID.String != "" {
			v := parentID.String
			sess.ParentID = &v
		}
		if leafID.Valid {
			sess.LeafID = leafID.String
		}
		if branchSummary.Valid {
			sess.BranchSummary = branchSummary.String
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// SetLeaf atomically updates leaf_id under Store.mu (single UPDATE, last-writer-wins, REQ-B5).
func (s *Store) SetLeaf(leafID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if leafID == "" {
		return fmt.Errorf("leafID required")
	}
	var exists int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", leafID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("leaf %q not found", leafID)
	}
	// Single UPDATE under mu — all rows leaf_id tracks active leaf (global pointer) without JOIN.
	// Use param ? only.
	_, err := s.db.Exec(`UPDATE sessions SET leaf_id = ?`, leafID)
	return err
}

// GetLeafPath walks parent_id iteratively leaf→root, depth 100, cycle guard, param ? only (REQ-B4).
func (s *Store) GetLeafPath(leafID string) ([]Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if leafID == "" {
		return nil, fmt.Errorf("leafID required")
	}
	visited := make(map[string]bool, 16)
	var path []Session
	current := leafID
	for i := 0; i < 100 && current != ""; i++ {
		if visited[current] {
			break
		}
		visited[current] = true
		sess, err := s.getBranchLocked(current)
		if err != nil {
			break
		}
		path = append(path, *sess)
		if sess.ParentID == nil || *sess.ParentID == "" {
			break
		}
		current = *sess.ParentID
	}
	return path, nil
}

// SessionContextBranched returns leaf→root path when leafID non-empty, else fallback to linear SessionContext (REQ-B4).
func (s *Store) SessionContextBranched(leafID string, limit int) ([]Session, error) {
	if leafID == "" {
		return s.SessionContext(limit)
	}
	path, err := s.GetLeafPath(leafID)
	if err != nil {
		return nil, err
	}
	if len(path) == 0 {
		return s.SessionContext(limit)
	}
	if limit > 0 && len(path) > limit {
		path = path[:limit]
	}
	return path, nil
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
	return s.SavePromptCtx(context.Background(), content, sessionID)
}

// SavePromptCtx persists a user prompt with ctx + timeout (CTX-2/3/4).
// Holds the logic; SavePrompt is a thin Background() wrapper (D2).
func (s *Store) SavePromptCtx(ctx context.Context, content, sessionID string) (*SavedPrompt, error) {
	ctx, cancel := WithTimeout(ctx)
	defer cancel()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("bigmem save prompt: %w", ctx.Err())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS prompts
		(id TEXT PRIMARY KEY, content TEXT, session_id TEXT, created_at TEXT)`); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem save prompt: %w", ctx.Err())
		}
		// Legacy ignores DDL errors; preserve parity for non-ctx failures.
	}
	// Batch S: private-tag redaction + truncation (Engram parity)
	content = stripPrivateTags(content)
	truncated, _ := truncateIfNeeded(content)
	content = truncated
	p := &SavedPrompt{
		ID:        fmt.Sprintf("prompt-%d", time.Now().UnixNano()),
		Content:   content,
		SessionID: sessionID,
		CreatedAt: time.Now(),
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO prompts (id, content, session_id, created_at) VALUES (?, ?, ?, ?)",
		p.ID, p.Content, p.SessionID, p.CreatedAt.Format(time.RFC3339))
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem save prompt: %w", ctx.Err())
		}
		return p, err
	}
	return p, nil
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
	return s.TimelineCtx(context.Background(), opts)
}

// TimelineCtx returns observations in chronological order with ctx + timeout (CTX-2/3/4).
// Holds the logic; Timeline is a thin Background() wrapper (D2).
func (s *Store) TimelineCtx(ctx context.Context, opts TimelineOptions) ([]TimelineEntry, error) {
	ctx, cancel := WithTimeout(ctx)
	defer cancel()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("bigmem timeline: %w", ctx.Err())
	}
	var rows *sql.Rows
	var err error
	var result []TimelineEntry

	if opts.FocusID != "" {
		var focusTime string
		err := s.db.QueryRowContext(ctx, "SELECT created_at FROM observations WHERE id = ?", opts.FocusID).Scan(&focusTime)
		if err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("bigmem timeline: %w", ctx.Err())
			}
			return nil, fmt.Errorf("focus not found: %w", err)
		}

		beforeLimit := 5
		if opts.Before > 0 {
			beforeLimit = opts.Before
		}
		beforeRows, err := s.db.QueryContext(ctx,
			`SELECT id, title, type, created_at FROM observations
			 WHERE created_at < ? AND id != ? AND deleted_at IS NULL
			 ORDER BY created_at DESC LIMIT ?`, focusTime, opts.FocusID, beforeLimit)
		if err != nil && ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem timeline: %w", ctx.Err())
		}
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
			if err := beforeRows.Err(); err != nil && ctx.Err() != nil {
				return nil, fmt.Errorf("bigmem timeline: %w", ctx.Err())
			}
		}

		var focusEntry TimelineEntry
		var ca string
		if err := s.db.QueryRowContext(ctx, "SELECT id, title, type, created_at FROM observations WHERE id = ?", opts.FocusID).
			Scan(&focusEntry.ID, &focusEntry.Title, &focusEntry.Type, &ca); err == nil {
			focusEntry.CreatedAt, _ = time.Parse(time.RFC3339, ca)
			focusEntry.IsFocus = true
			result = append(result, focusEntry)
		} else if ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem timeline: %w", ctx.Err())
		}

		afterLimit := 5
		if opts.After > 0 {
			afterLimit = opts.After
		}
		afterRows, err := s.db.QueryContext(ctx,
			`SELECT id, title, type, created_at FROM observations
			 WHERE created_at > ? AND id != ? AND deleted_at IS NULL
			 ORDER BY created_at ASC LIMIT ?`, focusTime, opts.FocusID, afterLimit)
		if err != nil && ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem timeline: %w", ctx.Err())
		}
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
			if err := afterRows.Err(); err != nil && ctx.Err() != nil {
				return nil, fmt.Errorf("bigmem timeline: %w", ctx.Err())
			}
		}
		if ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem timeline: %w", ctx.Err())
		}
		return result, nil
	}

	limit := 20
	if opts.Limit > 0 {
		limit = opts.Limit
	}
	rows, err = s.db.QueryContext(ctx,
		"SELECT id, title, type, created_at FROM observations WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem timeline: %w", ctx.Err())
		}
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
	if err := rows.Err(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem timeline: %w", ctx.Err())
		}
		return nil, err
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
	StoreExists          bool   `json:"store_exists"`
	Observations         int    `json:"observations"`
	CorruptFiles         int    `json:"corrupt_files"`
	StoragePath          string `json:"storage_path"`
	Corrupt              bool   `json:"corrupt"`
	BranchColumnsMissing bool   `json:"branch_columns_missing,omitempty"`
	NeedsMigration       bool   `json:"needs_migration,omitempty"`
}

// Doctor runs diagnostics including PRAGMA integrity_check and branching schema check (REQ-B2).
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

	// Branching schema check: flag missing columns as fixable (REQ-B2).
	missing := hasMissingBranchColumns(s.db)
	if missing {
		r.BranchColumnsMissing = true
		r.NeedsMigration = true
		r.Corrupt = true // fixable corruption
	}
	return r, nil
}

// hasMissingBranchColumns reports true if sessions lacks parent_id/leaf_id/branch_summary.
func hasMissingBranchColumns(db *sql.DB) bool {
	rows, err := db.Query("PRAGMA table_info(sessions)")
	if err != nil {
		return false
	}
	defer rows.Close()
	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		existing[name] = true
	}
	return !existing["parent_id"] || !existing["leaf_id"] || !existing["branch_summary"]
}

// DoctorFix repairs WAL, FTS and schema issues, matching engram's behavior.
// It is idempotent and safe to run on a healthy database. Atomic FTS rebuild
// per A2: DROP TRIGGER, DELETE FROM observations_fts, INSERT SELECT,
// REINDEX + PRAGMA integrity_check + PRAGMA wal_checkpoint(TRUNCATE) before
// any copy. Also migrates sessions.start_time NULLs to RFC3339 now.
func (s *Store) DoctorFix() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Flush WAL — PASSIVE then TRUNCATE to match engram's fix.
	_, _ = s.db.Exec("PRAGMA busy_timeout=5000")
	_, _ = s.db.Exec("PRAGMA wal_checkpoint(PASSIVE)")
	_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")

	// 2. VACUUM to compact and fix malformed database errors.
	_, _ = s.db.Exec("VACUUM")

	// 3. Schema migration: ensure sessions table and branching columns (REQ-B1/B2).
	var sessExists int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&sessExists)
	if sessExists == 0 {
		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS sessions
			(id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT, parent_id TEXT REFERENCES sessions(id) ON DELETE SET NULL, leaf_id TEXT, branch_summary TEXT)`)
	} else {
		rows, err := s.db.Query("PRAGMA table_info(sessions)")
		if err == nil {
			hasDir := false
			hasStartTime := false
			hasParent := false
			hasLeaf := false
			hasBranchSummary := false
			for rows.Next() {
				var cid int
				var name, ctype string
				var notnull, pk int
				var dflt sql.NullString
				if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
					continue
				}
				if name == "directory" {
					hasDir = true
				}
				if name == "start_time" {
					hasStartTime = true
				}
				if name == "parent_id" {
					hasParent = true
				}
				if name == "leaf_id" {
					hasLeaf = true
				}
				if name == "branch_summary" {
					hasBranchSummary = true
				}
			}
			rows.Close()
			if !hasDir {
				_, _ = s.db.Exec("ALTER TABLE sessions ADD COLUMN directory TEXT")
			}
			if !hasStartTime {
				_, _ = s.db.Exec("ALTER TABLE sessions ADD COLUMN start_time TEXT")
			}
			if !hasParent {
				_, _ = s.db.Exec("ALTER TABLE sessions ADD COLUMN parent_id TEXT REFERENCES sessions(id) ON DELETE SET NULL")
			}
			if !hasLeaf {
				_, _ = s.db.Exec("ALTER TABLE sessions ADD COLUMN leaf_id TEXT")
			}
			if !hasBranchSummary {
				_, _ = s.db.Exec("ALTER TABLE sessions ADD COLUMN branch_summary TEXT")
			}
		}
	}
	// Ensure branching indexes idempotently.
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_parent_id ON sessions(parent_id)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_leaf_id ON sessions(leaf_id)`)
	// Migrate NULL/empty/zero start_time to RFC3339 now (covers 0001-01-01 from NULL parsing)
	nowRFC3339 := time.Now().UTC().Format(time.RFC3339)
	_, _ = s.db.Exec(`UPDATE sessions SET start_time = ? WHERE start_time IS NULL OR start_time = '' OR start_time = '0001-01-01T00:00:00Z' OR start_time = '0001-01-01 00:00:00 +0000 UTC'`, nowRFC3339)
	// Also handle sessions with NULL but created via direct SQL that may have empty string
	_, _ = s.db.Exec("UPDATE sessions SET start_time = ? WHERE start_time IS NULL", nowRFC3339)
	// Branch backfill: legacy rows leaf_id=self (REQ-B2), parent_id stays NULL.
	_, _ = s.db.Exec(`UPDATE sessions SET leaf_id = id WHERE leaf_id IS NULL OR leaf_id = ''`)

	// 4. Atomic FTS rebuild — DROP TRIGGER, DELETE, INSERT SELECT, REINDEX, integrity_check, checkpoint.
	// Always rebuild atomically to fix stale FTS desync (FTS hit that Get cannot find).
	_, _ = s.db.Exec("DROP TRIGGER IF EXISTS obs_fts_ai")
	_, _ = s.db.Exec("DROP TRIGGER IF EXISTS obs_fts_ad")
	_, _ = s.db.Exec("DROP TRIGGER IF EXISTS obs_fts_au")
	_, _ = s.db.Exec("DROP TRIGGER IF EXISTS obs_fts_insert")
	_, _ = s.db.Exec("DROP TRIGGER IF EXISTS obs_fts_delete")
	_, _ = s.db.Exec("DROP TRIGGER IF EXISTS obs_fts_update")
	// Ensure FTS table exists (create if missing, e.g. after DROP TABLE in corrupted state)
	var ftsExists int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='observations_fts'").Scan(&ftsExists)
	if ftsExists == 0 {
		_, _ = s.db.Exec(`CREATE VIRTUAL TABLE observations_fts USING fts5(
				title, content, topic_key, tool_name, type, project,
				content='observations',
				content_rowid='rowid'
			);`)
	} else {
		// Atomic clear: DELETE removes all indexed rows without dropping table.
		// If DELETE fails due to stale/orphan state, fallback to DROP+CREATE.
		if _, err := s.db.Exec("DELETE FROM observations_fts"); err != nil {
			_, _ = s.db.Exec("DROP TABLE IF EXISTS observations_fts")
			_, _ = s.db.Exec(`CREATE VIRTUAL TABLE observations_fts USING fts5(
				title, content, topic_key, tool_name, type, project,
				content='observations',
				content_rowid='rowid'
			);`)
		}
	}
	// Repopulate from observations (only non-deleted). If INSERT fails (e.g. after failed DELETE),
	// ensure table exists and retry once.
	if _, err := s.db.Exec(`INSERT INTO observations_fts(rowid, title, content, topic_key, tool_name, type, project)
		SELECT rowid, title, content, topic_key, tool_name, type, project FROM observations WHERE deleted_at IS NULL`); err != nil {
		_, _ = s.db.Exec("DROP TABLE IF EXISTS observations_fts")
		_, _ = s.db.Exec(`CREATE VIRTUAL TABLE observations_fts USING fts5(
				title, content, topic_key, tool_name, type, project,
				content='observations',
				content_rowid='rowid'
			);`)
		_, _ = s.db.Exec(`INSERT INTO observations_fts(rowid, title, content, topic_key, tool_name, type, project)
			SELECT rowid, title, content, topic_key, tool_name, type, project FROM observations WHERE deleted_at IS NULL`)
	}
	// Recreate triggers
	_, _ = s.db.Exec(`
			CREATE TRIGGER IF NOT EXISTS obs_fts_ai AFTER INSERT ON observations BEGIN
				INSERT INTO observations_fts(rowid, title, content, topic_key, tool_name, type, project)
				VALUES (new.rowid, new.title, new.content, new.topic_key, new.tool_name, new.type, new.project);
			END;
			CREATE TRIGGER IF NOT EXISTS obs_fts_ad AFTER DELETE ON observations BEGIN
				INSERT INTO observations_fts(observations_fts, rowid, title, content, topic_key, tool_name, type, project)
				VALUES ('delete', old.rowid, old.title, old.content, old.topic_key, old.tool_name, old.type, old.project);
			END;
			CREATE TRIGGER IF NOT EXISTS obs_fts_au AFTER UPDATE ON observations BEGIN
				INSERT INTO observations_fts(observations_fts, rowid, title, content, topic_key, tool_name, type, project)
				VALUES ('delete', old.rowid, old.title, old.content, old.topic_key, old.tool_name, old.type, old.project);
				INSERT INTO observations_fts(rowid, title, content, topic_key, tool_name, type, project)
				VALUES (new.rowid, new.title, new.content, new.topic_key, new.tool_name, new.type, new.project);
			END;
		`)
	// REINDEX to rebuild internal FTS indexes
	_, _ = s.db.Exec("REINDEX observations_fts")
	// Fallback FTS5 rebuild command for older SQLite
	_, _ = s.db.Exec(`INSERT INTO observations_fts(observations_fts) VALUES('rebuild')`)
	// Integrity check + WAL checkpoint before any copy (per spec)
	_, _ = s.db.Exec("PRAGMA integrity_check")
	_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")

	return nil
}

// FixResult reports blob migration results.
type FixResult struct {
	Migrated int `json:"migrated"`
	Skipped  int `json:"skipped"`
	Errors   int `json:"errors"`
}

// DoctorFixBlobs migrates legacy large rows to blob storage.
func (s *Store) DoctorFixBlobs() (*FixResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := &FixResult{}
	_ = s.db.QueryRow("SELECT COUNT(*) FROM observations WHERE content LIKE 'blob:sha256:%' AND deleted_at IS NULL").Scan(&res.Skipped)
	rows, err := s.db.Query(`SELECT id, content FROM observations WHERE (length(content) > 100000 OR content LIKE 'data:image/%') AND content NOT LIKE 'blob:sha256:%' AND deleted_at IS NULL`)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	var ids []string
	var contents []string
	for rows.Next() {
		var id, content string
		if err := rows.Scan(&id, &content); err != nil {
			res.Errors++
			continue
		}
		ids = append(ids, id)
		contents = append(contents, content)
	}
	if err := rows.Err(); err != nil {
		return res, err
	}
	for i, id := range ids {
		content := contents[i]
		addr, err := PutBlob([]byte(content))
		if err != nil {
			res.Errors++
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = s.db.Exec("UPDATE observations SET content = ?, updated_at = ? WHERE id = ?", addr, now, id)
		if err != nil {
			res.Errors++
			continue
		}
		res.Migrated++
	}
	return res, nil
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
	chunkPath := filepath.Join(dir, name)
	f, err := os.Create(chunkPath)
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
		// Bundle referenced blob bytes into the export so a fresh root can
		// restore them on import instead of receiving an orphan address.
		if IsBlobAddr(obs.Content) {
			data, err := GetBlob(obs.Content)
			if err != nil {
				f.Close()
				os.Remove(chunkPath)
				return fmt.Errorf("sync export: blob %s missing bytes for observation %s: %w", obs.Content, obs.ID, err)
			}
			obs.Content = string(data)
		}
		if err := enc.Encode(obs); err != nil {
			return err
		}
		count++
	}
	if count == 0 {
		os.Remove(chunkPath)
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
	orphans := 0
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
			// Orphan blob refs (legacy exports or missing bytes) must not be
			// inserted: Get would hand back the raw address. Skip visibly.
			if IsBlobAddr(obs.Content) {
				if _, err := GetBlob(obs.Content); err != nil {
					orphans++
					continue
				}
			} else if len(obs.Content) > maxStoredBytes || ShouldExternalize(obs.Content) {
				// Restore large payloads to blob storage in this root so the
				// imported row matches source behavior (addr, not inline bulk).
				// PutBlob failure keeps raw inline: bytes preserved, no orphan.
				if addr, err := PutBlob([]byte(obs.Content)); err == nil {
					obs.Content = addr
				}
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
	if orphans > 0 {
		return total, fmt.Errorf("sync import: %d observation(s) reference missing blobs; skipped instead of inserting orphan refs", orphans)
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
