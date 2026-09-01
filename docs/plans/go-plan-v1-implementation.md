# Implementation plan: go-plan v1

Status: Approved
Depends on: `docs/specs/go-plan-v1.md`
Execution model: Strictly sequential; do not begin implementation before the
specification and this plan receive explicit approval.

## Approach

Implement `goplan` as a small Cobra adapter over two deep internal modules:

1. A pure `plan` module that owns canonical parsing, validation, approval
   normalization, lifecycle rules, and live queries.
2. A stateful `workspace` module that owns Git discovery, embedded assets,
   managed filesystem operations, `AGENTS.md`, locking, transactions, and safe
   removal.

Deliver the CLI in sequential vertical slices. Each slice adds an observable
command behavior with focused tests, then runs the complete suite. Do not port
the reference Python implementation line-for-line. Preserve only the product
principles approved in the v1 specification.

## Architecture

```text
cmd/goplan
  │
  ▼
internal/cli
  │
  ▼
internal/workspace ──────► Git executable + repository filesystem
  │
  ▼
internal/plan ───────────► strict YAML decoder + pure Markdown/domain logic
```

### `internal/plan`

A deep module whose interface accepts canonical bytes or typed inputs and
returns parsed plans, validation findings, approval digests, lifecycle
transitions, and query results. Callers do not know YAML field ordering,
Markdown parsing rules, digest normalization, task renumbering mechanics, or
readiness calculations.

### `internal/workspace`

A deep module whose interface operates on one discovered repository. It hides
Git-root discovery, canonical paths, embedded template rendering, local locks,
safe reads, staged transactions, rollback, managed `AGENTS.md` edits, and
removal checks. Tests exercise the same interface with temporary real Git
repositories and narrow injected fault seams.

### `internal/cli`

A thin adapter that defines Cobra commands, converts flags and arguments into
workspace operations, and renders typed results as human text or stable JSON.
It contains no plan rules or raw filesystem mutation.

No additional package is introduced until its deletion would force meaningful
complexity into multiple callers. Parsing helpers, JSON encoding, templates,
and Git command execution begin as private implementation details.

## Technology and dependencies

- Go module: `github.com/robertguss/go-plan`.
- Executable package: `cmd/goplan`.
- Cobra for commands, flags, validation, and layered help.
- No Viper.
- `github.com/goccy/go-yaml` pinned to `v1.19.2`; strict decode is combined
  with parser-level rejection of aliases, custom tags, multiple documents, and
  ambiguous node types.
- Go standard library for embedded files, JSON, hashing, Markdown scanning,
  paths, process execution, temporary files, locking primitives, and tests.
- No Windows build tags or Windows-specific adapters. Supported systems are
  macOS and Linux.
- No network calls in runtime code.

## Interfaces and data flow

### Read flow

```text
Cobra command
  → discover Git root
  → load canonical files and managed AGENTS block
  → parse strict typed records
  → validate all invariants
  → compute status / ready task / graph
  → render human text or stable JSON
```

### Mutation flow

```text
Cobra command
  → acquire repository-local mutation lock
  → discover and validate managed targets
  → load current canonical state
  → apply one typed domain transition in memory
  → render a complete candidate file set
  → validate the candidate
  → publish staged files transactionally
  → release lock
  → report success
```

### Approval flow

The plan module normalizes approval-bound content, hashes it with SHA-256, and
returns the digest. The workspace module writes only the updated `plan.yaml`.
Every subsequent load recomputes the digest to derive approval freshness.

### JSON flow

Domain and workspace modules return typed results and typed findings. The CLI
owns the single v1 JSON envelope and stable error representation. Human output
and JSON are rendered from the same result so their behavior cannot diverge.

## Change surface

Expected production paths:

```text
go.mod
go.sum
cmd/goplan/main.go
internal/cli/
internal/plan/
internal/workspace/
```

Expected test and documentation paths:

```text
internal/cli/*_test.go
internal/plan/*_test.go
internal/workspace/*_test.go
internal/testutil/          # Only if shared real-repository setup earns a module
testdata/compat/v1/         # Frozen v1 compatibility plans and JSON goldens
README.md
docs/specs/go-plan-v1.md
docs/plans/go-plan-v1-implementation.md
```

Embedded templates live under the module that renders and installs them rather
than in a public runtime directory.

## Verification strategy

Every task runs focused tests and the full suite before completion. The common
verification baseline is:

```sh
gofmt -w <files changed by the task>
go test ./...
go vet ./...
go build ./cmd/goplan
```

High-risk filesystem and lifecycle slices additionally run:

```sh
go test -race ./...
```

Compatibility and installation verification additionally run:

```sh
go install ./cmd/goplan
```

Tests must not write to the reference repository. Temporary Git repositories
are created beneath the test process's temporary directory. Network access is
not part of any runtime or acceptance test.

## Decisions and tradeoffs

- The installed binary is `goplan`; there is no second `go-plan` executable.
- Cobra is worth one dependency because command discoverability, validation,
  nested task verbs, and help are product behavior. Viper is rejected because
  environment/config precedence would weaken repository determinism.
- Markdown plus strict YAML frontmatter remains canonical because agents must
  directly read and edit requirements and evidence.
- All templates and instructions are embedded in the binary, while rendered
  canonical files are committed in each project.
- One active `.go-plan/` avoids stale cross-plan context and makes safe removal
  tractable.
- No generated projections are persisted; every view is live.
- Numeric task IDs are intentionally order-bearing. Revisions may cause Git
  renames, but the active sequence remains obvious and Git preserves prior IDs.
- A linear DAG replaces general dependencies because execution is always
  sequential. There is no ownership or distributed coordination model.
- Approval is a content digest rather than an identity or audit record.
- The CLI validates verification documentation but never executes project
  verification commands.
- Completed task immutability concentrates replanning in the unfinished suffix.
- Same-major compatibility is guaranteed, while schema migrations and upgrade
  commands are intentionally omitted.
- Git is an archive and discovery/safety dependency, not a mutation target.
- macOS and Linux are supported; Windows work is deliberately excluded.
- `go install` is the only v1 distribution mechanism.
- JSON uses one `schema`/`command`/`ok` envelope; failures carry
  `error.code`, `error.message`, and ordered `error.details`.
- Exit codes are `0` for success, `1` for domain or runtime failure, and `2`
  for usage failure.
- Task coverage uses repeatable `--cover`; insertion uses `--after`; mutable
  suffix ordering uses one comma-separated `--order` value.
- Human graphs use deterministic, colorless ASCII; JSON exposes ordered nodes
  and edges. No other graph format is included in v1.

## Risks and recovery

| Risk | Impact | Mitigation and recovery |
|---|---|---|
| Digest normalization excludes too much or too little | Stale approval or needless reapproval | Golden vectors cover every bound and excluded field; compatibility fixtures freeze behavior |
| YAML decoder accepts ambiguous constructs | Different readers interpret a plan differently | Inspect YAML nodes, reject duplicates, aliases, tags, unknown keys, and wrong types before typed decoding |
| Task renumbering rewrites prose accidentally | Corrupted references or noisy diffs | Rewrite only exact parsed `T-NNN` tokens inside `.go-plan`; dry-run every mapping; validate staged output |
| Multi-file publication is interrupted | Partial plan or AGENTS integration | Stage complete candidates, fsync where supported, inject failures in tests, and restore backups on reported failure |
| Removal deletes unrelated content | Irrecoverable local loss | Fixed `.go-plan` target, symlink/path checks, exact AGENTS markers, managed manifest facts, dry-run, confirmation, and Git cleanliness |
| Existing AGENTS content changes concurrently | Lost user instructions | Lock local mutations, preserve outside bytes exactly, compare expected markers before replace, and abort on mismatch |
| Human and JSON behavior drift | Agents receive inconsistent results | Render both from shared typed results and freeze JSON/error goldens |
| A newer v1 binary changes old behavior | Active plans stop working | Check frozen v1 plan and JSON fixtures in every full test run |
| Cobra defaults leak usage or decoration into errors | Poor agent ergonomics | Configure silence behavior intentionally and cover subprocess stdout/stderr/exit results |

If a mutation cannot prove rollback-safe publication, it must fail before
touching canonical files. Manual Git recovery remains available, but it is not
a substitute for transaction tests.

## Ordered implementation tasks

### T-001: Establish the Go module and agent-friendly Cobra shell

**Goal:** Produce an installable `goplan` executable with the approved command tree,
global flags, layered help, examples, and testable error/output adapters, while
leaving domain commands explicitly unimplemented.

**Covers:** AC-019, AC-020, AC-023

**Likely files:** `go.mod`, `go.sum`, `cmd/goplan/main.go`,
`internal/cli/root.go`, `internal/cli/root_test.go`

**Acceptance criteria:**

- [ ] The executable name and root use are `goplan`, and no `go-plan` executable is
  built.
- [ ] Every approved plan-wide and `task` subcommand appears in scoped help with
  at least one real example.
- [ ] `--repo`, `--json`, runtime-error usage suppression, stdout, and stderr are
  wired through constructor-created commands that tests can invoke in process.
- [ ] JSON goldens enforce the v1 success/error envelopes and subprocess tests
  enforce exit codes `0`, `1`, and `2`.

**Verification:**

```sh
go test ./internal/cli -run 'TestRoot|TestHelp'
go test ./...
go vet ./...
go build ./cmd/goplan
```

**Complete when:** The binary builds and help/error goldens prove the complete
command surface without implementing product behavior.

### T-002: Implement strict canonical parsing and embedded templates

**Goal:** Make the `plan` module parse and render v1 metadata, specification,
implementation-plan, and task documents from embedded deterministic templates.

**Covers:** AC-003, AC-005, AC-008

**Likely files:** `internal/plan/model.go`, `internal/plan/markdown.go`,
`internal/plan/templates.go`, `internal/plan/templates/*`,
`internal/plan/format_test.go`

**Acceptance criteria:**

- [ ] Strict YAML rejects duplicates, aliases, tags, unknown fields, and wrong
  scalar/list types.
- [ ] The parser uses pinned `github.com/goccy/go-yaml@v1.19.2` and rejects
  multiple YAML documents before typed decoding.
- [ ] Required sections, `AC-NNN` identifiers, `T-NNN` IDs, checklists, and exact
  filenames parse into typed records.
- [ ] Equal inputs render byte-identical UTF-8 files with one newline and no
  timestamp or environment-derived value.

**Verification:**

```sh
go test ./internal/plan -run 'TestParse|TestRender|TestTemplates'
go test ./...
go vet ./...
```

**Complete when:** Golden template output round-trips through strict parsing and
all malformed-format table cases fail deterministically.

### T-003: Implement whole-plan validation and approval normalization

**Goal:** Validate the complete canonical plan and compute the content digest
that distinguishes planning changes from execution-only changes.

**Covers:** AC-005, AC-006, AC-007, AC-008, AC-012, AC-024

**Likely files:** `internal/plan/validate.go`, `internal/plan/digest.go`,
`internal/plan/findings.go`, `internal/plan/validate_test.go`,
`internal/plan/digest_test.go`

**Acceptance criteria:**

- [ ] Validation aggregates stable file/field findings for headings,
  placeholders, open questions, acceptance IDs, coverage, contiguous tasks,
  lifecycle, and exact references.
- [ ] Digest vectors prove that every approved planning edit changes the digest.
- [ ] State, checkbox-marker, evidence, and approval-field changes leave the
  digest unchanged while retaining their validated structure.

**Verification:**

```sh
go test ./internal/plan -run 'TestValidate|TestApprovalDigest'
go test ./...
go vet ./...
```

**Complete when:** Validation and digest goldens cover every field in the v1
contracts and a frozen compatibility vector is committed.

### T-004: Implement Git workspace discovery and transaction-safe storage

**Goal:** Give all later commands one safe workspace interface for locating a
Git root and reading or publishing managed files.

**Covers:** AC-001, AC-018, AC-021

**Likely files:** `internal/workspace/workspace.go`,
`internal/workspace/git.go`, `internal/workspace/transaction.go`,
`internal/workspace/workspace_test.go`, `internal/workspace/transaction_test.go`

**Acceptance criteria:**

- [ ] Discovery works from nested directories and `--repo`, and rejects non-Git
  directories.
- [ ] Managed operations reject traversal, absolute escape, symlink components,
  duplicate mutation locks, and overwrite of unmanaged targets.
- [ ] Staged multi-file publication either completes fully or restores every
  pre-operation byte under injected failure.

**Verification:**

```sh
go test ./internal/workspace -run 'TestDiscover|TestManagedPath|TestTransaction'
go test -race ./internal/workspace
go test ./...
go vet ./...
```

**Complete when:** Temporary real repositories and fault injection prove the
workspace interface can safely support initialization and later mutations.

### T-005: Deliver `goplan init` and managed `AGENTS.md` installation

**Goal:** Provide the first end-to-end product workflow by initializing a draft
plan and installing discoverable agent instructions.

**Covers:** AC-001, AC-002, AC-003, AC-004, AC-018

**Likely files:** `internal/workspace/init.go`,
`internal/workspace/agents.go`, `internal/workspace/init_test.go`,
`internal/cli/init.go`, `internal/cli/init_test.go`

**Acceptance criteria:**

- [ ] `goplan init --title` creates exactly the canonical v1 layout from embedded
  assets and refuses any pre-existing `.go-plan` path.
- [ ] Existing `AGENTS.md` bytes outside the marked block are preserved; a
  missing file is created; malformed or duplicate markers are rejected.
- [ ] Subprocess tests prove non-interactive human and JSON success/error output
  and byte-identical initialization in clean temporary Git repositories.

**Verification:**

```sh
go test ./internal/workspace -run 'TestInitialize|TestAgents'
go test ./internal/cli -run TestInit
go test ./...
go vet ./...
go build ./cmd/goplan
```

**Complete when:** A compiled `goplan` initializes a complete offline draft without
creating any projection or touching unrelated repository content.

### T-006: Deliver `goplan check` and `goplan status`

**Goal:** Expose full read-only validation and derived lifecycle status through
shared typed results and stable human/JSON renderers.

**Covers:** AC-004, AC-005, AC-007, AC-014, AC-019, AC-020

**Likely files:** `internal/plan/status.go`,
`internal/workspace/load.go`, `internal/cli/check.go`,
`internal/cli/status.go`, `internal/cli/read_test.go`

**Acceptance criteria:**

- [ ] `check` reports every independent finding in deterministic order and does
  not mutate files.
- [ ] `status` derives draft, review-required, approved, executing, and completed
  plus counts, active task, approval freshness, and next task.
- [ ] Human, success JSON, and error JSON goldens are versioned and subprocess
  tests distinguish usage and domain failure.

**Verification:**

```sh
go test ./internal/plan -run TestStatus
go test ./internal/cli -run 'TestCheck|TestStatus|TestJSON'
go test ./...
go vet ./...
```

**Complete when:** Corrupt and valid fixture plans produce stable read-only
diagnostics and lifecycle responses through the compiled binary.

### T-007: Deliver task creation and read commands

**Goal:** Let agents append task templates and inspect the canonical sequence
without manually inventing IDs or parsing Markdown.

**Covers:** AC-008, AC-012, AC-019, AC-020

**Likely files:** `internal/plan/tasks.go`,
`internal/workspace/tasks.go`, `internal/cli/task_read.go`,
`internal/cli/task_add.go`, `internal/cli/task_test.go`

**Acceptance criteria:**

- [ ] `task add --title` appends the next contiguous ID without overwriting;
  repeatable `--cover` accepts an empty or validated acceptance-coverage list.
- [ ] `task add --after` inserts only where lifecycle rules allow, and
  `task reorder --order T-NNN,T-NNN,...` requires every open mutable-suffix ID
  exactly once before previewing or publishing a renumbering.
- [ ] `task list` and `task show` expose numeric order, state, coverage, and
  completion factors in human and stable JSON formats.
- [ ] Repeat, malformed ID, malformed coverage, and stale-plan cases fail with
  actionable examples and no file changes.

**Verification:**

```sh
go test ./internal/plan -run TestTasks
go test ./internal/workspace -run TestTaskAdd
go test ./internal/cli -run 'TestTaskAdd|TestTaskList|TestTaskShow'
go test ./...
go vet ./...
```

**Complete when:** An agent can build and inspect a canonical task set solely
through non-interactive task commands and direct body editing.

### T-008: Deliver approval, readiness, and graph queries

**Goal:** Close the planning loop by approving valid content and exposing the
single legal execution frontier and its linear graph.

**Covers:** AC-006, AC-007, AC-008, AC-009, AC-010, AC-014, AC-019

**Likely files:** `internal/plan/workflow.go`,
`internal/plan/query_test.go`, `internal/workspace/approve.go`,
`internal/cli/approve.go`, `internal/cli/query.go`

**Acceptance criteria:**

- [ ] `approve` refuses incomplete/invalid content and transactionally stores
  the exact current digest without identity or audit fields.
- [ ] `ready` returns at most one task and structured factors explain every
  no-task condition.
- [ ] `graph` derives the complete numeric chain live, renders the specified
  colorless ASCII by default, returns ordered nodes and edges with `--json`, and
  writes no repository output.

**Verification:**

```sh
go test ./internal/plan -run 'TestApprove|TestReady|TestGraph'
go test ./internal/cli -run 'TestApprove|TestReady|TestGraph'
go test ./...
go vet ./...
```

**Complete when:** A valid temporary plan can move from review-required to
approved and return exactly `T-001` through both human and JSON queries.

### T-009: Deliver task start and completion lifecycle

**Goal:** Enforce one active task, strict numeric progression, documented
verification, and immutable completion.

**Covers:** AC-007, AC-009, AC-010, AC-011, AC-014, AC-021

**Likely files:** `internal/plan/lifecycle.go`,
`internal/plan/lifecycle_test.go`, `internal/workspace/lifecycle.go`,
`internal/cli/task_lifecycle.go`, `internal/cli/task_lifecycle_test.go`

**Acceptance criteria:**

- [ ] `task start` accepts only the one ready task of a currently approved plan
  and is safe on retry.
- [ ] `task complete` requires the active task, checked deliverables and
  acceptance criteria, non-placeholder verification instructions, and recorded
  evidence with valid local links.
- [ ] No lifecycle path creates a second active task, skips a numeric predecessor,
  executes verification, or changes approval freshness.

**Verification:**

```sh
go test ./internal/plan -run 'TestStart|TestComplete|TestSequentialInvariant'
go test ./internal/cli -run 'TestTaskStart|TestTaskComplete'
go test -race ./...
go vet ./...
```

**Complete when:** A subprocess integration test executes multiple tasks in
numeric order and derives completed status only after the final valid completion.

### T-010: Deliver mutable-suffix additions, removals, and reorderings

**Goal:** Support plan revision without weakening completed history or leaving
stale task references.

**Covers:** AC-007, AC-008, AC-013, AC-018

**Likely files:** `internal/plan/revision.go`,
`internal/plan/revision_test.go`, `internal/workspace/revision.go`,
`internal/cli/task_revision.go`, `internal/cli/task_revision_test.go`

**Acceptance criteria:**

- [ ] Add-after, remove, and reorder operate only on the mutable suffix, preserve
  completed tasks and the current active task ID, and reject completed-task edits.
- [ ] Dry-run reports the exact old-to-new mapping and changed paths without
  mutation.
- [ ] Confirmed publication renames files and exact in-plan task references
  atomically, validates the candidate, and makes prior approval stale.

**Verification:**

```sh
go test ./internal/plan -run 'TestRevision|TestRenumber'
go test ./internal/workspace -run TestPublishRevision
go test ./internal/cli -run 'TestTaskReorder|TestTaskRemove'
go test -race ./...
go vet ./...
```

**Complete when:** Adversarial reorder cases either publish a completely valid
renumbered suffix or preserve every original byte.

### T-011: Deliver safe whole-plan removal

**Goal:** Make the stale-context cleanup workflow easy while ensuring `goplan`
cannot delete unrelated or unrecoverable content accidentally.

**Covers:** AC-015, AC-016, AC-017, AC-018, AC-021

**Likely files:** `internal/workspace/remove.go`,
`internal/workspace/remove_test.go`, `internal/cli/remove.go`,
`internal/cli/remove_test.go`, `internal/cli/remove_integration_test.go`

**Acceptance criteria:**

- [ ] Dry-run lists the `.go-plan` tree and exact AGENTS block change and never
  mutates.
- [ ] Default removal requires valid completed content plus tracked, clean plan
  files; `--yes` is mandatory and `--force` never bypasses managed-target safety.
- [ ] Confirmed removal preserves unrelated files and AGENTS bytes, removes an
  otherwise empty CLI-created AGENTS file, and rolls back injected failures.

**Verification:**

```sh
go test ./internal/workspace -run 'TestRemove|TestRemoveRollback|TestGitCleanliness'
go test ./internal/cli -run TestRemove
go test -race ./...
go vet ./...
```

**Complete when:** Temporary Git repositories prove the default, dirty, forced,
malformed-marker, symlink, interruption, and unrelated-change cases.

### T-012: Harden parsers, links, paths, and subprocess contracts

**Goal:** Consolidate adversarial coverage across the complete product before
claiming safety or release readiness.

**Covers:** AC-005, AC-018, AC-019, AC-021, AC-022

**Likely files:** `internal/plan/fuzz_test.go`,
`internal/workspace/adversarial_test.go`,
`internal/cli/subprocess_test.go`, `testdata/invalid/*`,
`testdata/golden/*`

**Acceptance criteria:**

- [ ] Seeded fuzz tests cover frontmatter boundaries, Markdown delimiters,
  acceptance/task identifiers, reference rewriting, and digest normalization.
- [ ] Filesystem cases cover traversal, symlink files/components, absolute paths,
  malformed markers, conflicting locks, faulted replaces, and rollback.
- [ ] Every command's success, usage error, domain error, stdout, stderr, JSON,
  help, retry, and dry-run contract has subprocess evidence.

**Verification:**

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/goplan
```

**Complete when:** The complete adversarial suite passes on supported systems and
there are no untested mutation or removal paths.

### T-013: Freeze v1 compatibility and document installation and operation

**Goal:** Complete the release boundary with frozen fixtures, user-facing usage,
and local installation verification.

**Covers:** AC-019, AC-020, AC-022, AC-023, AC-024

**Likely files:** `testdata/compat/v1/*`, `README.md`,
`internal/cli/compat_test.go`, `docs/specs/go-plan-v1.md`,
`docs/plans/go-plan-v1-implementation.md`

**Acceptance criteria:**

- [ ] Frozen canonical v1 plans and JSON responses are loaded and exercised by
  the current binary.
- [ ] README examples cover install, initialize, author, check, approve, execute,
  revise, query, dry-run, remove, and recovery without promising Windows or
  release archives.
- [ ] A fresh environment can `go install ./cmd/goplan`, run the documented workflow
  in a temporary Git repository, and remove the completed plan safely.

**Verification:**

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/goplan
go install ./cmd/goplan
```

**Complete when:** All specification acceptance criteria have passing automated
evidence, documentation matches the binary, and the v1 fixtures define the
compatibility floor.

## Acceptance-criteria coverage

| Criterion | Implementation tasks |
|---|---|
| AC-001 | T-004, T-005 |
| AC-002 | T-005 |
| AC-003 | T-002, T-005 |
| AC-004 | T-005, T-006 |
| AC-005 | T-002, T-003, T-006, T-012 |
| AC-006 | T-003, T-008 |
| AC-007 | T-003, T-006, T-008, T-009, T-010 |
| AC-008 | T-002, T-003, T-007, T-008, T-010 |
| AC-009 | T-008, T-009 |
| AC-010 | T-006, T-008, T-009 |
| AC-011 | T-009 |
| AC-012 | T-003, T-007 |
| AC-013 | T-010 |
| AC-014 | T-006, T-008, T-009 |
| AC-015 | T-011 |
| AC-016 | T-011 |
| AC-017 | T-011 |
| AC-018 | T-004, T-005, T-010, T-011, T-012 |
| AC-019 | T-001, T-006, T-007, T-008, T-012, T-013 |
| AC-020 | T-001, T-006, T-007, T-013 |
| AC-021 | T-004, T-009, T-011, T-012 |
| AC-022 | T-012, T-013 |
| AC-023 | T-001, T-013 |
| AC-024 | T-003, T-013 |

Every criterion has at least one implementation task. The table is an audit of
this implementation plan, not a generated runtime projection.

## Checkpoints

### Checkpoint A: After T-003

- [ ] The CLI skeleton, embedded canonical format, strict validation, and digest
  behavior pass focused and full tests.
- [ ] The pinned YAML dependency, JSON envelope, exit-code allocation, task
  flags, and graph rendering remain covered by frozen tests.

### Checkpoint B: After T-006

- [ ] A compiled `goplan` initializes, checks, and reports status for a real temporary
  Git repository without network or generated projections.
- [ ] Initialization and AGENTS integration rollback tests pass.

### Checkpoint C: After T-009

- [ ] The complete author → approve → ready → start → verify externally → complete
  workflow passes through subprocess tests.
- [ ] Strict sequential execution and approval invalidation have regression tests.

### Checkpoint D: After T-011

- [ ] Revision and whole-plan removal dry-runs are exact.
- [ ] Fault injection proves atomic renumbering and removal rollback.

### Checkpoint E: After T-013

- [ ] Full, race, vet, build, install, compatibility, documentation, and temporary
  repository end-to-end verification pass on macOS and Linux.
- [ ] Every v1 acceptance criterion has durable test evidence.

## Out of scope

- Implementing any command before explicit approval of the specification and
  this plan.
- Porting legacy Python migration or normalization behavior.
- Creating a `.go-plan/` for the `go-plan` project as part of planning this
  implementation.
- Parallelizing implementation tasks or delegating them across agents.
- Windows adaptations, hosted integrations, stored projections, template
  overrides, Viper, release automation, Homebrew, or schema migration.

## Open questions before approval

None. The product specification and implementation plan were explicitly
approved on 2026-08-31.
