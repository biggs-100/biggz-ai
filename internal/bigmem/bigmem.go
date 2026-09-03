// Package bigmem provides a persistent memory store using SQLite.
// Compatible with gentle-ai's engram protocol — 22 MCP tools.
// Full parity with engram except cloud sync.
package bigmem

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	projpkg "github.com/biggs-100/biggz-ai/internal/project"
	_ "modernc.org/sqlite"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// Observation is a single memory entry, matching engram's schema.
type Observation struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Type            string    `json:"type"`
	Content         string    `json:"content"`
	SessionID       string    `json:"session_id,omitempty"`
	ToolName        string    `json:"tool_name,omitempty"`
	TopicKey        string    `json:"topic_key,omitempty"`
	Project         string    `json:"project,omitempty"`
	Scope           string    `json:"scope,omitempty"`
	NormalizedHash  string    `json:"-"`
	RevisionCount   int       `json:"revision_count,omitempty"`
	DuplicateCount  int       `json:"duplicate_count,omitempty"`
	LastSeenAt      *string   `json:"last_seen_at,omitempty"`
	ReviewAfter     *string   `json:"review_after,omitempty"`
	ExpiresAt       *string   `json:"expires_at,omitempty"`
	EmbeddingModel  string    `json:"embedding_model,omitempty"`
	EmbeddingVector []byte    `json:"-"`
	Pinned          bool      `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	DeletedAt       *string   `json:"deleted_at,omitempty"`
}

// State returns "active" or "needs_review" based on review_after vs now.
func (o *Observation) State() string {
	if o.ReviewAfter == nil || *o.ReviewAfter == "" {
		return "active"
	}
	ra, err := time.Parse(time.RFC3339, *o.ReviewAfter)
	if err != nil {
		return "active"
	}
	if !ra.After(time.Now().UTC()) {
		return "needs_review"
	}
	return "active"
}

// ObservationState constants
const (
	ObservationStateActive      = "active"
	ObservationStateNeedsReview = "needs_review"
)

// Store manages observations using SQLite.
type Store struct {
	mu      sync.RWMutex
	db      *sql.DB
	rootDir string
}

// ─── Rescue Ownership Types ─────────────────────────────────────────────────

// AmbiguousEntry describes a session that cannot be adopted due to foreign observations.
type AmbiguousEntry struct {
	SessionID      string `json:"session_id"`
	ForeignProject string `json:"foreign_project"`
}

// RescuePlan is the two-phase plan for bulk rescue (dry-run).
type RescuePlan struct {
	Project   string           `json:"project"`
	Total     int              `json:"total"`
	Adoptable []string         `json:"adoptable"`
	Ambiguous []AmbiguousEntry `json:"ambiguous"`
}

// RescueResult is the result of a rescue operation.
type RescueResult struct {
	Adopted   int              `json:"adopted"`
	Skipped   int              `json:"skipped"`
	Ambiguous []AmbiguousEntry `json:"ambiguous"`
}

// RescueOptions controls RescueNullProjectOwnership behavior.
type RescueOptions struct {
	DryRun    bool
	SessionID string
}

// globalIDSeq ensures obs IDs are unique even when time.Now().UnixNano()
// collides (Windows clock resolution ~15ms causes rapid Saves to share the
// same timestamp and ON CONFLICT would overwrite the previous row).
var globalIDSeq int64

// privateTagRegex redacts <private>...</private> blocks (Engram parity, Batch S).
var privateTagRegex = regexp.MustCompile("(?is)<private>.*?</private>")

// ErrProjectRequired is returned when project is required but empty.
var ErrProjectRequired = errors.New("project required")

// ErrProjectOwnershipAmbiguous is returned when session has foreign observations and cannot be adopted.
var ErrProjectOwnershipAmbiguous = errors.New("project ownership ambiguous")

// TruncationMetadata tracks whether content was truncated to maxStoredBytes.
type TruncationMetadata struct {
	OriginalBytes int  `json:"original_bytes"`
	LimitBytes    int  `json:"limit_bytes"`
	Truncated     bool `json:"truncated"`
}

// stripPrivateTags replaces <private>...</private> with [REDACTED].
func stripPrivateTags(s string) string {
	return privateTagRegex.ReplaceAllString(s, "[REDACTED]")
}

// truncateIfNeeded enforces maxStoredBytes (50k) Engram parity.
func truncateIfNeeded(s string) (string, TruncationMetadata) {
	meta := TruncationMetadata{OriginalBytes: len(s), LimitBytes: maxStoredBytes}
	if len(s) <= maxStoredBytes {
		return s, meta
	}
	meta.Truncated = true
	return s[:maxStoredBytes] + " ... [truncated]", meta
}

// truncationWarning returns a human-readable warning when content was truncated.
func truncationWarning(meta TruncationMetadata) string {
	if !meta.Truncated {
		return ""
	}
	return fmt.Sprintf("⚠️ content truncated from %d to %d bytes", meta.OriginalBytes, meta.LimitBytes)
}

// parseMarkedBy splits "actor:kind:model" into three parts (Engram parity, Batch S).
func parseMarkedBy(s string) (actor, kind, model string) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) > 0 {
		actor = parts[0]
	}
	if len(parts) > 1 {
		kind = parts[1]
	}
	if len(parts) > 2 {
		model = parts[2]
	}
	return
}

// computeReviewAfterAt returns RFC3339 review_after using calendar months (Engram parity).
// Uses AddDate for accurate 6/12/3 month decay (Batch S).
func computeReviewAfterAt(now time.Time, obsType string) *string {
	var t time.Time
	switch obsType {
	case "decision":
		t = now.AddDate(0, 6, 0)
	case "policy":
		t = now.AddDate(0, 12, 0)
	case "preference":
		t = now.AddDate(0, 3, 0)
	default:
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// SearchOptions filter search results, matching engram's interface.
type SearchOptions struct {
	Project     string   `json:"project,omitempty"`
	Type        string   `json:"type,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	MatchMode   string   `json:"match_mode,omitempty"`   // "" | "all" (default) | "any"
	AllProjects bool     `json:"all_projects,omitempty"` // when true, project filter is ignored (Engram parity)
	BM25Floor   *float64 `json:"bm25_floor,omitempty"`   // nil = no floor; forwarded from MCPConfig
}

// SearchResult wraps an observation with its BM25 rank.
type SearchResult struct {
	Observation
	Rank float64 `json:"rank,omitempty"`
}

// ─── Config ──────────────────────────────────────────────────────────────────

const (
	defaultMaxObservationLength = 50000
	defaultMaxSearchResults     = 20
	defaultDedupeWindow         = 15 * time.Minute
	maxStoredBytes              = 50000
)

// ─── Unified DB Path Resolution (A1) ───────────────────────────────────────

// defaultBigmemRoot returns the default BigMem root (~/.biggz/bigmem).
func defaultBigmemRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".biggz", "bigmem")
}

// bigmemDBPathForRoot returns bigmem.db path for a given root.
func bigmemDBPathForRoot(root string) string { return filepath.Join(root, "bigmem.db") }

// recoveredDBPathForRoot returns the recovered DB path sibling to root.
func recoveredDBPathForRoot(root string) string {
	return filepath.Join(filepath.Dir(root), "bigmem_recovered", "bigmem.db")
}

// isGhostWAL reports whether dbPath has a stale WAL=0 + SHM>0 zombie lock (Windows).
// REQ-GW1: stale only when wal==0 && shm>0 && Since(ModTime)>5min.
func isGhostWAL(dbPath string) bool {
	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"
	walInfo, err := os.Stat(walPath)
	if err != nil || walInfo.Size() != 0 {
		return false
	}
	shmInfo, err := os.Stat(shmPath)
	if err != nil || shmInfo.Size() == 0 {
		return false
	}
	if time.Since(shmInfo.ModTime()) <= 5*time.Minute {
		return false
	}
	return true
}

// probeGhostLiveness attempts atomic O_CREATE|O_EXCL probe to detect live holder.
// Returns true if probe succeeds (no live holder, safe to reclaim), false if busy.
// Uses a sibling probe file with O_EXCL for atomic claim; fail means live holder or race.
func probeGhostLiveness(dbPath string) bool {
	probePath := dbPath + ".ghost_probe"
	f, err := os.OpenFile(probePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return false
	}
	f.Close()
	_ = os.Remove(probePath)
	return true
}

// checkpointDB runs PRAGMA wal_checkpoint(TRUNCATE) on the given DB file.
func checkpointDB(dbPath string) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	db.Exec("PRAGMA busy_timeout=5000")
	_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
}

// safeCopyDB copies srcPath to dstPath with a WAL checkpoint before copy.
// It first tries VACUUM INTO (SQLite backup API) which is crash-safe and
// handles WAL atomically; on failure it falls back to checkpoint+ReadFile
// with a documented risk comment (raw copy may miss uncheckpointed WAL if
// VACUUM INTO is unavailable).
func safeCopyDB(srcPath, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}
	// Checkpoint src before any copy to flush WAL into main DB.
	checkpointDB(srcPath)
	db, err := sql.Open("sqlite", srcPath)
	if err == nil {
		defer db.Close()
		db.Exec("PRAGMA busy_timeout=5000")
		escaped := strings.ReplaceAll(dstPath, "'", "''")
		if _, err := db.Exec(fmt.Sprintf("VACUUM INTO '%s'", escaped)); err == nil {
			return nil
		}
	}
	// Fallback: raw file copy after checkpoint (risk: if checkpoint failed,
	// WAL pages remain in -wal and raw copy will be stale; VACUUM INTO above
	// is preferred for atomicity).
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, 0644)
}

// getMaxUpdatedAt returns MAX(updated_at) from observations for the given DB.
func getMaxUpdatedAt(dbPath string) string {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ""
	}
	defer db.Close()
	db.Exec("PRAGMA busy_timeout=5000")
	var max sql.NullString
	_ = db.QueryRow("SELECT MAX(updated_at) FROM observations").Scan(&max)
	if max.Valid {
		return max.String
	}
	return ""
}

// mergeRecoveredIntoPrimary merges recovered DB into primary dedup by id,
// keeping the row with max(updated_at). Uses Go iteration to avoid SQLite
// version-sensitive UPSERT SELECT syntax; compares updated_at as RFC3339 strings.
func mergeRecoveredIntoPrimary(primaryPath, recoveredPath string) error {
	primDB, err := sql.Open("sqlite", primaryPath)
	if err != nil {
		return err
	}
	defer primDB.Close()
	primDB.Exec("PRAGMA busy_timeout=5000")
	recDB, err := sql.Open("sqlite", recoveredPath)
	if err != nil {
		return err
	}
	defer recDB.Close()
	recDB.Exec("PRAGMA busy_timeout=5000")
	// Ensure target tables exist (no-op if already)
	_, _ = primDB.Exec(`CREATE TABLE IF NOT EXISTS observations (
		id TEXT PRIMARY KEY, title TEXT, type TEXT, content TEXT, session_id TEXT,
		tool_name TEXT, topic_key TEXT, project TEXT, scope TEXT,
		normalized_hash TEXT, revision_count INTEGER, duplicate_count INTEGER,
		last_seen_at TEXT, review_after TEXT, pinned INTEGER,
		created_at TEXT, updated_at TEXT, deleted_at TEXT)`)
	_, _ = primDB.Exec(`CREATE TABLE IF NOT EXISTS sessions
		(id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT)`)
	_, _ = primDB.Exec(`CREATE TABLE IF NOT EXISTS prompts (id TEXT PRIMARY KEY, content TEXT, session_id TEXT, created_at TEXT)`)
	// Merge observations row by row
	rows, err := recDB.Query(`SELECT id, title, type, content, session_id, tool_name, topic_key, project, scope, normalized_hash, revision_count, duplicate_count, last_seen_at, review_after, pinned, created_at, updated_at, deleted_at FROM observations`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, title, typ, content, sessionID, toolName, topicKey, project, scope, normalizedHash, createdAt, updatedAt string
			var revisionCount, duplicateCount, pinned int
			var lastSeenAt, reviewAfter, deletedAt sql.NullString
			if err := rows.Scan(&id, &title, &typ, &content, &sessionID, &toolName, &topicKey, &project, &scope, &normalizedHash, &revisionCount, &duplicateCount, &lastSeenAt, &reviewAfter, &pinned, &createdAt, &updatedAt, &deletedAt); err != nil {
				continue
			}
			var primUpdated sql.NullString
			err := primDB.QueryRow("SELECT updated_at FROM observations WHERE id = ?", id).Scan(&primUpdated)
			if err == sql.ErrNoRows {
				_, _ = primDB.Exec(`INSERT INTO observations (id, title, type, content, session_id, tool_name, topic_key, project, scope, normalized_hash, revision_count, duplicate_count, last_seen_at, review_after, pinned, created_at, updated_at, deleted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					id, title, typ, content, sessionID, toolName, topicKey, project, scope, normalizedHash, revisionCount, duplicateCount, lastSeenAt, reviewAfter, pinned, createdAt, updatedAt, deletedAt)
			} else if err == nil && primUpdated.Valid && updatedAt > primUpdated.String {
				_, _ = primDB.Exec(`UPDATE observations SET title=?, type=?, content=?, session_id=?, tool_name=?, topic_key=?, project=?, scope=?, normalized_hash=?, revision_count=?, duplicate_count=?, last_seen_at=?, review_after=?, pinned=?, created_at=?, updated_at=?, deleted_at=? WHERE id=?`,
					title, typ, content, sessionID, toolName, topicKey, project, scope, normalizedHash, revisionCount, duplicateCount, lastSeenAt, reviewAfter, pinned, createdAt, updatedAt, deletedAt, id)
			} else if err == nil && !primUpdated.Valid && updatedAt != "" {
				_, _ = primDB.Exec(`UPDATE observations SET title=?, type=?, content=?, session_id=?, tool_name=?, topic_key=?, project=?, scope=?, normalized_hash=?, revision_count=?, duplicate_count=?, last_seen_at=?, review_after=?, pinned=?, created_at=?, updated_at=?, deleted_at=? WHERE id=?`,
					title, typ, content, sessionID, toolName, topicKey, project, scope, normalizedHash, revisionCount, duplicateCount, lastSeenAt, reviewAfter, pinned, createdAt, updatedAt, deletedAt, id)
			}
		}
		_ = rows.Err()
	}
	// Merge sessions: insert or ignore
	sRows, err := recDB.Query(`SELECT id, start_time, end_time, summary, project, directory FROM sessions`)
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var id, startTime, endTime, summary, project, directory sql.NullString
			if err := sRows.Scan(&id, &startTime, &endTime, &summary, &project, &directory); err != nil {
				continue
			}
			if !id.Valid {
				continue
			}
			_, _ = primDB.Exec(`INSERT OR IGNORE INTO sessions (id, start_time, end_time, summary, project, directory) VALUES (?, ?, ?, ?, ?, ?)`, id.String, startTime.String, endTime.String, summary.String, project.String, directory.String)
		}
	}
	// Merge prompts
	pRows, err := recDB.Query(`SELECT id, content, session_id, created_at FROM prompts`)
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var id, content, sessionID, createdAt sql.NullString
			if err := pRows.Scan(&id, &content, &sessionID, &createdAt); err != nil {
				continue
			}
			if !id.Valid {
				continue
			}
			_, _ = primDB.Exec(`INSERT OR IGNORE INTO prompts (id, content, session_id, created_at) VALUES (?, ?, ?, ?)`, id.String, content.String, sessionID.String, createdAt.String)
		}
	}
	return nil
}

// mergeDB merges srcPath into dstPath dedup by id, keeping max(updated_at).
func mergeDB(srcPath, dstPath string) error {
	srcDB, err := sql.Open("sqlite", srcPath)
	if err != nil {
		return err
	}
	defer srcDB.Close()
	srcDB.Exec("PRAGMA busy_timeout=5000")
	dstDB, err := sql.Open("sqlite", dstPath)
	if err != nil {
		return err
	}
	defer dstDB.Close()
	dstDB.Exec("PRAGMA busy_timeout=5000")
	_, _ = dstDB.Exec(`CREATE TABLE IF NOT EXISTS observations (
		id TEXT PRIMARY KEY, title TEXT, type TEXT, content TEXT, session_id TEXT,
		tool_name TEXT, topic_key TEXT, project TEXT, scope TEXT,
		normalized_hash TEXT, revision_count INTEGER, duplicate_count INTEGER,
		last_seen_at TEXT, review_after TEXT, pinned INTEGER,
		created_at TEXT, updated_at TEXT, deleted_at TEXT)`)
	_, _ = dstDB.Exec(`CREATE TABLE IF NOT EXISTS sessions
		(id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT)`)
	_, _ = dstDB.Exec(`CREATE TABLE IF NOT EXISTS prompts (id TEXT PRIMARY KEY, content TEXT, session_id TEXT, created_at TEXT)`)
	rows, err := srcDB.Query(`SELECT id, title, type, content, session_id, tool_name, topic_key, project, scope, normalized_hash, revision_count, duplicate_count, last_seen_at, review_after, pinned, created_at, updated_at, deleted_at FROM observations`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, title, typ, content, sessionID, toolName, topicKey, project, scope, normalizedHash, createdAt, updatedAt string
			var revisionCount, duplicateCount, pinned int
			var lastSeenAt, reviewAfter, deletedAt sql.NullString
			if err := rows.Scan(&id, &title, &typ, &content, &sessionID, &toolName, &topicKey, &project, &scope, &normalizedHash, &revisionCount, &duplicateCount, &lastSeenAt, &reviewAfter, &pinned, &createdAt, &updatedAt, &deletedAt); err != nil {
				continue
			}
			var dstUpdated sql.NullString
			err := dstDB.QueryRow("SELECT updated_at FROM observations WHERE id = ?", id).Scan(&dstUpdated)
			if err == sql.ErrNoRows {
				_, _ = dstDB.Exec(`INSERT INTO observations (id, title, type, content, session_id, tool_name, topic_key, project, scope, normalized_hash, revision_count, duplicate_count, last_seen_at, review_after, pinned, created_at, updated_at, deleted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					id, title, typ, content, sessionID, toolName, topicKey, project, scope, normalizedHash, revisionCount, duplicateCount, lastSeenAt, reviewAfter, pinned, createdAt, updatedAt, deletedAt)
			} else if err == nil && dstUpdated.Valid && updatedAt > dstUpdated.String {
				_, _ = dstDB.Exec(`UPDATE observations SET title=?, type=?, content=?, session_id=?, tool_name=?, topic_key=?, project=?, scope=?, normalized_hash=?, revision_count=?, duplicate_count=?, last_seen_at=?, review_after=?, pinned=?, created_at=?, updated_at=?, deleted_at=? WHERE id=?`,
					title, typ, content, sessionID, toolName, topicKey, project, scope, normalizedHash, revisionCount, duplicateCount, lastSeenAt, reviewAfter, pinned, createdAt, updatedAt, deletedAt, id)
			} else if err == nil && !dstUpdated.Valid && updatedAt != "" {
				_, _ = dstDB.Exec(`UPDATE observations SET title=?, type=?, content=?, session_id=?, tool_name=?, topic_key=?, project=?, scope=?, normalized_hash=?, revision_count=?, duplicate_count=?, last_seen_at=?, review_after=?, pinned=?, created_at=?, updated_at=?, deleted_at=? WHERE id=?`,
					title, typ, content, sessionID, toolName, topicKey, project, scope, normalizedHash, revisionCount, duplicateCount, lastSeenAt, reviewAfter, pinned, createdAt, updatedAt, deletedAt, id)
			}
		}
		_ = rows.Err()
	}
	sRows, err := srcDB.Query(`SELECT id, start_time, end_time, summary, project, directory FROM sessions`)
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var id, startTime, endTime, summary, project, directory sql.NullString
			if err := sRows.Scan(&id, &startTime, &endTime, &summary, &project, &directory); err != nil {
				continue
			}
			if !id.Valid {
				continue
			}
			_, _ = dstDB.Exec(`INSERT OR IGNORE INTO sessions (id, start_time, end_time, summary, project, directory) VALUES (?, ?, ?, ?, ?, ?)`, id.String, startTime.String, endTime.String, summary.String, project.String, directory.String)
		}
	}
	pRows, err := srcDB.Query(`SELECT id, content, session_id, created_at FROM prompts`)
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var id, content, sessionID, createdAt sql.NullString
			if err := pRows.Scan(&id, &content, &sessionID, &createdAt); err != nil {
				continue
			}
			if !id.Valid {
				continue
			}
			_, _ = dstDB.Exec(`INSERT OR IGNORE INTO prompts (id, content, session_id, created_at) VALUES (?, ?, ?, ?)`, id.String, content.String, sessionID.String, createdAt.String)
		}
	}
	return nil
}

// ResolveDBPath returns the canonical DB path for rootDir, handling ghost WAL/SHM,
// WAL checkpoint before copy, VACUUM INTO backup, and merging recovered DB by
// max(updated_at) dedup. It emits a warning to stderr if both DBs exist.
func ResolveDBPath(rootDir string) (string, error) {
	if rootDir == "" {
		rootDir = defaultBigmemRoot()
		if rootDir == "" {
			return "", fmt.Errorf("home dir: not found")
		}
	}
	// Ensure caller root exists for primary
	_ = os.MkdirAll(rootDir, 0755)
	primaryPath := bigmemDBPathForRoot(rootDir)
	recoveredPath := recoveredDBPathForRoot(rootDir)
	walPath := primaryPath + "-wal"
	shmPath := primaryPath + "-shm"
	needsFallback := false
	if isGhostWAL(primaryPath) {
		if probeGhostLiveness(primaryPath) {
			// GW2: stale + O_EXCL probe ok -> Remove wal/shm + checkpoint before sql.Open
			_ = os.Remove(walPath)
			_ = os.Remove(shmPath)
			if _, err := os.Stat(shmPath); err == nil {
				if f, err := os.OpenFile(shmPath, os.O_WRONLY, 0644); err == nil {
					_ = f.Truncate(0)
					f.Close()
					_ = os.Remove(shmPath)
				}
			}
			if _, err := os.Stat(walPath); err == nil {
				if f, err := os.OpenFile(walPath, os.O_WRONLY, 0644); err == nil {
					_ = f.Truncate(0)
					f.Close()
					_ = os.Remove(walPath)
				}
			}
			// checkpoint best-effort before sql.Open, swallow errors (GW2)
			checkpointDB(primaryPath)
			if _, err := os.Stat(shmPath); err == nil {
				needsFallback = true
			}
			if _, err := os.Stat(walPath); err == nil {
				needsFallback = true
			}
		} else {
			// GW3: busy/O_EXCL fails -> preserve wal/shm, fallback intact
			needsFallback = true
		}
	}
	if needsFallback {
		recoveredDir := filepath.Dir(recoveredPath)
		_ = os.MkdirAll(recoveredDir, 0755)
		if _, err := os.Stat(recoveredPath); os.IsNotExist(err) {
			if _, err := os.Stat(primaryPath); err == nil {
				_ = safeCopyDB(primaryPath, recoveredPath)
			}
		} else {
			fmt.Fprintf(os.Stderr, "[bigmem] warning: ghost WAL/SHM detected; using recovered DB at %s (primary %s blocked)\n", recoveredPath, primaryPath)
			// Merge primary's newer rows into recovered so recent saves to primary are not lost
			if _, err := os.Stat(primaryPath); err == nil {
				if err := mergeDB(primaryPath, recoveredPath); err != nil {
					fmt.Fprintf(os.Stderr, "[bigmem] ghost merge warning: %v\n", err)
				} else {
					checkpointDB(recoveredPath)
				}
			}
		}
		if _, err := os.Stat(recoveredPath); err == nil {
			return recoveredPath, nil
		}
		return primaryPath, nil
	}
	// No ghost — handle recovered existence
	if _, err := os.Stat(recoveredPath); err == nil {
		if _, err := os.Stat(primaryPath); err == nil {
			// Both exist — warn and merge by max(updated_at)
			fmt.Fprintf(os.Stderr, "[bigmem] warning: both primary (%s) and recovered (%s) exist; merging by max(updated_at)\n", primaryPath, recoveredPath)
			primMax := getMaxUpdatedAt(primaryPath)
			recMax := getMaxUpdatedAt(recoveredPath)
			// Log comparison for observability
			if recMax > primMax {
				fmt.Fprintf(os.Stderr, "[bigmem] recovered max updated_at %q newer than primary %q\n", recMax, primMax)
			}
			if err := mergeRecoveredIntoPrimary(primaryPath, recoveredPath); err != nil {
				fmt.Fprintf(os.Stderr, "[bigmem] merge warning: %v\n", err)
			}
			checkpointDB(primaryPath)
		} else {
			// Primary missing — promote recovered
			fmt.Fprintf(os.Stderr, "[bigmem] recovered DB promoted to primary: %s -> %s\n", recoveredPath, primaryPath)
			_ = safeCopyDB(recoveredPath, primaryPath)
		}
	}
	return primaryPath, nil
}

// BigmemStorePath is an alias for ResolveDBPath for external callers (e.g. sdd engram).
func BigmemStorePath(rootDir string) (string, error) { return ResolveDBPath(rootDir) }

// inferBigMemProject delegates to internal/project 5-case detection.
// Kept for compatibility; new code should call project.DetectProject directly.
func inferBigMemProject(dir string) string {
	info := projpkg.DetectProjectFull(dir)
	if info.Error == nil && info.Project != "" {
		return info.Project
	}
	if errors.Is(info.Error, projpkg.ErrAmbiguousProject) {
		if base := strings.TrimSpace(filepath.Base(dir)); base != "" && base != "." {
			return projpkg.NormalizeProjectName(base)
		}
		return "unknown"
	}
	if info.Project != "" {
		return projpkg.NormalizeProjectName(info.Project)
	}
	if base := strings.TrimSpace(filepath.Base(dir)); base != "" && base != "." {
		return projpkg.NormalizeProjectName(base)
	}
	return "unknown"
}

// InferProject is the exported wrapper for inferBigMemProject.
func InferProject(dir string) string { return inferBigMemProject(dir) }

// ─── Open / Schema ───────────────────────────────────────────────────────────

// Open creates or opens the SQLite store at the given root directory.
// Runs schema migration to add missing columns from previous versions.
func Open(rootDir string) (*Store, error) {
	origRoot := rootDir
	if rootDir == "" {
		rootDir = defaultBigmemRoot()
		if rootDir == "" {
			return nil, fmt.Errorf("home dir: not found")
		}
	}
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	dbPath, err := ResolveDBPath(origRoot)
	if err != nil {
		return nil, err
	}
	rootDir = filepath.Dir(dbPath)
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// WAL mode for better concurrency — busy_timeout must be set BEFORE journal_mode
	// so that journal_mode retries on SQLITE_BUSY (stale SHM from zombie handles).
	db.Exec("PRAGMA busy_timeout=5000")
	// Retry journal_mode on BUSY (ghost WAL/SHM locks from unreaped pids).
	for i := 0; i < 5; i++ {
		_, err = db.Exec("PRAGMA journal_mode=WAL")
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "busy") && !strings.Contains(err.Error(), "locked") {
			break
		}
		time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
	}
	// If WAL still BUSY, fall back to DELETE mode so the DB remains usable.
	if err != nil && (strings.Contains(err.Error(), "busy") || strings.Contains(err.Error(), "locked")) {
		db.Exec("PRAGMA journal_mode=DELETE")
	}
	db.Exec("PRAGMA synchronous=NORMAL")
	db.Exec("PRAGMA foreign_keys=ON")

	// Migrate legacy databases before creating indexes: CREATE TABLE IF NOT
	// EXISTS cannot alter existing tables, so columns added in later versions
	// must be added explicitly or the index/insert statements fail.
	if err := migrateSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	// Ensure sessions branching schema for fresh DB (REQ-B1) + indexes.
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			start_time TEXT,
			end_time TEXT,
			summary TEXT,
			project TEXT,
			directory TEXT,
			parent_id TEXT REFERENCES sessions(id) ON DELETE SET NULL,
			leaf_id TEXT,
			branch_summary TEXT
		);
	`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_parent_id ON sessions(parent_id)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_leaf_id ON sessions(leaf_id)`)

	// Create core tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS observations (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			type TEXT DEFAULT '',
			content TEXT DEFAULT '',
			session_id TEXT DEFAULT '',
			tool_name TEXT DEFAULT '',
			topic_key TEXT DEFAULT '',
			project TEXT DEFAULT '',
			scope TEXT DEFAULT '',
			normalized_hash TEXT DEFAULT '',
			revision_count INTEGER DEFAULT 1,
			duplicate_count INTEGER DEFAULT 1,
			last_seen_at TEXT,
			review_after TEXT,
			expires_at TEXT,
			embedding_model TEXT,
			embedding_vector BLOB,
			pinned INTEGER DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			deleted_at TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_obs_topic ON observations(topic_key);
		CREATE INDEX IF NOT EXISTS idx_obs_type ON observations(type);
		CREATE INDEX IF NOT EXISTS idx_obs_project ON observations(project);
		CREATE INDEX IF NOT EXISTS idx_obs_scope ON observations(scope);
		CREATE INDEX IF NOT EXISTS idx_obs_updated ON observations(updated_at);
		CREATE INDEX IF NOT EXISTS idx_obs_session ON observations(session_id);
		CREATE INDEX IF NOT EXISTS idx_obs_deleted ON observations(deleted_at);
		CREATE INDEX IF NOT EXISTS idx_obs_hash ON observations(normalized_hash);
		CREATE INDEX IF NOT EXISTS idx_obs_topic_lookup ON observations(topic_key, project, scope);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create observations: %w", err)
	}

	// Create FTS5 virtual table for full-text search with BM25 ranking.
	_, err = db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
			title, content, topic_key, tool_name, type, project,
			content='observations',
			content_rowid='rowid'
		);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create fts: %w", err)
	}

	// Create FTS triggers
	db.Exec(`
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
			marked_by_actor TEXT,
			marked_by_kind TEXT,
			marked_by_model TEXT,
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

	// Create lifecycle reviews table
	db.Exec(`
		CREATE TABLE IF NOT EXISTS reviews (
			observation_id TEXT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'needs_review',
			updated_at TEXT NOT NULL
		);
	`)

	// Create prompts FTS table
	db.Exec(`
		CREATE TABLE IF NOT EXISTS prompts (
			id TEXT PRIMARY KEY,
			content TEXT,
			session_id TEXT,
			created_at TEXT
		);
	`)

	// Sync tracking table
	db.Exec(`
		CREATE TABLE IF NOT EXISTS sync_chunks (
			target_key TEXT NOT NULL,
			chunk_id TEXT NOT NULL,
			imported_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (target_key, chunk_id)
		);
	`)
	ensureSyncJournalTables(db)
	db.Exec("DROP TABLE IF EXISTS prompts_fts")
	db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS prompts_fts USING fts5(
			content, project,
			content='prompts',
			content_rowid='rowid'
		);
	`)

	// Auto-heal on Open: detect FTS desync / WAL ghost / missing branching columns and fix once per hour.
	s := &Store{db: db, rootDir: rootDir}
	if r, err := s.Doctor(); err == nil && r != nil && (r.Corrupt || r.BranchColumnsMissing || r.NeedsMigration) {
		throttlePath := filepath.Join(rootDir, ".last_auto_fix")
		throttled := false
		if st, statErr := os.Stat(throttlePath); statErr == nil {
			if time.Since(st.ModTime()) < time.Hour {
				fmt.Fprintf(os.Stderr, "[bigmem] auto-fix throttled, last fix <1h ago\n")
				throttled = true
			}
		}
		if !throttled {
			fmt.Fprintf(os.Stderr, "[bigmem] auto-fix: corruption detected (corrupt=%v branchMissing=%v), running DoctorFix...\n", r.Corrupt, r.BranchColumnsMissing)
			if fixErr := s.DoctorFix(); fixErr != nil {
				fmt.Fprintf(os.Stderr, "[bigmem] auto-fix failed: %v\n", fixErr)
			} else {
				fmt.Fprintf(os.Stderr, "[bigmem] auto-fix OK\n")
				now := time.Now()
				_ = os.WriteFile(throttlePath, []byte(now.Format(time.RFC3339)), 0644)
				_ = os.Chtimes(throttlePath, now, now)
			}
		}
	}

	return &Store{db: db, rootDir: rootDir}, nil
}

// ─── Schema migration ────────────────────────────────────────────────────────

// columnDef describes a column that may need to be added to an existing table.
type columnDef struct {
	name string
	ddl  string
}

// migrateSchema adds columns introduced in later versions to databases created
// by older releases. CREATE TABLE IF NOT EXISTS is a no-op for existing
// tables, so without this step inserts and index creation fail with
// "no such column".
func migrateSchema(db *sql.DB) error {
	// observations grew provenance and lifecycle columns over versions.
	if err := ensureColumns(db, "observations", []columnDef{
		{name: "session_id", ddl: "TEXT DEFAULT ''"},
		{name: "tool_name", ddl: "TEXT DEFAULT ''"},
		{name: "normalized_hash", ddl: "TEXT DEFAULT ''"},
		{name: "revision_count", ddl: "INTEGER DEFAULT 1"},
		{name: "duplicate_count", ddl: "INTEGER DEFAULT 1"},
		{name: "last_seen_at", ddl: "TEXT"},
		{name: "review_after", ddl: "TEXT"},
		{name: "expires_at", ddl: "TEXT"},
		{name: "embedding_model", ddl: "TEXT"},
		{name: "embedding_vector", ddl: "BLOB"},
		{name: "pinned", ddl: "INTEGER DEFAULT 0"},
		{name: "deleted_at", ddl: "TEXT"},
	}); err != nil {
		return err
	}
	if err := ensureColumns(db, "memory_relations", []columnDef{
		{name: "session_id", ddl: "TEXT DEFAULT ''"},
		{name: "marked_by_actor", ddl: "TEXT"},
		{name: "marked_by_kind", ddl: "TEXT"},
		{name: "marked_by_model", ddl: "TEXT"},
	}); err != nil {
		return err
	}
	// Backfill marked_by_actor from legacy marked_by for Batch S compat.
	if err := backfillMarkedByActor(db); err != nil {
		return err
	}
	// sessions branching schema (REQ-B1/B2): parent_id FK, leaf_id, branch_summary — O(1) ADD COLUMN.
	if err := ensureColumns(db, "sessions", []columnDef{
		{name: "parent_id", ddl: "TEXT REFERENCES sessions(id) ON DELETE SET NULL"},
		{name: "leaf_id", ddl: "TEXT"},
		{name: "branch_summary", ddl: "TEXT"},
	}); err != nil {
		return err
	}
	return nil
}

// ensureColumns inspects an existing table and adds any missing column.
// Tables that do not exist yet are skipped: the subsequent CREATE TABLE IF
// NOT EXISTS creates them with the full current schema.
func ensureColumns(db *sql.DB, table string, cols []columnDef) error {
	var exists int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&exists); err != nil {
		return fmt.Errorf("check %s: %w", table, err)
	}
	if exists == 0 {
		return nil
	}
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return fmt.Errorf("inspect %s: %w", table, err)
	}
	existing := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return fmt.Errorf("scan %s columns: %w", table, err)
		}
		existing[name] = true
	}
	rows.Close()
	for _, c := range cols {
		if existing[c.name] {
			continue
		}
		if _, err := db.Exec("ALTER TABLE " + table + " ADD COLUMN " + c.name + " " + c.ddl); err != nil {
			return fmt.Errorf("migrate %s.%s: %w", table, c.name, err)
		}
	}
	return nil
}

// backfillMarkedByActor copies legacy marked_by into marked_by_actor when the new column is empty (Batch S compat).
func backfillMarkedByActor(db *sql.DB) error {
	// Only if both columns exist.
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('memory_relations') WHERE name='marked_by_actor'").Scan(&n); err != nil || n == 0 {
		return nil
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('memory_relations') WHERE name='marked_by'").Scan(&n); err != nil || n == 0 {
		return nil
	}
	_, _ = db.Exec(`UPDATE memory_relations SET marked_by_actor = marked_by WHERE (marked_by_actor IS NULL OR marked_by_actor = '') AND marked_by IS NOT NULL AND marked_by != ''`)
	return nil
}

// ─── Sync tracking ──────────────────────────────────────────────────────────

const (
	// LocalChunkTargetKey is used for local filesystem sync tracking.
	LocalChunkTargetKey = "local"
)

// RecordSyncChunk records a chunk as imported/exported for a target.
func (s *Store) RecordSyncChunk(targetKey, chunkID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("INSERT OR IGNORE INTO sync_chunks (target_key, chunk_id) VALUES (?, ?)", targetKey, chunkID)
	return err
}

// GetSyncChunks returns all recorded chunk IDs for a target.
func (s *Store) GetSyncChunks(targetKey string) (map[string]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query("SELECT chunk_id FROM sync_chunks WHERE target_key = ?", targetKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]bool)
	for rows.Next() {
		var id string
		rows.Scan(&id)
		result[id] = true
	}
	return result, nil
}

// GetLastChunkTime returns the newest created_at across all sessions and observations,
// to use as the cutoff for incremental sync.
func (s *Store) GetLastChunkTime() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var lastTime sql.NullString
	s.db.QueryRow(`
		SELECT MAX(max_time) FROM (
			SELECT MAX(created_at) as max_time FROM observations WHERE deleted_at IS NULL
			UNION ALL
			SELECT MAX(updated_at) FROM observations WHERE deleted_at IS NULL
			UNION ALL
			SELECT MAX(start_time) FROM sessions
		)
	`).Scan(&lastTime)
	if lastTime.Valid {
		return lastTime.String, nil
	}
	return "", nil
}

// Close shuts down the store.
func (s *Store) Close() error {
	return s.db.Close()
}

// RootDir returns the store directory.
func (s *Store) RootDir() string { return s.rootDir }

// ─── Helpers ─────────────────────────────────────────────────────────────────

// hashNormalized returns a SHA-256 hex digest of the input.
func hashNormalized(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

// normalizeScope normalizes scope to "project" (default) or "personal".
func normalizeScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "personal", "global":
		return strings.ToLower(strings.TrimSpace(scope))
	default:
		return "project"
	}
}

// dedupeWindowExpression returns a SQLite modifier for dedup window.
func dedupeWindowExpression(d time.Duration) string {
	if d <= 0 {
		d = defaultDedupeWindow
	}
	seconds := int(d.Seconds())
	return fmt.Sprintf("-%d seconds", seconds)
}

// buildLikeSearchSQL builds a LIKE-based fallback query when FTS fails or returns no rows.
func buildLikeSearchSQL(query string, opts SearchOptions, limit int) (string, []any) {
	likeQuery := "%" + strings.ReplaceAll(query, "%", "\\%") + "%"
	sqlQ := `SELECT o.id, o.title, o.type, o.content, o.session_id, o.tool_name,
		o.topic_key, o.project, o.scope, o.normalized_hash, o.revision_count, o.duplicate_count,
		o.last_seen_at, o.review_after, o.pinned, o.created_at, o.updated_at, o.deleted_at
		FROM observations o WHERE o.deleted_at IS NULL AND (o.title LIKE ? ESCAPE '\' OR o.content LIKE ? ESCAPE '\' OR o.topic_key LIKE ? ESCAPE '\')`
	args := []any{likeQuery, likeQuery, likeQuery}
	if opts.Type != "" {
		sqlQ += " AND o.type = ?"
		args = append(args, opts.Type)
	}
	// all_projects or scope==personal ignores project filter (Engram parity: personal is cross-project)
	if opts.Project != "" && !opts.AllProjects && normalizeScope(opts.Scope) != "personal" {
		sqlQ += " AND o.project = ?"
		args = append(args, opts.Project)
	}
	if opts.Scope != "" {
		sqlQ += " AND o.scope = ?"
		args = append(args, normalizeScope(opts.Scope))
	}
	sqlQ += " ORDER BY o.updated_at DESC LIMIT ?"
	args = append(args, limit)
	return sqlQ, args
}

// shouldFilterProject reports whether the project filter should be applied.
func shouldFilterProject(opts SearchOptions) bool {
	return opts.Project != "" && !opts.AllProjects && normalizeScope(opts.Scope) != "personal"
}

// ─── Rescue Ownership Helpers (REQ-RO1/RO2) ─────────────────────────────────

// foreignRecordOwnerTx checks whether session has foreign observations (project != requested).
// Returns true with foreign project name if a blocking observation exists.
func foreignRecordOwnerTx(tx *sql.Tx, sessionID, requestedProject string) (bool, string, error) {
	var foreign string
	err := tx.QueryRow(`SELECT project FROM observations WHERE session_id = ? AND trim(coalesce(project,'')) != '' AND project != ? LIMIT 1`, sessionID, requestedProject).Scan(&foreign)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	if strings.TrimSpace(foreign) == "" {
		return false, "", nil
	}
	return true, foreign, nil
}

// adoptSessionOwnershipTx atomically claims an orphan session for the requested project.
// It updates sessions.project where currently NULL or blank and probes sync_mutations for sync enqueue.
func (s *Store) adoptSessionOwnershipTx(tx *sql.Tx, sessionID, project string) error {
	_, err := tx.Exec(`UPDATE sessions SET project = ? WHERE id = ? AND (project IS NULL OR trim(coalesce(project,'')) = '')`, project, sessionID)
	if err != nil {
		return err
	}
	// Probe sync_mutations existence (Engram parity, no DDL migration)
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sync_mutations'`).Scan(&exists); err != nil {
		return nil
	}
	if exists == 0 {
		return nil
	}
	rows, err := tx.Query(`PRAGMA table_info(sync_mutations)`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
			cols[strings.ToLower(name)] = true
		}
	}
	if cols["entity"] && cols["entity_key"] {
		opCol := ""
		if cols["op"] {
			opCol = "op"
		} else if cols["operation"] {
			opCol = "operation"
		}
		id := fmt.Sprintf("sync-%d", time.Now().UnixNano())
		nowStr := time.Now().UTC().Format(time.RFC3339)
		if opCol != "" && cols["project"] {
			_, _ = tx.Exec(fmt.Sprintf(`INSERT INTO sync_mutations (id, entity, entity_key, %s, project, created_at) VALUES (?, ?, ?, ?, ?, ?)`, opCol), id, "session", sessionID, "adopt", project, nowStr)
			return nil
		}
		if opCol != "" {
			_, _ = tx.Exec(fmt.Sprintf(`INSERT INTO sync_mutations (id, entity, entity_key, %s) VALUES (?, ?, ?, ?)`, opCol), id, "session", sessionID, "adopt")
			return nil
		}
		_, _ = tx.Exec(`INSERT INTO sync_mutations (id, entity, entity_key, project) VALUES (?, ?, ?, ?)`, id, "session", sessionID, project)
		return nil
	}
	if cols["project"] {
		id := fmt.Sprintf("sync-%d", time.Now().UnixNano())
		_, _ = tx.Exec(`INSERT INTO sync_mutations (id, project) VALUES (?, ?)`, id, project)
	}
	return nil
}

// resolveWriteProjectTx atomically claims an orphan session when no foreign owner exists.
// Must be called inside the caller's BEGIN IMMEDIATE TX holding Store.mu.
func (s *Store) resolveWriteProjectTx(tx *sql.Tx, sessionID, requestedProject string) error {
	if sessionID == "" {
		return nil
	}
	trimmed := strings.TrimSpace(requestedProject)
	if trimmed == "" || strings.ToLower(trimmed) == "unknown" {
		return nil
	}
	normalizedRequested := projpkg.NormalizeProjectName(trimmed)
	if normalizedRequested == "unknown" || normalizedRequested == "" {
		return nil
	}
	requestedProject = normalizedRequested
	var existing sql.NullString
	err := tx.QueryRow(`SELECT project FROM sessions WHERE id = ?`, sessionID).Scan(&existing)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	existingProj := ""
	if existing.Valid {
		existingProj = strings.TrimSpace(existing.String)
	}
	if existingProj != "" {
		normalizedExisting := projpkg.NormalizeProjectName(existingProj)
		if normalizedExisting == requestedProject {
			return nil
		}
		if strings.ToLower(existingProj) == "unknown" {
			return nil
		}
		return fmt.Errorf("%w: session %q owned by project %q (requested %q); run biggz bigmem rescue-ownership --project %s --session %s", ErrProjectOwnershipAmbiguous, sessionID, existingProj, requestedProject, requestedProject, sessionID)
	}
	hasForeign, foreignProj, err := foreignRecordOwnerTx(tx, sessionID, requestedProject)
	if err != nil {
		return err
	}
	if hasForeign {
		return fmt.Errorf("%w: session %q has foreign observations in project %q; run biggz bigmem rescue-ownership --project %s --session %s", ErrProjectOwnershipAmbiguous, sessionID, foreignProj, requestedProject, sessionID)
	}
	return s.adoptSessionOwnershipTx(tx, sessionID, requestedProject)
}

// ─── Bulk Rescue (REQ-RO3) ────────────────────────────────────────────────────

// PlanRescue returns a two-phase plan for bulk rescue without mutation.
func (s *Store) PlanRescue(project string) (*RescuePlan, error) {
	trimmed := strings.TrimSpace(project)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: project required for rescue", ErrProjectRequired)
	}
	normalized := projpkg.NormalizeProjectName(trimmed)
	if normalized == "" || normalized == "unknown" {
		return nil, fmt.Errorf("%w: invalid project %q", ErrProjectRequired, project)
	}
	project = normalized
	s.mu.RLock()
	rows, err := s.db.Query(`SELECT id FROM sessions WHERE (project IS NULL OR trim(coalesce(project,'')) = '') AND coalesce(project,'') != 'unknown'`)
	if err != nil {
		s.mu.RUnlock()
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	s.mu.RUnlock()
	plan := &RescuePlan{Project: project, Total: len(ids)}
	for _, sid := range ids {
		var foreign string
		qErr := s.db.QueryRow(`SELECT project FROM observations WHERE session_id = ? AND trim(coalesce(project,'')) != '' AND project != ? LIMIT 1`, sid, project).Scan(&foreign)
		if qErr == sql.ErrNoRows {
			plan.Adoptable = append(plan.Adoptable, sid)
			continue
		}
		if qErr != nil {
			continue
		}
		if strings.TrimSpace(foreign) == "" {
			plan.Adoptable = append(plan.Adoptable, sid)
		} else {
			plan.Ambiguous = append(plan.Ambiguous, AmbiguousEntry{SessionID: sid, ForeignProject: foreign})
		}
	}
	if plan.Adoptable == nil {
		plan.Adoptable = []string{}
	}
	if plan.Ambiguous == nil {
		plan.Ambiguous = []AmbiguousEntry{}
	}
	return plan, nil
}

// RescueNullProjectOwnership bulk-adopts orphan sessions to project.
// When opts.DryRun is true, no mutation occurs and Adopted reflects would-be count.
// When opts.SessionID is set, scope is limited to that single session.
func (s *Store) RescueNullProjectOwnership(project string, opts RescueOptions) (*RescueResult, error) {
	trimmed := strings.TrimSpace(project)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: project required", ErrProjectRequired)
	}
	normalized := projpkg.NormalizeProjectName(trimmed)
	if normalized == "" || normalized == "unknown" {
		return nil, fmt.Errorf("%w: invalid project %q", ErrProjectRequired, project)
	}
	project = normalized
	if opts.SessionID != "" {
		sessionID := opts.SessionID
		var existing sql.NullString
		err := s.db.QueryRow(`SELECT project FROM sessions WHERE id = ?`, sessionID).Scan(&existing)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session %q not found", sessionID)
		}
		if err != nil {
			return nil, err
		}
		existingProj := ""
		if existing.Valid {
			existingProj = strings.TrimSpace(existing.String)
		}
		if existingProj != "" {
			if projpkg.NormalizeProjectName(existingProj) == project {
				return &RescueResult{Adopted: 0, Skipped: 1, Ambiguous: []AmbiguousEntry{}}, nil
			}
			if strings.ToLower(existingProj) == "unknown" {
				return &RescueResult{Adopted: 0, Skipped: 1, Ambiguous: []AmbiguousEntry{}}, nil
			}
			// Check if orphan? already not orphan since existingProj != ""
			// Treat as skipped (already owned by different project) — ambiguous only if foreign observations?
			var foreign string
			fErr := s.db.QueryRow(`SELECT project FROM observations WHERE session_id = ? AND trim(coalesce(project,'')) != '' AND project != ? LIMIT 1`, sessionID, project).Scan(&foreign)
			if fErr == nil && strings.TrimSpace(foreign) != "" {
				return &RescueResult{Adopted: 0, Skipped: 0, Ambiguous: []AmbiguousEntry{{SessionID: sessionID, ForeignProject: foreign}}}, nil
			}
			return &RescueResult{Adopted: 0, Skipped: 1, Ambiguous: []AmbiguousEntry{}}, nil
		}
		// Orphan: check foreign
		var foreign string
		fErr := s.db.QueryRow(`SELECT project FROM observations WHERE session_id = ? AND trim(coalesce(project,'')) != '' AND project != ? LIMIT 1`, sessionID, project).Scan(&foreign)
		if fErr == nil && strings.TrimSpace(foreign) != "" {
			return &RescueResult{Adopted: 0, Skipped: 0, Ambiguous: []AmbiguousEntry{{SessionID: sessionID, ForeignProject: foreign}}}, nil
		}
		if fErr != nil && fErr != sql.ErrNoRows {
			return nil, fErr
		}
		if opts.DryRun {
			return &RescueResult{Adopted: 1, Skipped: 0, Ambiguous: []AmbiguousEntry{}}, nil
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.db.Exec("PRAGMA busy_timeout=5000")
		tx, err := s.db.Begin()
		if err != nil {
			return nil, err
		}
		hasForeign, foreignProj, err := foreignRecordOwnerTx(tx, sessionID, project)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		if hasForeign {
			_ = tx.Rollback()
			return &RescueResult{Adopted: 0, Skipped: 0, Ambiguous: []AmbiguousEntry{{SessionID: sessionID, ForeignProject: foreignProj}}}, nil
		}
		var projInside sql.NullString
		if err := tx.QueryRow(`SELECT project FROM sessions WHERE id = ?`, sessionID).Scan(&projInside); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		insideProj := ""
		if projInside.Valid {
			insideProj = strings.TrimSpace(projInside.String)
		}
		if insideProj != "" && strings.ToLower(insideProj) != "unknown" {
			_ = tx.Rollback()
			if projpkg.NormalizeProjectName(insideProj) == project {
				return &RescueResult{Adopted: 0, Skipped: 1, Ambiguous: []AmbiguousEntry{}}, nil
			}
			return &RescueResult{Adopted: 0, Skipped: 1, Ambiguous: []AmbiguousEntry{}}, nil
		}
		if err := s.adoptSessionOwnershipTx(tx, sessionID, project); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		return &RescueResult{Adopted: 1, Skipped: 0, Ambiguous: []AmbiguousEntry{}}, nil
	}
	plan, err := s.PlanRescue(project)
	if err != nil {
		return nil, err
	}
	if opts.DryRun {
		return &RescueResult{Adopted: len(plan.Adoptable), Skipped: 0, Ambiguous: plan.Ambiguous}, nil
	}
	result := &RescueResult{Ambiguous: plan.Ambiguous}
	if result.Ambiguous == nil {
		result.Ambiguous = []AmbiguousEntry{}
	}
	for _, sessionID := range plan.Adoptable {
		s.mu.Lock()
		s.db.Exec("PRAGMA busy_timeout=5000")
		tx, err := s.db.Begin()
		if err != nil {
			s.mu.Unlock()
			result.Skipped++
			continue
		}
		hasForeign, foreignProj, err := foreignRecordOwnerTx(tx, sessionID, project)
		if err != nil {
			_ = tx.Rollback()
			s.mu.Unlock()
			result.Skipped++
			continue
		}
		if hasForeign {
			_ = tx.Rollback()
			s.mu.Unlock()
			result.Ambiguous = append(result.Ambiguous, AmbiguousEntry{SessionID: sessionID, ForeignProject: foreignProj})
			continue
		}
		var projInside sql.NullString
		if err := tx.QueryRow(`SELECT project FROM sessions WHERE id = ?`, sessionID).Scan(&projInside); err != nil {
			_ = tx.Rollback()
			s.mu.Unlock()
			result.Skipped++
			continue
		}
		insideProj := ""
		if projInside.Valid {
			insideProj = strings.TrimSpace(projInside.String)
		}
		if insideProj != "" && strings.ToLower(insideProj) != "unknown" {
			_ = tx.Rollback()
			s.mu.Unlock()
			result.Skipped++
			continue
		}
		if err := s.adoptSessionOwnershipTx(tx, sessionID, project); err != nil {
			_ = tx.Rollback()
			s.mu.Unlock()
			result.Skipped++
			continue
		}
		if err := tx.Commit(); err != nil {
			s.mu.Unlock()
			result.Skipped++
			continue
		}
		s.mu.Unlock()
		_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		result.Adopted++
	}
	return result, nil
}

// ─── Save with dedup ─────────────────────────────────────────────────────────

// Save persists an observation with full engram-compatible dedup:
//  1. topic_key match → update existing (increment revision_count)
//  2. normalized_hash + window → increment duplicate_count
//  3. Otherwise → fresh insert
//
// Optional parentID anchoring (REQ-B5): when provided, associates observation with branch.
func (s *Store) Save(obs *Observation, parentID ...string) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		if err == nil {
			_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		}
	}()

	// Branch anchoring: optional parentID (no-op when omitted, preserves FTS/dedup).
	if len(parentID) > 0 && parentID[0] != "" {
		if obs.SessionID == "" {
			obs.SessionID = parentID[0]
		}
	}

	if obs.ID == "" {
		obs.ID = fmt.Sprintf("obs-%d-%d", time.Now().UnixNano(), atomic.AddInt64(&globalIDSeq, 1))
	}
	now := time.Now().UTC()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	obs.UpdatedAt = now
	obs.Scope = normalizeScope(obs.Scope)
	// Batch S: private-tag redaction (Engram parity). Never truncate silently:
	// blob addresses and ShouldExternalize (>100k/data:image) payloads stay raw
	// inline so DoctorFixBlobs can migrate them later; the 50-100KiB window
	// between maxStoredBytes and ShouldExternalize is externalized via PutBlob
	// instead of being cut to 50k with a discarded warning.
	obs.Title = stripPrivateTags(obs.Title)
	stripped := stripPrivateTags(obs.Content)
	if IsBlobAddr(stripped) || ShouldExternalize(stripped) {
		obs.Content = stripped
	} else if len(stripped) > maxStoredBytes {
		if addr, err := PutBlob([]byte(stripped)); err == nil {
			obs.Content = addr
		} else {
			// Blob unavailable (e.g. empty HOME): preserve raw so no bytes
			// are lost; DoctorFixBlobs migrates once storage is available.
			obs.Content = stripped
		}
	} else {
		obs.Content = stripped
	}
	obs.NormalizedHash = hashNormalized(obs.Content)

	nowStr := now.Format(time.RFC3339)

	requestedProject := obs.Project
	if strings.TrimSpace(requestedProject) != "" && strings.ToLower(strings.TrimSpace(requestedProject)) != "unknown" {
		requestedProject = projpkg.NormalizeProjectName(requestedProject)
		obs.Project = requestedProject
	}
	s.db.Exec("PRAGMA busy_timeout=5000")
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if obs.SessionID != "" && strings.TrimSpace(requestedProject) != "" && strings.ToLower(strings.TrimSpace(requestedProject)) != "unknown" {
		if rErr := s.resolveWriteProjectTx(tx, obs.SessionID, requestedProject); rErr != nil {
			err = rErr
			return err
		}
	}
	// Phase 1: topic_key dedup — update existing with same topic_key + project + scope
	if obs.TopicKey != "" {
		var existingID string
		var existingCreated string
		if qErr := tx.QueryRow(
			`SELECT id, created_at FROM observations
			 WHERE topic_key = ? AND project = ? AND scope = ? AND deleted_at IS NULL
			 ORDER BY updated_at DESC LIMIT 1`,
			obs.TopicKey, obs.Project, obs.Scope,
		).Scan(&existingID, &existingCreated); qErr == nil {
			obs.ID = existingID
			if t, pErr := time.Parse(time.RFC3339, existingCreated); pErr == nil {
				obs.CreatedAt = t
			}
			_, err = tx.Exec(`UPDATE observations SET
				type=?, title=?, content=?, session_id=?, tool_name=?,
				topic_key=?, project=?, scope=?, normalized_hash=?,
				revision_count=revision_count+1, last_seen_at=?,
				pinned=?, updated_at=?
				WHERE id=?`,
				obs.Type, obs.Title, obs.Content, obs.SessionID, obs.ToolName,
				obs.TopicKey, obs.Project, obs.Scope, obs.NormalizedHash,
				nowStr, boolToInt(obs.Pinned), nowStr, existingID)
			if err != nil {
				return err
			}
			if errQ := tryEnqueueObservationTx(tx, obs); errQ != nil {
				err = errQ
				return err
			}
			if err = tx.Commit(); err != nil {
				return err
			}
			return nil
		}
	}

	// Phase 2: hash-based dedup within window
	window := dedupeWindowExpression(0)
	{
		var existingID string
		if qErr := tx.QueryRow(
			`SELECT id FROM observations
			 WHERE normalized_hash = ? AND project = ? AND scope = ?
			 AND type = ? AND title = ? AND deleted_at IS NULL
			 AND datetime(created_at) >= datetime('now', ?)
			 ORDER BY created_at DESC LIMIT 1`,
			obs.NormalizedHash, obs.Project, obs.Scope,
			obs.Type, obs.Title, window,
		).Scan(&existingID); qErr == nil {
			obs.ID = existingID
			// Preserve original created_at
			var origCreated string
			_ = tx.QueryRow("SELECT created_at FROM observations WHERE id=?", existingID).Scan(&origCreated)
			if t, pErr := time.Parse(time.RFC3339, origCreated); pErr == nil {
				obs.CreatedAt = t
			}
			_, err = tx.Exec(`UPDATE observations SET
			duplicate_count=duplicate_count+1, last_seen_at=?, updated_at=?
			WHERE id=?`,
				nowStr, nowStr, existingID)
			if err != nil {
				return err
			}
			if errQ := tryEnqueueObservationTx(tx, obs); errQ != nil {
				err = errQ
				return err
			}
			if err = tx.Commit(); err != nil {
				return err
			}
			return nil
		}
	}

	// Phase 3: upsert — handles both fresh inserts and updates where
	// the ID already exists (e.g., from Update → Get → Save path).
	reviewAfterStr := computeReviewAfterAt(now, obs.Type)

	_, err = tx.Exec(`INSERT INTO observations
		(id, title, type, content, session_id, tool_name, topic_key, project, scope,
		 normalized_hash, revision_count, duplicate_count, last_seen_at, review_after,
		 pinned, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title, type=excluded.type, content=excluded.content,
			session_id=excluded.session_id, tool_name=excluded.tool_name,
			topic_key=excluded.topic_key, project=excluded.project, scope=excluded.scope,
			normalized_hash=excluded.normalized_hash,
			revision_count=CASE WHEN excluded.revision_count > observations.revision_count
				THEN excluded.revision_count ELSE observations.revision_count + 1 END,
			duplicate_count=CASE WHEN excluded.duplicate_count > observations.duplicate_count
				THEN excluded.duplicate_count ELSE observations.duplicate_count + 1 END,
			last_seen_at=excluded.last_seen_at, review_after=excluded.review_after,
			pinned=excluded.pinned, updated_at=excluded.updated_at`,
		obs.ID, obs.Title, obs.Type, obs.Content, obs.SessionID, obs.ToolName,
		obs.TopicKey, obs.Project, obs.Scope, obs.NormalizedHash,
		nowStr, reviewAfterStr, boolToInt(obs.Pinned),
		obs.CreatedAt.Format(time.RFC3339), nowStr)
	if err != nil {
		return err
	}
	if errQ := tryEnqueueObservationTx(tx, obs); errQ != nil {
		err = errQ
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

// computeReviewAfter returns the decay duration for a type, or nil if none.
// Uses calendar months via AddDate for Engram parity (Batch S).
func computeReviewAfter(obsType string) *time.Duration {
	now := time.Now()
	switch obsType {
	case "decision":
		d := now.AddDate(0, 6, 0).Sub(now)
		return &d
	case "policy":
		d := now.AddDate(0, 12, 0).Sub(now)
		return &d
	case "preference":
		d := now.AddDate(0, 3, 0).Sub(now)
		return &d
	default:
		return nil
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ShouldExternalize reports whether content should be externalized to BlobStore.
func ShouldExternalize(c string) bool {
	return len(c) > 100000 || strings.Contains(c, "data:image/")
}

// resolveBlobContent returns blob bytes if content is a blob address, else content.
func resolveBlobContent(content string) string {
	if !IsBlobAddr(content) {
		return content
	}
	data, err := GetBlob(content)
	if err != nil {
		return content
	}
	return string(data)
}

// ─── Get ─────────────────────────────────────────────────────────────────────

// Get retrieves an observation by ID.
func (s *Store) Get(id string) (*Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	obs := &Observation{}
	var ca, ua string
	var ra, da, lsa sql.NullString
	var pinnedInt int
	err := s.db.QueryRow(`SELECT id, title, type, content, session_id, tool_name,
		topic_key, project, scope, normalized_hash, revision_count, duplicate_count,
		last_seen_at, review_after, pinned, created_at, updated_at, deleted_at
		FROM observations WHERE id = ?`, id).
		Scan(&obs.ID, &obs.Title, &obs.Type, &obs.Content, &obs.SessionID, &obs.ToolName,
			&obs.TopicKey, &obs.Project, &obs.Scope, &obs.NormalizedHash,
			&obs.RevisionCount, &obs.DuplicateCount, &lsa, &ra, &pinnedInt,
			&ca, &ua, &da)
	if err != nil {
		return nil, fmt.Errorf("not found: %w", err)
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
	// Transparent blob resolve: blob addr -> bytes, miss -> raw addr, no DB mutate.
	if IsBlobAddr(obs.Content) {
		if data, err := GetBlob(obs.Content); err == nil {
			obs.Content = string(data)
		}
	}
	return obs, nil
}

// ─── Search with BM25 ranking ────────────────────────────────────────────────

// Search finds observations using FTS5 full-text search with BM25 ranking.
// Supports "all" (AND) and "any" (OR) match modes.
// If query contains "/", does an exact topic_key lookup first.
func (s *Store) Search(query string, opts SearchOptions) (results []*Observation, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	defer func() {
		if err == nil {
			_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		}
	}()

	// Validate match_mode early (Engram parity: any invalid value errors regardless of query shape).
	switch opts.MatchMode {
	case "", "all", "any":
	default:
		return nil, fmt.Errorf("invalid match_mode %q: must be \"all\" or \"any\"", opts.MatchMode)
	}
	// Normalize project for comparison (lowercase) is handled by caller; store keeps original.
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultMaxSearchResults
	}
	if limit > 50 {
		limit = 50
	}
	// Cap is 50 (not 20) to allow BuildGraph(limit=50) and external --limit 50 (F10); default remains 20 via defaultMaxSearchResults.
	// BM25Floor is stored for caller instrumentation; Search does not rank-filter (candidates use BM25Floor)
	_ = opts.BM25Floor

	// Phase 1: topic-key direct lookup if query contains "/"
	var directResults []*Observation
	if strings.Contains(query, "/") {
		tkArgs := []any{query}
		tkSQL := `SELECT id, title, type, content, session_id, tool_name,
			topic_key, project, scope, normalized_hash, revision_count, duplicate_count,
			last_seen_at, review_after, pinned, created_at, updated_at, deleted_at
			FROM observations
			WHERE topic_key = ? AND deleted_at IS NULL`

		if opts.Type != "" {
			tkSQL += " AND type = ?"
			tkArgs = append(tkArgs, opts.Type)
		}
		if shouldFilterProject(opts) {
			tkSQL += " AND project = ?"
			tkArgs = append(tkArgs, opts.Project)
		}
		if opts.Scope != "" {
			tkSQL += " AND scope = ?"
			tkArgs = append(tkArgs, normalizeScope(opts.Scope))
		}
		tkSQL += " ORDER BY updated_at DESC LIMIT ?"
		tkArgs = append(tkArgs, limit)

		tkRows, err := s.db.Query(tkSQL, tkArgs...)
		if err == nil {
			defer tkRows.Close()
			for tkRows.Next() {
				obs := &Observation{}
				var ca, ua string
				var ra, da, lsa sql.NullString
				var pinnedInt int
				if err := tkRows.Scan(&obs.ID, &obs.Title, &obs.Type, &obs.Content,
					&obs.SessionID, &obs.ToolName, &obs.TopicKey, &obs.Project, &obs.Scope,
					&obs.NormalizedHash, &obs.RevisionCount, &obs.DuplicateCount,
					&lsa, &ra, &pinnedInt, &ca, &ua, &da); err != nil {
					break
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
				if IsBlobAddr(obs.Content) {
					if data, err := GetBlob(obs.Content); err == nil {
						obs.Content = string(data)
					}
				}
				directResults = append(directResults, obs)
			}
		}
	}

	// Phase 2: if query is empty, skip FTS5 and filter directly
	var rows *sql.Rows
	if query == "" {
		q := `SELECT o.id, o.title, o.type, o.content, o.session_id, o.tool_name,
			o.topic_key, o.project, o.scope, o.normalized_hash, o.revision_count, o.duplicate_count,
			o.last_seen_at, o.review_after, o.pinned, o.created_at, o.updated_at, o.deleted_at
			FROM observations o WHERE o.deleted_at IS NULL`
		var args []any
		if opts.Type != "" {
			q += " AND o.type = ?"
			args = append(args, opts.Type)
		}
		if shouldFilterProject(opts) {
			q += " AND o.project = ?"
			args = append(args, opts.Project)
		}
		if opts.Scope != "" {
			q += " AND o.scope = ?"
			args = append(args, normalizeScope(opts.Scope))
		}
		q += " ORDER BY o.updated_at DESC LIMIT ?"
		args = append(args, limit)
		rows, err = s.db.Query(q, args...)
	} else {
		// FTS5 with BM25 ranking — "all" = AND, "any" = OR (Engram parity)
		var ftsQuery string
		if opts.MatchMode == "any" {
			terms := strings.Fields(query)
			for i, t := range terms {
				terms[i] = strings.ReplaceAll(t, "\"", "")
			}
			ftsQuery = strings.Join(terms, " OR ")
		} else {
			// "all" (default): AND semantics — each term wrapped in quotes
			terms := strings.Fields(query)
			for i, t := range terms {
				terms[i] = "\"" + strings.ReplaceAll(t, "\"", "") + "\""
			}
			ftsQuery = strings.Join(terms, " AND ")
		}

		sqlQ := `SELECT o.id, o.title, o.type, o.content, o.session_id, o.tool_name,
			o.topic_key, o.project, o.scope, o.normalized_hash, o.revision_count, o.duplicate_count,
			o.last_seen_at, o.review_after, o.pinned, o.created_at, o.updated_at, o.deleted_at
			FROM observations_fts fts
			JOIN observations o ON o.rowid = fts.rowid
			WHERE observations_fts MATCH ? AND o.deleted_at IS NULL`

		args := []any{ftsQuery}

		if opts.Type != "" {
			sqlQ += " AND o.type = ?"
			args = append(args, opts.Type)
		}
		if shouldFilterProject(opts) {
			sqlQ += " AND o.project = ?"
			args = append(args, opts.Project)
		}
		if opts.Scope != "" {
			sqlQ += " AND o.scope = ?"
			args = append(args, normalizeScope(opts.Scope))
		}

		sqlQ += " ORDER BY rank LIMIT ?"
		args = append(args, limit)
		rows, err = s.db.Query(sqlQ, args...)
	}
	if err != nil {
		// FTS error — fallback to LIKE search
		likeSQL, likeArgs := buildLikeSearchSQL(query, opts, limit)
		rows, err = s.db.Query(likeSQL, likeArgs...)
		if err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}
	}
	defer rows.Close()

	var hasRows bool
	// Dedup: track seen IDs (direct results come first)
	seen := make(map[string]bool)
	for _, dr := range directResults {
		seen[dr.ID] = true
	}

	results = append(results, directResults...)
	for rows.Next() {
		hasRows = true
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
		if seen[obs.ID] {
			continue
		}
		seen[obs.ID] = true
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
		if IsBlobAddr(obs.Content) {
			if data, err := GetBlob(obs.Content); err == nil {
				obs.Content = string(data)
			}
		}
		results = append(results, obs)
	}
	// Fallback to LIKE if FTS returned no rows (covers rebuild race or corrupted FTS)
	if !hasRows {
		likeSQL, likeArgs := buildLikeSearchSQL(query, opts, limit)
		likeRows, err := s.db.Query(likeSQL, likeArgs...)
		if err == nil {
			defer likeRows.Close()
			for likeRows.Next() {
				obs := &Observation{}
				var ca, ua string
				var ra, da, lsa sql.NullString
				var pinnedInt int
				if err := likeRows.Scan(&obs.ID, &obs.Title, &obs.Type, &obs.Content,
					&obs.SessionID, &obs.ToolName, &obs.TopicKey, &obs.Project, &obs.Scope,
					&obs.NormalizedHash, &obs.RevisionCount, &obs.DuplicateCount,
					&lsa, &ra, &pinnedInt, &ca, &ua, &da); err != nil {
					continue
				}
				if seen[obs.ID] {
					continue
				}
				seen[obs.ID] = true
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
				if IsBlobAddr(obs.Content) {
					if data, err := GetBlob(obs.Content); err == nil {
						obs.Content = string(data)
					}
				}
				results = append(results, obs)
			}
		}
	}
	return results, nil
}

// ─── Update ──────────────────────────────────────────────────────────────────

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
	if v, ok := updates["tool_name"].(string); ok {
		obs.ToolName = v
	}
	return obs, s.Save(obs)
}

// ─── Delete ──────────────────────────────────────────────────────────────────

// Delete removes an observation permanently.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("DELETE FROM observations WHERE id = ?", id)
	return err
}

// DeleteObservation removes an observation. If hard is true, it's permanent;
// otherwise it's a soft-delete (clears content, marks type and deleted_at).
func (s *Store) DeleteObservation(id string, hard bool) error {
	if hard {
		return s.Delete(id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		"UPDATE observations SET content='', type='deleted', deleted_at=?, updated_at=? WHERE id=?",
		now, now, id)
	return err
}

// ─── Conflict Surfacing (memory_relations) ───────────────────────────────────

// ErrCrossProjectRelation is returned when source and target observations belong
// to different projects.
var ErrCrossProjectRelation = errors.New("cross-project relation not allowed")

// Candidate represents a potentially conflicting observation.
type Candidate struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Type       string  `json:"type"`
	Score      float64 `json:"score"`
	JudgmentID string  `json:"judgment_id"`
	TopicKey   string  `json:"topic_key,omitempty"`
}

// FindOptions controls FindCandidatesWithOptions filtering.
type FindOptions struct {
	Project   string   `json:"project,omitempty"`
	Scope     string   `json:"scope,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	BM25Floor *float64 `json:"bm25_floor,omitempty"` // nil = no floor; -2.0 = default engram floor
}

// FindCandidatesWithOptions searches for similar observations using FTS5 BM25
// ranking and creates pending memory_relations entries.
//
// BM25Floor filtering: when non-nil, candidates are fetched ordered by
// bm25(observations_fts) rank and only those with rank >= *BM25Floor are kept
// (BM25 scores are negative; closer to 0 = better). To allow filtering, up to
// limit*3 rows are fetched then filtered in Go until limit is reached.
// When BM25Floor is nil, no score filtering is applied (just LIMIT).
// Default floor for backward compatibility is -2.0 like engram.
func (s *Store) FindCandidatesWithOptions(savedID, title string, opts FindOptions) ([]Candidate, error) {
	if title == "" {
		return nil, nil
	}
	terms := strings.Fields(title)
	if len(terms) == 0 {
		return nil, nil
	}
	for i, t := range terms {
		terms[i] = "\"" + strings.ReplaceAll(t, "\"", "") + "\""
	}
	ftsQuery := strings.Join(terms, " OR ")

	limit := opts.Limit
	if limit <= 0 {
		limit = 5
	}

	// Determine BM25 floor filtering.
	var doFilter bool
	var floorVal float64
	fetchLimit := limit
	if opts.BM25Floor != nil {
		doFilter = true
		floorVal = *opts.BM25Floor
		fetchLimit = limit * 3
		if fetchLimit <= 0 {
			fetchLimit = limit
		}
	}

	// Build FTS5 query ordered by BM25 rank (fts.rank is bm25).
	// We SELECT rank so we can filter by floor in Go.
	query := `SELECT o.id, o.title, o.type, o.topic_key, fts.rank
		FROM observations_fts fts
		JOIN observations o ON o.rowid = fts.rowid
		WHERE fts MATCH ? AND o.id != ? AND o.deleted_at IS NULL`
	args := []any{ftsQuery, savedID}
	if opts.Project != "" {
		query += " AND o.project = ?"
		args = append(args, opts.Project)
	}
	if opts.Scope != "" {
		query += " AND o.scope = ?"
		args = append(args, opts.Scope)
	}
	query += " ORDER BY fts.rank LIMIT ?"
	args = append(args, fetchLimit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		// Fallback: try bm25() function if fts.rank column is not available
		// (modernc sqlite variation). This keeps vet/build clean on different builds.
		if strings.Contains(err.Error(), "no such column") || strings.Contains(err.Error(), "rank") {
			fallbackQuery := `SELECT o.id, o.title, o.type, o.topic_key, bm25(observations_fts)
				FROM observations_fts
				JOIN observations o ON o.rowid = observations_fts.rowid
				WHERE observations_fts MATCH ? AND o.id != ? AND o.deleted_at IS NULL`
			fbArgs := []any{ftsQuery, savedID}
			if opts.Project != "" {
				fallbackQuery += " AND o.project = ?"
				fbArgs = append(fbArgs, opts.Project)
			}
			if opts.Scope != "" {
				fallbackQuery += " AND o.scope = ?"
				fbArgs = append(fbArgs, opts.Scope)
			}
			fallbackQuery += " ORDER BY bm25(observations_fts) LIMIT ?"
			fbArgs = append(fbArgs, fetchLimit)
			rows, err = s.db.Query(fallbackQuery, fbArgs...)
		}
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var candidates []Candidate
	for rows.Next() {
		c := Candidate{}
		var score float64
		if err := rows.Scan(&c.ID, &c.Title, &c.Type, &c.TopicKey, &score); err != nil {
			continue
		}
		c.Score = score
		if doFilter && score < floorVal {
			continue
		}
		relID := fmt.Sprintf("rel-%d", time.Now().UnixNano())
		now := time.Now().UTC().Format(time.RFC3339)
		_, _ = s.db.Exec(`INSERT INTO memory_relations (id, source_id, target_id, relation, judgment_status, session_id, created_at, updated_at)
			VALUES (?, ?, ?, 'pending', 'pending', '', ?, ?)
			ON CONFLICT(id) DO NOTHING`,
			relID, savedID, c.ID, now, now)
		c.JudgmentID = relID
		candidates = append(candidates, c)
		if len(candidates) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return candidates, err
	}
	return candidates, nil
}

// FindCandidates searches for similar observations and creates pending
// memory_relations entries. Returns candidates for the caller to judge.
// Backward compatible wrapper: delegates to FindCandidatesWithOptions with
// default limit 5 and BM25Floor -2.0 (engram default).
func (s *Store) FindCandidates(savedID, title, project, scope string) ([]Candidate, error) {
	floor := -2.0
	return s.FindCandidatesWithOptions(savedID, title, FindOptions{
		Project:   project,
		Scope:     scope,
		Limit:     5,
		BM25Floor: &floor,
	})
}

// validateCrossProjectGuard checks whether the observations referenced by
// judgmentID belong to the same project. Returns ErrCrossProjectRelation if
// they differ and both projects are non-empty. Missing observations are treated
// as empty project and do not trigger the guard (to allow orphan handling).
func (s *Store) validateCrossProjectGuard(judgmentID string) error {
	var sourceID, targetID string
	if err := s.db.QueryRow(`SELECT source_id, target_id FROM memory_relations WHERE id = ?`, judgmentID).Scan(&sourceID, &targetID); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return nil
	}
	var srcProject, tgtProject string
	// Direct project lookup (via o.project). Fallback to session project if empty
	// is handled implicitly by the coalesce query when sessions exist — but for
	// BigMem the simple lookup suffices for the guard.
	_ = s.db.QueryRow(`SELECT COALESCE(project,'') FROM observations WHERE id = ?`, sourceID).Scan(&srcProject)
	if srcProject == "" {
		_ = s.db.QueryRow(`SELECT COALESCE(s.project,'') FROM observations o LEFT JOIN sessions s ON s.id = o.session_id WHERE o.id = ?`, sourceID).Scan(&srcProject)
	}
	_ = s.db.QueryRow(`SELECT COALESCE(project,'') FROM observations WHERE id = ?`, targetID).Scan(&tgtProject)
	if tgtProject == "" {
		_ = s.db.QueryRow(`SELECT COALESCE(s.project,'') FROM observations o LEFT JOIN sessions s ON s.id = o.session_id WHERE o.id = ?`, targetID).Scan(&tgtProject)
	}
	if srcProject != "" && tgtProject != "" && srcProject != tgtProject {
		return ErrCrossProjectRelation
	}
	return nil
}

// JudgeRelation updates a pending memory_relation with a verdict.
// Returns ErrCrossProjectRelation if source and target belong to different projects.
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

	if err := s.validateCrossProjectGuard(judgmentID); err != nil {
		return err
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

// SaveRelationWithMarkedBy creates a pending relation with provenance split (Batch S, Engram parity).
// markedBy is "actor:kind:model"; it is split into marked_by_actor/kind/model while keeping marked_by for compat.
func (s *Store) SaveRelationWithMarkedBy(sourceID, targetID, markedBy, sessionID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	actor, kind, model := parseMarkedBy(markedBy)
	id := fmt.Sprintf("rel-%d", time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT INTO memory_relations (id, source_id, target_id, relation, judgment_status, marked_by, marked_by_actor, marked_by_kind, marked_by_model, session_id, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', 'pending', ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING`,
		id, sourceID, targetID, markedBy, actor, kind, model, sessionID, now, now)
	return id, err
}

// RelationCount returns the number of pending judgments.
func (s *Store) RelationCount() int {
	var n int
	s.db.QueryRow("SELECT COUNT(*) FROM memory_relations WHERE judgment_status='pending'").Scan(&n)
	return n
}

// Relation represents a memory_relations row.
type Relation struct {
	ID             string    `json:"id"`
	SourceID       string    `json:"source_id"`
	TargetID       string    `json:"target_id"`
	Relation       string    `json:"relation"`
	JudgmentStatus string    `json:"judgment_status"`
	Reason         string    `json:"reason,omitempty"`
	Evidence       string    `json:"evidence,omitempty"`
	Confidence     float64   `json:"confidence,omitempty"`
	MarkedBy       string    `json:"marked_by,omitempty"`
	MarkedByActor  string    `json:"marked_by_actor,omitempty"`
	MarkedByKind   string    `json:"marked_by_kind,omitempty"`
	MarkedByModel  string    `json:"marked_by_model,omitempty"`
	SessionID      string    `json:"session_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ListRelations returns relations filtered by judgment status (empty = all).
func (s *Store) ListRelations(status string) ([]Relation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := `SELECT id, source_id, target_id, relation, judgment_status,
		COALESCE(reason,''), COALESCE(evidence,''), COALESCE(confidence,0),
		COALESCE(marked_by,''), COALESCE(marked_by_actor,''), COALESCE(marked_by_kind,''), COALESCE(marked_by_model,''),
		COALESCE(session_id,''), created_at, updated_at FROM memory_relations`
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
			&r.JudgmentStatus, &r.Reason, &r.Evidence, &r.Confidence,
			&r.MarkedBy, &r.MarkedByActor, &r.MarkedByKind, &r.MarkedByModel, &r.SessionID, &ca, &ua); err != nil {
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
		 COALESCE(marked_by,''), COALESCE(marked_by_actor,''), COALESCE(marked_by_kind,''), COALESCE(marked_by_model,''),
		 COALESCE(session_id,''), created_at, updated_at FROM memory_relations WHERE id = ?`, id).
		Scan(&r.ID, &r.SourceID, &r.TargetID, &r.Relation,
			&r.JudgmentStatus, &r.Reason, &r.Evidence, &r.Confidence,
			&r.MarkedBy, &r.MarkedByActor, &r.MarkedByKind, &r.MarkedByModel, &r.SessionID, &ca, &ua)
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

	q := `SELECT id, title, project, scope FROM observations WHERE deleted_at IS NULL`
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
		var cnt int
		s.db.QueryRow("SELECT COUNT(*) FROM memory_relations WHERE source_id = ? OR target_id = ?", id, id).Scan(&cnt)
		if cnt > 0 {
			continue
		}
		if dryRun {
			total++
			continue
		}
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
			 AND o.id != ? AND o.project = ? AND o.deleted_at IS NULL LIMIT 5`,
			ftsQ, id, proj)
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

// ─── Project summaries ───────────────────────────────────────────────────────

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

// ─── Delete variants ─────────────────────────────────────────────────────────

// DeleteSession removes a session by ID.
func (s *Store) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var cnt int
	s.db.QueryRow("SELECT COUNT(*) FROM prompts WHERE session_id=?", id).Scan(&cnt)
	if cnt > 0 {
		return fmt.Errorf("session %s has %d prompts; delete prompts first", id, cnt)
	}
	_, err := s.db.Exec("DELETE FROM sessions WHERE id=?", id)
	return err
}

// DeletePrompt removes a prompt by ID (permanent).
func (s *Store) DeletePrompt(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("DELETE FROM prompts WHERE id=?", id)
	return err
}

// DeleteProjectResult describes what was deleted.
type DeleteProjectResult struct {
	ObservationsDeleted int `json:"observations_deleted"`
	PromptsDeleted      int `json:"prompts_deleted"`
	SessionsDeleted     int `json:"sessions_deleted"`
}

// DeleteProject cascade-deletes a project.
func (s *Store) DeleteProject(name string, hard bool) (*DeleteProjectResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := &DeleteProjectResult{}

	res, _ := s.db.Exec("DELETE FROM observations WHERE project=?", name)
	if n, _ := res.RowsAffected(); n > 0 {
		r.ObservationsDeleted = int(n)
	}
	res, _ = s.db.Exec("DELETE FROM prompts WHERE session_id IN (SELECT id FROM sessions WHERE project=?)", name)
	if n, _ := res.RowsAffected(); n > 0 {
		r.PromptsDeleted = int(n)
	}
	if hard {
		res, _ = s.db.Exec("DELETE FROM sessions WHERE project=?", name)
	} else {
		res, _ = s.db.Exec("UPDATE sessions SET project='' WHERE project=?", name)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		r.SessionsDeleted = int(n)
	}
	return r, nil
}

// ─── Conflicts stats ─────────────────────────────────────────────────────────

// ConflictsStats returns conflict-related stats.
type ConflictsStats struct {
	TotalRelations int            `json:"total_relations"`
	Pending        int            `json:"pending"`
	Judged         int            `json:"judged"`
	ByVerdict      map[string]int `json:"by_verdict"`
}

// ConflictsStats returns statistics about memory relations.
func (s *Store) ConflictsStats(project string) (*ConflictsStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cs := &ConflictsStats{ByVerdict: make(map[string]int)}
	s.db.QueryRow("SELECT COUNT(*) FROM memory_relations").Scan(&cs.TotalRelations)
	s.db.QueryRow("SELECT COUNT(*) FROM memory_relations WHERE judgment_status='pending'").Scan(&cs.Pending)
	s.db.QueryRow("SELECT COUNT(*) FROM memory_relations WHERE judgment_status='judged'").Scan(&cs.Judged)
	rows, _ := s.db.Query("SELECT relation, COUNT(*) FROM memory_relations WHERE relation != 'pending' GROUP BY relation")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var r string
			var c int
			rows.Scan(&r, &c)
			cs.ByVerdict[r] = c
		}
	}
	return cs, nil
}

// ConflictsDeferred returns relations that were skipped (status=deferred).
func (s *Store) ConflictsDeferred(status string, limit int) ([]Relation, error) {
	return s.ListRelations("deferred")
}

// ─── Projects consolidate ────────────────────────────────────────────────────

// ConsolidateResult describes the result of consolidating projects.
type ConsolidateResult struct {
	Groups     []ConsolidateGroup `json:"groups,omitempty"`
	Merged     int                `json:"merged"`
	NewName    string             `json:"new_name,omitempty"`
	SourceName string             `json:"source_name,omitempty"`
}

// ConsolidateGroup is a group of similar project names.
type ConsolidateGroup struct {
	Projects  []string `json:"projects"`
	Canonical string   `json:"canonical"`
}

// ConsolidateProjects merges similar project names.
func (s *Store) ConsolidateProjects(all, dryRun bool) (*ConsolidateResult, error) {
	rows, err := s.db.Query("SELECT DISTINCT project FROM observations WHERE project != '' ORDER BY project")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []string
	for rows.Next() {
		var p string
		rows.Scan(&p)
		projects = append(projects, p)
	}

	normalize := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		s = strings.ReplaceAll(s, "-", "")
		s = strings.ReplaceAll(s, "_", "")
		s = strings.ReplaceAll(s, " ", "")
		return s
	}

	var groups []ConsolidateGroup
	used := map[string]bool{}

	for _, p := range projects {
		if used[p] {
			continue
		}
		norm := normalize(p)
		group := ConsolidateGroup{Canonical: p}
		group.Projects = append(group.Projects, p)
		used[p] = true

		for _, q := range projects {
			if used[q] {
				continue
			}
			if normalize(q) == norm {
				group.Projects = append(group.Projects, q)
				used[q] = true
			}
		}

		if len(group.Projects) > 1 {
			groups = append(groups, group)
		}
	}

	if dryRun {
		return &ConsolidateResult{Groups: groups}, nil
	}

	totalMerged := 0
	for _, g := range groups {
		for _, p := range g.Projects[1:] {
			n, err := s.MergeProjects(p, g.Canonical)
			if err == nil {
				totalMerged += n
			}
		}
	}
	return &ConsolidateResult{Merged: totalMerged}, nil
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
