# Product specification: Compatibility fixture

## Objective

Preserve the v1 reader contract.

## Context

This is a frozen compatibility plan.

## Users and workflows

Users read the fixture with a current binary.

## Goals

Retain same-major compatibility.

## Non-goals

Testing schema migration.

## Assumptions

The fixture is copied into a Git worktree.

## Requirements

The current binary reads every canonical file.

## Constraints

No network access.

## Acceptance criteria

- AC-001: The current binary reads this fixture.

## Open questions

None.
