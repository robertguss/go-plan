# go-plan

`go-plan` is a deterministic, offline CLI for creating, approving, executing,
revising, and retiring one Git-native implementation plan. The executable is
named `gp`.

## Install

Go 1.21 or newer is required.

```sh
go install github.com/robertguss/go-plan/cmd/gp@latest
```

The supported runtime platforms are macOS and Linux. `gp` does not call a
network service, invoke an LLM, run project verification, or mutate Git state.

## Workflow

Initialize from anywhere in a Git worktree:

```sh
gp init --title "Add offline project planning"
```

Edit `.go-plan/specification.md` and `.go-plan/implementation-plan.md`, then
create tasks. Task bodies are ordinary Markdown and must have their template
placeholders replaced before approval.

```sh
gp task add --title "Implement parser" --cover AC-001
gp task add --title "Add integration tests" --cover AC-002
gp task list
gp task show T-001
gp check
gp approve
```

Execute exactly one task at a time. Run the verification documented in the task
yourself, check its deliverable and acceptance checkboxes, and record the result
under `Evidence` before completing it.

```sh
gp status
gp ready
gp task start T-001
# implement and independently run the task's verification
gp task complete T-001
```

All useful read commands support the stable `go-plan/v1` JSON envelope:

```sh
gp status --json
gp ready --json
gp graph --json
gp task list --json
```

## Revisions

Completed tasks are immutable. Open tasks form a mutable suffix. Preview
renumbering before publishing it:

```sh
gp task add --title "Document behavior" --after T-002 --dry-run
gp task reorder --order T-004,T-003 --dry-run
gp task remove T-004 --dry-run
```

Remove `--dry-run` to apply a revision transactionally. Exact `T-NNN` references
within `.go-plan/` are updated with the renumbering. Any planning change makes
approval stale, so run `gp check` and `gp approve` again.

## Graph and removal

```sh
gp graph
gp remove --dry-run
gp remove --yes
```

Default removal requires a valid completed plan whose `.go-plan` files are
tracked and clean. Commit the completed plan first. `--yes --force` can bypass
validity, completion, and Git-cleanliness checks, but never path, symlink, or
managed-marker safety. Removal preserves all user-authored `AGENTS.md` bytes.

If an operation is refused, correct the reported canonical file and rerun it.
`gp` publishes multi-file changes with rollback on a reported failure; Git
remains the archive and manual recovery path. Use `--repo <path>` to select a
worktree explicitly in scripts.

Run `gp <command> --help` for scoped examples. Exit status is 0 for success, 1
for runtime/domain failure, and 2 for invalid usage.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/gp
go install ./cmd/gp
```
