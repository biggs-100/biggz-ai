# Delta for runtime

## ADDED Requirements

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

## MODIFIED Requirements

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
