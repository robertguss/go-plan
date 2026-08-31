# Product specification: go-plan v1

Status: Approved
Date: 2026-08-31
Product name: `go-plan`
Executable name: `gp`

## Objective

`go-plan` is a reusable, project-agnostic Go CLI that installs and enforces a
Git-native planning framework for coding projects. Humans, LLMs, and coding
agents use `gp` to create an active plan, author its documents, approve it,
select the next task, record execution progress, validate completion, and
remove the plan when it is finished.

`gp` is deterministic and offline. It does not invoke an LLM, run project
verification commands, use a hosted service, or require a network after the
binary has been installed. The calling human or agent supplies the judgment;
the CLI supplies the structure, lifecycle, validation, and filesystem safety.

The repository working tree contains only the current plan. Git history is the
archive. A completed plan is deliberately removed before the next plan is
initialized so stale planning documents do not confuse future agents.

## Capability map

| Module id | Responsibility | Depends on |
|---|---|---|
| `format` | Canonical plan metadata, Markdown contracts, task sequence, approval digest, and validation rules | — |
| `workspace` | Git-root discovery, managed paths, embedded templates, `AGENTS.md` integration, locking, transactions, and removal safety | `format` |
| `workflow` | Plan approval, derived status, task lifecycle, revision rules, readiness, and graph queries | `format`, `workspace` |
| `cli` | Cobra command tree, help, human output, stable JSON output, and exit behavior | `workflow` |

Build order: `format` → `workspace` → `workflow` → `cli`.

## Goals

- Standardize how coding plans are specified, decomposed, approved, executed,
  verified, and retired across repositories.
- Give humans and agents one concise, non-interactive CLI for plan-wide and
  task lifecycle operations.
- Keep canonical planning inputs human-readable and directly editable.
- Make task execution strictly sequential and unambiguous.
- Prevent implementation from starting against an unapproved or silently
  modified plan.
- Require acceptance criteria, declared verification, and recorded evidence
  before a task can be completed.
- Make all derived status, readiness, and graph views live computations so
  stale projections cannot become context.
- Make initialization, mutation, renumbering, and removal safe against partial
  writes, symlinks, path escape, and accidental overwrite.
- Remain compatible with plans created by any `gp` binary implementing the same
  major schema, `go-plan/v1`.

## Non-goals

- Invoking, embedding, choosing, or authenticating an LLM.
- Running build, test, lint, deployment, or task verification commands.
- Parallel task execution, ownership claims, or distributed locking.
- Multiple active or named plans in one repository.
- Epics, milestones, gates, or separate decision records.
- Hosted synchronization, databases, issue trackers, or network integrations.
- Persisted generated indexes, ready lists, graphs, reports, or caches.
- Template customization, plugins, schema migration, or in-place framework
  upgrades. A plan may be removed and initialized again instead.
- Automatic Git staging, commits, branch operations, fetches, or pushes.
- Windows support or Windows-specific compatibility code.
- Release archives, checksums, package managers, or Homebrew distribution in
  v1. Installation is through `go install`.
- Permanent architecture records inside the disposable plan. Durable ADRs
  belong to the host repository outside `.go-plan/`.

## Users and workflows

### Human or agent creates a plan

1. From anywhere inside a Git worktree, run:

   ```sh
   gp init --title "Add offline project planning"
   ```

2. `gp` discovers the Git root, creates `.go-plan/`, adds a managed block to
   root `AGENTS.md`, and reports the created canonical files.
3. The human or agent authors the specification and implementation plan and
   creates the ordered tasks.
4. Run `gp check` until the plan is structurally complete and every
   specification acceptance criterion is covered by at least one task.
5. Run `gp approve`. Humans and agents are equally permitted to approve.

### Agent executes the plan

1. Run `gp check` and `gp status`.
2. Run `gp ready` to obtain the only task that may start, or no task when the
   plan is blocked, under review, active, or complete.
3. Run `gp task start T-001`.
4. Implement the task and independently run the task's declared verification
   commands or arrange review by another model.
5. Check every deliverable and acceptance criterion and record verification
   results or evidence in the task document.
6. Run `gp task complete T-001`.
7. Repeat for the next numeric task.

### Human or agent revises a plan

1. Modify the specification, implementation plan, current task definition, or
   unfinished task suffix.
2. Use task mutation commands for additions, removals, or reordering. Task
   renumbering and reference updates are transactional.
3. `gp` reports that approval is stale because planning content no longer
   matches the recorded digest.
4. Run `gp check`, review the revised plan, and run `gp approve` again.
5. Completed tasks remain immutable; changed requirements become new later
   tasks.

### Human or agent retires a completed plan

1. Commit the completed plan so Git contains its history.
2. Run `gp remove --dry-run` and inspect the exact managed changes.
3. Run `gp remove --yes`.
4. `gp` removes `.go-plan/` and only its marked block in root `AGENTS.md`.
5. Commit the removal. A later `gp init` starts a new plan without stale files.

## Canonical repository layout

```text
<git-root>/
├── AGENTS.md
└── .go-plan/
    ├── plan.yaml
    ├── specification.md
    ├── implementation-plan.md
    └── tasks/
        ├── t-001.md
        ├── t-002.md
        └── ...
```

There is no `generated/` directory. Commands compute status, readiness, graph,
and JSON results directly from canonical inputs on every invocation.

### Authority model

Canonical planning inputs are:

- `.go-plan/plan.yaml`;
- `.go-plan/specification.md`;
- `.go-plan/implementation-plan.md`;
- `.go-plan/tasks/t-NNN.md`.

The managed `AGENTS.md` block is an installed instruction surface, not plan
content. Human and JSON command output are projections and are never writable
authorities.

The binary embeds the authoritative v1 schemas, default templates, and managed
agent instructions. A repository does not contain a customizable template
copy. Generated canonical files become ordinary Git-tracked plan inputs after
initialization.

## Plan metadata

`.go-plan/plan.yaml` contains CLI-managed metadata equivalent to:

```yaml
schema: "go-plan/v1"
title: "Add offline project planning"
approval_digest: null
```

The exact serialized field order is stable. Unknown fields are rejected in v1.
No timestamp, hostname, username, or wall-clock value is written to canonical
files.

`approval_digest` is `null` until approval. `gp approve` records a SHA-256
digest over approval-bound planning content. The approval record does not
contain an approver identity, signature, decision document, or audit metadata.

## Specification contract

`.go-plan/specification.md` is canonical Markdown with these required
second-level headings in order:

1. `Objective`
2. `Context`
3. `Users and workflows`
4. `Goals`
5. `Non-goals`
6. `Assumptions`
7. `Requirements`
8. `Constraints`
9. `Acceptance criteria`
10. `Open questions`

Acceptance criteria use stable, unique identifiers in document order, following
this pattern:

```markdown
- AC-NNN: State one observable completion condition.
```

`gp approve` rejects missing sections, template placeholders, duplicate or
malformed acceptance IDs, and unresolved open questions. A resolved document
uses `None.` in `Open questions`.

## Implementation-plan contract

`.go-plan/implementation-plan.md` is canonical Markdown with these required
second-level headings in order:

1. `Approach`
2. `Architecture`
3. `Technology and dependencies`
4. `Interfaces and data flow`
5. `Change surface`
6. `Verification strategy`
7. `Decisions and tradeoffs`
8. `Risks and recovery`
9. `Out of scope`

The document explains the solution and the task files provide the executable
decomposition. It does not duplicate a manually maintained task index.

Architecture or technology constraints that define required behavior remain in
the specification. Solution choices, alternatives, and rationale live in the
implementation plan. If a decision must outlive the active plan, a task creates
or updates a project-owned ADR outside `.go-plan/`.

## Task contract

Each task is one Markdown file named after its numeric ID. Task frontmatter is
equivalent to:

```yaml
---
schema: "go-plan/v1"
id: "T-001"
title: "Initialize the Go module and CLI"
state: "open"
covers:
  - "AC-001"
---
```

`covers` may be empty for necessary enabling work. Before approval, every
specification acceptance criterion must be referenced by at least one task.
Every non-empty `covers` entry must reference an existing acceptance criterion.

Task bodies contain these required second-level headings in order:

1. `Goal`
2. `Context`
3. `Deliverables`
4. `Acceptance criteria`
5. `Verification`
6. `Evidence`
7. `Out of scope`

Deliverables and task acceptance criteria are Markdown checklists. Verification
contains exact commands or explicit manual/reviewer steps authored before plan
approval. Evidence is populated during execution with a concise record of what
was run, the result, and any repository-relative supporting links.

## Sequence and dependency model

The task graph is a linear DAG represented entirely by contiguous numeric IDs:

```text
T-001 → T-002 → T-003 → ... → T-NNN
```

- The numeric ID is both stable identity within the current sequence and the
  authoritative execution order.
- IDs begin at `T-001`, are zero-padded to at least three digits, and contain no
  gaps.
- There is no separate `order`, `priority`, `depends_on`, `owner`, milestone, or
  parent field.
- Every task implicitly depends on successful completion of its immediate
  predecessor.
- At most one task may be `in_progress`.
- Later tasks cannot start early for any reason.
- `gp ready` returns either the first open task or no task. It returns no task
  when an earlier task is active, the plan requires approval, or the plan is
  complete.

### Revision and renumbering

- Completed tasks form an immutable prefix.
- The current `in_progress` task retains its ID during suffix reordering.
- Open tasks may be added, edited, removed, or reordered.
- `gp task add --after T-NNN`, `gp task remove`, and `gp task reorder` renumber
  the affected open suffix and update exact task references inside `.go-plan/`.
- Renumbering is previewable and transactional.
- Any change to approval-bound content invalidates approval.
- New requirements affecting completed work are implemented by appending a new
  open task, not by rewriting completed records.

## Approval model

Approval is caller-neutral. A human, LLM, or coding agent may run `gp approve`.
There are no gates, decision records, approver identities, or signed-commit
requirements.

The digest binds:

- the full specification and implementation plan;
- every task ID, title, coverage mapping, goal, context, deliverable text,
  acceptance-criterion text, verification instructions, and out-of-scope text;
- the ordered task set.

The digest excludes:

- `approval_digest` itself;
- task lifecycle state;
- checkbox completion markers while retaining their text;
- task evidence content.

This lets execution update state, checkboxes, and evidence without invalidating
approval while ensuring that a changed requirement, approach, task definition,
coverage mapping, or sequence requires reapproval.

## Derived lifecycle

Plan status is computed rather than stored:

- `draft`: required planning content or tasks are incomplete and no current
  approval exists;
- `review_required`: planning content is structurally valid but the approval
  digest is absent or stale;
- `approved`: planning content matches the approval digest and every task is
  open;
- `executing`: the plan is approved and at least one task is `in_progress` or
  done;
- `completed`: the plan is approved and every task is done.

Task state has exactly three values and transitions:

```text
open → in_progress → done
```

There is no cancelled or deferred state. Changed scope is handled by revising
and reapproving the unfinished plan.

## Verification and completion

`gp` never runs a task's project commands. The implementing LLM or human runs
verification, addresses failures, and may ask a separate reviewer or model to
verify the work.

`gp task complete T-NNN` succeeds only when:

- the plan has a current approval digest;
- the task is the single active task;
- all deliverable checkboxes are checked;
- all task acceptance-criterion checkboxes are checked;
- the `Verification` section is non-placeholder;
- the `Evidence` section is non-empty and non-placeholder;
- repository-local Markdown links in the task resolve safely.

The CLI validates structure and presence. It does not claim that the recorded
verification is truthful or technically sufficient.

## Command contract

The sole supported executable is `gp`. The repository and module remain named
`go-plan`.

### Plan-wide commands

| Command | Contract |
|---|---|
| `gp init --title <title>` | Initialize one draft plan at the discovered Git root; refuse an existing plan |
| `gp status` | Report derived plan state, counts, active task, approval freshness, and next task |
| `gp check` | Read-only validation of canonical files, links, sequence, lifecycle, approval, and managed integration |
| `gp approve` | Validate approval prerequisites and replace `approval_digest` transactionally |
| `gp ready` | Return the single open task that may start, or no task |
| `gp graph` | Render the live linear execution graph as deterministic ASCII or JSON |
| `gp remove` | Preview or remove the active plan and managed `AGENTS.md` block subject to safety rules |

### Task commands

| Command | Contract |
|---|---|
| `gp task add --title <title> [--cover AC-NNN]... [--after T-NNN]` | Append or insert the next numeric task from the embedded template |
| `gp task list` | List tasks in numeric order with state and coverage |
| `gp task show T-NNN` | Show one parsed task and its readiness/completion factors |
| `gp task start T-NNN` | Start only the first open task of a currently approved plan |
| `gp task complete T-NNN` | Complete only the active task after documentation checks pass |
| `gp task reorder --order T-NNN,T-NNN,...` | Preview or transactionally reorder and renumber the mutable suffix |
| `gp task remove T-NNN` | Preview or remove an open task and renumber the mutable suffix |

`--cover` is repeatable and may be omitted for an enabling task. `--after`
inserts after the identified task and is limited by completed-prefix and active-
task immutability. Without `--after`, `task add` appends. `task reorder --order`
accepts one comma-separated list containing every open task in the mutable
suffix exactly once and containing no completed or active task. No command
requires prompts, menus, arrow keys, a TTY, or timed interaction.

### Global behavior

- Commands operate from any path inside the repository.
- `--repo <path>` selects a repository explicitly for scripts and tests.
- Read commands support `--json` where machine-readable data is useful.
- Mutations support `--dry-run` when they affect multiple files or remove data.
- Destructive removal requires `--yes`; missing confirmation fails immediately.
- Help is layered. Every command and subcommand has examples with real
  invocations.
- Human success output is concise and includes useful IDs and paths.
- Runtime failures do not dump full usage text.
- Repeating an already-satisfied safe operation is either a no-op with an
  explicit result or a clear refusal; it never duplicates records or blocks.

### JSON compatibility

JSON is a supported `go-plan/v1` interface, not best-effort presentation.
Field names and meanings remain backward-compatible throughout v1. JSON output
contains a schema/version discriminator and never relies on color or decorative
text. When `--json` is present, errors are structured and paired with a nonzero
process status.

Every JSON response is one object followed by a newline. A successful response
has exactly these envelope fields:

```json
{
  "schema": "go-plan/v1",
  "command": "ready",
  "ok": true,
  "result": {
    "task": null,
    "reason": "plan_completed"
  }
}
```

An error response replaces `result` with an `error` object:

```json
{
  "schema": "go-plan/v1",
  "command": "check",
  "ok": false,
  "error": {
    "code": "plan_invalid",
    "message": "plan validation failed",
    "details": [
      {
        "path": ".go-plan/tasks/t-001.md",
        "field": "state",
        "message": "expected open, in_progress, or done"
      }
    ]
  }
}
```

`schema`, `command`, `ok`, and either `result` or `error` are always present.
`error.code` is a stable machine-readable snake-case identifier;
`error.message` is concise human text; and `error.details` is always an array,
possibly empty, ordered deterministically. Result fields are command-specific
and frozen by golden compatibility fixtures. A valid query with no ready task
is a successful result with a null `task` and a machine-readable `reason`, not
an error.

Exit codes are fixed for v1:

| Code | Meaning |
|---|---|
| `0` | The command completed successfully, including an idempotent no-op or a valid query with no ready task |
| `1` | A plan/domain refusal, validation failure, safety refusal, Git/filesystem failure, or other runtime failure |
| `2` | Invalid command usage, flags, or arguments |

No additional application exit codes are defined in v1. Operating-system
termination statuses remain outside this contract.

### Human graph rendering

The default graph is plain, colorless ASCII with one task per line:

```text
T-001 [done] Establish module
  |
T-002 [in_progress] Implement format
  |
T-003 [open] Add workflow
```

An empty sequence renders `(no tasks)`. `gp graph --json` returns ordered
`nodes` with `id`, `title`, and `state`, plus ordered `edges` with `from` and
`to`. v1 does not include DOT, Mermaid, Unicode connectors, or graph files.

## `AGENTS.md` integration

Initialization creates root `AGENTS.md` when it does not exist or inserts one
managed block when it does. The block uses visible HTML comment markers similar
to:

```markdown
<!-- go-plan:managed:start schema=go-plan/v1 -->
## Active go-plan

Generated by `gp` from https://github.com/robertguss/go-plan.
Run `gp status`, `gp check`, and `gp ready` before selecting work.
Do not edit content inside these markers.
<!-- go-plan:managed:end -->
```

- Initialization refuses duplicate or malformed managed markers.
- Mutations preserve bytes outside the managed block.
- Removal deletes only the exact managed block.
- If `gp` created `AGENTS.md` and no user content remains, removal deletes the
  file; otherwise the user-authored file remains.
- `gp check` detects missing, duplicated, or modified managed instructions.

## Git behavior

- A Git worktree is mandatory; `gp init` refuses any other directory.
- Git is used for repository discovery and removal safety checks.
- `gp` never stages, commits, switches branches, fetches, pulls, pushes, or
  changes Git configuration.
- Default removal requires every file under `.go-plan/` to be tracked and clean
  so Git contains the plan version being removed.
- Unrelated working-tree changes outside managed plan content are preserved.
- The removal itself is left as an ordinary working-tree change for the caller
  to review and commit.

## Filesystem and transaction safety

- Every managed path is resolved beneath the discovered Git root.
- `.go-plan` is the only recursively removable directory.
- Managed symlink files and symlink path components are rejected.
- Initialization refuses an existing `.go-plan` path, including an empty one,
  rather than silently adopting it.
- New records never overwrite existing files.
- Multi-file mutations stage complete output, validate it, and publish it with
  rollback on reported failure.
- A repository-local lock prevents concurrent `gp` mutations in one checkout.
- Read commands do not take an exclusive lock unless needed for a consistent
  snapshot.
- `--force` may bypass plan validity, completion, and Git-cleanliness checks
  only when combined with explicit removal confirmation; it never bypasses
  path-containment, symlink, marker-integrity, or managed-target checks.
- Interruption and injected publication failures must not leave a partially
  renumbered plan or partially removed managed integration.

## Markdown and link safety

- YAML frontmatter is decoded into strict typed Go structures.
- Duplicate keys, unknown keys, aliases, custom tags, and values of the wrong
  type are rejected.
- Required headings appear exactly once and in the declared order.
- Template placeholders and unresolved open questions block approval.
- Repository-local Markdown links must resolve inside the Git root.
- Absolute local paths and paths escaping the Git root are rejected.
- Remote `https:` links are allowed but are never fetched or validated online.
- Evidence may be inline text or repository-relative Markdown links. Linked
  evidence must exist when a task is completed.

## Failure behavior

Errors are actionable, deterministic, and scoped to a command, file, field, or
task whenever possible. Validation aggregates independent errors in stable
path/position order. Mutation commands validate before publication and print no
success result until publication is complete.

No failure may:

- modify the reference repository `rust-and-beam-os`;
- write outside the selected `go-plan` repository;
- silently overwrite canonical content;
- leave half-renumbered task IDs or references;
- remove user-authored `AGENTS.md` content;
- infer that project verification passed;
- treat stale approval as current.

## Compatibility policy

- The schema identifier is `go-plan/v1`.
- Any v1 binary must read and operate on a valid plan produced by an earlier v1
  binary.
- New optional behavior may be added without changing existing v1 JSON fields
  or canonical meaning.
- A future breaking `v2` need not migrate v1 plans. The supported workflow is
  to finish or remove the active plan and initialize a new one.
- Installing a new binary never silently rewrites an active plan.
- There is no `upgrade` or `migrate` command in v1.

## Technology and project structure

- Language: Go.
- Module: `github.com/robertguss/go-plan`.
- Executable: `cmd/gp` producing `gp`.
- CLI framework: Cobra, without Viper.
- Data decoding: `github.com/goccy/go-yaml` pinned to `v1.19.2`.
- The decoder uses strict unknown-field handling and the parser is inspected to
  reject duplicate keys, aliases, custom tags, multiple documents, and wrong
  scalar/list types before accepting typed metadata.
- All other implementation uses the Go standard library unless the approved
  plan is amended.
- Embedded templates and agent instructions use Go's embedded-files support.
- Supported platforms: macOS and Linux only.
- Distribution: `go install github.com/robertguss/go-plan/cmd/gp@latest`.

Proposed source layout:

```text
cmd/gp/                 # Minimal executable adapter
internal/cli/           # Cobra command tree and output adapters
internal/plan/          # Deep format, validation, lifecycle, digest, and query module
internal/workspace/     # Deep Git/filesystem/templates/transactions/AGENTS module
```

The package layout favors a few deep modules. Parsing helpers and file
operations remain implementation details unless a second real adapter requires
a seam.

## Code style

- Follow `gofmt` and idiomatic Go naming.
- Keep `cmd/gp` free of domain behavior.
- Return typed results and errors from modules; format them only in `internal/cli`.
- Accept filesystem and command dependencies at internal test seams rather than
  creating global mutable state.
- Avoid speculative interfaces, pass-through packages, and configuration that
  has only one implementation.
- Preserve deterministic sort and serialization order explicitly.

## Testing strategy

- Table-driven unit tests for parsing, validation, digest normalization,
  lifecycle rules, readiness, renumbering, and JSON contracts.
- Golden tests for every embedded template, human help example, canonical file,
  managed `AGENTS.md` block, and JSON response schema.
- Real temporary Git repositories for initialization, Git discovery, dirty-plan
  removal, and preservation of unrelated content.
- Subprocess-level tests of the compiled `gp` binary, stdout, stderr, and exit
  behavior.
- Adversarial filesystem tests for traversal, symlink components, overwrite,
  interrupted publication, rollback, malformed markers, and partial deletion.
- Fuzz tests for YAML/frontmatter and Markdown record parsing and task-reference
  rewriting.
- macOS and Linux verification for `go test`, race tests, `go vet`, and
  `go install` of the local module.

The CLI does not execute host-project verification commands in tests or in
production.

## Boundaries

### Always

- Keep canonical plan documents readable without a database or service.
- Compute derived state from current canonical inputs.
- Require current approval before task start or completion.
- Preserve user-authored repository content.
- Make multi-file changes previewable and transaction-safe.
- Keep task execution strictly sequential.
- Provide machine-readable output and actionable errors.

### Ask first

- Add a direct dependency beyond Cobra and the YAML library.
- Change canonical files, required sections, digest normalization, task ID
  semantics, JSON fields, exit behavior, or removal safeguards.
- Add a stored projection, network behavior, Git mutation, or new record type.
- Expand supported platforms or distribution channels.

### Never

- Invoke an LLM or project verification command.
- Require network access for plan operation.
- Silently overwrite or remove user-authored content.
- Permit more than one active plan or one active task.
- Treat generated output as canonical.
- Claim verification, compatibility, safety, or release readiness without tests.

## Acceptance criteria

- AC-001: `gp init --title <title>` creates a complete draft plan from embedded
  assets at the discovered Git root and requires no network access.
- AC-002: Initialization creates or safely augments root `AGENTS.md` using
  explicit managed markers and preserves all user-authored bytes outside them.
- AC-003: Canonical files are limited to `plan.yaml`, the specification, the
  implementation plan, and one Markdown file per numeric task.
- AC-004: `gp` persists no generated indexes, readiness files, graphs, reports,
  or caches in the repository.
- AC-005: `gp check` strictly validates schema types, required sections,
  placeholders, acceptance IDs, task coverage, links, task sequence, lifecycle,
  managed instructions, and approval freshness.
- AC-006: `gp approve` records a deterministic digest only after the
  specification, implementation plan, and task set satisfy approval rules.
- AC-007: Changing approval-bound planning content invalidates approval, while
  state transitions, checkbox markers, and evidence updates do not.
- AC-008: Tasks use contiguous numeric IDs beginning at `T-001`, and the numeric
  sequence is the sole execution dependency order.
- AC-009: At most one task may be `in_progress`, and no task may start before
  every numerically earlier task is done.
- AC-010: `gp ready` returns at most one task and explains why no task is ready
  when the plan is under review, active, invalid, or complete.
- AC-011: `gp task complete` requires checked deliverables and task acceptance
  criteria plus recorded verification evidence, without running verification.
- AC-012: Every specification acceptance criterion is covered by at least one
  task before approval; enabling tasks may have empty coverage.
- AC-013: Completed tasks are immutable, and mutable-suffix additions,
  removals, or reorderings atomically renumber tasks and exact references.
- AC-014: Plan status is derived as draft, review-required, approved, executing,
  or completed; completion requires current approval and all tasks done.
- AC-015: `gp remove --dry-run` previews exact managed changes without mutation.
- AC-016: Default removal requires a valid completed plan whose `.go-plan`
  files are tracked and clean in Git.
- AC-017: Confirmed removal deletes only `.go-plan/` and the exact managed
  `AGENTS.md` block, preserving unrelated files and user-authored content.
- AC-018: Managed writes and removal reject path escape and symlinked targets
  and roll back reported multi-file publication failures.
- AC-019: Human output is concise; documented read commands expose stable v1
  JSON with deterministic ordering and structured errors.
- AC-020: Every command is non-interactive and has scoped help with at least one
  copy-pasteable example.
- AC-021: `gp` never invokes an LLM, executes host-project verification, mutates
  Git state, or accesses a network during plan operation.
- AC-022: The complete suite passes on macOS and Linux with unit, golden,
  subprocess, temporary-Git, fuzz-seed, race, and adversarial filesystem tests.
- AC-023: The module installs through
  `go install github.com/robertguss/go-plan/cmd/gp@latest` and the executable is
  named only `gp`.
- AC-024: A current v1 binary remains compatible with canonical plans and JSON
  contracts produced by earlier v1 binaries.

## Open questions

None.

## References

- Reference implementation and evidence:
  `/Users/robertguss/Projects/startups/rust-and-beam-os` at commit `2a701c9`
  on 2026-08-31. It is read-only prior art and is not a migration target.
- Agent-friendly CLI design guidance:
  https://github.com/cursor/plugins/blob/main/cli-for-agent/skills/cli-for-agents/SKILL.md
- Cobra command guidance:
  https://cobra.dev/docs/how-to-guides/working-with-commands/
- Pinned YAML package documentation and release:
  https://pkg.go.dev/github.com/goccy/go-yaml and
  https://github.com/goccy/go-yaml/releases/tag/v1.19.2
- Viper was explicitly rejected for v1:
  https://github.com/spf13/viper#why-use-viper
