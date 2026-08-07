# Delta for CLI

## ADDED Requirements

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
- AND the system MUST print `go install github.com/biggs-100/biggz-ai@latest`

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
