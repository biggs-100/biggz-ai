// Package bigmem — local chunk sync for git-friendly memory sharing.
//
// BigMem implements Engram-compatible local sync: chunks of sessions,
// observations, and prompts are exported as gzipped JSONL into .bigmem/,
// tracked by a small append-only manifest.json. This is the non-cloud
// file-based sync that Engram had before cloud sync existed.
//
// .bigmem/
//
//	manifest.json         ← append-only index (git-diffable)
//	chunks/
//	  a3f8c1d2.jsonl.gz   ← content-addressed chunk (gzipped JSON)
//	  b7d2e4f1.jsonl.gz
//	  ...
package bigmem

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SyncDBPath returns the canonical BigMem DB path for the given root using the
// unified ResolveDBPath helper (ghost WAL handling, checkpoint, VACUUM INTO,
// max(updated_at) merge). Exposed for sync diagnostics and to ensure sync.go
// shares the same path resolver as Store.Open and engram_status.go.
func SyncDBPath(rootDir string) (string, error) { return ResolveDBPath(rootDir) }

// ─── Manifest ────────────────────────────────────────────────────────────────

// SyncManifest is the append-only index of all chunks for a project.
type SyncManifest struct {
	Version int          `json:"version"`
	Chunks  []ChunkEntry `json:"chunks"`
}

// ChunkEntry is a single entry in the manifest describing one chunk.
type ChunkEntry struct {
	ID        string `json:"id"`         // first 8 hex chars of SHA-256
	CreatedBy string `json:"created_by"` // username or hostname
	CreatedAt string `json:"created_at"` // RFC3339
	Sessions  int    `json:"sessions"`
	Memories  int    `json:"memories"`
	Prompts   int    `json:"prompts"`
}

// ─── Chunk Data ──────────────────────────────────────────────────────────────

// ChunkData is the payload of a single chunk file.
type ChunkData struct {
	Sessions     []Session      `json:"sessions"`
	Observations []Observation  `json:"observations"`
	Prompts      []SavedPrompt  `json:"prompts"`
	Mutations    []SyncMutation `json:"mutations,omitempty"`
}

// SyncMutation tracks a single entity change for incremental sync.
type SyncMutation struct {
	Seq         int64  `json:"seq,omitempty"`
	Entity      string `json:"entity"`     // "session" | "observation" | "prompt"
	EntityKey   string `json:"entity_key"` // the entity's ID
	Op          string `json:"op"`         // "upsert" | "delete"
	Project     string `json:"project,omitempty"`
	Payload     []byte `json:"payload,omitempty"`
	Source      string `json:"source,omitempty"`
	Disposition string `json:"disposition,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// ─── Transport interface ─────────────────────────────────────────────────────

// SyncTransport abstracts reading/writing manifest and chunk files.
type SyncTransport interface {
	ReadManifest() (*SyncManifest, error)
	WriteManifest(m *SyncManifest) error
	WriteChunk(chunkID string, data []byte, entry ChunkEntry) error
	ReadChunk(chunkID string) ([]byte, error)
	SyncDir() string
}

// FileTransport implements SyncTransport on the local filesystem.
type FileTransport struct {
	dir string // .bigmem/ directory
}

// NewFileTransport creates a transport for the given .bigmem/ directory.
func NewFileTransport(syncDir string) *FileTransport {
	return &FileTransport{dir: syncDir}
}

func (t *FileTransport) SyncDir() string { return t.dir }

func (t *FileTransport) ReadManifest() (*SyncManifest, error) {
	path := filepath.Join(t.dir, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SyncManifest{Version: 1}, nil
		}
		return nil, err
	}
	var m SyncManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (t *FileTransport) WriteManifest(m *SyncManifest) error {
	if err := os.MkdirAll(t.dir, 0755); err != nil {
		return err
	}
	// Sort chunks by CreatedAt so manifest is deterministic
	sort.Slice(m.Chunks, func(i, j int) bool {
		return m.Chunks[i].CreatedAt < m.Chunks[j].CreatedAt
	})
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(t.dir, "manifest.json")
	// Write atomically to avoid corruption
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (t *FileTransport) WriteChunk(chunkID string, data []byte, entry ChunkEntry) error {
	chunksDir := filepath.Join(t.dir, "chunks")
	if err := os.MkdirAll(chunksDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(chunksDir, chunkID+".jsonl.gz")
	return os.WriteFile(path, data, 0644)
}

func (t *FileTransport) ReadChunk(chunkID string) ([]byte, error) {
	path := filepath.Join(t.dir, "chunks", chunkID+".jsonl.gz")
	return os.ReadFile(path)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// chunkID computes a content-addressed ID (first 8 hex chars of SHA-256).
func chunkID(payload []byte) string {
	h := sha256.Sum256(payload)
	return hex.EncodeToString(h[:])[:8]
}

// gzipData compresses data with gzip.
func gzipData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// gunzipData decompresses gzip data.
func gunzipData(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GunzipData is the exported alias of gunzipData for Engram import reuse.
func GunzipData(data []byte) ([]byte, error) {
	return gunzipData(data)
}

// getUsername returns the current OS username for chunk attribution.
func getUsername() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("USERNAME"); u != "" {
		return u
	}
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return "unknown"
}

// ─── Export ──────────────────────────────────────────────────────────────────

// ExportResult describes what was exported.
type ExportResult struct {
	ChunkID              string `json:"chunk_id,omitempty"`
	SessionsExported     int    `json:"sessions_exported"`
	ObservationsExported int    `json:"observations_exported"`
	PromptsExported      int    `json:"prompts_exported"`
	IsEmpty              bool   `json:"is_empty"`
}

// SyncExportTimeBased exports sessions, observations, and prompts created
// since the last chunk's timestamp into a new gzipped chunk file.
//
// Returns ExportResult with chunk_id and counts. If nothing new, IsEmpty=true.
func (s *Store) SyncExportTimeBased(project string, transport SyncTransport) (*ExportResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Read manifest and get last chunk timestamp
	manifest, err := transport.ReadManifest()
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	lastTime, err := s.GetLastChunkTime()
	if err != nil {
		return nil, err
	}

	cutoff := lastTime
	if len(manifest.Chunks) > 0 {
		// Use the most recent chunk's timestamp as cutoff
		latest := manifest.Chunks[len(manifest.Chunks)-1]
		if latest.CreatedAt > cutoff {
			cutoff = latest.CreatedAt
		}
	}

	// Build chunk data
	chunk := ChunkData{}

	// Export sessions
	sessionQ := "SELECT id, start_time, end_time, summary, project, directory FROM sessions"
	var sessionArgs []any
	if project != "" {
		sessionQ += " WHERE project = ?"
		sessionArgs = append(sessionArgs, project)
	}
	if cutoff != "" {
		if project != "" {
			sessionQ += " AND start_time > ?"
		} else {
			sessionQ += " WHERE start_time > ?"
		}
		sessionArgs = append(sessionArgs, cutoff)
	}
	sessionRows, err := s.db.Query(sessionQ, sessionArgs...)
	if err == nil {
		defer sessionRows.Close()
		for sessionRows.Next() {
			var sess Session
			var st, et, summary, dir sql.NullString
			if err := sessionRows.Scan(&sess.ID, &st, &et, &summary, &sess.Project, &dir); err != nil {
				continue
			}
			if st.Valid {
				sess.StartTime, _ = time.Parse(time.RFC3339, st.String)
			}
			if et.Valid {
				sess.EndTime, _ = time.Parse(time.RFC3339, et.String)
			}
			if summary.Valid {
				sess.Summary = summary.String
			}
			if dir.Valid {
				sess.Directory = dir.String
			}
			chunk.Sessions = append(chunk.Sessions, sess)
		}
	}

	// Export observations
	obsQ := `SELECT id, title, type, content, session_id, tool_name, topic_key, project, scope,
		normalized_hash, revision_count, duplicate_count, last_seen_at, review_after,
		pinned, created_at, updated_at, deleted_at FROM observations WHERE deleted_at IS NULL`
	var obsArgs []any
	if project != "" {
		obsQ += " AND project = ?"
		obsArgs = append(obsArgs, project)
	}
	if cutoff != "" {
		obsQ += " AND (created_at > ? OR updated_at > ?)"
		obsArgs = append(obsArgs, cutoff, cutoff)
	}
	obsRows, err := s.db.Query(obsQ, obsArgs...)
	if err == nil {
		defer obsRows.Close()
		for obsRows.Next() {
			obs := Observation{}
			var ca, ua string
			var ra, da, lsa sql.NullString
			var pinnedInt int
			if err := obsRows.Scan(&obs.ID, &obs.Title, &obs.Type, &obs.Content,
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
			chunk.Observations = append(chunk.Observations, obs)
		}
	}

	// Export prompts
	promptQ := "SELECT id, content, session_id, created_at FROM prompts"
	var promptArgs []any
	if project != "" {
		promptQ += " WHERE session_id IN (SELECT id FROM sessions WHERE project = ?)"
		promptArgs = append(promptArgs, project)
	}
	if cutoff != "" {
		if project != "" {
			promptQ += " AND created_at > ?"
		} else {
			promptQ += " WHERE created_at > ?"
		}
		promptArgs = append(promptArgs, cutoff)
	}
	promptRows, err := s.db.Query(promptQ, promptArgs...)
	if err == nil {
		defer promptRows.Close()
		for promptRows.Next() {
			var p SavedPrompt
			var ca string
			if err := promptRows.Scan(&p.ID, &p.Content, &p.SessionID, &ca); err != nil {
				continue
			}
			p.CreatedAt, _ = time.Parse(time.RFC3339, ca)
			chunk.Prompts = append(chunk.Prompts, p)
		}
	}

	// Check if anything is new
	if len(chunk.Sessions) == 0 && len(chunk.Observations) == 0 && len(chunk.Prompts) == 0 {
		return &ExportResult{IsEmpty: true}, nil
	}

	// Serialize, hash, gzip
	payload, err := json.Marshal(chunk)
	if err != nil {
		return nil, fmt.Errorf("marshal chunk: %w", err)
	}

	cID := chunkID(payload)
	gzData, err := gzipData(payload)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}

	// Check if this chunk ID already exists (content dedup)
	known, err := s.GetSyncChunks(LocalChunkTargetKey)
	if err == nil && known[cID] {
		return &ExportResult{IsEmpty: true}, nil
	}

	// Write chunk
	now := time.Now().UTC().Format(time.RFC3339)
	entry := ChunkEntry{
		ID:        cID,
		CreatedBy: getUsername(),
		CreatedAt: now,
		Sessions:  len(chunk.Sessions),
		Memories:  len(chunk.Observations),
		Prompts:   len(chunk.Prompts),
	}

	if err := transport.WriteChunk(cID, gzData, entry); err != nil {
		return nil, fmt.Errorf("write chunk: %w", err)
	}

	// Append to manifest
	manifest.Chunks = append(manifest.Chunks, entry)
	if err := transport.WriteManifest(manifest); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}

	// Record locally
	s.mu.RUnlock() // release RLock before taking Lock
	s.mu.Lock()
	s.db.Exec("INSERT OR IGNORE INTO sync_chunks (target_key, chunk_id) VALUES (?, ?)", LocalChunkTargetKey, cID)
	s.mu.Unlock()
	s.mu.RLock()

	return &ExportResult{
		ChunkID:              cID,
		SessionsExported:     len(chunk.Sessions),
		ObservationsExported: len(chunk.Observations),
		PromptsExported:      len(chunk.Prompts),
	}, nil
}

// ─── Import ──────────────────────────────────────────────────────────────────

// ImportResult describes what was imported.
type ImportResult struct {
	ChunksImported       int `json:"chunks_imported"`
	ChunksSkipped        int `json:"chunks_skipped"`
	SessionsImported     int `json:"sessions_imported"`
	ObservationsImported int `json:"observations_imported"`
	PromptsImported      int `json:"prompts_imported"`
}

// SyncImportDependencySafe imports all pending chunks from the transport,
// handling cross-chunk dependencies with multi-pass retry.
//
// If an observation references a session that hasn't been imported yet,
// that chunk is deferred to the next pass. If progress stalls, stub
// sessions are auto-created for orphaned observations.
func (s *Store) SyncImportDependencySafe(transport SyncTransport) (*ImportResult, error) {
	manifest, err := transport.ReadManifest()
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	if len(manifest.Chunks) == 0 {
		return &ImportResult{}, nil
	}

	known, err := s.GetSyncChunks(LocalChunkTargetKey)
	if err != nil {
		known = make(map[string]bool)
	}

	result := &ImportResult{}

	// Build a list of pending chunks (not yet imported)
	type pendingChunk struct {
		id    string
		entry ChunkEntry
	}
	var pending []pendingChunk
	for _, entry := range manifest.Chunks {
		if !known[entry.ID] {
			pending = append(pending, pendingChunk{id: entry.ID, entry: entry})
		} else {
			result.ChunksSkipped++
		}
	}

	if len(pending) == 0 {
		return result, nil
	}

	// Phase 1: collect all session IDs from ALL pending chunks so we can
	// detect orphaned observations during import
	allSessionIDs := make(map[string]bool)
	for _, p := range pending {
		data, err := transport.ReadChunk(p.id)
		if err != nil {
			continue
		}
		raw, err := gunzipData(data)
		if err != nil {
			continue
		}
		var cd ChunkData
		if err := json.Unmarshal(raw, &cd); err != nil {
			continue
		}
		for _, s := range cd.Sessions {
			allSessionIDs[s.ID] = true
		}
	}

	// Multi-pass import
	var stalled bool
	remaining := pending

	for len(remaining) > 0 && !stalled {
		stalled = true
		var nextPending []pendingChunk

		for _, p := range remaining {
			data, err := transport.ReadChunk(p.id)
			if err != nil {
				nextPending = append(nextPending, p)
				continue
			}

			raw, err := gunzipData(data)
			if err != nil {
				nextPending = append(nextPending, p)
				continue
			}

			var cd ChunkData
			if err := json.Unmarshal(raw, &cd); err != nil {
				nextPending = append(nextPending, p)
				continue
			}

			// Try to import this chunk
			imported, err := s.importChunkData(&cd, allSessionIDs)
			if err != nil {
				nextPending = append(nextPending, p)
				continue
			}

			if imported {
				result.ChunksImported++
				result.SessionsImported += len(cd.Sessions)
				result.ObservationsImported += len(cd.Observations)
				result.PromptsImported += len(cd.Prompts)
				s.RecordSyncChunk(LocalChunkTargetKey, p.id)
				stalled = false
			} else {
				nextPending = append(nextPending, p)
			}
		}

		remaining = nextPending
	}

	// Phase 2: handle leftovers — auto-recover orphaned observations
	// by creating stub sessions
	for _, p := range remaining {
		data, err := transport.ReadChunk(p.id)
		if err != nil {
			continue
		}
		raw, err := gunzipData(data)
		if err != nil {
			continue
		}
		var cd ChunkData
		if err := json.Unmarshal(raw, &cd); err != nil {
			continue
		}

		// Check which sessions are missing and create stubs
		for _, obs := range cd.Observations {
			if obs.SessionID == "" {
				continue
			}
			var exists int
			s.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", obs.SessionID).Scan(&exists)
			if exists == 0 {
				// Create stub session
				s.mu.Lock()
				s.db.Exec("INSERT OR IGNORE INTO sessions (id, start_time, project, directory) VALUES (?, ?, ?, ?)",
					obs.SessionID, "1970-01-01T00:00:00Z", obs.Project, "(recovered-missing-session)")
				s.mu.Unlock()
			}
		}

		// Retry import now that stubs exist
		imported, err := s.importChunkData(&cd, allSessionIDs)
		if err == nil && imported {
			result.ChunksImported++
			result.SessionsImported += len(cd.Sessions)
			result.ObservationsImported += len(cd.Observations)
			result.PromptsImported += len(cd.Prompts)
			s.RecordSyncChunk(LocalChunkTargetKey, p.id)
		}
	}

	return result, nil
}

// importChunkData imports a single chunk's data into the store.
// Returns true if at least one entity was imported.
func (s *Store) importChunkData(cd *ChunkData, allSessionIDs map[string]bool) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	anyImported := false

	// Import sessions
	for _, sess := range cd.Sessions {
		var exists int
		s.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", sess.ID).Scan(&exists)
		if exists > 0 {
			continue
		}
		_, err := s.db.Exec(
			`INSERT INTO sessions (id, start_time, end_time, summary, project, directory)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			sess.ID,
			sess.StartTime.Format(time.RFC3339),
			sess.EndTime.Format(time.RFC3339),
			sess.Summary,
			sess.Project,
			sess.Directory,
		)
		if err == nil {
			anyImported = true
		}
	}

	// Import observations — check if referenced sessions exist (or will exist)
	for _, obs := range cd.Observations {
		var exists int
		s.db.QueryRow("SELECT COUNT(*) FROM observations WHERE id = ?", obs.ID).Scan(&exists)
		if exists > 0 {
			continue
		}

		// Check session dependency
		if obs.SessionID != "" {
			var sessExists int
			s.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", obs.SessionID).Scan(&sessExists)
			if sessExists == 0 && !allSessionIDs[obs.SessionID] {
				// Session doesn't exist locally AND isn't in any chunk — skip
				continue
			}
			if sessExists == 0 {
				// Session is in another chunk but not yet imported — defer
				return anyImported, fmt.Errorf("session %s not yet imported", obs.SessionID)
			}
		}

		reviewAfterStr := ""
		if obs.ReviewAfter != nil {
			reviewAfterStr = *obs.ReviewAfter
		}
		lastSeenStr := ""
		if obs.LastSeenAt != nil {
			lastSeenStr = *obs.LastSeenAt
		}
		deletedAtStr := ""
		if obs.DeletedAt != nil {
			deletedAtStr = *obs.DeletedAt
		}

		_, err := s.db.Exec(
			`INSERT INTO observations (id, title, type, content, session_id, tool_name,
			 topic_key, project, scope, normalized_hash, revision_count, duplicate_count,
			 last_seen_at, review_after, pinned, created_at, updated_at, deleted_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO NOTHING`,
			obs.ID, obs.Title, obs.Type, obs.Content, obs.SessionID, obs.ToolName,
			obs.TopicKey, obs.Project, obs.Scope, obs.NormalizedHash,
			obs.RevisionCount, obs.DuplicateCount,
			lastSeenStr, reviewAfterStr, boolToInt(obs.Pinned),
			obs.CreatedAt.Format(time.RFC3339),
			obs.UpdatedAt.Format(time.RFC3339),
			deletedAtStr,
		)
		if err == nil {
			anyImported = true
		}
	}

	// Import prompts
	for _, p := range cd.Prompts {
		var exists int
		s.db.QueryRow("SELECT COUNT(*) FROM prompts WHERE id = ?", p.ID).Scan(&exists)
		if exists > 0 {
			continue
		}

		// Check session dependency
		if p.SessionID != "" {
			var sessExists int
			s.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", p.SessionID).Scan(&sessExists)
			if sessExists == 0 && !allSessionIDs[p.SessionID] {
				continue
			}
			if sessExists == 0 {
				return anyImported, fmt.Errorf("session %s not yet imported", p.SessionID)
			}
		}

		_, err := s.db.Exec(
			`INSERT INTO prompts (id, content, session_id, created_at) VALUES (?, ?, ?, ?)
			 ON CONFLICT(id) DO NOTHING`,
			p.ID, p.Content, p.SessionID, p.CreatedAt.Format(time.RFC3339),
		)
		if err == nil {
			anyImported = true
		}
	}

	return anyImported, nil
}

// ─── Sync Status ─────────────────────────────────────────────────────────────

// SyncLocalStatus returns the number of local chunks and observations
// pending import vs already imported.
type SyncLocalStatus struct {
	TransportDir   string `json:"transport_dir"`
	ManifestChunks int    `json:"manifest_chunks"`
	ImportedChunks int    `json:"imported_chunks"`
	PendingImport  int    `json:"pending_import"`
}

// SyncLocalStatus returns sync status for a given transport.
func (s *Store) SyncLocalStatus(transport SyncTransport) (*SyncLocalStatus, error) {
	manifest, err := transport.ReadManifest()
	if err != nil {
		return nil, err
	}
	known, err := s.GetSyncChunks(LocalChunkTargetKey)
	if err != nil {
		known = make(map[string]bool)
	}

	status := &SyncLocalStatus{
		TransportDir:   transport.SyncDir(),
		ManifestChunks: len(manifest.Chunks),
	}

	for _, entry := range manifest.Chunks {
		if known[entry.ID] {
			status.ImportedChunks++
		}
	}
	status.PendingImport = status.ManifestChunks - status.ImportedChunks
	return status, nil
}

// ─── Git-friendly scaffolding ────────────────────────────────────────────────

// EnsureGitIgnore creates a .gitignore in the sync directory that allows
// .ndjson and .jsonl.gz files through even if .bigmem/ is gitignored at a
// higher level.
func EnsureGitIgnore(syncDir string) error {
	path := filepath.Join(syncDir, ".gitignore")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	content := []byte("# Sync chunks — safe to commit\n*\n!.gitignore\n*.ndjson\n*.jsonl.gz\n!manifest.json\n")
	return os.WriteFile(path, content, 0644)
}
