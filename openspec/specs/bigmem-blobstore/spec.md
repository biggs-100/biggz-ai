# bigmem-blobstore Specification

## Purpose

Externalize BigMem payloads >100KB or `data:image/` to `~/.biggz/blobs/<sha256>`, store only `blob:sha256:<hex>` in SQLite. Transparent read, bounded WAL, no D2 branching.

## Requirements

### Requirement: PutBlob — Content-Addressed Write

The system MUST provide `PutBlob([]byte) (string, error)` hashing SHA-256, writing atomically to `~/.biggz/blobs/<hex>` via write-if-not-exists, returning `blob:sha256:<64hex>`. Same bytes MUST dedup to same addr.

#### Scenario: Round-trip >100KB

- GIVEN 150KB bytes
- WHEN `PutBlob` called
- THEN addr MUST match `blob:sha256:[0-9a-f]{64}` and file MUST contain same bytes

#### Scenario: Dedup no overwrite

- GIVEN same bytes stored once
- WHEN `PutBlob` same bytes again
- THEN MUST return same addr without rewriting file

### Requirement: GetBlob — Addr Resolution

The system MUST provide `GetBlob(string) ([]byte, error)` validating `blob:sha256:<64hex>`, reading `~/.biggz/blobs/<hex>`. Invalid format MUST error; missing file MUST return not-found.

#### Scenario: Valid resolves

- GIVEN addr from `PutBlob`
- WHEN `GetBlob(addr)` called
- THEN MUST return original bytes

#### Scenario: Invalid rejected

- GIVEN `blob:sha256:zzzz`
- WHEN `GetBlob` called
- THEN MUST error and MUST NOT access outside blobs root

#### Scenario: Missing not-found

- GIVEN valid addr without file
- WHEN `GetBlob` called
- THEN MUST return not-found error

### Requirement: Externalization Threshold

`biggz-mcp` Save MUST intercept before `Store.Save`: if `len(content)>100000` OR contains `data:image/` then MUST `PutBlob` and persist addr; else MUST persist inline. No schema change.

#### Scenario: Large externalized

- GIVEN 150KB content
- WHEN Save called
- THEN DB MUST store `blob:sha256:` addr and file MUST exist

#### Scenario: Small inline

- GIVEN 10KB without `data:image/`
- WHEN Save called
- THEN DB MUST store verbatim content

#### Scenario: Small image externalized

- GIVEN 5KB with `data:image/png;base64,`
- WHEN Save called
- THEN MUST store addr despite size

### Requirement: Transparent Get/Search

`Get`/`Search` MUST resolve: if `content` has `blob:sha256:` prefix then MUST try `GetBlob`; on success return bytes, on miss return raw addr. Non-blob rows passthrough. MUST NOT mutate DB.

#### Scenario: Blob resolved

- GIVEN row with addr and file present
- WHEN `Get` called
- THEN MUST return blob bytes not addr

#### Scenario: Missing fallback

- GIVEN row with addr but file deleted
- WHEN `Get` called
- THEN MUST return addr without error

### Requirement: Doctor --fix-blobs Migration

`biggz bigmem doctor --fix-blobs` MUST scan `WHERE (length(content)>100000 OR content LIKE 'data:image/%') AND content NOT LIKE 'blob:sha256:%'`, `PutBlob` per row, update atomically, report `migrated/skipped/errors`, and be idempotent. Without flag MUST NOT modify blobs.

#### Scenario: Migrates legacy rows

- GIVEN 2 inline large rows + 1 blob row
- WHEN `doctor --fix-blobs` runs
- THEN 2 MUST become addrs, 1 skipped, report `migrated:2`

#### Scenario: Idempotent re-run

- GIVEN all rows already migrated
- WHEN `doctor --fix-blobs` re-runs
- THEN `migrated` MUST be 0, no duplicates

#### Scenario: No flag untouched

- GIVEN large inline rows exist
- WHEN `doctor` without flag runs
- THEN zero rows MUST change

### Requirement: Storage Layout and Concurrency

Blobs MUST live under `~/.biggz/blobs/<sha256>` (not `~/.omp/blobs/`). `PutBlob` MUST be concurrency-safe via atomic write-if-not-exists; blobs immutable.

#### Scenario: Concurrent same bytes

- GIVEN 2 goroutines `PutBlob(same 200KB)`
- WHEN both complete
- THEN same addr and uncorrupted file MUST result

#### Scenario: Traversal rejected

- GIVEN addr containing `..`
- WHEN `GetBlob` called
- THEN MUST error before FS access

### Requirement: GC Manual Only — No Auto-GC

System MUST NOT auto-delete blobs. `PutBlob`/`Get`/`doctor` MUST never remove files. Docs/output SHOULD advise `find ~/.biggz/blobs -type f -mtime +30`. Orphans tolerated via immutability+dedup.

#### Scenario: No auto deletion

- GIVEN blobs exist
- WHEN Saves/Gets run
- THEN blob count MUST NOT decrease

#### Scenario: Advisory hint

- GIVEN `doctor --fix-blobs` completes
- WHEN output rendered
- THEN SHOULD contain `find ~/.biggz/blobs -type f -mtime +30`

## Out of Scope

`leafId`/branching, merge, auto-GC, cloud sync, encryption, compression, `sdd-apply` branch.
