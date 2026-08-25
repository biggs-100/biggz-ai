package doctor

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
	_ "modernc.org/sqlite"
)

const (
	// BigmemCheckID is the check identifier for bigmem database integrity.
	BigmemCheckID CheckID = "bigmem"
)

// BigmemCheck verifies bigmem SQLite database integrity by running
// PRAGMA integrity_check.
type BigmemCheck struct {
	opener func() (*bigmem.Store, error)
}

// NewBigmemCheck creates a BigmemCheck that opens the default bigmem store.
func NewBigmemCheck() *BigmemCheck {
	return &BigmemCheck{
		opener: func() (*bigmem.Store, error) {
			return bigmem.Open("")
		},
	}
}

// NewBigmemCheckWithOpener creates a BigmemCheck with a custom store opener
// for testing.
func NewBigmemCheckWithOpener(opener func() (*bigmem.Store, error)) *BigmemCheck {
	return &BigmemCheck{opener: opener}
}

// ID returns the check identifier.
func (c *BigmemCheck) ID() CheckID { return BigmemCheckID }

// Run opens the bigmem store and runs PRAGMA integrity_check.
func (c *BigmemCheck) Run(ctx context.Context) *Result {
	store, err := c.opener()
	if err != nil {
		return &Result{
			ID:       BigmemCheckID,
			Status:   StatusFail,
			Message:  "Cannot open bigmem store",
			Severity: SeverityCritical,
			Error:    err.Error(),
		}
	}
	defer store.Close()

	// Run PRAGMA integrity_check by accessing the underlying *sql.DB.
	// We need to extract the db from the store. Since bigmem.Store has
	// unexported db field, we use the store's Close method not being nil
	// as evidence the store opened, then query a separate connection to
	// the same database.
	//
	// Open a second connection to the same database file for the integrity
	// check, since bigmem.Store's db field is unexported.
	dbPath := store.RootDir() + "/bigmem.db"
	result := c.checkIntegrity(ctx, dbPath)
	return result
}

// checkIntegrity opens a raw SQLite connection and runs PRAGMA integrity_check.
func (c *BigmemCheck) checkIntegrity(ctx context.Context, dbPath string) *Result {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return &Result{
			ID:       BigmemCheckID,
			Status:   StatusFail,
			Message:  "Cannot open database connection for integrity check",
			Severity: SeverityCritical,
			Error:    err.Error(),
		}
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return &Result{
			ID:       BigmemCheckID,
			Status:   StatusFail,
			Message:  "Failed to run PRAGMA integrity_check",
			Severity: SeverityCritical,
			Error:    err.Error(),
		}
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
	if err := rows.Err(); err != nil {
		return &Result{
			ID:       BigmemCheckID,
			Status:   StatusFail,
			Message:  "Error reading integrity_check results",
			Severity: SeverityCritical,
			Error:    err.Error(),
		}
	}

	if len(messages) > 0 {
		return &Result{
			ID:       BigmemCheckID,
			Status:   StatusFail,
			Message:  fmt.Sprintf("Database integrity violations: %s", messages[0]),
			Severity: SeverityCritical,
			Error:    fmt.Sprintf("integrity errors: %v", messages),
		}
	}

	return &Result{
		ID:       BigmemCheckID,
		Status:   StatusPass,
		Message:  "Bigmem database integrity OK",
		Severity: SeverityInfo,
	}
}

// Remedy returns a repair action that repairs WAL, FTS, and schema.
// It is safe and idempotent: re-running on a healthy database succeeds.
func (c *BigmemCheck) Remedy() *Remedy {
	return &Remedy{
		ID:          string(BigmemCheckID),
		Description: "Repair BigMem WAL/FTS/schema",
		Action: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			store, err := c.opener()
			if err == nil {
				// Comprehensive fix via Store API (WAL checkpoint, VACUUM, FTS rebuild, directory migration).
				fixErr := store.DoctorFix()
				_ = store.Close()
				return fixErr
			}
			home, herr := os.UserHomeDir()
			if herr != nil {
				return fmt.Errorf("bigmem remedy: cannot determine home dir: %w", herr)
			}
			dbPath := filepath.Join(home, ".biggz", "bigmem", "bigmem.db")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				// No database file yet — create a fresh store.
				s, oerr := c.opener()
				if oerr != nil {
					return fmt.Errorf("bigmem remedy: cannot create store: %w", oerr)
				}
				_ = s.Close()
				return nil
			}
			// Ensure parent dir exists (repair may be called before bigmem init).
			if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
				return fmt.Errorf("bigmem remedy: mkdir: %w", err)
			}
			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				return fmt.Errorf("bigmem remedy: open db: %w", err)
			}
			defer db.Close()

			// WAL checkpoint and VACUUM to fix malformed and checkpoint issues.
			_, _ = db.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)")
			_, _ = db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
			_, _ = db.ExecContext(ctx, "VACUUM")

			// Schema migration: ensure sessions.directory exists.
			var sessExists int
			_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&sessExists)
			if sessExists == 0 {
				_, _ = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS sessions
					(id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT)`)
			} else {
				rows, qerr := db.QueryContext(ctx, "PRAGMA table_info(sessions)")
				if qerr == nil {
					hasDir := false
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
					}
					rows.Close()
					if !hasDir {
						_, _ = db.ExecContext(ctx, "ALTER TABLE sessions ADD COLUMN directory TEXT")
					}
				}
			}

			// Try REINDEX first (standard SQLite).
			if _, err := db.ExecContext(ctx, "REINDEX observations_fts"); err == nil {
				return nil
			}
			// FTS5 rebuild syntax.
			if _, err := db.ExecContext(ctx, "INSERT INTO observations_fts(observations_fts) VALUES('rebuild')"); err == nil {
				return nil
			}
			// Fallback: drop and recreate FTS5 virtual table + triggers (mirrors bigmem.Open).
			_, _ = db.ExecContext(ctx, "DROP TRIGGER IF EXISTS obs_fts_ai")
			_, _ = db.ExecContext(ctx, "DROP TRIGGER IF EXISTS obs_fts_ad")
			_, _ = db.ExecContext(ctx, "DROP TRIGGER IF EXISTS obs_fts_au")
			_, _ = db.ExecContext(ctx, "DROP TRIGGER IF EXISTS obs_fts_insert")
			_, _ = db.ExecContext(ctx, "DROP TRIGGER IF EXISTS obs_fts_delete")
			_, _ = db.ExecContext(ctx, "DROP TRIGGER IF EXISTS obs_fts_update")
			if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS observations_fts"); err != nil {
				return fmt.Errorf("bigmem remedy: drop fts: %w", err)
			}
			if _, err := db.ExecContext(ctx, `
				CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
					title, content, topic_key, tool_name, type, project,
					content='observations',
					content_rowid='rowid'
				);
			`); err != nil {
				return fmt.Errorf("bigmem remedy: create fts: %w", err)
			}
			// Recreate triggers.
			_, _ = db.ExecContext(ctx, `
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
			// Repopulate from existing observations (best-effort).
			_, _ = db.ExecContext(ctx, "INSERT INTO observations_fts(observations_fts) VALUES('rebuild')")
			_, _ = db.ExecContext(ctx, `
				INSERT OR IGNORE INTO observations_fts(rowid, title, content, topic_key, tool_name, type, project)
				SELECT rowid, title, content, topic_key, tool_name, type, project FROM observations`)
			return nil
		},
	}
}
