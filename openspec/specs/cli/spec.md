# CLI Specification

## Purpose

The CLI domain defines the command-line entry point for biggz-ai. It reads a ReviewSubject from standard input, invokes the pipeline orchestrator, and writes the resulting ReviewState as JSON to standard output.

## Requirements

### Requirement: Stdin Input

The system MUST read a ReviewSubject from standard input. The input MUST be a JSON object conforming to the ReviewSubject schema.

#### Scenario: Happy path — valid JSON input

- GIVEN a valid ReviewSubject JSON on stdin with repository URL and commit SHA
- WHEN the CLI starts
- THEN the subject MUST be parsed successfully
- AND the pipeline MUST execute with the parsed subject

#### Scenario: Invalid JSON input

- GIVEN malformed JSON on stdin
- WHEN the CLI starts
- THEN the CLI MUST exit with a non-zero exit code
- AND print an error message to stderr

### Requirement: Pipeline Execution

The CLI MUST invoke the Orchestrator's Execute method with the parsed ReviewSubject. The CLI MUST use the configured Registry with at least one LensPlugin registered.

#### Scenario: Happy path — successful review

- GIVEN valid JSON input on stdin
- WHEN the pipeline completes successfully
- THEN the ReviewState MUST have Status set to Completed
- AND the CLI MUST NOT print errors to stderr

#### Scenario: Pipeline failure

- GIVEN valid JSON input on stdin
- WHEN the pipeline fails (e.g., provider error)
- THEN the ReviewState MUST have Status set to Failed
- AND the CLI MUST exit with a non-zero exit code

### Requirement: JSON Output

The CLI MUST print the resulting ReviewState to standard output as a JSON object. The output MUST include all ReviewState fields: Status, Evidence, MerkleRoot, SchemaVersion.

#### Scenario: Happy path — complete output

- GIVEN a completed pipeline execution
- WHEN the CLI prints the ReviewState to stdout
- THEN the JSON MUST contain Status, Evidence, MerkleRoot, and SchemaVersion fields
- AND the JSON MUST be valid

#### Scenario: Empty evidence chain output

- GIVEN a pipeline that completes with no evidence entries
- WHEN the CLI prints the ReviewState to stdout
- THEN the Evidence field MUST be an empty array in the JSON output
- AND MerkleRoot MUST be an empty string

### Requirement: Exit Codes

The CLI MUST exit with code 0 on success and non-zero on any error condition.

#### Scenario: Success exit

- GIVEN a successful pipeline execution
- WHEN the CLI exits
- THEN the exit code MUST be 0

#### Scenario: Error exit

- GIVEN any error during parsing, execution, or output
- WHEN the CLI exits
- THEN the exit code MUST be non-zero
- AND an error description MUST be written to stderr

### Requirement: Doctor Subcommand

The CLI MUST add a "doctor" subcommand dispatched via the existing switch-based router. The subcommand MUST NOT read or parse a ReviewSubject from stdin — it operates independently of the review pipeline.

#### Scenario: Doctor dispatch

- GIVEN the CLI is invoked as `biggz doctor`
- WHEN the router matches the "doctor" command
- THEN doctorRun() MUST be invoked
- AND no stdin review parsing MUST occur

### Requirement: --json Flag

The doctor subcommand MUST parse a --json flag. When present, output MUST be valid JSON marshaled from the Report struct, parsable by standard JSON tools.

#### Scenario: JSON output

- GIVEN `biggz doctor --json`
- WHEN all checks complete
- THEN stdout MUST contain valid JSON with all check Results and severity buckets
- AND the exit code MUST be 0 (unless a check framework error occurs)

#### Scenario: JSON with --fix

- GIVEN `biggz doctor --fix --json`
- WHEN remedies execute and checks re-run
- THEN remedies MUST execute before JSON serialization
- AND the JSON output MUST reflect the post-fix state

### Requirement: --fix Flag

The doctor subcommand MUST parse a --fix flag. When present, the Runner MUST iterate declared remedies and execute their Actions after all initial checks complete. If no remedies are declared, zero actions execute and the output MUST indicate this.

#### Scenario: Remediation applied

- GIVEN `biggz doctor --fix` and at least one check declares a Remedy
- WHEN checks complete
- THEN each remedy Action MUST execute
- AND the output MUST include post-remedy status per check

#### Scenario: No remedies declared

- GIVEN `biggz doctor --fix` and no check declares a Remedy
- WHEN checks complete
- THEN zero actions MUST execute
- AND the output MUST indicate "0 remedies applied"

### Requirement: Default Renderer

The doctor subcommand MUST render results in human-readable format by default using tabla humana, grouped by severity bucket. Each check row MUST display a status icon: [ok] for pass, [!!] for warn, [xx] for fail. A summary line MUST show total counts per severity.

#### Scenario: Default table output

- GIVEN `biggz doctor` (no flags)
- WHEN all checks complete
- THEN stdout MUST contain severity-grouped sections
- AND each row MUST show check ID, status icon, and message
- AND a footer MUST show total pass/warn/fail counts

### Requirement: Update Subcommand

The CLI MUST add an `update` subcommand dispatched via the existing switch-based router. On Unix, the system MUST download the release archive, verify the checksum signature with the committed minisign public key, extract the binary, and atomically replace the running binary via os.Rename. On Windows, the system MUST NOT replace the binary and MUST instruct the user to run `go install`.

#### Scenario: Update on Unix — success

- GIVEN the CLI is invoked as `biggz update` on Linux or macOS
- WHEN the latest release is fetched, verified, and extracted
- THEN the binary MUST be replaced atomically (os.Rename)
- AND the new version string MUST be printed on success

#### Scenario: Update on Windows — fallback

- GIVEN `biggz update` on Windows
- WHEN the engine identifies the platform
- THEN binary replacement MUST NOT be attempted
- AND the system MUST print `go install github.com/biggz-ai/biggz@latest`

#### Scenario: Signature verification failure

- GIVEN checksums.txt.minisig does not verify against the committed public key
- WHEN the engine attempts verification
- THEN the update MUST abort
- AND a signature error MUST be printed to stderr

#### Scenario: Channel-aware update

- GIVEN BIGGZ_CHANNEL=beta
- WHEN `biggz update` fetches releases
- THEN the system MUST select the latest pre-release
- AND proceed with download and verification

#### Scenario: Already up to date

- GIVEN the running binary version equals the latest release version
- WHEN `biggz update` checks
- THEN the system MUST print "already up to date"
- AND exit with code 0

### Requirement: Sync Subcommand

The CLI MUST add a `sync` subcommand dispatched via the existing switch-based router. The system MUST accept the flags `--skills`, `--config`, `--prompts`, `--commands`, `--all`, and `--dry-run`. When `--all` is provided or no category flag is provided, the system MUST deploy all four categories. When specific category flags are provided, the system MUST deploy only those categories. When `--dry-run` is provided, the system MUST NOT write any files and MUST report the sync plan. The subcommand MUST NOT read or parse a `ReviewSubject` from stdin. Each category walks its source directory and calls `WriteFileAtomic` for each file.

#### Scenario: Sync all categories

- GIVEN the CLI is invoked as `biggz sync`
- WHEN the router matches the "sync" command
- THEN `syncRun()` MUST be invoked
- AND all four categories (skills, config, prompts, commands) MUST be deployed
- AND no stdin review parsing MUST occur

#### Scenario: Selective sync

- GIVEN the CLI is invoked as `biggz sync --skills --config`
- WHEN `syncRun()` executes
- THEN only skills and config MUST be deployed
- AND prompts and commands MUST be skipped

#### Scenario: Dry-run reports without writing

- GIVEN the CLI is invoked as `biggz sync --dry-run`
- WHEN `syncRun()` executes
- THEN no files MUST be written to the filesystem
- AND a summary of what would be deployed MUST be printed to stdout
- AND the exit code MUST be 0

#### Scenario: All flag is equivalent to no flags

- GIVEN the CLI is invoked as `biggz sync --all`
- WHEN `syncRun()` executes
- THEN all four categories MUST be deployed

#### Scenario: Help output

- GIVEN the CLI is invoked as `biggz sync --help` or `-h`
- WHEN the help flag is parsed
- THEN usage information MUST be printed
- AND the exit code MUST be 0

#### Scenario: Unknown flag

- GIVEN the CLI is invoked as `biggz sync --unknown`
- WHEN `syncRun()` parses flags
- THEN the system MUST print an error to stderr
- AND the exit code MUST be non-zero
