# Plugin System Specification

## Purpose

The Plugin System domain defines the plugin interfaces and registry for extending review capabilities. It specifies LensPlugin for analysis, ProviderPlugin for external service integration, the build-time Registry, and the Pipeline execution model with rollback support.

## Requirements

### Requirement: LensPlugin Interface

The system MUST define a LensPlugin interface with ID() returning string, Name() returning string, Version() returning string, Analyze(ctx, subject) returning LensResult, and Policies() returning []Policy.

#### Scenario: Happy path — lens analysis

- GIVEN a LensPlugin registered with ID "dummy-lens" and a valid ReviewSubject
- WHEN Analyze is called with the subject
- THEN the LensResult MUST contain findings relevant to the subject
- AND Policies() MUST return at least one Policy

#### Scenario: Invalid subject

- GIVEN a LensPlugin
- WHEN Analyze is called with a nil or empty subject
- THEN the plugin MUST return an error
- AND NOT panic or hang

### Requirement: ProviderPlugin Interface

The system MUST define a ProviderPlugin interface with ID() returning string, Name() returning string, Capabilities() returning []string, and Execute(ctx, req) returning ProviderResponse.

#### Scenario: Happy path — provider execution

- GIVEN a ProviderPlugin registered with ID "mock-provider" and a valid request
- WHEN Execute is called with the request
- THEN the ProviderResponse MUST contain results
- AND the provider MUST indicate which capability was used

#### Scenario: Unknown capability request

- GIVEN a ProviderPlugin with Capabilities() returning ["code-review"]
- WHEN Execute is called with a request requiring ["deploy"]
- THEN the provider SHOULD return an error indicating the capability is not supported

### Requirement: Build-Time Registry

The system MUST provide a Registry with RegisterLens(plugin), RegisterProvider(plugin), GetLens(id), and GetProvider(id) methods. Registration MUST happen at build time via explicit wiring. The Registry MUST NOT support dynamic loading.

#### Scenario: Happy path — register and retrieve

- GIVEN an empty Registry
- WHEN a LensPlugin with ID "dummy-lens" is registered via RegisterLens
- THEN GetLens("dummy-lens") MUST return the registered plugin
- AND GetLens("unknown") MUST return nil

#### Scenario: Duplicate registration

- GIVEN a Registry with a registered LensPlugin "dummy-lens"
- WHEN RegisterLens is called again with the same ID "dummy-lens"
- THEN the Registry MUST return an error or replace the existing registration
- AND the behavior MUST be documented and consistent

### Requirement: Pipeline Stage Execution

The system MUST define a Stage interface with Name() returning string, Execute(ctx, state) returning error, and Rollback(ctx, state) returning error. Pipeline MUST execute stages sequentially and run reverse-ordered rollback on any stage failure.

#### Scenario: Happy path — all stages succeed

- GIVEN a Pipeline with three registered stages: A, B, C
- WHEN Execute is called on a ReviewState
- THEN stage A MUST run, then B, then C
- AND Rollback MUST NOT be called on any stage
- AND the ReviewState MUST be updated by all stages

#### Scenario: Stage failure triggers rollback

- GIVEN a Pipeline with three stages: A, B, C
- WHEN stage B fails with an error
- THEN stage B's Rollback MUST be called
- AND stage A's Rollback MUST be called
- AND stage C's Rollback MUST NOT be called (stage C did not execute)
- AND the pipeline MUST return the error from stage B

### Requirement: Orchestrator

The system MUST define an Orchestrator with a single Execute(ctx, subject) method that runs the full pipeline and returns *ReviewState and error.

#### Scenario: Happy path — full execution

- GIVEN an Orchestrator with a configured pipeline and registry
- WHEN Execute is called with a valid ReviewSubject
- THEN a *ReviewState MUST be returned with Status set to Completed
- AND the evidence chain MUST contain entries from each pipeline stage

#### Scenario: Pipeline failure

- GIVEN an Orchestrator with a configured pipeline
- WHEN Execute is called and a stage fails
- THEN *ReviewState MUST be returned with Status set to Failed
- AND the error MUST be non-nil
