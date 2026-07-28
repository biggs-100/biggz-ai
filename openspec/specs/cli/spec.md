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
