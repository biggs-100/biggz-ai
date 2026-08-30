# Delta for policy

## ADDED Requirements

### Requirement: Bash Destructive Pattern Deny

The system MUST implement `internal/policy/guardrails.go` `deniedBashPatterns[6]` and `IsDenied(command string) bool` that blocks `rm -rf` rooted at `/`, `~`, `$HOME`, or `..`; `git reset --hard`; `git clean` only when both a force flag (`-f`/`--force`) and a directory flag (`-d`/`--directories`) are present; `git push` only when `--force` or short `-f` is present (e.g., `-f`, `-uf`); `chmod -R 777`; and `chown -R`. Other commands MUST NOT be denied.

#### Scenario: Blocks rm -rf and git reset hard and chmod/chown

- GIVEN command `rm -rf /` or `rm -rf ~` or `rm -rf $HOME` or `rm -rf ..` or `git reset --hard` or `chmod -R 777 /tmp` or `chown -R user /`
- WHEN `IsDenied` is called
- THEN it MUST return `true`

#### Scenario: Blocks git clean only with both force and directory flags

- GIVEN command `git clean -f -d` or `git clean --force --directories` or `git clean -fd`
- WHEN `IsDenied` is called
- THEN it MUST return `true`; with `git clean -f` or `git clean -d` or `git clean` it MUST return `false`

#### Scenario: Blocks git push only with force

- GIVEN command `git push --force` or `git push -f` or `git push -uf origin` versus `git push` or `git push origin main`
- WHEN `IsDenied` is called
- THEN force variants MUST return `true` and non-force MUST return `false`

### Requirement: Guarded Command Classification with AutonomousMode

The system MUST implement `internal/policy/guardrails.go` `guardedKeyPatterns[5]` (`gitPush` `git push`, `gitRebase` `git rebase`, `gitBranchDeleteForce` `git branch -D / -d -f / --delete --force`, `npmPublish` `npm publish`, `piRemove` `pi remove`) plus `autonomousDefaultActions` (`gitPush allow`, `gitRebase confirm`, `gitBranchDeleteForce confirm`, `npmPublish block`, `piRemove confirm`) and `ClassifyGuardedCommand(command string, cfg RuntimeGuardrailsConfig) string` that returns `block` for any `IsDenied` match first, otherwise per-key: `!autonomousMode → confirm`, `autonomousMode && guardedCommands[key] present → that value`, else `autonomousDefaultActions[key]`; unknown commands return `not-guarded`.

#### Scenario: Denied command always blocks regardless of mode

- GIVEN command `rm -rf /` or `git push --force` and cfg `autonomousMode true` with `gitPush allow`
- WHEN `ClassifyGuardedCommand` is called
- THEN it MUST return `block`

#### Scenario: Autonomous defaults and custom overrides apply

- GIVEN cfg `autonomousMode true` with no overrides and commands `git push`, `npm publish`, `git rebase` versus cfg `autonomousMode true` with `guardedCommands: {gitPush:"block", npmPublish:"allow"}`
- WHEN classified
- THEN defaults MUST be `allow`, `block`, `confirm`; custom overrides MUST return `block` for `git push` and `allow` for `npm publish`

#### Scenario: Non-autonomous and unknown commands

- GIVEN command `git push origin main` with `autonomousMode false` versus `go test ./...`
- WHEN classified
- THEN the guarded non-auto MUST return `confirm` and unknown MUST return `not-guarded`

### Requirement: Guardrails Config Parse and Two-File Merge

The system MUST implement `internal/policy/guardrails.go` `ParseGuardrailsConfigFile(raw string) (*RuntimeGuardrailsConfig, bool)` parsing `{"autonomousMode": bool, "guardedCommands": {key: action}}` with allowlists `validActions {allow,confirm,block}` and `validKeys {gitPush,gitRebase,gitBranchDeleteForce,npmPublish,piRemove}` filtering invalid entries, returning `(cfg,true)` on valid JSON and `(nil,false)` on malformed JSON; and `LoadRuntimeGuardrailsConfig(cwd string, configHome ...string) RuntimeGuardrailsConfig` that short-circuits to `{autonomous:true, empty}` when `GENTLE_PI_AUTONOMOUS_MODE=1`, otherwise reads `globalPath = home/runtime-guardrails.json` and `projectPath = cwd/.pi/gentle-ai/runtime-guardrails.json`, merging global then project (project `autonomousMode` wins, `guardedCommands` merged), returning `safeGuardrailsConfig` if either malformed, and ensuring non-nil map.

#### Scenario: Valid config parses and filters invalid entries, malformed returns not ok

- GIVEN raw `{"autonomousMode":true,"guardedCommands":{"gitPush":"allow","badKey":"allow","npmPublish":"badAction"}}` versus raw `{bad`
- WHEN `ParseGuardrailsConfigFile` is called
- THEN valid MUST return `true` with only `gitPush allow` kept; malformed MUST return `(nil,false)`

#### Scenario: Autonomous env fast-path and malformed file safe fallback

- GIVEN env `GENTLE_PI_AUTONOMOUS_MODE=1` versus a malformed global `{"bad`
- WHEN `LoadRuntimeGuardrailsConfig` is called
- THEN env `1` MUST return `{autonomous:true, empty}` without reading files; malformed MUST return `safeGuardrailsConfig` (`false`, empty)

#### Scenario: Global and project merge with project autonomous winning

- GIVEN `global` file `{"autonomousMode":false,"guardedCommands":{"gitPush":"block"}}` and `project` file `{"autonomousMode":true,"guardedCommands":{"npmPublish":"allow"}}`
- WHEN loaded
- THEN result MUST be `autonomousMode true` with both `gitPush block` and `npmPublish allow`

### Requirement: Sensitive Path Tool Guard

The system MUST implement `internal/policy/guardrails.go` `EvaluateSensitivePathTool(toolName string, input any) *ToolCallDecision` that only guards `read,write,edit`; collects paths via `collectPathInputs` (recursive `map[string]any`/`[]any` keyed by `path/paths/file/files/filePath/filePaths`) plus direct map extraction for `[]string`; normalizes via `isSensitivePath` (`TrimSpace`, `\→/`, `ToLower`, `~→HOME`) against `sensitivePathPatterns[8]` (`\.ssh`, `\.credentials`, `library/keychains`, `\.aws/credentials`, `\.config/gh/hosts.yaml`, `secrets/`, `\.env`, `.(pem|key|p12|pfx)$`); returns `Block` on first match else `nil`.

#### Scenario: Blocks ssh, credentials, secrets and env on guarded tools

- GIVEN tool `read` with `~/.ssh/id_rsa` or `/home/user/.aws/credentials` or `secrets/app/token` or `app/.env.local`
- WHEN `EvaluateSensitivePathTool` is called
- THEN it MUST return `Block` with reason containing the offending path

#### Scenario: Blocks pem and hosts yaml patterns

- GIVEN tool `edit` with `certs/app.pem` or `~/.config/gh/hosts.yml` or `Library/Keychains/login.keychain`
- WHEN evaluated
- THEN it MUST return `Block`

#### Scenario: Allows non-sensitive and non-guarded tools and collects from arrays

- GIVEN tool `read` with `src/app.go` or tool `exec` with `~/.ssh/id_rsa` versus `read` with `{"paths":["/tmp/a","~/.ssh/id_rsa"]}` or nested `{"a":{"file":"secrets/x"}}`
- WHEN evaluated
- THEN non-sensitive / non-guarded MUST return `nil`; array/nested sensitive MUST return `Block`

### Requirement: DENIED Block (6)

The system MUST implement `IsDenied` (`internal/policy/guardrails.go`) blocking verbatim `DENIED[6]` (`gentle-ai.ts:280-720`): `rm -rf` at `/|~|$HOME|..`, `git reset --hard`, `git clean` with both `-f`/`--force`+`-d`/`--directories` (incl. `-fd`), `git push` with `--force`/`-f` (incl. `git -C`), `chmod -R 777`, `chown -R`. Others MUST NOT block.

#### Scenario: Blocks rooted rm/reset/chmod/chown

- GIVEN `rm -rf /`/`~`/`$HOME/x`/`..`,`git reset --hard`,`chmod -R 777 /tmp`,`chown -R u /`
- WHEN `IsDenied` called
- THEN MUST `true`

#### Scenario: git clean needs both flags

- GIVEN `git clean -fd`/`-f -d`/`--force --directories`
- WHEN `IsDenied` called
- THEN MUST `true`; `git clean -f`/`-d` MUST `false`

#### Scenario: git push needs force

- GIVEN `git push --force`/`-f`/`git -C /r push -uf origin`
- WHEN `IsDenied` called
- THEN MUST `true`; `git push` MUST `false`

#### Scenario: Scoped rm not blocked

- GIVEN `rm -rf ./scoped/a`
- WHEN `IsDenied` called
- THEN MUST `false`

### Requirement: SENSITIVE Guard (8)

The system MUST implement `EvaluateSensitivePathTool` guarding only `read|write|edit`; collecting via `collectPathInputs` on `path|paths|file|files|filePath|filePaths` + direct map extraction; normalizing `TrimSpace`,`\→/`,`ToLower`,`~→HOME`; matching 8 patterns: `\.ssh`, `\.credentials`, `library/keychains`, `\.aws/credentials$`, `\.config/gh/hosts\.ya?ml$`, `secrets/`, `\.env`+`$|[./_-]`, `\.(pem|key|p12|pfx)$`; `Block` with path on match else `nil`.

#### Scenario: Blocks 8 families

- GIVEN `read` with `~/.ssh/id_rsa`, `.aws/credentials`, `secrets/tok`, `hosts.yaml`, `Keychains/login.keychain`, `app/.env`, `cert/key.pem`
- WHEN `EvaluateSensitivePathTool` called
- THEN MUST `Block` with path

#### Scenario: .env variants and key exts

- GIVEN `write` with `.env.local`/`.env.production`/`a.PEM`/`cert.p12`/`store.pfx`
- WHEN evaluated
- THEN MUST `Block`; `src/app.go` MUST `nil`

#### Scenario: Non-guarded and array collection

- GIVEN `exec` with `~/.ssh/id_rsa` vs `read` `{"paths":["a","~/.ssh/id_rsa"]}`
- WHEN evaluated
- THEN `exec` MUST `nil`; array sensitive MUST `Block`

### Requirement: GUARDED Classification

The system MUST implement `guardedKeyPatterns[5]` (`gitPush` incl. `-C`, `gitRebase`, `gitBranchDeleteForce` `-D`/`-d -f`/`--delete --force`, `npmPublish`, `piRemove`) with defaults (`gitPush allow`,`gitRebase confirm`,`BranchDeleteForce confirm`,`npmPublish block`,`piRemove confirm`) and `ClassifyGuardedCommand`: `IsDenied`→`block`; else matched key → `!AutonomousMode`→`confirm`, else override if present else default; else `not-guarded`.

#### Scenario: Denied overrides allow

- GIVEN `git push --force` with `cfg{AutonomousMode:true, gitPush:allow}`
- WHEN classified
- THEN MUST `block`

#### Scenario: Defaults and overrides

- GIVEN `cfg{true, empty}` with `git push`/`npm publish`/`git rebase` vs `cfg{true, {gitPush:block, npmPublish:allow}}`
- WHEN classified
- THEN defaults MUST `allow`/`block`/`confirm`; overrides MUST `block`/`allow`

#### Scenario: Non-auto confirm and unknown

- GIVEN `git push origin main` `AutonomousMode:false` vs `go test ./...`
- WHEN classified
- THEN MUST `confirm` and `not-guarded`

### Requirement: Runtime Config Merge Safe Fallback

The system MUST implement `ParseGuardrailsConfigFile` (allowlist 5 keys/3 actions, `malformed→(nil,false)`) and `LoadRuntimeGuardrailsConfig(cwd,configHome...)`: if `GENTLE_PI_AUTONOMOUS_MODE=1` → `{true,empty}` no I/O; else reads `home/runtime-guardrails.json` then `cwd/.pi/gentle-ai/runtime-guardrails.json`; merges global→project copy-on-merge (project `AutonomousMode` wins, map shallow-copy, global not mutated); malformed→`safeGuardrailsConfig` (`false`, empty non-nil); non-nil map.

#### Scenario: Env fast-path

- GIVEN env `GENTLE_PI_AUTONOMOUS_MODE=1` with malformed files
- WHEN loaded
- THEN MUST `{true, map[]}` not safe fallback

#### Scenario: Malformed→safe

- GIVEN global or project `{bad`
- WHEN loaded without env
- THEN MUST `safeGuardrailsConfig`

#### Scenario: Merge copy-on-merge

- GIVEN global `{false,{gitPush:block}}` + project `{true,{npmPublish:allow}}`
- WHEN loaded
- THEN MUST `{true,{gitPush:block,npmPublish:allow}}` and global unchanged reread

### Requirement: Cross-Surface Parity (3 surfaces, 3 checks)

The system MUST enforce identical 3 checks (`IsDenied`,`ClassifyGuardedCommand`,`EvaluateSensitivePathTool`) at `biggz-synthesis-gate.js` (Pi), `safety.ts` (OpenCode), `gate.go` (pre-check). Denied→`block`, guarded per class, sensitive→`block`; verbatim 6/8/5. No surface MAY add/omit.

#### Scenario: Same deny on 3 surfaces

- GIVEN `git push --force` or `read ~/.ssh/id_rsa`
- WHEN via pi gate, plugin, gate.go
- THEN each MUST `block` same category

#### Scenario: Guarded parity !auto

- GIVEN `git rebase main` `AutonomousMode:false`
- WHEN checked on each surface
- THEN each MUST `confirm`

### Requirement: Safety Logging and Human Non-Blocking

The system MUST log every `block`/`confirm` with `surface`+`kind`+pattern/path; MUST NOT block non-denied/sensitive human actions; `confirm` is prompt not hard block; scout fallback logs without human spam.

#### Scenario: Block logged

- GIVEN `IsDenied`/`EvaluateSensitivePathTool` → `Block`
- WHEN emitted
- THEN log MUST contain `surface`+`kind=block`+path

#### Scenario: Non-sensitive not blocked

- GIVEN `write src/app.go`
- WHEN checked
- THEN MUST `nil`/`not-guarded`, no block log

