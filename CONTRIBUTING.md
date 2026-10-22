# Contributing to Critical Hit

Thanks for your interest. This page is the developer entry point; the project's rules and design
decisions live in the docs it links, not here.

## Prerequisites

- [Go](https://go.dev/dl/) (version in `api/go.mod`) and [Node.js](https://nodejs.org/) with npm
- [just](https://github.com/casey/just) (every task is a `just` recipe; run `just` to list them)
- [Docker](https://docs.docker.com/get-docker/) with Compose (the dev database is PostgreSQL)

Run `just install` once to fetch the Go and npm dependencies.

## The developer loop

| Command | What it does |
|---|---|
| `just dev` | Starts the full stack with live reload: database, API and web UI |
| `just status` | Shows what is running |
| `just stop` | Stops the stack, database included |
| `just dev-reset` | Deletes the local databases so the next `just dev` starts clean |
| `just validate` | The mandatory gate: backend and frontend lint, tests and build, plus the repo checks |
| `just e2e` | Runs the headless Playwright suite (first time: `just e2e-install`) |

`just validate` must pass before any change is considered done. Ports, environment variables and
how to run the app for checks are described in
[docs/agent-workflow.md](docs/agent-workflow.md).

## Repository layout

| Path | Contents |
|---|---|
| `api/` | Go backend |
| `web/` | React frontend |
| `seeds/` | Game data loaded into the database |
| `docs/` | Documentation and the decision records ([ADRs](docs/adr/README.md)) |
| `scripts/` | Tooling used by the justfile and CI |

## Where the rules are

- [AGENTS.md](AGENTS.md): the working rules for contributors, human or AI.
- [docs/agent-workflow.md](docs/agent-workflow.md): validation, worktrees, generated code, hard rules.
- [docs/adr/README.md](docs/adr/README.md): index of the decisions and house rules.
- [docs/all.md](docs/all.md), [docs/backend.md](docs/backend.md), [docs/frontend.md](docs/frontend.md):
  scope, backend and frontend guides.
- [GLOSSARY.md](GLOSSARY.md): the project vocabulary (terms only).
- [docs/domain-model.md](docs/domain-model.md): the entities, their relations and invariants.

Generated code (database access, mocks) is committed and never edited by hand; how to regenerate it
is in [docs/backend.md](docs/backend.md).

## Commits and pull requests

Commits and PR titles follow [Conventional Commits](https://www.conventionalcommits.org/)
(`feat`, `fix`, `docs`, `chore`, ...). The PR title becomes the squash commit on `main` and decides
the release version bump; CI checks its format. Details are in
[docs/agent-workflow.md](docs/agent-workflow.md#commits).

## Rulebooks

The game is based on the AD&D 2nd edition rules. The text of the Player's Handbook and the Dungeon
Master's Guide is never copied into this repository. Rules are implemented from the decisions in
the ADRs; see the Rulebooks section of [AGENTS.md](AGENTS.md) and
[docs/agent-workflow.md](docs/agent-workflow.md#rulebooks).

## License

By contributing you agree that your contribution is released under the [MIT License](LICENSE).
