# Pre-Proposal: fix-attempt-ledger-scope

```yaml
schema: biggz-ai.sdd-preproposal/v1
revision: 1
change: fix-attempt-ledger-scope
exploration_outcome: done
exploration_ref: openspec/changes/fix-attempt-ledger-scope/exploration.md
research:
  selected: false
  classes: []
  reason: >-
    La evidencia que faltaba es código de referencia LOCAL (los clones gentle-ai y
    gentle-pi), no fuentes externas. La lane sdd-research solo admite los grants
    `documentation`/`open-web` y exige sources con id/class/publisher/URL/accessed_at;
    meter rutas locales habría producido un artefacto no conforme y bloqueado la
    propuesta. La comparación se hizo como addendum §12 de la exploración.
  evidence_refs:
    - "GA/internal/sddstatus/runtime_ledger.go:238-266 (RuntimeObjective)"
    - "GA/internal/sddstatus/runtime_admission.go:70-167 (admission, advance, scope-changed)"
    - "GA/internal/sddstatus/runtime_objective_advance_test.go:68-226 (spec de avance)"
    - "GA/internal/sddstatus/runtime_ledger_reset_test.go:70-95 (reset conserva la cadena)"
    - "GA/internal/sddstatus/budget_consent.go:92-107 (presupuesto por objetivo)"
product_decisions: confirmed
decisions:
  - id: D1
    topic: Alcance del port
    decision: >-
      Port semántico COMPLETO del modelo upstream: avance por work unit distinto tras
      passed-en-presupuesto (`objective/advance`) conservando la cadena de intentos;
      `Reset` que PRESERVA los intentos; `ObjectiveGeneration` persistida en cada
      intento; contadores `Lifetime*` a nivel change; presupuesto fresco para el
      sucesor tomado de su propio request.
  - id: D2
    topic: Superficie CLI
    decision: >-
      Sin verbo nuevo. El avance ES `sdd-attempt acquire <change> --work-unit
      <label-distinto>` reusando el shape posicional actual. Solo cambian los mensajes
      de bloqueo (`ledger is complete; reset required to continue` debe pasar a nombrar
      el sucesor, como `ErrRuntimeObjectiveDone`).
  - id: D3
    topic: Compatibilidad con ledgers existentes
    decision: >-
      Los ledgers ya emitidos con `complete: true` deben poder avanzar: cumplen el
      predicado de avance (Complete ∧ sin intento activo ∧ intentos>0 ∧ último passed ∧
      label distinto). Requiere tratar `generation` cero como 1 y que todo campo nuevo
      sea `omitempty` (regla CAS: un campo no-omitempty o un default aplicado en lectura
      invalida la dirección de contenido de TODOS los records).
proposal_ready: true
```

## Evidencia de la que cuelga la propuesta

- **Problema reproducido con el binario real** (no inferido): `settle --outcome passed` devuelve `complete: true` con `remaining_attempts: 2`; el siguiente `acquire` con otro work unit muere en `blocked(corrupt_authority)`; con `--outcome progress` muere igual en `blocked(invalid_continuation)`; `reset` reporta `Previous attempts cleared: N` y deja `Attempts: 0`.
- **Diagnóstico**: en biggz-ai, `work-unit` es la identidad de scope de toda la generación y `Complete` es terminal para el change. En gentle-ai, el objetivo es el scope de UN work unit, `Complete` es terminal solo para ese scope, y el sucesor se abre con un work unit distinto preservando la cadena.
- **Los dos defectos de port con nombre y línea**: `Reset` hace `store.Attempts = nil` (`internal/sddattempt/sddattempt.go:1020`) donde upstream conserva la cadena; y la rama `if store.Complete` de `deriveAdmissionBlocked` (`:453-455`) produce `corrupt_authority` donde upstream abre el sucesor.
- **Conclusión de seguridad**: desacoplar `passed` de la terminalidad del change NO toca la ruta de remediation (la admisión correctiva en `:1316-1345` depende solo de un fallo sin remediar, nunca de un `passed` previo).

## Restricciones para la fase de propuesta

- No entrevistar ni inferir: las decisiones D1-D3 están cerradas.
- No tocar el `Rescope` de librería (`internal/sddattempt/sddattempt.go:2043-2170`): ya diverge del upstream con reglas widened/exhausted propias y tiene tests que lo fijan (`rescope_test.go:9-58`).
- El esquema CAS de biggz-ai (`biggz/sdd-runtime/v1`, request digests propios) no es el `runtimeRecord` upstream: el avance debe ser un tipo de registro nuevo en la codificación de biggz-ai, no una copia.
- La spec mínima a replicar es la lista de asserts de `GA/internal/sddstatus/runtime_objective_advance_test.go`.
