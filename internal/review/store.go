package review

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// Content-Addressed Event Store
// ---------------------------------------------------------------------------
//
// Each review lineage is stored under:
//
//	.git/biggz/review-transactions/<lineage>/
//
// Inside the lineage directory:
//   - HEAD            — contains the SHA-256 hex of the latest event
//   - <sha256-hex>    — event files named by their SHA-256 content hash
//   - .lock           — file-based lock (see lock.go)
//
// Event integrity:
//   - Each file is named by SHA-256(file content)
//   - Each event carries PrevRevision linking to the previous event's hash
//   - The chain is validated by walking from HEAD back to genesis
//   - publishNoReplace: if a file already exists, same content is OK;
//     different content is an error (hash collision).

const recordSchemaVersion = "biggz-ai.review-record/v1"

// Burn semantics: last-event closure — every terminal capture burns the
// exact active lineage under lock+lease, deletes 3 owned paths
// (authority + effect-markers/v1 + incidents), verifies absence, and
// retires delivery without receipt/tombstone/mirror. Reuse→not-found.
// Compact receipts retired; no compact receipt file is created or consumed.
const (
	BurnOperation        = "burn_review"
	BurnEventSchema      = "biggz-ai.review-burn-event/v1"
	BurnedMarkerFile     = "burned.json"
	BurnedReceiptDirName = "burned"
)

// Record is a single event in the content-addressed review store.
// The revision IS the file name — it is not stored in the Record itself.
type Record struct {
	Schema       string          `json:"schema"`
	PrevRevision string          `json:"prevRevision"`
	Operation    string          `json:"operation"`
	Role         string          `json:"role"`
	Actor        string          `json:"actor"`
	Timestamp    string          `json:"timestamp"`
	Payload      json.RawMessage `json:"payload"`
}

// ValidatedChain holds the result of loading and validating an event chain.
type ValidatedChain struct {
	LineageID   string   `json:"lineage_id"`
	Records     []Record `json:"records"`
	HeadHash    string   `json:"head_hash"`
	GenesisHash string   `json:"genesis_hash"`
	Valid       bool     `json:"valid"`
	Count       int      `json:"count"`
}

// IntegrityVerdict describes the result of a chain integrity check.
type IntegrityVerdict struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason"`
}

// Store is a content-addressed event store for a single review lineage.
type Store struct {
	Dir       string
	LineageID string
}

func Open(repo, lineageID string) (*Store, error) {
	commonDir, err := resolveGitCommonDir(repo)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	dir := filepath.Join(commonDir, "biggz", "review-transactions", lineageID)
	return &Store{Dir: dir, LineageID: lineageID}, nil
}

func (s *Store) eventsDir() string { return filepath.Join(s.Dir, "v1", "events") }

// EventsDir returns the canonical v1 events directory for this store.
// The canonical path is <store.Dir>/v1/events/<sha256>; legacy flat
// files at <store.Dir>/<sha256> remain readable via dual-read fallback.
func (s *Store) EventsDir() string { return s.eventsDir() }

// EventPath returns the canonical path for an event revision, with
// dual-read fallback: if the v1 path does not exist but a legacy flat
// file does, the legacy path is returned. This preserves migration
// compatibility for tools that stat the file directly.
func (s *Store) EventPath(revision string) string {
	canonical := filepath.Join(s.eventsDir(), revision)
	if _, err := os.Stat(canonical); err == nil {
		return canonical
	}
	legacy := filepath.Join(s.Dir, revision)
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return canonical
}

// OpenWithDir creates a store with an explicit directory path.
// This is primarily useful for testing with t.TempDir().
func OpenWithDir(dir, lineageID string) *Store {
	return &Store{Dir: dir, LineageID: lineageID}
}

// Append persists a new event record to the store. The record is written
// atomically: serialized to JSON, named by SHA-256(content), and the
// HEAD file is updated to point to the new revision.
//
// prevRevision should be the hash of the previous event, or empty for
// the genesis event.
//
// If the file already exists:
//   - Same content → no-op (idempotent, HEAD still updated).
//   - Different content → error (hash collision).
//
// The store directory is created on the first Append call.
func (s *Store) Append(prevRevision string, rec Record) (revision string, err error) {
	err = WithFileLock(s.Dir, func() error {
		var appendErr error
		revision, appendErr = s.appendLocked(prevRevision, rec)
		return appendErr
	})
	return revision, err
}

// appendLocked appends an event while the caller already holds the lineage
// file lock. It is the body of Append without lock acquisition, so callers
// that need multiple store operations to be atomic can run them under one
// WithFileLock without deadlocking.
func (s *Store) appendLocked(prevRevision string, rec Record) (revision string, err error) {
	rec.Schema = recordSchemaVersion
	rec.PrevRevision = prevRevision
	data, err := json.Marshal(rec)
	if err != nil {
		return "", fmt.Errorf("marshal record: %w", err)
	}
	revision = sha256Hex(data)
	path := filepath.Join(s.eventsDir(), revision)
	if err := os.MkdirAll(s.eventsDir(), 0755); err != nil {
		return "", fmt.Errorf("create store dir: %w", err)
	}
	if existing, err := os.ReadFile(path); err == nil {
		if !bytes.Equal(existing, data) {
			return "", fmt.Errorf("hash collision: %s exists with different content", revision)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat: %w", err)
	} else if existing, err := os.ReadFile(filepath.Join(s.Dir, revision)); err == nil {
		if !bytes.Equal(existing, data) {
			return "", fmt.Errorf("hash collision: %s exists with different content", revision)
		}
		_ = publishImmutable(path, data)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat legacy: %w", err)
	} else {
		if err := publishImmutable(path, data); err != nil {
			return "", err
		}
	}
	if err := atomicWrite(filepath.Join(s.Dir, "HEAD"), []byte(revision+"\n")); err != nil {
		return "", fmt.Errorf("write HEAD: %w", err)
	}
	return revision, nil
}

// LoadChain reads the event chain from HEAD backwards to genesis,
// returning all records in chronological order (genesis first).
func (s *Store) LoadChain() (ValidatedChain, error) {
	var vc ValidatedChain
	vc.LineageID = s.LineageID

	head, err := readHEAD(s.Dir)
	if err != nil {
		return vc, fmt.Errorf("load chain: %w", err)
	}
	if head == "" {
		vc.Valid = true // empty chain is valid
		return vc, nil
	}
	vc.HeadHash = head

	// Walk backwards: head → prev → prev → ... → genesis.
	records := make([]Record, 0)
	hashes := make([]string, 0)
	visited := make(map[string]bool)
	for hash := head; hash != ""; {
		if visited[hash] {
			return vc, fmt.Errorf("load chain: cycle detected at %s", hash)
		}
		visited[hash] = true
		hashes = append(hashes, hash)

		rec, err := readRecord(s.Dir, hash)
		if err != nil {
			return vc, fmt.Errorf("load chain: read %s: %w", hash, err)
		}
		records = append(records, rec)
		hash = rec.PrevRevision
	}

	// Reverse to chronological order (genesis first).
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}
	// Reverse hashes too.
	for i, j := 0, len(hashes)-1; i < j; i, j = i+1, j-1 {
		hashes[i], hashes[j] = hashes[j], hashes[i]
	}

	vc.Records = records
	vc.Count = len(records)
	if len(records) > 0 {
		vc.GenesisHash = hashes[0]
	}
	vc.Valid = true

	return vc, nil
}

// Validate performs a full chain integrity check:
//   - Every file's SHA-256 matches its name
//   - Every event's PrevRevision links correctly
//   - The HEAD file points to the last event
func (s *Store) Validate() IntegrityVerdict {
	eventFiles := make(map[string]bool)
	if entries, err := os.ReadDir(s.eventsDir()); err == nil {
		for _, e := range entries {
			if e.IsDir() || strings.HasSuffix(e.Name(), ".tmp") {
				continue
			}
			if len(e.Name()) == 64 {
				eventFiles[e.Name()] = true
			}
		}
	} else if !os.IsNotExist(err) {
		return IntegrityVerdict{Valid: false, Reason: fmt.Sprintf("read dir: %v", err)}
	}
	if entries, err := os.ReadDir(s.Dir); err == nil {
		for _, e := range entries {
			if e.IsDir() || e.Name() == "HEAD" || e.Name() == ".lock" || strings.HasSuffix(e.Name(), ".tmp") || e.Name() == "v1" || e.Name() == BurnedMarkerFile {
				continue
			}
			if len(e.Name()) == 64 {
				eventFiles[e.Name()] = true
			}
		}
	} else if !os.IsNotExist(err) {
		return IntegrityVerdict{Valid: false, Reason: fmt.Sprintf("read dir: %v", err)}
	}
	if len(eventFiles) == 0 {
		return IntegrityVerdict{Valid: true, Reason: "empty store — no event files"}
	}
	for name := range eventFiles {
		var data []byte
		var err error
		if d, e := os.ReadFile(filepath.Join(s.eventsDir(), name)); e == nil {
			data = d
		} else if d, e := os.ReadFile(filepath.Join(s.Dir, name)); e == nil {
			data = d
		} else {
			err = e
		}
		if err != nil {
			return IntegrityVerdict{Valid: false, Reason: fmt.Sprintf("read %s: %v", name, err)}
		}
		if err != nil {
			return IntegrityVerdict{Valid: false, Reason: fmt.Sprintf("read %s: %v", name, err)}
		}
		if sha256Hex(data) != name {
			return IntegrityVerdict{Valid: false, Reason: fmt.Sprintf("hash mismatch for file %s", name)}
		}

		var rec Record
		if err := json.Unmarshal(data, &rec); err != nil {
			return IntegrityVerdict{Valid: false, Reason: fmt.Sprintf("parse %s: %v", name, err)}
		}

		if rec.PrevRevision != "" && !eventFiles[rec.PrevRevision] {
			return IntegrityVerdict{Valid: false, Reason: fmt.Sprintf("broken link: %s → prev %s not found", name, rec.PrevRevision)}
		}
	}

	// Verify HEAD points to a valid event file.
	head, err := readHEAD(s.Dir)
	if err == nil && head != "" && !eventFiles[head] {
		return IntegrityVerdict{Valid: false, Reason: fmt.Sprintf("HEAD %s not found in event files", head)}
	}

	return IntegrityVerdict{Valid: true, Reason: "chain integrity preserved"}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// IsBurned reports whether the lineage is burned (ephemeral receipt
// already consumed). A lineage is burned if a burned.json marker exists
// or any record carries BurnOperation.
func (s *Store) IsBurned() bool {
	if _, err := os.Stat(filepath.Join(s.Dir, BurnedMarkerFile)); err == nil {
		return true
	}
	chain, err := s.LoadChain()
	if err != nil {
		return false
	}
	return IsChainBurned(chain)
}

// IsChainBurned reports whether the validated chain contains a burn event.
func IsChainBurned(chain ValidatedChain) bool {
	for _, rec := range chain.Records {
		if rec.Operation == BurnOperation {
			return true
		}
	}
	return false
}

// readHEAD reads the HEAD file and returns the trimmed hash.
// Returns ("", nil) if the HEAD file does not exist (empty lineage).
func readHEAD(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "HEAD"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func readRecord(dir, hash string) (Record, error) {
	var rec Record
	if data, err := os.ReadFile(filepath.Join(dir, "v1", "events", hash)); err == nil {
		if err := json.Unmarshal(data, &rec); err == nil {
			return rec, nil
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, hash))
	if err != nil {
		return rec, err
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		return rec, err
	}
	return rec, nil
}

func resolveGitCommonDir(repo string) (string, error) {
	args := []string{"rev-parse", "--git-common-dir"}
	if repo != "" {
		args = append([]string{"-C", repo}, args...)
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return resolveGitDir(repo)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return resolveGitDir(repo)
	}
	if !filepath.IsAbs(dir) {
		base := repo
		if base == "" {
			base, _ = os.Getwd()
		}
		dir = filepath.Join(base, dir)
	}
	return filepath.Clean(dir), nil
}

func publishImmutable(path string, payload []byte) error {
	if existing, err := os.ReadFile(path); err == nil {
		if bytes.Equal(existing, payload) {
			return nil
		}
		return fmt.Errorf("publish immutable: %s exists with different content", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(payload); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	if d, err := os.Open(filepath.Dir(path)); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}

func atomicWrite(path string, payload []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(payload); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	if d, err := os.Open(filepath.Dir(path)); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}

// sha256Hex returns the lowercase hex SHA-256 hash of data (content-address).
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// StoreChainDomain is the domain for chain identity binding with length prefix.
const StoreChainDomain = "biggz-ai.review-store-chain/v1"

// chainIdentityHash binds chain fields with domain + length-prefix (gentle parity).
func chainIdentityHash(fields ...[]byte) string {
	payload := writeLengthPrefixed(fields...)
	return domainHash(StoreChainDomain, payload)
}

// resolveGitDir runs `git rev-parse --git-dir` to find the git directory
// for the given repository path. If repo is empty, uses the current
// working directory.
func resolveGitDir(repo string) (string, error) {
	args := []string{"rev-parse", "--git-dir"}
	if repo != "" {
		args = append([]string{"-C", repo}, args...)
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	gitDir := strings.TrimSpace(string(out))
	if gitDir == "" {
		return "", fmt.Errorf("empty git dir from rev-parse")
	}

	// Resolve relative paths.
	if !filepath.IsAbs(gitDir) {
		base := repo
		if base == "" {
			base, err = os.Getwd()
			if err != nil {
				return "", fmt.Errorf("getwd: %w", err)
			}
		}
		gitDir = filepath.Join(base, gitDir)
	}

	return filepath.Clean(gitDir), nil
}

// sha256HexBytes returns the SHA-256 hex of a string content.
func sha256HexString(s string) string {
	return sha256Hex([]byte(s))
}
