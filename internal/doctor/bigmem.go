package doctor

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biggz-ai/biggz/internal/bigmem"
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

// Remedy returns nil — database repair requires external tooling.
func (c *BigmemCheck) Remedy() *Remedy { return nil }
