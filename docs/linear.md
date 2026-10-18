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
