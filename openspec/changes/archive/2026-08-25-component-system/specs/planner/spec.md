# Planner Specification

## Purpose

The Planner provides graph-based dependency resolution for component and skill installation. It uses a directed graph, a resolver that computes installation order, topological sort for cycle-tolerant ordering, and soft ordering that warns on cycles instead of failing. This is a direct port of gentle-ai's planner with simplified types.

## Requirements

### Requirement: Graph Node and Edge Definition

The system MUST define a `Graph` type that supports adding nodes (by string ID) and directed edges (from → to). The Graph MUST reject duplicate edges and MUST allow adding edges to already-registered nodes only.

#### Scenario: Happy path — build a valid graph

- GIVEN an empty Graph
- WHEN node A and node B are added, and an edge A→B is added
- THEN the Graph MUST contain exactly 2 nodes
- AND the edge A→B MUST be present in the adjacency list

#### Scenario: Orphan edge rejected

- GIVEN an empty Graph
- WHEN an edge A→B is added before node B exists
- THEN the Graph MUST return an error
- AND the edge MUST NOT be stored

### Requirement: Dependency Resolver

The system MUST define a `Resolver` that accepts a Graph and a set of target node IDs, and returns an ordered slice of node IDs representing the installation sequence. Dependencies MUST appear before their dependents.

#### Scenario: Happy path — linear dependency

- GIVEN a Graph with edges A→B, B→C
- WHEN Resolve({A}) is called
- THEN the result MUST order C, B, A (dependencies first)
- AND the result MUST contain exactly 3 nodes

#### Scenario: Diamond dependency

- GIVEN a Graph with edges A→B, A→C, B→D, C→D
- WHEN Resolve({A}) is called
- THEN D MUST appear before B and C
- AND both B and C MUST appear before A

### Requirement: Topological Sort with Soft Ordering

The system MUST implement `TopologicalSort(graph)` that returns an ordered node list. If a cycle is detected, the system MUST NOT panic — instead it MUST return the nodes in best-effort order and signal that a cycle was found via a separate error or warning return.

#### Scenario: Happy path — acyclic sort

- GIVEN a Graph with edges A→B, B→C (acyclic)
- WHEN TopologicalSort is called
- THEN the result MUST list C, B, A
- AND no cycle warning MUST be returned

#### Scenario: Cycle tolerated without panic

- GIVEN a Graph with a cycle A→B, B→C, C→A
- WHEN TopologicalSort is called
- THEN the function MUST NOT panic
- AND it MUST return a best-effort ordering
- AND it MUST signal that a cycle was detected

### Requirement: Planner Orchestration

The system MUST define a `Planner` struct that combines a Graph and Resolver. The Planner MUST expose a `Plan(targets []string)` method that returns a slice of node IDs and an error.

#### Scenario: Happy path — full plan

- GIVEN a Graph with 5 nodes in a tree structure, and a Planner wrapping it
- WHEN Plan({root}) is called
- THEN the result MUST be a valid topological order of all 5 nodes
- AND dependencies MUST precede dependents throughout
