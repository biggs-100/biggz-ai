# BigMem Specification

## Purpose

Local SQLite-backed memory store (Engram-compatible). Covers persistence, FTS dedup, sync, and Engram import.

## Requirements

### Requirement: REQ-1 — Engram Import Dispatch (--from-engram)

The system MUST import from Engram storage when `--from-engram` is set; otherwise MUST use default BigMem transport (`~/.biggz/bigmem`). Flag MUST be exclusive source selector.

#### Scenario: Import from Engram

- GIVEN valid `.engram/manifest.json` with chunks
- WHEN `biggz bigmem sync import --from-engram` runs
- THEN system MUST read manifest + `chunks/*.jsonl.gz` into `bigmem.db`

#### Scenario: Default transport unchanged

- GIVEN no `--from-engram`
- WHEN `biggz bigmem sync import` runs
- THEN existing BigMem transport MUST be used

### Requirement: REQ-2 — Custom Engram Dir (--engram-dir)

System MUST accept `--engram-dir <PATH>` to override default `.engram/`; if omitted MUST resolve default.

#### Scenario: Custom dir

- GIVEN `--from-engram --engram-dir /tmp/.engram`
- WHEN import runs
- THEN manifest/chunks MUST be read from `/tmp/.engram`

#### Scenario: Default resolution

- GIVEN `--from-engram` without `--engram-dir`
- WHEN import runs
- THEN default `.engram` path MUST be resolved

### Requirement: REQ-3 — Project Filter (--project)

With `--project <NAME>`, system MUST import only entities where `project==NAME` after gunzip.

#### Scenario: Filtered import

- GIVEN chunks with `biggz-ai` and `other`
- WHEN `--project biggz-ai` runs
- THEN only `biggz-ai` entities MUST be inserted

#### Scenario: No filter imports all

- GIVEN no `--project`
- WHEN import runs
- THEN all projects MUST be imported

### Requirement: REQ-4 — sync_id to ID Mapping

System MUST map `engram.sync_id -> bigmem.ID`; int64 `id` MUST be ignored. Empty `sync_id` MUST generate deterministic `engram-<sha256(title+content)[0:12]>`.

#### Scenario: sync_id preserved

- GIVEN Engram obs `sync_id="obs-abc123"`, `id=42`
- WHEN imported
- THEN `bigmem.ID` MUST be `"obs-abc123"`

#### Scenario: Empty sync_id fallback

- GIVEN Engram obs with empty `sync_id`
- WHEN imported
- THEN ID MUST be `engram-<hex>` deterministic for same content

### Requirement: REQ-5 — Idempotent Dedup (sync_chunks)

Re-import MUST be no-op. System MUST record `sync_chunks(target_key='engram:'+chunkID)` and skip known chunks; inserts MUST use `ON CONFLICT DO NOTHING`.

#### Scenario: Re-import no-op

- GIVEN chunk `a3f8c1d2` already in `sync_chunks`
- WHEN re-imported
- THEN chunk MUST be skipped, no duplicates created

#### Scenario: Partial import

- GIVEN manifest 3 chunks, 2 already recorded
- WHEN import runs
- THEN only 1 pending chunk MUST be processed

### Requirement: REQ-6 — Error: Missing Manifest

Missing `manifest.json` MUST emit to stderr and exit 1 with zero DB mutations.

#### Scenario: Missing manifest

- GIVEN `--engram-dir /tmp/empty` without `manifest.json`
- WHEN import runs
- THEN stderr MUST mention `manifest.json` and exit 1

### Requirement: REQ-7 — Error: Corrupt Chunk

Corrupt gzip/JSON MUST warn per chunk to stderr, skip chunk, continue others. Exit 0 if any chunk succeeds.

#### Scenario: Corrupt gzip skipped

- GIVEN `chunks/bad.jsonl.gz` invalid gzip
- WHEN import runs
- THEN stderr MUST warn with chunk ID; other chunks MUST import

#### Scenario: Corrupt JSON skipped

- GIVEN gunzipped chunk has invalid JSON
- WHEN import runs
- THEN chunk MUST be skipped with warning, not counted

### Requirement: REQ-8 — Pi Isolation (Pi Untouched)

Feature MUST NOT modify `pi/` or TUI; `.engram/` MUST remain read-only; tracking MUST live only in `bigmem.db`.

#### Scenario: Pi unchanged

- GIVEN change applied
- WHEN `git diff -- pi/` checked
- THEN output MUST be empty

#### Scenario: Engram read-only

- GIVEN import succeeded
- WHEN `.engram/` inspected
- THEN no file MUST be modified; `sync_chunks` rows MUST exist in `bigmem.db`
