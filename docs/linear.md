# Linear configuration

The beads -> Linear mapping is written once, in [`scripts/linear-mapping.json`](../scripts/linear-mapping.json).
`scripts/linear-configure.sh` (`just linear-configure`, `--dry-run` to preview) shapes the workspace from it and
the import reads the same file; change the mapping there, never in a copy.

| Beads | Linear |
|---|---|
| P0 / P1 / P2 / P3 / P4 | Urgent / High / Medium / Low / None |
| open | Todo |
| in_progress | In Progress |
| deferred | Backlog + label `deferred` |
| PR open | In Review |
| closed | Done |
| epic | parent issue with label `epic` |
| bug / feature | the workspace labels Bug / Feature |
| chore / docs / question | labels of those names |
| task | no label |

Team `CRI`; project "critical-app" belongs to it. Process labels: `epic`, `needs-info`, `risky`, `deferred`,
`chore`, `docs`, `question`, plus the area labels still in use (`mvp-saturday`, `backend`, `frontend`, `infra`,
`data-prep`, `tooling`). The built-in workflow states are enough; none is created.

The script is idempotent: a second run changes nothing and says "already configured". It never deletes or
renames anything, and stops before any write when the team, the project or a mapped state is not found.

## Import from beads

`scripts/linear-import.sh` (`just linear-import`, dry run by default, `--apply` to write) brings the live work
over. It is a script of our own against the GraphQL API because Linear's importer has no generic input, no parent
or blocks relations and folds comments into the description. The mapping above is read from
`scripts/linear-mapping.json`.

- **Scope:** every epic (closed ones as Done, label `epic`) and every item that is not closed. Tombstones and
  the other closed history are not imported; a relation to such an item is listed as text in a comment on the
  issue ("Relations to items not imported").
- **Old id:** the title ends with ` [<bead id>]`, so a search for the bead id finds the issue. That suffix is also
  how a rerun recognizes what it imported.
- **Carried over:** title, description unchanged (an acceptance-criteria field missing from it is appended as a
  section), state, priority, labels, created and completed dates, the parent link when the parent is imported,
  `blocks` relations between imported items (the blocker blocks the dependent), comments and notes as comments
  (a line `imported from beads [comment n]` marks each one). Assignees are not carried over.
- **Idempotent:** a rerun after a partial or complete run creates no duplicate and fills in what is missing.
  Transient errors (HTTP 429/5xx, rate limit) are retried; any other failure stops the run and names the last
  step done.
- **Id map:** `docs/beads-id-map.json` (`{"critical-…": "CRI-n"}`), rewritten after every created issue, one
  entry per imported bead. `check-no-tracker-ids` reads old ids from it once the archive is gone.
- Needs the labels and states of the mapping in the team: run `just linear-configure` first. The live run is part
  of the cutover (re-export the beads first: the committed `.beads/issues.jsonl` can be behind the tracker).
