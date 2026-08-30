# Delta policy

## ADDED Requirements

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
