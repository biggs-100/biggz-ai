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

// Burn semantics: ephemeral receipt — after successful finalize the receipt
// is burned (deleted) and a burn_review event marks the lineage burned.
// Burns prevent replay; gates become informational (non-deciding).
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

// Open creates or opens a store for the given lineage. The store directory
// is resolved relative to the repository's git directory.
//
// If repo is empty, Open auto-detects the git directory from the current
// working directory. The actual store directory is created on the first
// call to Append, not during Open.
func Open(repo, lineageID string) (*Store, error) {
	gitDir, err := resolveGitDir(repo)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	dir := filepath.Join(gitDir, "biggz", "review-transactions", lineageID)
	return &Store{Dir: dir, LineageID: lineageID}, nil
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
	path := filepath.Join(s.Dir, revision)

	// Create directory on first append (task 1.4).
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return "", fmt.Errorf("create store dir: %w", err)
	}

	// publishNoReplace: check if file already exists.
	existing, err := os.ReadFile(path)
	if err == nil {
		if !bytes.Equal(existing, data) {
			return "", fmt.Errorf("hash collision: %s exists with different content", revision)
		}
		// Same content — idempotent. HEAD is still updated below.
	} else if os.IsNotExist(err) {
		// Write to temp file and rename for atomicity.
		tmpPath := path + ".tmp"
		if err := os.WriteFile(tmpPath, data, 0644); err != nil {
			return "", fmt.Errorf("write temp: %w", err)
		}
		if err := os.Rename(tmpPath, path); err != nil {
			os.Remove(tmpPath) // best-effort cleanup
			return "", fmt.Errorf("rename: %w", err)
		}
	} else {
		return "", fmt.Errorf("stat: %w", err)
	}

	// Update HEAD.
	headPath := filepath.Join(s.Dir, "HEAD")
	if err := os.WriteFile(headPath, []byte(revision+"\n"), 0644); err != nil {
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
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return IntegrityVerdict{Valid: true, Reason: "empty store — no events"}
		}
		return IntegrityVerdict{Valid: false, Reason: fmt.Sprintf("read dir: %v", err)}
	}

	// Collect event file names (64-char hex strings).
	eventFiles := make(map[string]bool)
	for _, e := range entries {
		if e.IsDir() || e.Name() == "HEAD" || e.Name() == ".lock" ||
			strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		if len(e.Name()) == 64 {
			eventFiles[e.Name()] = true
		}
	}

	if len(eventFiles) == 0 {
		return IntegrityVerdict{Valid: true, Reason: "empty store — no event files"}
	}

	// Verify each file: SHA-256(content) == file name.
	for name := range eventFiles {
		data, err := os.ReadFile(filepath.Join(s.Dir, name))
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

// readRecord reads and unmarshals a record file by its hash name.
func readRecord(dir, hash string) (Record, error) {
	var rec Record
	data, err := os.ReadFile(filepath.Join(dir, hash))
	if err != nil {
		return rec, err
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		return rec, err
	}
	return rec, nil
}

// sha256Hex returns the lowercase hex SHA-256 hash of data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
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
