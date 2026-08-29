package bigmem

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

var ErrFKMissing = errors.New("fk missing")

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
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS sync_apply_deferred (seq INTEGER PRIMARY KEY, sync_id TEXT UNIQUE, project TEXT, entity TEXT, entity_key TEXT, payload TEXT, attempts INTEGER DEFAULT 1, next_attempt_at TEXT, created_at TEXT)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sync_apply_deferred_project ON sync_apply_deferred(project)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sync_apply_deferred_sync_id ON sync_apply_deferred(sync_id)`)
}

func relationApplyFailureSyncID(project, entityKey string, payload []byte) string {
	h := sha256.Sum256([]byte(project + "\x00" + entityKey + "\x00" + string(payload) + "\x00raf"))
	return "raf-" + hex.EncodeToString(h[:])[:12]
}
func pulledSessionDeadLetterSyncID(project, entityKey string, payload []byte) string {
	h := sha256.Sum256([]byte(project + "\x00" + entityKey + "\x00" + string(payload) + "\x00psdl"))
	return "psdl-" + hex.EncodeToString(h[:])[:12]
}
func deadLetterID(project, entityKey string, payload []byte) string {
	return pulledSessionDeadLetterSyncID(project, entityKey, payload)
}
func recordRelationApplyFailureTx(tx *sql.Tx, syncID, project, entity, entityKey, payload string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	next := time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339)
	var a int
	if err := tx.QueryRow(`SELECT attempts FROM sync_apply_deferred WHERE sync_id=?`, syncID).Scan(&a); err == sql.ErrNoRows {
		_, err = tx.Exec(`INSERT INTO sync_apply_deferred (sync_id, project, entity, entity_key, payload, attempts, next_attempt_at, created_at) VALUES (?,?,?,?,?,1,?,?)`, syncID, project, entity, entityKey, payload, next, now)
		return err
	} else if err != nil {
		return err
	}
	if a >= 5 {
		dID := deadLetterID(project, entityKey, []byte(payload))
		if entity == "memory_relation" || entity == "relation" {
			dID = relationApplyFailureSyncID(project, entityKey, []byte(payload))
		}
		if dID == syncID {
			_, err := tx.Exec(`UPDATE sync_apply_deferred SET attempts=6, next_attempt_at=? WHERE sync_id=?`, "", syncID)
			return err
		}
		_, err := tx.Exec(`UPDATE sync_apply_deferred SET attempts=6, sync_id=?, next_attempt_at=?, payload=? WHERE sync_id=?`, dID, "", payload, syncID)
		if err != nil && strings.Contains(err.Error(), "UNIQUE") {
			_, _ = tx.Exec(`DELETE FROM sync_apply_deferred WHERE sync_id=?`, syncID)
		}
		return err
	}
	_, err := tx.Exec(`UPDATE sync_apply_deferred SET attempts=?, next_attempt_at=?, payload=? WHERE sync_id=?`, a+1, next, payload, syncID)
	return err
}
func applyPulledMutationTx(tx *sql.Tx, m SyncMutation) error {
	switch m.Entity {
	case "observation":
		var obs Observation
		if err := json.Unmarshal(m.Payload, &obs); err != nil {
			var mp map[string]any
			if jerr := json.Unmarshal(m.Payload, &mp); jerr != nil {
				return jerr
			}
			sid, _ := mp["session_id"].(string)
			if sid != "" {
				var c int
				_ = tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id=?`, sid).Scan(&c)
				if c == 0 {
					return ErrFKMissing
				}
			}
			id := m.EntityKey
			if v, ok := mp["id"].(string); ok && v != "" {
				id = v
			}
			if id == "" {
				return fmt.Errorf("observation id missing")
			}
			title, _ := mp["title"].(string)
			typ, _ := mp["type"].(string)
			content, _ := mp["content"].(string)
			proj, _ := mp["project"].(string)
			if proj == "" {
				proj = m.Project
			}
			scope, _ := mp["scope"].(string)
			now := time.Now().UTC().Format(time.RFC3339)
			_, err := tx.Exec(`INSERT INTO observations (id,title,type,content,session_id,project,scope,normalized_hash,revision_count,duplicate_count,pinned,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?, ?,1,1,0,?,?) ON CONFLICT(id) DO UPDATE SET title=excluded.title, content=excluded.content, updated_at=excluded.updated_at`, id, title, typ, content, sid, proj, scope, hashNormalized(content), now, now)
			return err
		}
		if obs.ID == "" {
			obs.ID = m.EntityKey
		}
		if obs.Project == "" {
			obs.Project = m.Project
		}
		if obs.SessionID != "" {
			var c int
			_ = tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id=?`, obs.SessionID).Scan(&c)
			if c == 0 {
				return ErrFKMissing
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		ca := obs.CreatedAt.Format(time.RFC3339)
		if obs.CreatedAt.IsZero() {
			ca = now
		}
		ua := obs.UpdatedAt.Format(time.RFC3339)
		if obs.UpdatedAt.IsZero() {
			ua = now
		}
		h := obs.NormalizedHash
		if h == "" {
			h = hashNormalized(obs.Content)
		}
		_, err := tx.Exec(`INSERT INTO observations (id,title,type,content,session_id,tool_name,topic_key,project,scope,normalized_hash,revision_count,duplicate_count,pinned,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET title=excluded.title, type=excluded.type, content=excluded.content, updated_at=excluded.updated_at`, obs.ID, obs.Title, obs.Type, obs.Content, obs.SessionID, obs.ToolName, obs.TopicKey, obs.Project, obs.Scope, h, 1, 1, 0, ca, ua)
		_ = h
		return err
	case "session":
		return nil
	case "memory_relation", "relation":
		var mp map[string]any
		if err := json.Unmarshal(m.Payload, &mp); err != nil {
			return err
		}
		sid, _ := mp["source_id"].(string)
		tid, _ := mp["target_id"].(string)
		if sid != "" {
			var c int
			_ = tx.QueryRow(`SELECT COUNT(*) FROM observations WHERE id=?`, sid).Scan(&c)
			if c == 0 {
				return ErrFKMissing
			}
		}
		if tid != "" {
			var c int
			_ = tx.QueryRow(`SELECT COUNT(*) FROM observations WHERE id=?`, tid).Scan(&c)
			if c == 0 {
				return ErrFKMissing
			}
		}
		id, _ := mp["id"].(string)
		if id == "" {
			id = m.EntityKey
		}
		if id == "" {
			id = fmt.Sprintf("rel-%d", time.Now().UnixNano())
		}
		now := time.Now().UTC().Format(time.RFC3339)
		rel, _ := mp["relation"].(string)
		if rel == "" {
			rel = "pending"
		}
		_, err := tx.Exec(`INSERT INTO memory_relations (id,source_id,target_id,relation,judgment_status,created_at,updated_at) VALUES (?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, id, sid, tid, rel, "pending", now, now)
		return err
	default:
		return nil
	}
}
func (s *Store) ApplyPulledMutation(m SyncMutation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	ae := applyPulledMutationTx(tx, m)
	if errors.Is(ae, ErrFKMissing) {
		ps := string(m.Payload)
		var sid string
		if m.Entity == "memory_relation" || m.Entity == "relation" {
			sid = relationApplyFailureSyncID(m.Project, m.EntityKey, m.Payload)
		} else {
			sid = pulledSessionDeadLetterSyncID(m.Project, m.EntityKey, m.Payload)
		}
		if err := recordRelationApplyFailureTx(tx, sid, m.Project, m.Entity, m.EntityKey, ps); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return ErrFKMissing
	}
	if ae != nil {
		_ = tx.Rollback()
		return ae
	}
	return tx.Commit()
}
func (s *Store) ReplayDeferredForScope(scope string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := `SELECT sync_id,project,entity,entity_key,payload,attempts FROM sync_apply_deferred`
	var a []any
	if scope != "" {
		q += ` WHERE project=?`
		a = append(a, scope)
	}
	q += ` ORDER BY seq ASC`
	rows, err := s.db.Query(q, a...)
	if err != nil {
		return err
	}
	defer rows.Close()
	type r struct {
		sid, proj, ent, key, pl string
		att                      int
	}
	var list []r
	for rows.Next() {
		var sid, proj, ent, key, pl sql.NullString
		var att sql.NullInt64
		if err := rows.Scan(&sid, &proj, &ent, &key, &pl, &att); err != nil {
			continue
		}
		rr := r{}
		if sid.Valid {
			rr.sid = sid.String
		}
		if proj.Valid {
			rr.proj = proj.String
		}
		if ent.Valid {
			rr.ent = ent.String
		}
		if key.Valid {
			rr.key = key.String
		}
		if pl.Valid {
			rr.pl = pl.String
		}
		if att.Valid {
			rr.att = int(att.Int64)
		}
		list = append(list, rr)
	}
	for _, it := range list {
		m := SyncMutation{Project: it.proj, Entity: it.ent, EntityKey: it.key, Payload: []byte(it.pl)}
		tx, _ := s.db.Begin()
		if tx == nil {
			continue
		}
		ae := applyPulledMutationTx(tx, m)
		if ae == nil {
			_, _ = tx.Exec(`DELETE FROM sync_apply_deferred WHERE sync_id=?`, it.sid)
			_ = tx.Commit()
			continue
		}
		if errors.Is(ae, ErrFKMissing) {
			_ = tx.Rollback()
			tx2, _ := s.db.Begin()
			var cur int
			_ = tx2.QueryRow(`SELECT attempts FROM sync_apply_deferred WHERE sync_id=?`, it.sid).Scan(&cur)
			if cur >= 5 {
				dID := deadLetterID(it.proj, it.key, []byte(it.pl))
				if it.ent == "memory_relation" || it.ent == "relation" {
					dID = relationApplyFailureSyncID(it.proj, it.key, []byte(it.pl))
				}
				if dID == it.sid {
					_, _ = tx2.Exec(`UPDATE sync_apply_deferred SET attempts=6, next_attempt_at=? WHERE sync_id=?`, "", it.sid)
				} else {
					_, err = tx2.Exec(`UPDATE sync_apply_deferred SET attempts=6, sync_id=?, next_attempt_at=?, payload=? WHERE sync_id=?`, dID, "", it.pl, it.sid)
					if err != nil && strings.Contains(err.Error(), "UNIQUE") {
						_, _ = tx2.Exec(`DELETE FROM sync_apply_deferred WHERE sync_id=?`, it.sid)
					}
				}
			} else {
				nxt := time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339)
				_, _ = tx2.Exec(`UPDATE sync_apply_deferred SET attempts=?, next_attempt_at=? WHERE sync_id=?`, cur+1, nxt, it.sid)
			}
			_ = tx2.Commit()
			continue
		}
		_ = tx.Rollback()
	}
	return nil
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
