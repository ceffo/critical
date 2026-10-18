# ADR 0031: Linear is the work-item tracker; beads is a read-only archive

**Status**: accepted, 2026-10-18 (user, triage: "agree with all", after the investigation of beads against
Linear). Takes effect at the cutover, when this change merges.

## Context

Work items lived in beads (`br`), exported to `.beads/issues.jsonl` and committed. User, 2026-10-18:
"investigate what it would take to migrate issue management from beads to linear. i've installed the
linear mcp and logged in. i've created a critical project in linear". Parallel PRs kept conflicting on the
jsonl after every merge ("tracker changes do not ride on feature branches", the `chore: tracker export`
PRs, `just sync-main`); the user wants issues in a tool with a UI, relations, and a GitHub integration.

## Decision

1. **Linear holds the work items.** Team `CRI`, project "critical-app"; an issue is `CRI-<n>`. The
   mapping (priorities, states, labels) is written once in `scripts/linear-mapping.json` and described in
   [docs/linear.md](../linear.md); no doc copies it. Agents reach Linear through the MCP tools named in their
   `tools:` list and through `scripts/linear.sh` (GraphQL with a personal `LINEAR_API_KEY`, never in the
   repo).
2. **States.** Todo (open), In Progress (claimed by the supervisor at dispatch), In Review (the supervisor
   moves it when the PR is opened), Done (Linear's GitHub integration, when the PR merges; nobody closes an
   issue by hand). A PR closed unmerged sends the issue back to Todo. Deferred work is Backlog with the
   label `deferred`.
3. **Who reads and writes.** Developer, code-reviewer and tester get the Linear read tools only; the
   supervisor and the scoper also write. Tracker state is not in git: no export, no tracker PR, no sync
   recipe, no rule that tracker changes stay off feature branches.
4. **What moved.** Priority P0..P4 becomes Urgent, High, Medium, Low, None; open/in_progress/closed become
   Todo/In Progress/Done; an epic becomes a parent issue labelled `epic`; blocking becomes a "blocked by"
   relation; `needs-info` and `risky` stay labels, and the "## Questions" section stays in the issue
   description. The commit and PR suffix is `(CRI-n)`, and the PR body carries `Closes CRI-n`.
5. **Import and archive.** Live items and all epics were imported; `docs/beads-id-map.json` maps each old id
   to its `CRI-n`. `.beads/issues.jsonl` stays as a read-only archive for one release, then is deleted and
   kept as a git tag. Old ids, in code or anywhere else, are looked up in the map and the archive;
   `check-no-tracker-ids` refuses both kinds of id in code.
6. **One switch.** The agent definitions and the workflow docs change in a single PR, merged at the
   cutover, so that main never has half the agents on beads and half on Linear.

## Consequences

- Lost on purpose: the P3 / P4 split is coarser (Low and None); types other than bug and feature are
  labels; the dotted ids (`critical-518.3`) are gone, `CRI-<n>` is flat and the hierarchy is a parent link;
  the tracker no longer travels with the code, so an old checkout cannot show the items of its day.
- Gained: no jsonl conflicts, issues move on merge without an agent, a UI and relations for the user.
- The tracker depends on a network service and a personal API key; a missing key or an API error is a
  stated failure of the helper scripts, never an empty queue.
- Whether a subagent inherits the interactive Linear MCP session was checked when this change was made
  (see the work item's report); the shell helper is the fallback for reads.
