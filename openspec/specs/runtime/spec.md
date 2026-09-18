# Runtime Specification

## Purpose

Runtime covers platform and execution primitives: OpenCode grouped isolation as scheduling-only, Windows beta quoting/process control and handle-relative durable writes, cooperative filecoord locking with `BusyError`, Pi bounded manifest reads and progress tracking, review authority lock hardening, and Codex `hooks.json` atomic delegation. This spec also covers parity-gentle-69 ledger atomicity and dual-budget invariants ported from gentle `e8cc0fcc→782e8dfe`.

## Requirements

### Requirement: Grouped Isolation and Windows Beta

The system MUST implement OpenCode grouped isolation where isolation applies to scheduling only, not a security boundary. On Windows, the system MUST correctly quote paths, handle `rundll32`/`xdg-open` branching, and pass platform-specific process control and lock primitives. Cooperative filecoord lock acquisition MUST be one non-blocking attempt: contention returns `BusyError` without mutation, and caller owns retry pacing.

#### Scenario: Grouped isolation is scheduling-only

- GIVEN OpenCode background subagents run under grouped isolation
- WHEN two lanes schedule concurrent attempts
- THEN ordering MUST be coordinated via scheduling, not via filesystem security isolation

#### Scenario: Windows path and process handling

- GIVEN `biggz` runs on `windows`
- WHEN it quotes a change root or spawns a hook
- THEN it MUST use Windows-safe quoting and `rundll32`/`cmd` branching and MUST NOT attempt Unix-only `os.Rename` atomic replace

#### Scenario: Cooperative lock contention is non-mutating

- GIVEN `filecoord` lock for target `internal/sdd/status.go` is held
- WHEN another `Acquire` attempts the same target
- THEN it MUST return `BusyError` and MUST NOT mutate the protected resource

### Requirement: Pi Progress, Cooperative Locking, and Codex Hooks

The system MUST expose Pi progress and manifest resolution with bounded reads (`MaxPackageManifestBytes`), explicit `manifest-too-large` / `malformed-manifest` kinds, and non-mutating reads. Review authority stores MUST use hardened cooperative `MaintenanceLock` / `AuthorityFileLock` with `no-follow` open and PID/host as non-authoritative metadata. Codex `hooks.json` skill-registry refresh via `SessionStart` MUST be installed and removed atomically.

#### Scenario: Pi manifest bounded read

- GIVEN `package.json` exceeds `MaxPackageManifestBytes`
- WHEN `selectPackageBin` reads it
- THEN it MUST fail with `manifest-too-large` and MUST NOT mutate the manifest

#### Scenario: Pi progress tracking

- GIVEN an install pipeline with steps `prepare`/`apply`/`rollback`
- WHEN `ProgressFromExecution` aggregates result
- THEN `ProgressState` MUST report `Percent`, `CurrentStep`, and `HasFailures` deterministically

#### Scenario: Codex hooks delegation to backup

- GIVEN Codex global config `hooks.json` exists
- WHEN `ensureCodexSkillRegistryHook` runs
- THEN it MUST add `gentle-ai skill-registry refresh` under `hooks.SessionStart` atomically
- AND uninstall MUST remove only that hook entry, preserving other hooks

#### Scenario: Maintenance lock Timeout

- GIVEN review store `v2/LOCK` is held
- WHEN `BurnApprovedCompactAuthority` tries to acquire with `storeResetLockTimeout=2s`
- THEN it MUST fail with `ErrAuthorityLockTimeout` after timeout and MUST NOT delete authority

### Requirement: Ledger Verify-Before-Commit (CAS)

The system MUST enforce verify-before-commit: `commitRecordLocked` MUST replay `loadRecord(revision)` inside `withStoreLock` before `writeLedgerHead` and advance HEAD only if `candidate.Status.Revision == revision`. On mismatch it MUST fail closed without mutating HEAD/records.

#### Scenario: CAS refuses stale revision

- GIVEN HEAD is `R1`
- WHEN commit presents candidate `R0 != R1`
- THEN it MUST be rejected with CAS conflict and HEAD MUST stay `R1`

#### Scenario: HEAD advances on match

- GIVEN HEAD `R1` and candidate derived from `R1`
- WHEN commit presents `Revision == R1`
- THEN record MUST be persisted and HEAD MUST advance to `R2 == sha256(canonical)`

#### Scenario: Concurrent serialize

- GIVEN two writers loaded `R1` and writer A committed `R1→R2`
- WHEN writer B attempts `R1→R3`
- THEN B MUST be rejected with CAS conflict

### Requirement: Dual Budget Single Owner

The system MUST track `MaxAttempts` and `MaxChangedLines` with per-attempt `ChangedLines` and carried `CumulativeChangedLines`. The single predicate `CumulativeChangedLines + changedLines > MaxChangedLines` MUST own line-budget exhaustion for `Acquire`, `Finish`/`Settle`, and replay paths. No duplicate inequality SHALL exist.

#### Scenario: Blocked when cumulative+delta exceeds max

- GIVEN `Cumulative=300`, `MaxLines=400`
- WHEN acquiring `changedLines=150`
- THEN it MUST be rejected `blocked(budget_exhausted)` (300+150>400)

#### Scenario: Admitted within budget

- GIVEN `Cumulative=300`, `MaxLines=400`
- WHEN acquiring `changedLines=80`
- THEN it MUST succeed and after settle cumulative MUST be `380`

#### Scenario: Single predicate ownership

- GIVEN finish and status replay evaluate line budget
- WHEN inspecting code paths
- THEN both MUST call the same predicate without inline duplication

### Requirement: Interrupted Refund Capped at 2×

El sistema MUST usar `runtimeAttemptDeliveredIncrement`: `interrupted && changedLines>0` incrementa delivered; en caso contrario, no. `runtimeRefundedAttempts() <= MaxAttempts` MUST acotar los reembolsos a `2×MaxAttempts` de la generación de objetivo viva (`MaxAttempts` de la generación abierta), y el cap MUST NOT heredarse hacia una generación sucesora ni cargarse contra ella. `Acquire`/`Begin` MUST rechazar cuando el cap de la generación viva está agotado (gentle 2243/2217).
(Previously: el cap aplicaba `2×MaxAttempts` **total** del change, sin alcance de generación.)

#### Scenario: Interrupted with lines counts

- GIVEN `MaxAttempts=3`, `interrupted` `changedLines=20`
- WHEN se evalúa el incremento delivered
- THEN MUST contar como delivered

#### Scenario: Interrupted without lines refunded

- GIVEN `interrupted` `changedLines=0`
- WHEN se evalúa
- THEN MUST NOT contar como delivered y MUST quedar refund-eligible

#### Scenario: Blocks after 2× cap

- GIVEN la generación viva `MaxAttempts=3`, `refunded==3`
- WHEN se adquiere
- THEN MUST rechazarse con `blocked(budget_exhausted)`

#### Scenario: Successor starts with full refund budget

- GIVEN la generación predecesora agotó su cap de reembolsos
- WHEN el sucesor adquiere su primer attempt
- THEN MUST admitirse: los reembolsos del predecesor MUST NOT bloquearlo ni cargarse contra su cap fresco `2×MaxAttempts`

### Requirement: Rescope Exhausted Wedge

When exhausted (`DecisionRequired` or budgets reached), `Rescope` MUST require `MaxAttempts > carried CumulativeAttempts` AND `MaxChangedLines > carried CumulativeChangedLines`. Cumulative counters MUST never reset; attempts slice MUST be preserved.

#### Scenario: Refuses unless both exceed carried

- GIVEN exhausted `cumul=5/600`, `Max=5/600`
- WHEN `Rescope` proposes `MaxAttempts=5` `MaxLines=700`
- THEN it MUST be rejected (attempts not > carried)

#### Scenario: Admits when wedge satisfied

- GIVEN same exhausted ledger
- WHEN `Rescope` proposes `7/800`
- THEN it MUST succeed and cumulative MUST still read `5/600`

#### Scenario: Cumulative preserved

- GIVEN ledger 4 attempts 350 lines
- WHEN rescope
- THEN attempts length and cumulative sum MUST be unchanged

### Requirement: Runtime Record Rejection Taxonomy

The system MUST use single typed `RuntimeRecordRejectedError` for all record rejections (hash, schema, lineage, staleness). Callers MUST detect via `errors.As`. No parallel string-only paths SHALL exist.

#### Scenario: Hash mismatch typed

- GIVEN record bytes hash to `H' != H`
- WHEN `loadRecord` validates
- THEN it MUST return `RuntimeRecordRejectedError`

#### Scenario: Unified handling

- GIVEN any rejection
- WHEN checking `errors.As(err, *RuntimeRecordRejectedError)`
- THEN it MUST succeed

### Requirement: Admissible Settle (warn, not block)

`Settle` MUST admit settlements whose token is unknown (recording an unclaimed attempt) or maps to an unfinished non-active attempt, returning the settlement with a non-empty `warning` instead of `invalid_continuation`. `Strict: true` (CLI `--strict`, reserved for prod apply) MUST restore the old block. Fraud and shape errors (finished-attempt double-settle, remediation mismatch, request-id reuse, missing ledger) MUST still block in all modes.

#### Scenario: Unknown token admits with warning

- GIVEN a ledger with no matching token
- WHEN `Settle` without `Strict`
- THEN it MUST succeed with `Warning != ""` and a recorded unclaimed attempt

#### Scenario: Strict restores the block

- GIVEN a ledger with no matching token
- WHEN `Settle` with `Strict: true`
- THEN it MUST return `BlockedError` with `invalid_continuation`

### Requirement: Background Subagents 4-Source Policy Resolution

The system MUST resolve background subagents policy via `internal/sdd/background.go` `resolveBackgroundSubagentsPolicy(cwd,opts)` with precedence `project > global > env > default off`. `project` reads `cwd/.biggz/background-subagents.json`, `global` reads `~/.biggz/background-subagents.json` (honoring `GENTLE_PI_CONFIG_HOME`/`BIGGZ_CONFIG_HOME`), `env` reads `BIGGZ_BACKGROUND_SUBAGENTS` (fallback `GENTLE_PI_BACKGROUND_SUBAGENTS`), strict 2-key decode `{"schema":"gentle-pi.background-subagents/v1","policy":"on"|"off"}` where extra keys → malformed → `off` no fallback, `malformed true`. Max 2 JSON reads per resolve. Capability `ready|absent` via the runtime marker probe (deployed subagent-runtime extension under `~/.pi/agent/extensions/`), NOT third-party package presence.
(Previously: capability § probed `subagent_run`; now probed via the own runtime marker.)

#### Scenario: Project overrides global and env

- GIVEN `cwd/.biggz/background-subagents.json` `{"schema":"gentle-pi.background-subagents/v1","policy":"on"}` and global `off` and env `on`
- WHEN `resolveBackgroundSubagentsPolicy(cwd,opts)` called
- THEN it MUST return `policy on` `source project_file` `malformed false`

#### Scenario: Strict 2-key extra fails closed without fallback

- GIVEN project file `{"schema":"gentle-pi.background-subagents/v1","policy":"on","extra":1}`
- WHEN resolving
- THEN it MUST return `policy off` `malformed true` `source project_file` and MUST NOT consult global/env

#### Scenario: Malformed JSON fails closed

- GIVEN project file contains `{bad`
- WHEN resolving
- THEN it MUST return `policy off` `malformed true` and MUST NOT fall back

#### Scenario: Global beats env when project absent

- GIVEN no project file, global `{"schema":"gentle-pi.background-subagents/v1","policy":"off"}` and env `on`
- WHEN resolving
- THEN it MUST return `policy off` `source global_file`

#### Scenario: Env fallback and default

- GIVEN no project/global and env `on`
- WHEN resolving
- THEN it MUST return `policy on` `source environment`; with no env → `policy off` `source default`

### Requirement: Background Policy Delegate and Reporting

The system MUST own resolution in `internal/sdd/background.go`; `internal/opencode/background.go` `BackgroundPolicy`/`BackgroundResolution` and `internal/agents/pi/adapter.go` MUST be thin delegates to `sdd` resolver. `renderBackgroundSubagentsReport` MUST expose `source`, `policy`, `capability`, `malformed`, `projectFile`/`globalFile` existence and `envValue`, and `loadBackgroundSubagentsPolicy` MUST delegate to `resolve().policy`. Unknown env values MUST be reported as ignored; `wrote` outranked by project file MUST warn.
(Previously: pi adapter owned resolution; opencode recomputed sources)

#### Scenario: Delegate preserves policy
- GIVEN `sdd` resolver returns `on` from project
- WHEN `opencode/background.go` `BackgroundPolicy` called
- THEN it MUST return same `on` without recomputing sources

#### Scenario: Report renders source and malformed
- GIVEN resolution `{source:"project_file",policy:"off",malformed:true}`
- WHEN `renderBackgroundSubagentsReport` called with `capability ready`
- THEN output MUST contain `source=project_file`, `policy=off`, `malformed=true`, and `capability: ready`

#### Scenario: Pi adapter delegates
- GIVEN `pi` `ResolveBackgroundSubagentsPolicy` called
- WHEN invoked
- THEN it MUST delegate to `sdd` resolver and preserve precedence

### Requirement: Background Capability Probe and Disabled Reporting

The system MUST compute `capability` as `ready` when the deployed subagent-runtime marker (runtime extension under `~/.pi/agent/extensions/`) is present, else `absent`; third-party `pi-subagents*` package presence alone MUST NOT yield `ready`. Status line MUST be `background subagents: <policy> (decided by <source>; capability: <capability>)` with `disabled|unmanaged` reporting when policy `off` or capability absent. `BIGGZ_BACKGROUND_SUBAGENTS` takes precedence over `GENTLE_PI_BACKGROUND_SUBAGENTS` when both set.
(Previously: `ready` when `subagent_run` tool or `pi-subagents` package was present.)

#### Scenario: Capability ready when runtime marker present

- GIVEN the deployed subagent-runtime marker present
- WHEN the capability probe runs
- THEN it MUST return `ready`

#### Scenario: Capability absent without runtime marker

- GIVEN no runtime marker present
- WHEN the probe runs
- THEN it MUST return `absent` and background launches MUST be inert

#### Scenario: Legacy package alone is not ready

- GIVEN `pi-subagents-j0k3r` installed but no runtime marker
- WHEN the probe runs
- THEN it MUST return `absent`

#### Scenario: Disabled reporting when policy off

- GIVEN `policy off` `capability absent`
- WHEN status line rendered
- THEN it MUST contain `policy: off` and `capability: absent` and `disabled/unmanaged` notice

### Requirement: Successor Advance

Un settle `passed` dentro de presupuesto MUST completar solo su work unit, no el change; un `acquire` con otro `--work-unit` MUST abrir un sucesor que preserve todo intento previo, con presupuesto fresco propio y sin `reset`.

#### Scenario: Advance

- GIVEN settle `passed` dentro de presupuesto
- WHEN un acquire nombra otro `--work-unit`
- THEN el sucesor procede, cadena intacta

#### Scenario: Fresh budget

- GIVEN predecesor `--max-attempts 1`
- WHEN el sucesor pide `--max-attempts 5`
- THEN presupuesto MUST ser 5; el acumulado reinicia

### Requirement: Repeated Work Unit Refusal

Repetir el MISMO `--work-unit` tras un settle `passed` MUST NOT admitirse; el rechazo MUST nombrar al sucesor de `--work-unit` distinto, nunca `reset` como única salida.

#### Scenario: Repeat refused

- GIVEN objetivo completo
- WHEN se repite el mismo `--work-unit`
- THEN rechazado; sucesor nombrado

#### Scenario: Ledger intact

- GIVEN el rechazo
- WHEN retorna
- THEN el ledger MUST quedar intacto

### Requirement: Scope-Change Guard

Sobre un objetivo no completo, cambiar `--work-unit`, `--evidence-goal`, `--max-attempts` o `--max-lines` MUST rechazarse sin reset explícito; una sola regla unificada.

#### Scenario: Any field refused

- GIVEN objetivo abierto
- WHEN cambia cualquiera de los cuatro
- THEN rechazado sin reset

#### Scenario: Unchanged scope

- GIVEN objetivo abierto
- WHEN el scope repite igual
- THEN admitido sin reset

### Requirement: Reset Preserves the Audit

`Reset` MUST limpiar el objetivo vivo (`Complete`, `DecisionRequired`, attempt activo, evidencia) sin mutar intentos previos; el conteo de intentos MUST nunca decrecer.

#### Scenario: Live state cleared

- GIVEN objetivo completo, intentos asentados
- WHEN corre `reset`
- THEN el estado vivo MUST limpiarse

#### Scenario: Attempts survive

- GIVEN N intentos
- WHEN `reset` completa
- THEN los N MUST permanecer, evidencia intacta

### Requirement: Reset Opens a Fresh Budget Epoch

`Reset` MUST abrir para el objetivo vivo una época de presupuesto fresca: conteo y cap `2×MaxAttempts` reinician, sin reducir auditoría ni lifetime. Tras `reset`, el MISMO `--work-unit` MUST admitirse con el presupuesto declarado en su request, sin `invalid_continuation` («budget changed without reset») ni heredar exhaustión; un sucesor MUST NOT heredar exhaustión ni cap del predecesor; `reset` MUST ser explícito, nunca automático.

#### Scenario: Fresh epoch

- GIVEN presupuesto y cap agotados
- WHEN corre `reset`
- THEN conteo vivo reinicia; cadena y evidencia intactas

#### Scenario: Same work unit admitted

- GIVEN exhaustión previa tras `reset`
- WHEN re-adquiere el MISMO `--work-unit`
- THEN admitido con su presupuesto; nunca `invalid_continuation`

#### Scenario: Successor not gated

- GIVEN predecesor agotado tras `reset`
- WHEN `acquire` nombra otro `--work-unit`
- THEN admitido sin heredar exhaustión ni cap

#### Scenario: Never automatic

- GIVEN presupuesto agotado
- WHEN el ciclo continúa
- THEN `reset` MUST NOT dispararse solo

### Requirement: Generation Attribution

Cada attempt MUST registrar su generación de objetivo, manteniendo atribuibles los intentos reemplazados.

#### Scenario: New generation

- GIVEN avance de generación
- WHEN registra el attempt sucesor
- THEN generación 2 adjunta

#### Scenario: Kept generation

- GIVEN el mismo avance
- WHEN se leen predecesores
- THEN siguen en generación 1

### Requirement: Lifetime Accounting

`LifetimeAttempts`/`LifetimeChangedLines` MUST sobrevivir `advance` y `reset`, verse en status, y nunca re-acreditarse; los acumulados por generación MUST reiniciar con el sucesor.

#### Scenario: Survives advance

- GIVEN dos attempts entregados en generación 1
- WHEN entrega el attempt sucesor
- THEN status muestra `LifetimeAttempts` 3

#### Scenario: Survives reset

- GIVEN lifetime en 3
- WHEN corre `reset`
- THEN lifetime MUST quedar en 3

### Requirement: Pre-Change Ledger Compatibility

Un registro `complete: true` sin generación MUST cargar, conservar su verificación content-address y avanzar; la generación cero MUST renderizarse como 1 solo en vistas derivadas, nunca en bytes canónicos.

#### Scenario: Legacy advances

- GIVEN registro `complete: true` sin generación
- WHEN se adquiere otro `--work-unit`
- THEN carga, verifica, avanza

#### Scenario: Derived default

- GIVEN registro sin generación
- WHEN el status la deriva
- THEN muestra 1; bytes sin cambios

### Requirement: Remediation Admission

Un `acquire` que declara `--remediates-evidence-revision` sobre evidencia fallida sin remediar MUST seguir admitiéndose, incluso con un avance de generación de por medio.

#### Scenario: Corrective admitted

- GIVEN evidencia fallida sin remediar
- WHEN el acquire correctivo la declara
- THEN admitido, incluso tras avance

#### Scenario: Remediated refused

- GIVEN evidencia ya remediada
- WHEN el acquire correctivo la declara
- THEN rechazado

### Requirement: No New CLI Surface

El avance MUST ser alcanzable vía la forma posicional existente de `sdd-attempt acquire` y sus flags; sin verbo ni flag nuevos.

#### Scenario: Existing surface

- GIVEN la invocación documentada `acquire <change> --work-unit ...`
- WHEN nombra otro `--work-unit`
- THEN el avance procede tal cual

#### Scenario: No new verb

- GIVEN la superficie de `sdd-attempt`
- WHEN se enumeran verbos y flags
- THEN el avance no agrega nada
