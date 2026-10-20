# AI Agent Instructions

## Mandatory Validation (Pre-flight)

Before closing any task, you **MUST** run the validation command from the project root:

```bash
just validate
```

This command ensures:

- **Backend:** Code compiles, lints, and all tests pass.
- **Frontend:** TypeScript types are valid, linting passes, and the production build succeeds.

**NO TASK IS COMPLETE WITHOUT VALIDATION.** Failure to run validation leads to broken builds and "un-runnable" states. If validation fails, fix the issues before proceeding.

## Agent Workflow

Project-specific configuration for the agents in `.claude/agents/` (validation, worktrees, generated
code, tracker usage, review triggers and checklist, model selection, commit format) lives in
[docs/agent-workflow.md](docs/agent-workflow.md). Read it before working as one of those agents.
Its "Hard rules for implementers" bind every agent, including: never open a visible window on the
user's desktop; UI checks run headless.

Rules and design decisions (house rules, architecture choices) live only in the ADRs indexed in
[docs/adr/README.md](docs/adr/README.md).

## Rulebooks

The PHB and DMG live as agent-readable files in a private sibling repo, `~/code/critical-rulebooks`
(shared by every worktree). Run `just rulebooks` to clone it (or fast-forward it); it needs access to
the private repo and is never part of `just validate` or `just dev`.

For a rules question, ask the `rules-lookup` agent (Agent tool, `subagent_type: rules-lookup`; Haiku,
one question per call): it reads the books and returns the rule with its file, section and a short
quote, so the book text stays out of your context. Check its quote before a rule drives game logic.
If you can't start agents (you run as a subagent yourself), look it up directly: `grep -i '<term>'
~/code/critical-rulebooks/INDEX.md` first, then open only the file it names. Never read whole chapters
or INDEX.md whole. If the clone is missing, run
`just rulebooks`; if that fails, say so instead of guessing the rule. The book is a reference: the
project's decided rules (ADRs in `docs/adr`, user rulings) win where they differ. Never copy book text
into this public repo.

## Generated code

DAL code (sqlc) and mocks (mockery) are generated and committed: regenerate with `just generate` /
`just mocks`, never edit them by hand. When and how, plus the generators' SQL constraints: "DAL code
generation" in [docs/backend.md](docs/backend.md).

## Tracker

Linear (team `CRI`, issues `CRI-<n>`). Only the supervisor and the scoper write to it; tools, states and rules
are in the "Tracker" section of [docs/agent-workflow.md](docs/agent-workflow.md).
