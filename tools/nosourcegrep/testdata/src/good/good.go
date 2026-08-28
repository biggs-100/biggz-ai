package good

import (
	"database/sql"
)

// Good: transformation/branch + external contract via modernc.org/sqlite — see docs/testing-guidance.md
func Good() {
	var parent sql.NullString
	var leaf string
	// This is the Good pattern: assert DB state, not source text
	_ = parent
	_ = leaf
	// Simulate TestBranch_Traversal DB query
	db, _ := sql.Open("sqlite", ":memory:")
	row := db.QueryRow("SELECT parent_id, leaf_id FROM sessions WHERE id = ?", "test")
	_ = row.Scan(&parent, &leaf)
}
