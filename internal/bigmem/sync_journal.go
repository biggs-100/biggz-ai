package bigmem

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type SyncState struct {
	TargetKey           string  `json:"target_key"`
	Project             string  `json:"project"`
	Lifecycle           string  `json:"lifecycle"`
	LastEnqueuedSeq     int64   `json:"last_enqueued_seq"`
	LastAckedSeq        int64   `json:"last_acked_seq"`
	LastPulledSeq       int64   `json:"last_pulled_seq"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
	BackoffUntil        *string `json:"backoff_until,omitempty"`
	LeaseOwner          *string `json:"lease_owner,omitempty"`
	LeaseUntil          *string `json:"lease_until,omitempty"`
	ReasonCode          *string `json:"reason_code,omitempty"`
}

const (
	SyncLifecycleIdle     = "idle"
	SyncLifecyclePending  = "pending"
	SyncLifecycleRunning  = "running"
	SyncLifecycleHealthy  = "healthy"
	SyncLifecycleDegraded = "degraded"
)

func ensureSyncJournalTables(db *sql.DB) {
	has := false
	if rows, err := db.Query(`PRAGMA table_info(sync_mutations)`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var cid int
			var n, t string
			var nn, pk int
			var d sql.NullString
			_ = rows.Scan(&cid, &n, &t, &nn, &d, &pk)
			if n == "seq" {
				has = true
			}
		}
	} else {
		has = true
	}
	if !has {
		_, _ = db.Exec(`DROP TABLE IF EXISTS sync_mutations`)
	}
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS sync_mutations (seq INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT, project TEXT, entity TEXT, entity_key TEXT, op TEXT, payload TEXT, source TEXT, disposition TEXT DEFAULT 'pending', created_at TEXT)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sync_mutations_project_seq ON sync_mutations(project, seq)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sync_mutations_disposition ON sync_mutations(disposition)`)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS sync_state (target_key TEXT NOT NULL, project TEXT NOT NULL, lifecycle TEXT NOT NULL DEFAULT 'idle', last_enqueued_seq INTEGER DEFAULT 0, last_acked_seq INTEGER DEFAULT 0, last_pulled_seq INTEGER DEFAULT 0, consecutive_failures INTEGER DEFAULT 0, backoff_until TEXT, lease_owner TEXT, lease_until TEXT, reason_code TEXT, PRIMARY KEY (target_key, project))`)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS sync_enrolled_projects (project TEXT PRIMARY KEY, enrolled_at TEXT NOT NULL)`)
}
func enqueueSyncMutationTx(tx *sql.Tx, project, entity, entityKey, op string, payload []byte) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	ps := ""
	if len(payload) > 0 {
		ps = string(payload)
	}
	res, err := tx.Exec(`INSERT INTO sync_mutations (project, entity, entity_key, op, payload, source, disposition, created_at) VALUES (?, ?, ?, ?, ?, ?, 'pending', ?)`, project, entity, entityKey, op, ps, "local", now)
	if err != nil {
		return 0, fmt.Errorf("enqueue: %w", err)
	}
	seq, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("enqueue seq: %w", err)
	}
	tk := project
	if tk == "" {
		tk = "default"
	}
	_, _ = tx.Exec(`INSERT INTO sync_state (target_key, project, lifecycle, last_enqueued_seq, last_acked_seq) VALUES (?, ?, 'pending', ?, 0) ON CONFLICT(target_key, project) DO UPDATE SET lifecycle='pending', last_enqueued_seq=excluded.last_enqueued_seq`, tk, project, seq)
	_, _ = tx.Exec(`INSERT OR IGNORE INTO sync_enrolled_projects (project, enrolled_at) VALUES (?, ?)`, project, now)
	return seq, nil
}
func tryEnqueueObservationTx(tx *sql.Tx, obs *Observation) error {
	if obs.Project == "" {
		return nil
	}
	var has int
	_ = tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sync_mutations'`).Scan(&has)
	if has == 0 {
		return nil
	}
	pl, _ := json.Marshal(obs)
	_, err := enqueueSyncMutationTx(tx, obs.Project, "observation", obs.ID, "upsert", pl)
	return err
}
func (s *Store) ListPendingMutations(project string, limit int) ([]SyncMutation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := `SELECT seq, project, entity, entity_key, op, payload, source, disposition, created_at FROM sync_mutations WHERE disposition='pending'`
	var a []any
	if project != "" {
		q += ` AND project = ?`
		a = append(a, project)
	}
	q += ` ORDER BY seq ASC`
	if limit > 0 {
		q += ` LIMIT ?`
		a = append(a, limit)
	}
	rows, err := s.db.Query(q, a...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SyncMutation
	for rows.Next() {
		var m SyncMutation
		var pls, src, disp, ca sql.NullString
		var seq sql.NullInt64
		var proj, ent, key, op sql.NullString
		if err := rows.Scan(&seq, &proj, &ent, &key, &op, &pls, &src, &disp, &ca); err != nil {
			continue
		}
		if seq.Valid {
			m.Seq = seq.Int64
		}
		if proj.Valid {
			m.Project = proj.String
		}
		if ent.Valid {
			m.Entity = ent.String
		}
		if key.Valid {
			m.EntityKey = key.String
		}
		if op.Valid {
			m.Op = op.String
		}
		if pls.Valid {
			m.Payload = []byte(pls.String)
		}
		if src.Valid {
			m.Source = src.String
		}
		if disp.Valid {
			m.Disposition = disp.String
		}
		if ca.Valid {
			m.CreatedAt = ca.String
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) AckSyncMutation(seq int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var proj string
	if err := s.db.QueryRow(`SELECT project FROM sync_mutations WHERE seq = ?`, seq).Scan(&proj); err != nil {
		return fmt.Errorf("ack lookup: %w", err)
	}
	if _, err := s.db.Exec(`UPDATE sync_mutations SET disposition='acked' WHERE seq = ?`, seq); err != nil {
		return err
	}
	tk := proj
	if tk == "" {
		tk = "default"
	}
	var pend int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM sync_mutations WHERE project = ? AND disposition='pending'`, proj).Scan(&pend)
	lc := SyncLifecyclePending
	if pend == 0 {
		lc = SyncLifecycleHealthy
	}
	_, _ = s.db.Exec(`INSERT INTO sync_state (target_key, project, lifecycle, last_acked_seq, last_enqueued_seq) VALUES (?, ?, ?, ?, 0) ON CONFLICT(target_key, project) DO UPDATE SET lifecycle=excluded.lifecycle, last_acked_seq=excluded.last_acked_seq`, tk, proj, lc, seq)
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = s.db.Exec(`INSERT OR IGNORE INTO sync_enrolled_projects (project, enrolled_at) VALUES (?, ?)`, proj, now)
	return nil
}
func (s *Store) GetSyncState(project string) (*SyncState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tk := project
	if tk == "" {
		tk = "default"
	}
	st := &SyncState{TargetKey: tk, Project: project, Lifecycle: SyncLifecycleIdle}
	var le, la, lp, cf sql.NullInt64
	var bu, lo, lu, rc, lc sql.NullString
	err := s.db.QueryRow(`SELECT lifecycle, last_enqueued_seq, last_acked_seq, last_pulled_seq, consecutive_failures, backoff_until, lease_owner, lease_until, reason_code FROM sync_state WHERE target_key = ? AND project = ?`, tk, project).Scan(&lc, &le, &la, &lp, &cf, &bu, &lo, &lu, &rc)
	if err == sql.ErrNoRows {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	if lc.Valid {
		st.Lifecycle = lc.String
	}
	if le.Valid {
		st.LastEnqueuedSeq = le.Int64
	}
	if la.Valid {
		st.LastAckedSeq = la.Int64
	}
	if lp.Valid {
		st.LastPulledSeq = lp.Int64
	}
	if cf.Valid {
		st.ConsecutiveFailures = int(cf.Int64)
	}
	if bu.Valid {
		st.BackoffUntil = &bu.String
	}
	if lo.Valid {
		st.LeaseOwner = &lo.String
	}
	if lu.Valid {
		st.LeaseUntil = &lu.String
	}
	if rc.Valid {
		st.ReasonCode = &rc.String
	}
	return st, nil
}
