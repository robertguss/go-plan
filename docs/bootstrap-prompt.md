# Bootstrap prompt: design and implement `go-plan`

You are starting a new product-design and implementation session in:

`/Users/robertguss/Projects/startups/go-plan`

The goal is to create a reusable, project-agnostic Go CLI for Git-native project
planning that can be used across all of my coding projects going forward.

This is a conversation-first task. Do not begin implementation merely because
this prompt contains technical detail. Start by understanding the existing
framework and then help me decide what the new product should be.

## Source and target boundaries

The planning framework currently lives inside this read-only reference
repository:

`/Users/robertguss/Projects/startups/rust-and-beam-os`

Treat that repository as evidence and prior art. Do not modify it, migrate it,
or remove its Python tooling. `rust-and-beam-os` will continue using its current
framework unchanged. Make every new document, source file, test, and build
artifact for this effort only in `go-plan`.

The Go product may redesign the current CLI, schema, file layout, terminology,
and behavior. Backward compatibility with the Python implementation is not an
assumption. Compatibility and migration are product decisions for us to discuss.

Before acting, inspect the current state of both repositories, including any
`AGENTS.md` instructions. Preserve user-authored or unrelated changes.

## Interaction contract

1. Inspect the reference implementation and summarize the important mechanisms,
   invariants, strengths, accidental complexity, and unresolved design choices.
2. Discuss the Go CLI with me interactively. Ask only one question at a time.
3. Continue until you are at least 95% confident about the intended product,
   initial release boundary, and implementation approach.
4. Once we agree, write a product specification and an ordered, verifiable
   implementation plan into `go-plan`.
5. Present those documents for my explicit approval.
6. Begin implementation only after I approve the specification and plan.
7. After approval, implement incrementally with tests, verification, and small
   reviewable commits. Continue through the agreed initial release scope.

Read-only investigation and drafting discussion notes are allowed before
approval. Product scaffolding and implementation code wait for approval.

## Authoritative reference material

Start with these files in `rust-and-beam-os` rather than broadly reading
unrelated operating-system content:

- `docs/specs/repo-plan-v1.md` — format, authority model, readiness semantics,
  commands, safety boundaries, and success criteria.
- `docs/plan/README.md` and `docs/plan/execution-policy.md` — how humans and
  coding agents execute a plan.
- `scripts/repo_plan.py` — current reference CLI.
- `scripts/plan_tool.py` — legacy import behavior and historical model.
- `templates/repo-plan/` — versioned record and scaffold templates.
- `tests/test_repo_plan.py` and `tests/test_plan_tool.py` — executable contracts,
  edge cases, transaction behavior, and regression history.
- `docs/evidence/phase-0/repo-plan-hardening/verification.md`
- `docs/evidence/phase-0/repo-plan-lifecycle/verification.md`
- `docs/evidence/phase-0/plan-consistency/verification.md`

The most relevant framework-remediation commits are:

- `8d72669` — parser, validation, path, symlink, evidence, date, template, and
  atomic-write hardening.
- `3ea7a17` — executable epic, dependency, rejected-gate, and lifecycle rules.
- `905f78f` — metadata/prose consistency, link validation, and plan-contract
  reconciliation.

Use the current checkout as the source of truth if it has advanced beyond those
commits.

## Existing capability baseline

The Python framework is currently a standard-library-only, offline CLI with
these commands:

- `init`
- `new` for task, epic, milestone, gate, and decision records
- `build`
- `check`
- `ready`
- `migrate-legacy`
- `normalize-legacy-refs`

Its present model includes:

- human-readable canonical Markdown records with constrained YAML frontmatter;
- versioned templates and deterministic, byte-identical generation;
- disposable JSON index, graph, and ready-work projections;
- typed IDs and explicit blocking, parent, related, milestone, gate, and
  evidence relationships;
- organizational epics that never become executable blockers;
- deterministic readiness ordering with explainable factors;
- explicit evaluation dates for deferred work;
- human-only gate decisions, immutable rejected decisions, and replacement
  gates for later reconsideration;
- repository-local evidence and Markdown-link validation;
- strict parsing and field/type validation;
- cycle, state, ownership, dependency, and generated-freshness checks;
- managed-path containment, symlink rejection, no silent overwrite, atomic
  multi-file publication, and rollback on failure;
- deterministic legacy migration and extensive filesystem/CLI integration tests.

These are inputs to the design conversation, not requirements to clone blindly.
Separate durable product principles from constraints that only existed because
the first implementation was embedded in one Python repository.

## Design questions we should resolve

Guide the conversation through the decisions that materially affect the
product. Do not dump all questions at once; choose the highest-leverage open
question each turn. Topics should include, where relevant:

- the target users and core workflows for humans, coding agents, and automation;
- product and binary naming, command ergonomics, discoverability, and exit codes;
- plan-root discovery, repository configuration, defaults, and multi-plan repos;
- canonical data model, record types, lifecycle, readiness, and gate semantics;
- whether Markdown plus frontmatter remains the canonical format;
- schema evolution, compatibility guarantees, and migrations;
- stable ID generation and safe concurrent/distributed editing;
- template ownership, customization, and upgrade behavior;
- derived views, query/reporting needs, graph output, and machine-readable APIs;
- extension fields, plugins, issue trackers, and boundaries around networked
  integrations;
- deterministic output, time handling, Git awareness, and reproducibility;
- filesystem trust boundaries, symlinks, locking, atomic updates, rollback, and
  recovery after interruption;
- Go package architecture and the boundary between reusable library code and
  CLI presentation;
- dependency policy and whether a YAML library or other third-party packages
  are justified;
- cross-platform behavior on macOS, Linux, and Windows;
- test strategy: unit, golden, property/fuzz, filesystem fault injection,
  subprocess integration, and compatibility fixtures;
- installation and distribution through `go install`, release binaries,
  checksums, and possibly Homebrew;
- the smallest coherent initial release and which ideas belong in later phases.

Push back when a simpler or deeper design is available. Surface tradeoffs and
distinguish decisions required for the first release from intentionally deferred
options.

## Required pre-implementation artifacts

After the discussion reaches 95% confidence, propose the exact filenames and
write at least:

1. A product specification covering goals, non-goals, user workflows, command
   contract, data/authority model, invariants, failure behavior, compatibility
   policy, and acceptance criteria.
2. An implementation plan that decomposes the approved initial release into
   dependency-ordered, testable slices with a verification command and completion
   criterion for every slice.

Add architecture decisions when a consequential choice needs durable rationale.
Keep unknowns explicit instead of silently deciding them in the documents.
Stop after presenting the artifacts and request my approval before coding.

## Implementation expectations after approval

Use the approved specification as the boundary. Build the minimum coherent Go
CLI, not speculative extensions. Prefer a clean library/CLI separation, explicit
errors, deterministic behavior, and transaction-safe filesystem operations.
Drive behavior with tests, including subprocess-level command tests and
adversarial filesystem cases proportional to the risks discovered in the
Python implementation. Verify documentation and release/install instructions as
part of completion.

Do not claim parity, compatibility, safety, or release readiness without
evidence. Do not modify `rust-and-beam-os` at any point in this effort.

## First response

Do not write code. Briefly restate your understanding of the product and the
read-only source/target boundary, then ask me the single highest-leverage first
question for the design conversation.
