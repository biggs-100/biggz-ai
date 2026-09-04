# Delta for bigmem

## MODIFIED Requirements

### Requirement: Blob-miss visibility on read

The system MUST return an explicit missing-blob marker embedding the addr plus a log entry when `GetBlob` misses, and MUST NOT return the bare `blob:sha256:` addr silently. Reads MUST NOT mutate the DB.
(Previously: miss returned raw addr with `err==nil` guard and no signal.)

#### Scenario: Get miss returns marker

- GIVEN observation content is valid `blob:sha256:<hex>` with no file on disk
- WHEN `Get`/`GetCtx`/`Search`/`mem_get` resolves it
- THEN content MUST contain marker with embedded addr (e.g. `[missing-blob blob:sha256:<hex>]`) AND a log MUST record miss

#### Scenario: Hit unchanged

- GIVEN blob file exists for addr
- WHEN read resolves it
- THEN raw bytes MUST return with no marker

#### Scenario: Invalid addr rejected

- GIVEN malformed addr (non-hex, traversal)
- WHEN `GetBlob` validates
- THEN it MUST return `ErrInvalidAddr` visibly, never marker-as-content

### Requirement: PutBlob failure visibility on save

`Save` paths (MCP `mem_save`, CLI save, `session_guard` fallback, `Store.Save`) MUST surface `PutBlob` failure beyond stderr via wrapped error or explicit status, and MUST still persist raw inline so no bytes are lost pending `DoctorFixBlobs`.
(Previously: `if err == nil` guard with stderr-only line, failure invisible to caller.)

#### Scenario: Externalize failure surfaces

- GIVEN content triggers `ShouldExternalize` and `PutBlob` fails (e.g. empty HOME)
- WHEN save executes
- THEN caller MUST see wrapped error/log status AND row MUST persist raw inline

#### Scenario: Success stores addr

- GIVEN `PutBlob` succeeds
- WHEN save executes
- THEN content MUST store `blob:sha256:<hex>`, no error surfaced

## ADDED Requirements

### Requirement: Single BigMem blob reference doc

The system MUST provide `docs/bigmem-DOCS.md` as the single blob authority covering: schema + `BlobRoot` sibling layout, `maxStoredBytes=50k` vs `ShouldExternalize` (>100KB or `data:image/`), 300-char search preview, 1MiB stdin scanner limit, blob lifecycle (Put→addr→Get→`DoctorFixBlobs`), and `DoctorFixBlobs` migration note. Code comments MUST point at it, superseding scattered notes.

#### Scenario: Doc covers thresholds

- GIVEN doc read
- WHEN inspected
- THEN it MUST state 50k truncate vs 100KB externalize, 300-char preview, 1MiB scanner, lifecycle, and `DoctorFixBlobs` note

#### Scenario: No competing source

- GIVEN blob question arises
- WHEN docs searched
- THEN `docs/bigmem-DOCS.md` MUST be the referenced authority

### Requirement: Doctor DB path via filepath.Join

`internal/doctor/bigmem.go` MUST build the DB path with `filepath.Join(store.RootDir(), "bigmem.db")` and MUST NOT use string concatenation.

#### Scenario: Join used

- GIVEN `BigmemCheck.Run` resolves path
- WHEN code inspected
- THEN it MUST call `filepath.Join` and open the same `bigmem.db`

#### Scenario: Migration

- GIVEN consumers matched raw `blob:sha256:` output to detect blobs
- WHEN marker ships
- THEN they MUST handle marker-embedded addr (`IsBlobAddr` on substring or marker prefix check)
