package recoverytrace

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store persists recovery ledgers in SQLite.
type Store struct {
	db *sql.DB
}

// Open opens the recovery trace store.
func Open(rootDir string) (*Store, error) {
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home dir: %w", err)
		}
		rootDir = filepath.Join(home, ".biggz", "recovery")
	}
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	dbPath := filepath.Join(rootDir, "recovery.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.Exec("PRAGMA journal_mode=WAL")
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ledgers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			project TEXT DEFAULT '',
			created_at TEXT NOT NULL,
			reconciliation TEXT NOT NULL,
			backlog TEXT NOT NULL,
			rows_data TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_ledger_project ON ledgers(project);
		CREATE INDEX IF NOT EXISTS idx_ledger_name ON ledgers(name);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}
	return &Store{db: db}, nil
}

// SaveLedger persists a ledger under a given name/project.
func (s *Store) SaveLedger(name, project string, ledgers Ledgers) (string, error) {
	id := fmt.Sprintf("rec-%d", time.Now().UnixNano())
	recJSON, _ := json.Marshal(ledgers.Reconciliation)
	backlogJSON, _ := json.Marshal(ledgers.Backlog)
	rowsJSON, _ := json.Marshal(ledgers.Rows)
	_, err := s.db.Exec(
		`INSERT INTO ledgers (id, name, project, created_at, reconciliation, backlog, rows_data)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, name, project, time.Now().UTC().Format(time.RFC3339),
		string(recJSON), string(backlogJSON), string(rowsJSON))
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetLedger retrieves a ledger by ID.
func (s *Store) GetLedger(id string) (*Ledgers, string, string, error) {
	var name, project, recJSON, backlogJSON, rowsJSON string
	err := s.db.QueryRow(
		`SELECT name, project, reconciliation, backlog, rows_data FROM ledgers WHERE id = ?`, id).
		Scan(&name, &project, &recJSON, &backlogJSON, &rowsJSON)
	if err != nil {
		return nil, "", "", fmt.Errorf("ledger %s not found", id)
	}
	var ledgers Ledgers
	json.Unmarshal([]byte(recJSON), &ledgers.Reconciliation)
	json.Unmarshal([]byte(backlogJSON), &ledgers.Backlog)
	json.Unmarshal([]byte(rowsJSON), &ledgers.Rows)
	return &ledgers, name, project, nil
}

// ListLedgers returns all ledger summaries.
type LedgerSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Project   string `json:"project,omitempty"`
	CreatedAt string `json:"created_at"`
	RowCount  int    `json:"row_count"`
}

// ListLedgers returns all ledgers ordered by creation time.
func (s *Store) ListLedgers(project string) ([]LedgerSummary, error) {
	q := `SELECT id, name, COALESCE(project,''), created_at,
	         (SELECT COUNT(*) FROM json_each(rows_data)) as row_count
		  FROM ledgers`
	var args []any
	if project != "" {
		q += " WHERE project = ?"
		args = append(args, project)
	}
	q += " ORDER BY created_at DESC LIMIT 50"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []LedgerSummary
	for rows.Next() {
		var s LedgerSummary
		if err := rows.Scan(&s.ID, &s.Name, &s.Project, &s.CreatedAt, &s.RowCount); err != nil {
			continue
		}
		result = append(result, s)
	}
	return result, nil
}

// DeleteLedger removes a ledger by ID.
func (s *Store) DeleteLedger(id string) error {
	_, err := s.db.Exec("DELETE FROM ledgers WHERE id = ?", id)
	return err
}

// ExportLedger returns a ledger as JSON bytes.
func (s *Store) ExportLedger(id string) ([]byte, error) {
	ledgers, _, _, err := s.GetLedger(id)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(ledgers, "", "  ")
}

// ImportLedger imports a ledger from JSON bytes.
func (s *Store) ImportLedger(data []byte, name, project string) (string, error) {
	var ledgers Ledgers
	if err := json.Unmarshal(data, &ledgers); err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	return s.SaveLedger(name, project, ledgers)
}

// Close shuts down the store.
func (s *Store) Close() error {
	return s.db.Close()
}
