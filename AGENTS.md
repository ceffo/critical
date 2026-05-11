# AI Agent Instructions

<!-- bv-agent-instructions-v2 -->

---

## Beads Workflow Integration

This project uses [beads_rust](https://github.com/Dicklesworthstone/beads_rust) (`br`) for issue tracking and [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer) (`bv`) for graph-aware triage. Issues are stored in `.beads/` and tracked in git.

### Using bv as an AI sidecar

bv is a graph-aware triage engine for Beads projects (.beads/beads.jsonl). Instead of parsing JSONL or hallucinating graph traversal, use robot flags for deterministic, dependency-aware outputs with precomputed metrics (PageRank, betweenness, critical path, cycles, HITS, eigenvector, k-core).

**Scope boundary:** bv handles *what to work on* (triage, priority, planning). `br` handles creating, modifying, and closing beads.

**CRITICAL: Use ONLY --robot-* flags. Bare bv launches an interactive TUI that blocks your session.**

#### The Workflow: Start With Triage

**`bv --robot-triage` is your single entry point.** It returns everything you need in one call:
- `quick_ref`: at-a-glance counts + top 3 picks
- `recommendations`: ranked actionable items with scores, reasons, unblock info
- `quick_wins`: low-effort high-impact items
- `blockers_to_clear`: items that unblock the most downstream work
- `project_health`: status/type/priority distributions, graph metrics
- `commands`: copy-paste shell commands for next steps

```bash
bv --robot-triage        # THE MEGA-COMMAND: start here
bv --robot-next          # Minimal: just the single top pick + claim command
```

#### Other bv Commands

| Command | Returns |
|---------|---------|
| `--robot-plan` | Parallel execution tracks with unblocks lists |
| `--robot-priority` | Priority misalignment detection with confidence |
| `--robot-insights` | Full metrics: PageRank, betweenness, HITS, eigenvector, critical path, cycles, k-core |
| `--robot-alerts` | Stale issues, blocking cascades, priority mismatches |
| `--robot-suggest` | Hygiene: duplicates, missing deps, label suggestions, cycle breaks |
| `--robot-diff --diff-since <ref>` | Changes since ref: new/closed/modified issues |
| `--robot-graph [--graph-format=json\|dot\|mermaid]` | Dependency graph export |

#### Scoping & Filtering

```bash
bv --robot-plan --label backend              # Scope to label's subgraph
bv --robot-insights --as-of HEAD~30          # Historical point-in-time
bv --recipe actionable --robot-plan          # Pre-filter: ready to work (no blockers)
bv --recipe high-impact --robot-triage       # Pre-filter: top PageRank scores
```

### br Commands for Issue Management

```bash
br ready              # Show issues ready to work (no blockers)
br list --status=open # All open issues
br show <id>          # Full issue details with dependencies
br create --title="..." --type=task --priority=2
br update <id> --status=in_progress
br close <id> --reason="Completed"
br close <id1> <id2>  # Close multiple issues at once
br sync --flush-only  # Export DB to JSONL
```

### Workflow Pattern

1. **Triage**: Run `bv --robot-triage` to find the highest-impact actionable work
2. **Claim**: Use `br update <id> --status=in_progress`
3. **Work**: Implement the task
4. **Validate**: Run `just validate` and ensure it passes. **NO TASK IS COMPLETE WITHOUT VALIDATION.**
5. **Complete**: Use `br close <id>`
6. **Sync**: Always run `br sync --flush-only` at session end

### Mandatory Validation (Pre-flight)

Before closing any task, you **MUST** run the validation command from the project root:

```bash
just validate
```

This command ensures:

- **Backend:** Code compiles, lints, and all tests pass.
- **Frontend:** TypeScript types are valid, linting passes, and the production build succeeds.

Failure to run validation leads to broken builds and "un-runnable" states. If validation fails, fix the issues before proceeding.

### Key Concepts

- **Dependencies**: Issues can block other issues. `br ready` shows only unblocked work.
- **Priority**: P0=critical, P1=high, P2=medium, P3=low, P4=backlog (use numbers 0-4, not words)
- **Types**: task, bug, feature, epic, chore, docs, question
- **Blocking**: `br dep add <issue> <depends-on>` to add dependencies

### Session Protocol

```bash
git status              # Check what changed
git add <files>         # Stage code changes
br sync --flush-only    # Export beads changes to JSONL
git commit -m "..."     # Commit everything
git push                # Push to remote
```

<!-- end-bv-agent-instructions -->

## DAL Code Generation (bob)

The DAL adapters in `api/internal/adapters/*/` use [bob](https://github.com/stephenafamo/bob) for typed SQL queries. Generated files are committed and should not be edited by hand.

### When to regenerate

Regenerate whenever:
- A migration is added or changed (new table/column)
- A `.sql` query file under `api/internal/adapters/*/queries/` is added or changed
- The bob version in `api/go.mod` is bumped

### How to regenerate

```bash
just generate
```

This recipe (defined in `justfile`):
1. Applies all migrations in `api/internal/storage/sqlite/migrations/` to a temp SQLite DB (`.tmp_schema.db`)
2. Runs `bobgen-sqlite -c bobgen.yaml` to regenerate Go wrappers
3. Deletes the temp DB
4. Runs `go mod tidy`

### Layout

| Path | Contents |
|------|----------|
| `api/bobgen.yaml` | Generator config: DSN, query dirs, output destinations |
| `api/internal/storage/sqlite/bob/models/` | ORM models for all tables (generated) |
| `api/internal/storage/sqlite/bob/factory/` | Test factories (generated) |
| `api/internal/adapters/*/queries/*.sql` | Hand-written SQL query sources |
| `api/internal/adapters/*/queries/*.bob.go` | Generated typed Go query wrappers |

### Key constraints

- `SUM()`, `COUNT()`, and other aggregates are **not supported** by bob's static parser — keep those as raw `d.q.QueryRowContext(...)` calls in the DAL.
- LIMIT parameters generate as `string` — pass `strconv.Itoa(n)` from the DAL.
- `DATETIME` columns require an inline type annotation in the `.sql` file: `created_at /*:time.Time:notnull*/`
- The `sqlite.BobExec` wrapper in `api/internal/storage/sqlite/bob_exec.go` bridges `sqlite.Querier` (returns `*sql.Rows`) to `bob.Executor` (expects `scan.Rows`)
