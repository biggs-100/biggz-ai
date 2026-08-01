// Clone-scoped CAS runtime ledger store (Phase C2 review-workflow parity).
//
// The SDD attempt ledger moves out of the home directory into the git common
// directory, mirroring gentle-ai's clone-scoped sdd-runtime ledger
// (internal/sddstatus/runtime_ledger.go) adapted to biggz's snapshot-per-
// revision records:
//
//	<git-common-dir>/biggz/sdd-runtime/v1/<change>/
//	  HEAD                     — <revision>\n (atomically replaced)
//	  LOCK                     — exclusive lock file (O_EXCL create/release,
//	                             reusing the review store's file-lock helper)
//	  record-<revision>.json   — content-addressed snapshot records
//
// Every mutation appends ONE immutable record (a full store snapshot) and
// advances HEAD. Replay reads the HEAD record and verifies its content
// address, so a torn or tampered record fails closed. CAS semantics are
// preserved: a mutation compares its expected revision against the current
// HEAD and refuses on mismatch (concurrent writer).
//
// Content addressing is canonical-form: the record's revision is the SHA-256
// of the snapshot serialized with the CAS metadata excluded — the top-level
// "revision" field AND every embedded request-receipt outcome "revision"
// field. The embedded receipt revision is self-referential (it names the
// record that carries it), so it cannot participate in its own content
// address; excluding it keeps the record content-addressed while still
// persisting the exact first-application outcome (C1 convergent-replay
// guarantee). Tampering with anything else changes the canonical form and
// fails verification.
//
// Migration: on first access, when the legacy home-dir single-file ledger
// ~/.biggz/sdd-runtime/v1/<change>.json exists and the clone-scoped store is
// empty, the legacy content is imported as the initial record. The migration
// is reported once (Migrated flags on the results) and the legacy file is
// kept untouched — never deleted.
package sddattempt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/internal/review"
)

// runtimeStoreContainer is the directory name under the git common dir that
// holds all runtime ledgers ("biggz/sdd-runtime/v1").
const runtimeStoreContainer = "biggz"

// Store locates the clone-scoped runtime ledger for one SDD change.
type Store struct {
	Dir        string // <git-common-dir>/biggz/sdd-runtime/v1/<change>
	Change     string
	LegacyPath string // legacy home-dir single-file location (migration source)
}

// validateChangeName rejects change names that would escape the ledger
// directory (path separators, "." and "..", control bytes).
func validateChangeName(changeName string) error {
	if changeName == "" {
		return errors.New("SDD change name is required")
	}
	if changeName == "." || changeName == ".." ||
		strings.ContainsAny(changeName, `/\`) || strings.IndexByte(changeName, 0) >= 0 {
		return fmt.Errorf("invalid SDD change name %q", changeName)
	}
	return nil
}

// resolveStore locates the ledger store for a change. Outside a git
// repository the clone-scoped ledger cannot be placed and an error names
// the requirement. The storeRootOverride redirects both the new and the
// legacy location for tests.
func resolveStore(changeName, repoRoot string) (Store, error) {
	if err := validateChangeName(changeName); err != nil {
		return Store{}, err
	}
	if storeRootOverride != "" {
		return Store{
			Dir:        filepath.Join(storeRootOverride, changeName),
			Change:     changeName,
			LegacyPath: filepath.Join(storeRootOverride, changeName+".json"),
		}, nil
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	legacyPath := filepath.Join(home, ".biggz", RuntimeDir, RuntimeVersion, changeName+".json")

	args := []string{"rev-parse", "--git-common-dir"}
	if repoRoot != "" {
		args = append([]string{"-C", repoRoot}, args...)
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return Store{}, fmt.Errorf(
			"not a git repository: the SDD runtime ledger lives in the git common directory under biggz/sdd-runtime/v1 (the legacy home-dir store at %s stays untouched)", legacyPath)
	}
	commonDir := strings.TrimSpace(string(out))
	if commonDir == "" {
		return Store{}, errors.New("empty git common directory from rev-parse")
	}
	if !filepath.IsAbs(commonDir) {
		base := repoRoot
		if base == "" {
			base, _ = os.Getwd()
		}
		commonDir = filepath.Join(base, commonDir)
	}
	return Store{
		Dir:        filepath.Join(filepath.Clean(commonDir), runtimeStoreContainer, RuntimeDir, RuntimeVersion, changeName),
		Change:     changeName,
		LegacyPath: legacyPath,
	}, nil
}

// replay loads the current ledger state. It returns (nil, false, nil) when
// no ledger exists yet. When the store is empty and the legacy home-dir
// ledger exists, its content is imported as the initial record and
// migrated=true is returned. The caller must hold the store lock.
func (s Store) replay() (*RuntimeStore, bool, error) {
	head, err := readLedgerHead(s.Dir)
	if err != nil {
		return nil, false, err
	}
	if head != "" {
		store, err := s.loadRecord(head)
		if err != nil {
			return nil, false, err
		}
		return store, false, nil
	}

	legacy, err := os.ReadFile(s.LegacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read legacy runtime ledger %s: %w", s.LegacyPath, err)
	}
	// The legacy file carries its revision inside the JSON ("revision"
	// field), which the RuntimeStore struct now never serializes; read it
	// separately to verify the legacy ledger's self-consistency.
	var legacyRevision string
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(legacy, &raw); err != nil {
		return nil, false, fmt.Errorf("legacy runtime ledger %s is unreadable: %w", s.LegacyPath, err)
	}
	if field, ok := raw["revision"]; ok {
		if err := json.Unmarshal(field, &legacyRevision); err != nil {
			return nil, false, fmt.Errorf("legacy runtime ledger %s carries an unreadable revision: %w", s.LegacyPath, err)
		}
	} else {
		return nil, false, fmt.Errorf("legacy runtime ledger %s has no revision field; it cannot be verified — repair or remove it manually before migrating", s.LegacyPath)
	}
	var store RuntimeStore
	if err := json.Unmarshal(legacy, &store); err != nil {
		return nil, false, fmt.Errorf("legacy runtime ledger %s is unreadable: %w", s.LegacyPath, err)
	}
	if store.ChangeName != "" && store.ChangeName != s.Change {
		return nil, false, fmt.Errorf("legacy runtime ledger %s belongs to change %q, not %q", s.LegacyPath, store.ChangeName, s.Change)
	}
	// Fail closed: an integrity-broken legacy ledger must not be imported.
	if rev := computeRevision(&store); legacyRevision != rev {
		return nil, false, fmt.Errorf("legacy runtime ledger %s fails its own revision check (expected %s, got %s); repair or remove it manually before migrating", s.LegacyPath, rev, legacyRevision)
	}
	if err := s.commit(&store); err != nil {
		return nil, false, fmt.Errorf("migrate legacy runtime ledger: %w", err)
	}
	return &store, true, nil
}

// commit publishes a new immutable record for the given store state and
// advances HEAD. store.Revision is overwritten with the computed revision.
// The caller must hold the store lock.
func (s Store) commit(store *RuntimeStore) error {
	canonical := canonicalRecordPayload(store)
	revision := sha256Hex(canonical)
	payload := marshalSnapshot(store)
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return fmt.Errorf("mkdir ledger store: %w", err)
	}
	path := filepath.Join(s.Dir, recordFileName(revision))
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		// publish-no-replace: same canonical state is a no-op; different
		// canonical state under the same revision is a hash collision.
		existingCanonical := canonicalRecordPayload(parsedRecord(existing, s.Change))
		if !bytes.Equal(existingCanonical, canonical) {
			return fmt.Errorf("hash collision: %s exists with different content", path)
		}
	case os.IsNotExist(err):
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, payload, 0644); err != nil {
			return fmt.Errorf("write record temp: %w", err)
		}
		if err := os.Rename(tmp, path); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("rename record into place: %w", err)
		}
	default:
		return fmt.Errorf("stat record: %w", err)
	}
	if err := writeLedgerHead(s.Dir, revision); err != nil {
		return err
	}
	store.Revision = revision
	return nil
}

// recordRevision computes the content address the store state WILL have when
// committed, without writing anything. It is used to fill the idempotency
// receipt's outcome revision before commit: the canonical form clears every
// embedded outcome revision, so patching the embedded revision afterwards
// does not change the address.
func recordRevision(store *RuntimeStore) string {
	return sha256Hex(canonicalRecordPayload(store))
}

// setRequestOutcomeRevision patches the embedded outcome revision of one
// recorded request receipt. The canonical content address excludes this
// field, so the patch never invalidates a previously computed address.
func setRequestOutcomeRevision(store *RuntimeStore, requestID, revision string) {
	if store.Requests == nil {
		return
	}
	rec, ok := store.Requests[requestID]
	if !ok {
		return
	}
	var object map[string]any
	if err := json.Unmarshal(rec.Outcome, &object); err != nil {
		return
	}
	object["revision"] = revision
	cleaned, err := json.Marshal(object)
	if err != nil {
		return
	}
	rec.Outcome = cleaned
	store.Requests[requestID] = rec
}

// parsedRecord is a best-effort parse used only for collision comparison.
func parsedRecord(data []byte, change string) *RuntimeStore {
	var store RuntimeStore
	if err := json.Unmarshal(data, &store); err != nil || store.ChangeName != change {
		return &RuntimeStore{ChangeName: change}
	}
	return &store
}

// loadRecord loads and verifies the record at the given revision: the
// canonical form of the record must hash to the revision.
func (s Store) loadRecord(revision string) (*RuntimeStore, error) {
	path := filepath.Join(s.Dir, recordFileName(revision))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ledger record %s: %w", revision, err)
	}
	var store RuntimeStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parse ledger record %s: %w", path, err)
	}
	if store.ChangeName != s.Change {
		return nil, fmt.Errorf("ledger record %s belongs to change %q, not %q", path, store.ChangeName, s.Change)
	}
	if sha256Hex(canonicalRecordPayload(&store)) != revision {
		return nil, fmt.Errorf("ledger record %s does not match its content address", path)
	}
	store.Revision = revision
	return &store, nil
}

// marshalSnapshot serializes the store. The revision field is never
// serialized (json:"-"), so the record bytes are fully determined by the
// ledger state.
func marshalSnapshot(store *RuntimeStore) []byte {
	data, _ := json.Marshal(store)
	return append(data, '\n')
}

// canonicalRecordPayload serializes the store with the CAS metadata
// excluded: the top-level "revision" field (never serialized) and every
// embedded request receipt outcome "revision" field. This is the verified
// content address preimage.
func canonicalRecordPayload(store *RuntimeStore) []byte {
	requests := store.Requests
	if requests != nil {
		store.Requests = make(map[string]RuntimeRequestRecord, len(requests))
		for id, rec := range requests {
			rec.Outcome = clearOutcomeRevision(rec.Outcome)
			store.Requests[id] = rec
		}
	}
	data, _ := json.Marshal(store)
	store.Requests = requests
	return append(data, '\n')
}

// clearOutcomeRevision removes the "revision" key from a request receipt
// outcome JSON object, if it is an object.
func clearOutcomeRevision(outcome json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(outcome)) == 0 || bytes.TrimSpace(outcome)[0] != '{' {
		return outcome
	}
	var object map[string]any
	if err := json.Unmarshal(outcome, &object); err != nil {
		return outcome
	}
	delete(object, "revision")
	cleaned, err := json.Marshal(object)
	if err != nil {
		return outcome
	}
	return cleaned
}

// recordFileName returns the content-addressed record file name.
func recordFileName(revision string) string {
	return "record-" + revision + ".json"
}

// readLedgerHead reads the HEAD revision. A missing HEAD (no ledger yet)
// returns "".
func readLedgerHead(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "HEAD"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read ledger HEAD: %w", err)
	}
	revision := strings.TrimSpace(string(data))
	if !validLedgerRevision(revision) {
		return "", fmt.Errorf("invalid ledger HEAD revision %q", revision)
	}
	return revision, nil
}

// writeLedgerHead atomically replaces the HEAD file (temp + rename).
func writeLedgerHead(dir, revision string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "HEAD")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(revision+"\n"), 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// validLedgerRevision reports whether value is a 64-char lowercase hex hash.
func validLedgerRevision(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// sha256Hex returns the lowercase hex SHA-256 hash of data.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// withStoreLock runs f under the ledger's exclusive LOCK file, reusing the
// review store's file-lock helper with the LOCK name this contract publishes.
func (s Store) withStoreLock(f func() error) error {
	return review.WithNamedFileLock(s.Dir, "LOCK", f)
}
