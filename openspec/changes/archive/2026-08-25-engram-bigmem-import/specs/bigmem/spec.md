# Delta for bigmem — NEW (full spec)

This capability is NEW; no prior `openspec/specs/bigmem/spec.md` existed. The full spec is at `openspec/specs/bigmem/spec.md` and is incorporated here by reference.

## ADDED Requirements

### Requirement: REQ-1 — Engram Import Dispatch via --from-engram

The system MUST import from Engram chunk storage when `biggz bigmem sync import --from-engram` is invoked.

#### Scenario: Import from Engram source

- GIVEN a valid `.engram/manifest.json` with chunks exists
- WHEN `biggz bigmem sync import --from-engram` runs
- THEN the system MUST read manifest and `chunks/*.jsonl.gz` and insert into `bigmem.db`

#### Scenario: Default source unchanged

- GIVEN `--from-engram` not provided
- WHEN `biggz bigmem sync import` runs
- THEN existing BigMem transport MUST be used

### Requirement: REQ-2 — Custom Engram Directory via --engram-dir

The system MUST accept `--engram-dir <PATH>` to override the default `.engram/` directory.

#### Scenario: Custom directory

- GIVEN `--from-engram --engram-dir /tmp/.engram`
- WHEN import runs
- THEN manifest and chunks MUST be read from `/tmp/.engram`

### Requirement: REQ-3 — Project Filter via --project

When `--project <NAME>` is provided, the system MUST import only entities matching that project after gunzip.

#### Scenario: Filtered import

- GIVEN chunks contain `biggz-ai` and `other`
- WHEN `--project biggz-ai` is passed
- THEN only `biggz-ai` entities MUST be inserted

### Requirement: REQ-4 — sync_id to ID Mapping

The system MUST map `engram.sync_id` to `bigmem.ID`; legacy int64 id MUST be ignored; empty `sync_id` MUST yield deterministic `engram-<hash>` fallback.

#### Scenario: sync_id preserved

- GIVEN Engram obs with `sync_id="obs-abc123"` and `id=42`
- WHEN imported
- THEN `bigmem.ID` MUST be `"obs-abc123"`

### Requirement: REQ-5 — Idempotent Dedup via sync_chunks

Re-import of same chunks MUST be no-op via `sync_chunks('engram:'+chunkID)` plus `ON CONFLICT DO NOTHING`.

#### Scenario: Re-import is no-op

- GIVEN chunk already in `sync_chunks`
- WHEN re-imported
- THEN it MUST be skipped and no duplicates created

### Requirement: REQ-6 — Error Handling: Missing Manifest

Missing `manifest.json` MUST emit to stderr and exit 1 with no DB mutations.

#### Scenario: Missing manifest

- GIVEN no `manifest.json` at resolved dir
- WHEN import runs
- THEN stderr MUST mention `manifest.json` and exit code MUST be 1

### Requirement: REQ-7 — Error Handling: Corrupt Chunk

Corrupt gzip/JSON chunks MUST be warned per chunk, skipped, and remaining chunks continued.

#### Scenario: Corrupt gzip skipped

- GIVEN a chunk is invalid gzip
- WHEN import runs
- THEN warning MUST be emitted and remaining chunks MUST import

### Requirement: REQ-8 — Pi Isolation (Pi Untouched)

The feature MUST NOT modify any file under `pi/` and `.engram/` MUST remain read-only; tracking only in `bigmem.db`.

#### Scenario: Pi unchanged

- GIVEN change is applied
- WHEN `git diff -- pi/` checked
- THEN output MUST be empty
