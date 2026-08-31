# Implementation plan: Compatibility fixture

## Approach

Load the frozen canonical files.

## Architecture

Use the public CLI behavior.

## Technology and dependencies

Use the v1 parser.

## Interfaces and data flow

Files flow through workspace loading into status output.

## Change surface

The compatibility fixture only.

## Verification strategy

Compare stable JSON output.

## Decisions and tradeoffs

Keep a small representative fixture.

## Risks and recovery

An incompatible change fails the golden test.

## Out of scope

Future major schemas.
