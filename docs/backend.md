# Critical — Backend Context (Go Agents)

## Stack

- **Language**: Go (latest stable)
- **HTTP**: standard `net/http` or `chi` router
- **Database**: PostgreSQL ([ADR 0025](adr/0025-postgresql-storage.md)) through `database/sql` and the `github.com/lib/pq` driver (sqlc's `database/sql` output binds arrays with `pq.Array`). `storage/postgres.Open` opens the pool from `CRITICAL_DSN` (a PostgreSQL URL or connection string) and every new connection of its pool, the first one at boot included, goes through a gate (`gate` in `postgres.go`): while the database answers, one attempt, bounded to 5 s with the handshake (lib/pq's `connect_timeout`, unless the DSN sets one); once an attempt fails as a stopped or starting database fails, the gate is down, one probe alone retries every second, and every connection asked for meanwhile waits for it without dialling, at most 60 s and never beyond its caller's context (a request's 8 s deadline, `router.RequestDeadline`, then `postgres.ErrUnavailable`, which is `domain.ErrUnavailable`; the boot's 60 s). Every request answers within that deadline, even when lib/pq is stuck on a statement sent to a server that stopped answering (`middleware.Deadline` buffers the response and answers at the deadline); a request the database could not serve in time answers 503 `{"code": "database_unavailable"}` ([ADR 0025](adr/0025-postgresql-storage.md) step 10). It retries exactly SQLSTATE 57P03 ("starting up"), EOF during the handshake (what Flycast answers while the cluster starts) and network errors (refused, reset, timeout) except a DNS name that does not exist; everything else fails at once: any other PostgreSQL error (wrong password, missing database), an unknown host, `pq.ErrSSLNotSupported` (sslmode=require without TLS), TLS and certificate errors, any other driver error (`retryable` in `postgres.go`); a SIGINT/SIGTERM during the boot's wait stops the server at once; the pool holds at most 4 connections per server, keeps 1 idle and closes a connection idle for 30 seconds (the shared fly node has 256 MB), so it never holds the fly cluster awake nor reuses a connection across the cluster's idle stop; in development it is the docker compose service `postgres` ("Development database" below), on fly the cluster of [ADR 0024](adr/0024-deployment.md) step 2. The server binary links no SQLite driver (`just check-no-sqlite`, part of `just validate`).
- **Writer lock**: every write transaction (`InTx`) runs at READ COMMITTED and takes one transaction-level advisory lock (`pg_advisory_xact_lock`, one fixed key; the sqlc query `LockWriters`) as its first statement ([ADR 0025](adr/0025-postgresql-storage.md) step 4). A second writer waits until the first commits or rolls back, then reads its result, so the action stack's stale expected top is refused as a normal 409, never a 500. The commit or the rollback releases the lock; nothing else does. A stricter isolation level would break this (the snapshot would predate the lock). Write paths must go through a transaction, never read-then-write on the bare pool; reads outside a transaction take no lock.
- **Migrations**: simple sequential SQL files, applied at startup and recorded in `schema_migrations`
  ("Migrations at startup" below)
- **Seed loader**: reads `seeds/*.json` at startup if corresponding table is empty; spells come from the
  catalogue artifact instead ("Spell catalogue" below)

## Project Structure (api/)

```
api/
  main.go               ← HTTP server, router, startup
  internal/
    db/
      db.go             ← PostgreSQL open, migrate, seed
      migrations/       ← 001_init.sql, 002_hp_ledger.sql, etc.
    domain/
      character.go      ← Character struct, attributes
      hp.go             ← HpEvent struct, ledger logic
      combat.go         ← RollResult, attack/damage resolution
      buff.go           ← BuffEffect struct, charge/duration logic
      spell.go          ← SpellSlot, Spell structs
    handlers/
      hp.go             ← HTTP handlers for /api/hp/*
      combat.go         ← HTTP handlers for /api/roll/*
      buffs.go          ← HTTP handlers for /api/buffs/*
      spells.go         ← HTTP handlers for /api/spells/*
      round.go          ← HTTP handler for /api/round/advance
    engine/
      dice.go           ← All dice rolling (seeded RNG for tests)
      combat.go         ← attack and damage resolution
      buffs.go          ← Buff application, expiry detection
```

## Data Model

**Hybrid model:** core scalars as columns, complex variable-length state as JSON columns.

### Normalized tables (core scalars)
```sql
-- characters: one row per character (ADR 0029 step 7)
id, name, str, dex, con, int, wis, cha,
hp_max, thac0_base, level, xp_total, class, race_code
-- (class and race_code are codes, '' race: none set; level, THAC0, saves and slots are derived
--  from them and the XP total on every read, ADR 0030 steps 3 to 8 (charactersvc.Derive): the stored
--  level, thac0_base and save_* columns are the figures the imported sheet stated, kept as
--  imported and compared with nothing (step 24, retired), never inputs)

-- action_log: THE append-only log of table actions (HP ledger + combat log)
id, character_id, round, kind TEXT, summary TEXT, hp_delta INTEGER, payload JSON, created_at,
transaction_id
-- (transaction_id: the id of the first entry of the request that recorded it, ADR 0018 step 1;
--  undo pops a transaction whole. Rows older than the column are each their own transaction)
-- action_snapshots: before/after state an entry changed, one row per state key
id, action_id, state_key, before_envelope JSON, after_envelope JSON, before_state, after_state
-- (before_envelope / after_envelope: the action log codec's envelope, {"id","state"} or
--  {"id","absent":true}, ADR 0018 steps 8 and 9; before_state / after_state hold the encoding of
--  before the codec, no longer read, kept for the release before it and dropped later)
-- action_corrections: audited amend/void events pointing at an entry (never edit an entry)
id, action_id, kind ('amend'|'void'), hp_delta, summary, payload JSON, note, step_id, created_at
-- (step_id: the 'correction' entry a post-roll correction belongs to, ADR 0010 step 6; popping
--  that entry puts the amend out of effect. NULL on older amends, which stay in effect)
-- (each amend field, when set, replaces the entry's: latest amend that set it wins; the log's
--  amend now sets only note, post-roll adjustments set the payload)
-- summary holds only text the player typed (an HP change's source); every other entry is a
-- structured payload the UI words in the player's language (older rows keep an English summary)
-- an attack's and a save's payload is also their endpoints' answer: one Go struct
-- (combatsvc.AttackBreakdown, combatsvc.SaveBreakdown), the answer adding only logEntryId as its
-- last key (and, for one release, the save's previous names hit, hitMargin and baseThac0)

-- hp_events: the legacy HP ledger, carried into action_log by the SQLite migration 007 (before the PostgreSQL baseline) and dropped by 018

-- xp_entries: the XP ledger, outside the action log (edited and deleted in place, ADR 0015);
-- the total is characters.xp_total plus every entry (base + 10% when bonus)
id, character_id, entry_date TEXT, session TEXT, reason TEXT, base INTEGER, bonus INTEGER (0/1)

-- proficiencies, bard_skills: each character's skill catalog (ADR 0014); the seed character's is
-- reconciled by id with seeds/skills.json at startup; a check is an action_log entry (skill_check),
-- nothing is stored here
id TEXT, character_id, position, ability TEXT ('str'..'cha'), modifier INTEGER
id TEXT, character_id, position, percent INTEGER (0-100)

-- items: each character's possessions, weapons included (character_id; "Per-character data" below;
-- ADR 0033: category, magic_kind, catalogue_code, state carried/worn/wielded and each category's
-- columns); equipment_review: the review list (ADR 0033 step 27); item_id_renames: the conversion's
-- renamed item ids (step 26.3); weapons: unread since migration 016 (converted into items)
-- spellbook_entries: the spells each character knows, by catalogue key, in sheet order
character_id, spell_key TEXT, position

-- dice_favourites: the composer's saved formulas, per character, in normal form (unique per character)
id, character_id, formula TEXT
```

### Dice composer (dicesvc)

`POST /api/roll/dice {formula}` rolls a free formula through the dice port and records a `dice` action
log entry (payload `{formula, groups, modifier, total}`, no snapshot: undo only pops it). `dice` is
deliberately not a roll kind (`actionlog.RollKinds`): a stray composer roll neither replaces the result
card nor ends the correction window of the latest save or attack (ADR 0006 step 6). Favourites:
`GET|POST /api/dice/favourites`, `PUT /api/dice/favourites/order` (body `{"ids":[…]}`, the full order: every favourite id once, else 400 on `ids`; answers the list), `DELETE /api/dice/favourites/{id}` (answers the remaining list).

Grammar (`dicesvc.Parse`, not `pkg/formula`, which serves spell effects): terms joined by `+` or `-`, each
`NdM`, `dM`, `d%` (a d100) or an integer constant; spaces ignored, `d` case-insensitive. At least one
die. Limits: formula up to 200 characters, 1 to 100 dice per term and 100 in total, 2 to 1000 sides,
constants adding up to at most 10000 (in absolute value), 50 favourites. A violation is a 400 with a
`formula` field error. The answer's `formula` is the normal form (same dice merged in order of first
appearance, one trailing constant, `d%` for 100 sides, no count for one die), which is also what a
favourite stores. Out of scope: keep-highest, advantage, named rolls.

### JSON column tables (complex state)
```sql
-- active_buffs: JSON array of BuffEffect objects
id, character_id, buffs JSON

-- spell_slots: JSON object of slots by level
id, character_id, slots JSON

-- round_state: current round counter and its initiative (d10, the log entry that rolled it; NULL: none)
id, character_id, round_number INTEGER

-- attack_forms: the attack form's last choices, one row per character (no row: the defaults)
character_id, weapon_id, previous_weapon_id, ammo_item_id, target_ac, target_size,
situational_value, situational_label
```

**Recording actions.** A service that changes state records an `actionlog.NewEntry` through the
action log's `Record` inside the same `domain.TxRunner.InTx` as its state write (the runner is
`actionlogsvc.Transactions(db)`, built once in the composition root: every entry recorded in one
outermost `InTx` is one transaction, popped by one undo, ADR 0018), with
`Snapshots` of the state it changed (a `before` and an `after` JSON per state key). It registers an
`actionlog.Restorer` for its state key on the action log in the composition root, so voiding
(popping) the entry restores the "before" state; a service exposes its restorers (for example
`buffSvc.Restorers()`) for the composition root to register. A state key `family:id` (for example
`items:arrows_plus3`) is restored by the Restorer of its family. A snapshot of state that other
services also write (the slot list, the buffs) is produced by one helper of the owning service
(`prepsvc.SlotsSnapshot`, `buffsvc.Add`) so every writer encodes it the same way.

What each service records, and the rules it follows, are in the ADRs ([index](adr/README.md)).

## Migrations at startup

`postgres.Migrate` (run by the composition root before the server listens) applies every embedded
`api/internal/storage/postgres/migrations/*.sql` file not yet recorded in `schema_migrations`, in
lexical order, all in one transaction that holds the writer lock from its first statement
([ADR 0025](adr/0025-postgresql-storage.md) step 3.1): of two machines starting together, the second
waits, then finds everything applied. A failing migration rolls the whole run back. A file runs as
one statement list (no parameters), so it may hold several statements; DDL is transactional.

`001_baseline.sql` is the schema the 33 SQLite migrations built, squashed (step 2): identity ids,
`BIGINT` integers, `TIMESTAMPTZ` timestamps; 0/1 flags and JSON stay checked `BIGINT` and `TEXT`
(the file says why). Migrations are schema only (no `INSERT`, `UPDATE` or `DELETE`,
`TestMigrations_Step2_1_SchemaOnly`) and new files only: never rename, edit or delete an applied one.
Data a schema change needs first (rows a `NOT NULL` would refuse) is a **data step**: Go code the run
executes on its transaction just before that migration, only when the migration is pending
(`migrationDataSteps`; today the owners before migration 004, "Per-character data"; after a migration,
the seeded weapons' types after 005 and the seed character's race code after 007). Its rules,
including when a step and its queries may be deleted, are [ADR 0025](adr/0025-postgresql-storage.md)
step 2.1.1.

Before applying anything it refuses a database that records a migration the binary does not embed
([ADR 0024](adr/0024-deployment.md) step 6.2): the server exits with
`migrate: database records migrations this binary does not embed: <names>; …`. An older binary
does not undo a newer schema, so it must not run on one. In production this is a rollback across a
migration without the backup restore (ADR 0024 steps 6.1 and 9.3).

In development it happens when the compose database was migrated by a branch that added a migration
and you then run a branch that does not have it (e.g. `main` before that PR merged). To get past it:
- go back to (or merge `main` into) a branch that embeds the named migrations; or
- in a worktree, or for fixture data only, `just dev-reset` and restart `just dev` (deletes the
  compose database). Never in the main checkout once it holds the user's campaign data; or
- keep a `pg_dump` of the database before trying a migration branch and restore it afterwards.

Never delete the rows from `schema_migrations` to get past the check: the schema would still be the
newer one.

## Local development

The loop, from the checkout root (each worktree is its own compose project, with its own volume):

| Command | What it does |
|---|---|
| `just dev` | Starts PostgreSQL (docker compose), the API (air, :8080) and Vite (:5173, proxies `/api`). Ctrl-C stops them. |
| `just stop` | Stops what `just dev` started for this checkout, the database included, from any terminal, also after a crashed `just dev`. Data kept. Safe when nothing runs. |
| `just status` | API, web dev server and database: running, stopped, port. |
| `just dev-db` | Starts the database only (e.g. for the SQLite import below). |
| `just dev-reset` | `just stop`, then deletes the databases (compose volume `critical-pg`). The next start migrates, seeds and imports the catalogue again. |
| `just dev-smoke` | Proof of the above on free ports and a throwaway compose project (needs docker, ~1 min). Run it after changing the justfile, `docker-compose.yml`, `api/.air.toml`, `scripts/dev.sh` or the start-up of the storage. CI job `dev-smoke`. |

Login: `just dev` runs without OAuth settings and with `CRITICAL_ENV=dev`, so the server has the dev login: the login page
shows "Log in" (the dev login, the code icon), which logs in as the root admin (`dev@localhost`, created on the first start of an empty database, also
after `just dev-reset`); the session lasts 30 days. Nothing to set up ("Accounts and login" below).

First run after a clone: `cd web && npm ci`, optionally `just catalogue` (real spells, database
`critical`; without it the fixture catalogue and database `critical_fixture` are used and the start
says so), then `just dev`. Open http://localhost:5173.

Ports: `CRITICAL_API_PORT` (8080), `CRITICAL_WEB_PORT` (5173), `CRITICAL_PG_PORT` (5432, the service
listens on 127.0.0.1 only). A taken port, or a docker daemon that is not running, stops `just dev`
before anything starts, with a message saying what to do.

What `scripts/dev.sh` guarantees:
- **Ctrl-C.** Each process runs in its own session, so the terminal's SIGINT reaches only the script,
  which stops Vite, then the API, then the database if this run started it (a database that already
  ran stays up until `just stop`). Exit is clean, no process left, ports free. `just` itself prints
  one line `error: interrupted by SIGINT` (its own exit message for a Ctrl-C, exit status 130); it
  cannot be silenced.
- **Graceful API.** `api/.air.toml` sets `send_interrupt` and `kill_delay = "10s"`: on every rebuild
  and on exit the server gets SIGINT, finishes in-flight requests, closes its pool, and is killed
  only after 10 s (air's default is an immediate SIGKILL).
- **Only this checkout's processes.** Processes are recorded in `.dev/run/` (pid and start time) and
  stopped by that record, the API binary by its exact path; never by name. Two checkouts do not
  interfere except on ports (move them with the variables above).
- **Database.** Data lives in the volume `critical-pg`; stopping keeps it. After `just stop` the
  container stays stopped (`restart: unless-stopped`), also across a reboot, until `just dev` or
  `just dev-db`.

- database `critical` with the real catalogue (`.catalogue/catalogue.json`) and the campaign seeds;
  `critical_fixture` with the fixture catalogue and seeds of `api/testdata` when there is no real
  catalogue (the init script `deploy/postgres-init/` creates it with the volume);
- DSN `postgres://critical:critical@localhost:5432/<database>?sslmode=disable`, also the default
  `CRITICAL_DSN`;
- files in `.dev/` (apart from `run/` and `tmp/`) are never touched. A SQLite `.dev/critical.db` left
  from before the engine switch is no longer read: the copy tool that moved it into PostgreSQL is
  retired ([ADR 0025](adr/0025-postgresql-storage.md) step 9.5).

## Accounts and login

[ADR 0029](adr/0029-users-login-and-access.md) on the server: `authsvc` (users, invites, sessions,
the Google login, the dev login, the root admin), `authdal` (sqlc, migration 003: `users`, `invites`,
`sessions`), `googleadpt` (OpenID Connect: code exchange with PKCE, ID token verified with Google's
keys, fetched at the first login and cached; nothing at start-up), `authhdl` (`/api/auth/*`) and the
gate `middleware.Gate`.

- **Gate** (step 6): one middleware on the router, with one allow-list (`middleware.Public`): `/login`
  and everything under it, `/api/auth/*`, `/api/health`. Without a session `/api/*` and any asset
  answer 401 `unauthenticated`; a page (a GET asking for `text/html`) redirects to `/login`. A new
  route is protected by default. The gate reads the session once and puts the user in the request
  context (`middleware.PrincipalFrom`); handlers and services never read the cookie.
- **Sessions** (step 4): cookie `critical_session_<app>` (HttpOnly, SameSite=Lax, path `/`, host-only,
  `Secure` when the public URL is https) holding a 256-bit random token; the table stores its SHA-256,
  never the token. 30 days, extended (one write) when used more than a day after the last extension.
  A mutation whose `Origin` (else `Referer`) names another host than the app's is refused 403
  `cross_origin`; a request with neither (curl, tests) passes, SameSite=Lax keeps the cookie off
  cross-site subrequests. `<app>` is the public URL's host and port (`critical-hit.fly.dev`,
  `localhost_8091`), in dev login mode `dev_<listen port>` (`dev_8080`): browsers send a cookie to
  every port of a host, so with one name `just dev` on `localhost:5173` and a hosted app opened with
  `just fly-open` on another localhost port would log each other out and hand one app's token to the
  other.
- **Endpoints**: `GET /api/auth/google/start` (state and PKCE verifier in the 10-minute cookie
  `critical_login_<app>`, redirect to Google), `GET /api/auth/google/callback` (session and redirect to `/`,
  or a refusal to `/login?error=<code>`), `POST /api/auth/logout`, `GET /api/auth/me`
  (`{id, email, role, root}`, or 401: `session_expired` when the request carried a session the server no
  longer knows, `unauthenticated` without one), `GET /api/auth/options` (`{"google": bool, "devLogin":
  bool}`, the login page's buttons), and in dev login mode only `POST /api/auth/dev` (`{"email": …}`
  optional: none is the root admin, created as `dev@localhost` on a database without one; an unknown
  email is created as an admin).
- **Settings**: `CRITICAL_GOOGLE_CLIENT_ID`, `CRITICAL_GOOGLE_CLIENT_SECRET`, `CRITICAL_PUBLIC_URL`:
  all three, the Google login; partial, a start-up error. None is the dev login only with the explicit
  opt-in `CRITICAL_ENV=dev` (set by `just dev`, the e2e stacks, image-smoke and the compose `critical`
  service); without it the server refuses to start (step 8.2: fails closed, e.g. the image run outside
  fly). On fly.io (`FLY_APP_NAME`) `CRITICAL_ENV=dev` is refused, so the dev login cannot exist there. Runbook: docs/deploy.md, "Login on the
  hosted apps".
- **Root admin** (step 5): `just root-admin <email>` (`api/cmd/rootadmin`) on `CRITICAL_DSN`, unset:
  the local database `just dev` uses. The schema allows one root, and a root is an enabled admin.
- **Admin section** (steps 1.3, 2, 5.2; `adminhdl`, mounted behind `middleware.AdminOnly`, which
  answers 403 `admin_only` to a player before any service runs; `PUT /api/crit-table` sits behind it
  too): `GET /api/admin/users` (`{"users": [{id, email, role, root, disabled, lastLogin|null,
  createdAt, characters: [names]}]}`), `PATCH /api/admin/users/{id}` (`{disabled?, role?}`, one
  transaction; disabling deletes the user's sessions; the user as changed), `GET /api/admin/invites`
  (`{"invites": [{id, email, invitedBy, createdAt}]}`, pending only), `POST /api/admin/invites`
  (`{email}`, 201 with the invite), `DELETE /api/admin/invites/{id}` (cancelled, never deleted; the
  pending invites left). No migration: the tables are migration 003's.
- **Error codes**: `unauthenticated`, `session_expired` (an `unauthenticated` refusal, only from
  `GET /api/auth/me`), `cross_origin`, `login_state_mismatch`, `login_failed`,
  `email_unverified`, `not_invited`, `user_disabled`, `root_admin_protected`, `root_admin_taken`,
  `admin_only`, `email_has_user`, `already_invited`, `invite_not_found`, `user_not_found`
  (`internal/render`, worded by the web's `apiError.*`).
- **Public paths** (step 6.1, as amended 2026-10-18), judged on the cleaned path (`/assets/../api/x` is
  `/api/x`): `/login` (and under it), the static build
  (`/assets/*`, `/favicon.svg`: the login page is a page of the one bundle), `/api/auth/*`, `/api/health`.
  Any other page without a session redirects to `/login` (`/login?error=session_expired` when its cookie
  was stale); any other `/api` route answers 401 `unauthenticated`.
- **Web**: the login page and Log out, docs/frontend.md "Login page".

## DAL code generation

The DAL adapters in `api/internal/adapters/*dal/` run hand-written SQL through Go code generated by
**sqlc** ([ADR 0025](adr/0025-postgresql-storage.md) step 5; bob, the generator before it, is
removed). Generated files are committed and never edited by hand; the CI `generated` job reruns
`just generate` and `just mocks` and fails on any difference.

### sqlc

Every adapter has a `queries` package generated by sqlc (`emit_interface: true`), with a
`Querier` interface, and a mockery mock of it in `queries/mocks`:

```mermaid
flowchart LR
    SQL["queries/*.sql (hand-written)"] -->|"sqlc (just generate)"| GEN["queries: Querier, Queries, params, rows"]
    GEN -->|"mockery (just mocks)"| MOCK["queries/mocks: MockQuerier"]
    CR["composition root"] -->|"queries.New(postgres.Routed(pool))"| DAL["adapter: New(queries.Querier)"]
    GEN --> DAL
    MOCK -->|"adapter tests"| DAL
```

- **Wiring.** The adapter's constructor takes its package's `queries.Querier`. The composition root
  passes `queries.New(postgres.Routed(db.Pool()))`: `Routed` runs each statement on the transaction
  the statement's context carries (`InTx`), else on the pool, the same routing as `postgres.Conn`. The
  root builds every adapter over that one routed connection (`postgresStorage`, through `storageOn`);
  a test builds `postgresStorage` over an unopened DB and calls each storage port under a context
  whose transaction (`postgres.WithTx`) records being reached, so an adapter wired over the pool itself,
  which would escape the caller's transaction, fails it.
- **Tests.** An adapter test builds the adapter on `mocks.NewMockQuerier(t)` and asserts which
  `Querier` methods are called, with which params, in which order where it matters (`mock.InOrder`),
  and how rows and errors map to domain values (`sql.ErrNoRows` -> the domain's not-found error, zero
  rows affected -> not found). An adapter that writes localized texts (`itemdal`, `weapondal`) also
  takes a `textdal.Store`, built on its own mocked `Querier` in the tests. No database, no SQL text ([ADR 0025](adr/0025-postgresql-storage.md)
  step 6). What only the SQL decides (an `ORDER BY`, a `CHECK` or `UNIQUE` constraint, a query plan) is checked by sqlc at generate time and by running
  the app (step 7), not by these tests; the rollback of a transaction is the engine's, and the
  services' tests check that a failure leaves the transaction body.
- **Config.** `api/sqlc.yaml`: one `sql` entry per adapter (`queries:` and `out:` both its
  `queries` directory, package `queries`), engine `postgresql`, plus one for the storage's own
  statements (`storage/postgres/queries`: the writer lock, the seeds, the catalogue import).
  `omit_unused_structs` keeps each package to the tables its queries use.
- **Schema.** sqlc reads the migration files (`api/internal/storage/postgres/migrations`) itself:
  `just generate` needs no database server. `schema_migrations` is not in them (the guard reads it
  before any migration ran), so its few statements are plain SQL in `storage/postgres/migrations.go`.
- **Storage tests.** The seed and the catalogue import run on the storage's `queries.Querier`; their
  tests (`storage/postgres`) drive the real code over `fakeStore`, an in-test implementation of
  that `Querier` holding the tables and refusing what the schema refuses (duplicate keys, the
  foreign keys to `characters`, the `CHECK`s of weapons and items, empty text bodies), with
  all-or-nothing transactions. They assert the stored result of multi-step reconciles (the seed's
  name languages, the import's keys and texts), not SQL.

**Adding a query** to an adapter:
1. Add it to the adapter's `queries/*.sql` with a sqlc header, `-- name: <GoName> :one|:many|:exec|:execrows`,
   and `$1`, `$2`… placeholders (sqlc names the params after their columns).
2. `just generate`, then `just mocks` (the `Querier` interface changed).
3. Call it from the adapter through `d.q`, and test the call on the mock.

**Adding an adapter**: its `queries/*.sql` with sqlc headers; an entry in `api/sqlc.yaml`; its
`queries` package in `.mockery.yaml` (interface `Querier`, dir `queries/mocks`); a constructor taking
`queries.Querier`, wired over the routed connection in `storageOn` (and a call in the wiring test);
its tests on the mock; `just generate`, `just mocks`. A renamed or deleted test cited by an ADR is
updated in that ADR (`just check-adr-tests`).

**Constraints and conventions.**
- `:execrows` returns the rows affected: use it for an update or delete whose caller needs to know
  whether the row existed, instead of `RETURNING id`. When the caller needs a column of the changed
  row (the name key an item or weapon update or delete passes on to `textdal`), use `:one` with
  `RETURNING` that column: no row is `sql.ErrNoRows`, mapped to not found.
- A `:one` query with no row returns `sql.ErrNoRows`; the adapter maps it to its domain error (or to a
  default, as `dicedal` does for the first position).
- Aggregates (`SUM`, `COUNT`) are named queries like the others; `COUNT(*)` is `int64`, cast
  anything else (`(... )::bigint`) so sqlc types it `int64`. A sum that applies a rule is not an aggregate: the query reads the rows
  and the domain sums them (the action log's HP total, `actionlog.HPTotal`, ADR 0007 step 2), so
  the rule has one tested implementation.
- A list binds as an array: `= ANY(sqlc.arg(ids)::bigint[])` (sqlc wraps it in `pq.Array`); an
  empty list matches nothing, so an optional list filter takes a flag too (`spelldal`'s
  `SearchSpells`: `any_level`). A parameter used twice is named with `sqlc.arg(name)`.
- A `LIMIT` parameter is typed `int32` unless named and cast: `LIMIT sqlc.arg('limit')::bigint`.
- A `TIMESTAMPTZ` column reads and writes as `time.Time`, with microseconds and in the session's time
  zone. The API's `createdAt` keeps its format, UTC in whole seconds (`2026-10-16T12:00:00Z`): the
  adapter reading a `created_at` passes it through its `storedTime` (the in-memory store stamps the same).
- sqlc may refuse an unqualified column in a query with a correlated subquery over another table
  ("column reference is ambiguous"): qualify it (`action_log.kind`).
- Case-insensitive text matching is `lower(column) = ?` with the adapter lowering the parameter
  (`spelldal`'s class filter), never `LIKE`: PostgreSQL's `LIKE` is case-sensitive.
- `BIGINT` is `int64`, `DOUBLE PRECISION` `float64`, `TEXT` `string`, a nullable column a `sql.Null*`;
  a 0/1 flag column stays `int64` and the adapter converts it.
- sqlc is a `tool` in `api/go.mod` (`go tool sqlc`); it builds with cgo (its PostgreSQL parser), so
  `just generate` needs a C compiler. The tool depends on a SQLite driver of its own; the server
  binary does not link it.

### When to regenerate

Regenerate whenever:
- A migration is added or changed (new table/column)
- A `.sql` query file under `api/internal/adapters/*/queries/` or `api/internal/storage/postgres/queries/` is added or changed
- The sqlc version in `api/go.mod` is bumped

### How to regenerate

```bash
just generate
```

This recipe (defined in `justfile`) runs `sqlc generate` (it reads the migrations of
`api/internal/storage/postgres/migrations/` and the query files, offline), then `go mod tidy`.

### Dependency bumps

Dependabot (`.github/dependabot.yml`) opens one grouped minor/patch PR per week for each of Go modules (`api/go.mod`, including the `tool` directives: golangci-lint, mockery, sqlc, air), npm, GitHub Actions and Docker base images. sqlc is excluded from the Go group: its bumps arrive as their own PR, are never auto-merged, and change the generated output (sqlc writes its version into every generated file).

When a bump's `generated code is up to date` CI job fails (sqlc or mockery changed their output), check out the Dependabot branch, run `just generate` and `just mocks`, and commit the regenerated files on it; then `just validate`. Never hand-edit generated files.

Auto-merge: `.github/workflows/dependabot-auto-merge.yml` enables GitHub auto-merge (squash) on a Dependabot PR whose updates are all semver patch or minor (not sqlc), and labels a major one `major-update` and leaves it open. GitHub merges once main's required checks pass (validate, e2e, image, dev-smoke, generated code is up to date; the PR-title check is not required). Both the repository's auto-merge setting and those required checks are set once by `scripts/github-branch-protection.sh` (admin). Red, major, sqlc and conflicting PRs go through the supervisor ("PR watch" in `.claude/agents/supervisor.md`).

### Layout

| Path | Contents |
|------|----------|
| `api/internal/adapters/*/queries/*.sql` | Hand-written SQL query sources |
| `api/sqlc.yaml` | sqlc config, one entry per adapter (section above) |
| `api/internal/adapters/*/queries/{db,models,querier}.go`, `*.sql.go` | sqlc output of an adapter |
| `api/internal/adapters/*/queries/mocks/` | mockery mock of an adapter's `Querier` |

## Key Go Structs

```go
type HpEvent struct {
    ID          int64
    CharacterID int64
    Delta       int
    Source      string // "goblin arrow", "cure light wounds", etc.
    CreatedAt   time.Time
}

type BuffEffect struct {
    ID       string // uuid
    Name     string
    Charges  int
    Duration int
    Effects  BuffEffects // pointers: nil = no contribution
}

type RollResult struct {
    D20Roll         int
    D100CritRoll    int
    IsCrit          bool
    CritMultiplier  float64
    CritPlaceholder bool
    BaseThac0       int
    Modifiers       []Modifier // {kind, detail, value}: every to-hit modifier
    EffectiveThac0  int
    TargetACNeeded  int
    Hit             bool
    HitMargin       int
    // Damage breakdown (populated if hit)
    BaseDamage      int
    DamageDice      string
    SpellsBuff      float64
    WeaponBuff      int
    Multipliers     float64
    DiceMultiplier  float64
    StrengthBonus   int
    BonusDamage     int
    DamageTerms     []DamageTerm // {term, kind, detail, value}: every damage contribution
    UnroundedDamage float64      // before the final floor; the UI lays out the formula
    TotalDamage     int
    // Attack context: weapon, target AC/size, ammo spent + remaining, log entry id
}
```

## Seed Loader Pattern

The seed files are read and checked by `api/internal/storage/seedfile` (the spellbook by
`spellbookadpt.ReadSeed`), shared by every storage engine (the database and the in-memory storage).
At startup, in one transaction (`Seed`): `character.json` only into an empty `characters` table;
the other files are the starting set of the **seed character** ("Per-character data" below):
weapons and items only while that character has none (then only their names are reconciled, ADR
0023 step 10), skills reconciled by id, the spellbook only while that character has none.

## Per-character data

Weapons, items (counts, charges), nonweapon proficiencies, bard skills and the spellbook belong to a
character (`character_id`; `spellbook_entries` for the spellbook). Spell texts, the PHB tables and
the item and weapon rules stay shared. Every DAL method of these ports takes the character id and
reads or writes only that character's rows; an id is looked up among that character's (another
character's row of the same id is unknown: not found).

- **Seed character**: the character `character.json` names (the lowest id of that name). The seed
  files are its starting set; a character created later starts with nothing. The spellbook seed is
  applied once, like the weapons and items: only while the seed character has no spellbook entry
  (decided 2026-10-18 in review: spells are learned and removed in the app, [ADR 0030](adr/0030-classes-and-races.md)
  step 20, so a start must never re-add a removed spell or reset the order). Editing
  `seeds/spellbook.json` afterwards changes nothing for an existing character.
- **Every row has an owner** (migration 004): `character_id` is `NOT NULL` and the primary keys are
  `(character_id, id)`, so two characters may hold the same weapon, item or skill id. Rows without an
  owner (stored before migration 002, or written by a release without it during a rolling deploy)
  are given to the seed character by the migration run itself, just before migration 004, in the
  same transaction (`ownRows`, a data step of `Migrate`, ADR 0025 step 2.1.1): existing data keeps
  every count, position and field, with nothing to run by hand, and a
  database that gets 002 and 004 in one run (one upgraded from a release before 002) starts.
  Rows without an owner and no seed character (the name in `character.json` changed) refuse the
  start, rather than belong to no one; nothing is migrated.
- **Catalogue check at write time**: the spellbook seed is checked against the catalogue (a Wizard
  spell, its English name and level, each once) before the seed writes anything, so `Build` loads
  the catalogue artifact and checks the seeds before it seeds. Reading a spellbook names each entry
  from the catalogue (`spellbookadpt.Book`); the catalogue import keeps every spell a spellbook
  holds (`StateSpellRefs`, ADR 0021 step 5.4.3).
- **Undo**: snapshot keys are `items:<id>` (every piece, ADR 0033); old `weapons:<id>` snapshots are
  restored into the piece of that id by itemsvc. A restorer writes the row of the popped entry's
  character (the `characterID` every `Restorer` receives).
- **Expand, then contract** (the migrations are additive across a rolling deploy, ADR 0024 step
  5.2): migration 002 added `character_id` nullable, with a unique index on `(character_id, id)`,
  and kept the single-column primary keys the release before still upserted on. Migration 004 is
  the contract step: `NOT NULL`, and the 002 indexes become the primary keys. During its rolling
  deploy a release with 002 keeps working (it always writes the owner and upserts on
  `(character_id, id)`); a release from before 002 fails only its inserts of weapons and items (no
  owner) for the minutes both run. The in-memory store keys these rows per character too.

## Spell catalogue

Spells are not seeds: they come from the versioned catalogue artifact built in the private rulebooks repo
([ADR 0021](adr/0021-spell-catalogue.md)). The public repo never holds it.

- **Getting it**: `just catalogue` clones or fast-forwards the rulebooks repo (`just rulebooks`) and copies
  its `dist/catalogue.json` (at `CATALOGUE_REF`, default the checked-out branch) to the gitignored
  `.catalogue/catalogue.json`. Never part of `just validate`.
- **Config**: `CRITICAL_CATALOGUE`, default `.catalogue/catalogue.json`; a relative path is resolved against
  the repo root (the nearest directory above the working directory with a `.git` entry; outside a checkout,
  against the working directory). A missing file stops the API with a message naming the path and
  `just catalogue`.
- **Formats**: the API starts on formats 4 (the spells, the PHB rule tables and the class and race tables,
  ADR 0021 steps 9 and 10) and 5 (format 4 plus the racial level limits of DMG Table 7, step 11); any other
  is refused, naming it and `just catalogue`. Format 3 (no class tables) is retired since the characters'
  level, THAC0 and saves are derived from the class tables (step 10.4 as amended, ADR 0030): a dev
  checkout with an older `.catalogue/catalogue.json` runs `just catalogue`. Format 4 is read until the dev
  checkouts are format 5 (step 11.4). See "Rule tables" below.
- **Without access**: `api/testdata/catalogue.json` is a small fixture, format 5, with invented text
  (every spell the `api/testdata/seeds` spellbook and spell effects use, some with French, one list, one
  table) and the books' real rule-table, class-table and level-limit numbers (ADR 0021 steps 9.7, 10.5 and
  11.5). Go tests and the e2e suite always use it; `just dev` uses it, with `api/testdata/seeds` and its
  own database `critical_fixture`, when `.catalogue/catalogue.json` is absent (the campaign seeds refer to
  spells only the real artifact holds). The database `critical` is only ever opened with the real catalogue.
- **Served from memory**: each server reads every served spell and its texts once at boot, right after its
  import (`spelldal.Catalogue`, wired by the composition root as spellsvc's Reader and castsvc's Texts);
  the lookup, its filters, the names and the spell details never read the catalogue from the database
  again. It only changes at an import, which only runs at boot. During a rolling deploy a server of the
  previous version serves its own artifact's catalogue until it is replaced. Spell slots stay in the database.
- **Seed checks**: the spellbook seed and every `spell_effects.json` id are checked against the catalogue at
  startup, before the seed writes; an id it does not hold refuses the start.
- **Import** (`catalogueadpt` reads and validates against the embedded copy of
  `schemas/catalogue.schema.json`, kept identical by a test; `postgres.ImportCatalogue` imports): same
  version as recorded in `catalogue_meta` → only the typed `stats` of rows whose stored English stat texts
  the current parser reads differently are rewritten (none when the parser is unchanged); otherwise one transaction keys old rows (first
  import on a database seeded before the catalogue: `spellsvc.Keys`, verified against the artifact),
  upserts every spell by `spells.key`, deletes the spells the artifact dropped unless the seeds or the game
  state (slots, undo snapshots, log payloads) refer to them, and records the version. Each step and its test
  are in ADR 0021 step 5.
- **Columns**: `spells.key` (the artifact id, unique; NULL on older duplicate rows, never served), `class`,
  `level`, `source`, the typed `stats` JSON, and ten text columns (`name`, `description`, `school`, `sphere`,
  the six raw stat texts) that hold a **key into `localized_texts`**, not text
  ([ADR 0023](adr/0023-localized-text.md), migration 031).
- **Localized text**: `localized_texts (text_id, lang, body)`, primary key `(text_id, lang)`, one row per
  language present; English is the `en` row and the fallback. A text column is NULL (a text in no language,
  such as a Wizard spell's sphere) or keys at least one row; an absent language has no row, never an empty
  body. The importer keeps a spell's text keys across imports, writes only the rows that changed, deletes the
  row of a language the artifact dropped (and the key of a text it dropped), deletes a removed spell's texts
  with it, and gives a new text one more than the highest key. In Go a text is `localized.Text` (a map from
  language code to body, `domain/localized`); the rules code and the stats parser read `Default()`, the
  books' English. No column, field, JSON key or TS property is ever named for a language.
- **Reading**: `spelldal.Search` reads only the texts a search filters and ranks on (name, school, sphere) in
  one batched lookup by key (`TextsByID`, `text_id IN (…)` on the primary key, never a scan of
  `localized_texts`; `TestSearchAndDetails_MapFields` asserts which keys are read); the service ranks the query against the names in every language, cuts the
  page, then reads the page's full texts with one `Details` call.
- **REST**: every text field of a spell payload (`name`, `description`, `school`, `sphere`, the six stat
  texts) is a language map, `{"en": "…", "fr": "…"}`, holding only the languages the catalogue has (`{}` for
  a text in none); `GET /api/spells/names` is `[{id, name: {en, fr?}}]`. The client picks its language and
  falls back to English per field; there is no `lang` parameter. Descriptions are in the catalogue's Markdown
  subset (paragraphs, `- ` lists, pipe tables).
- **Checking a live database**: the Go check on a copy of a live SQLite file (`CRITICAL_UPGRADE_CHECK_DB`)
  is gone, since no Go test opens a database ([ADR 0025](adr/0025-postgresql-storage.md) step 6 and its
  Consequences). The seed check (`internal/seedcheck`) still validates the real seeds and artifact through
  the server's adapters, without a database.

## Production image

One container serves everything ([ADR 0024](adr/0024-deployment.md) step 1): the Go binary answers
`/api/*` and serves the built web app. There is no nginx.

- **Build**: the root `Dockerfile`, from the repo root, in stages: the web build (`npm ci`, `npm run build`),
  the Go build (`CGO_ENABLED=0`, on the `golang` tag of the Go version in `api/go.mod`: the build fails when
  they differ), and an Alpine runtime running as the non-root user `critical`, with a `HEALTHCHECK` on
  `/api/health`. `.dockerignore` keeps `.catalogue/`, `.dev/`, databases and any `catalogue.json` (at any
  depth), `.git`, `docs/`, `node_modules`, test results and `api/testdata` out of the default
  build context.
- **Build inputs**: the named build context `catalogue` (a directory holding `catalogue.json`) and the build
  argument `COMMIT`. Without the `catalogue` context the build stops with a message naming the flag. PR and CI
  builds and `just image` pass the fixture (`--build-context catalogue=api/testdata`); only the release
  build and the local `docker compose` pass the real catalogue, and it never enters the public repo, a PR
  build or a shared cache (step 7). `just image` also smoke-checks the result.
- **In the image**: `/app/critical-srv`, the web build in `/app/web`, the campaign `seeds/` in `/app/seeds`,
  the catalogue at `/app/catalogue.json`, and the PostgreSQL 17 client (`psql`, `pg_dump`, `pg_restore`;
  `postgresql17-client`, the cluster's major version), which the server never uses: the promotion takes its
  restore point, a `pg_dump`, from this image in a console machine (`scripts/restore-point.sh`, ADR 0024 step 9.2). The `CRITICAL_*` variables point at those paths; any of them
  can be overridden at `docker run`. The image holds no data, has no volume and names no database: the
  database is PostgreSQL at `CRITICAL_DSN`, given at run time (on fly the app's secret, pointing at its
  database in the shared cluster). `deploy/fly.*.toml` have no `[[mounts]]` and no DSN, and their `[env]`
  equals the image's `ENV` (`internal/flycheck`, ADR 0024 steps 2.4 and 5.1).
- **`CRITICAL_WEB_DIR`** (optional): the web build to serve. Set, every path outside `/api` is the file of
  that path, or `index.html` when there is none (SPA fallback, so a deep link reloads into the app); files
  under `assets/` (content-hashed by Vite) are sent with `Cache-Control: public, max-age=31536000,
  immutable`, everything else with `no-cache`; only GET and HEAD are served. A directory without
  `index.html` stops the server at boot. Unset (as in `just dev`, where Vite serves the app), nothing is
  served outside `/api`. Either way an unknown path that is not served is the JSON 404
  (`{"error": "not found: /api/nope", "code": "not_found"}`).
- **`/api/health`**: `{"status": "ok", "commit": "<sha>", "version": "<X.Y.Z>"}`. The image build sets them
  with `-ldflags -X …/internal/cmd/api.commit=$COMMIT -X …/internal/cmd/api.version=$VERSION` (build args
  `COMMIT` and `VERSION`); one not given, and any other build (`just dev`), reports `unknown`. The release
  build computes the version from the conventional commits since the last one (ADR 0024 step 3,
  docs/deploy.md "Release version"); CI's image check and `just image` build `0.0.0`.
- **Smoke check**: `scripts/image-smoke.sh <image> <commit> <version>` runs the image on the fixture seeds (the fixture
  catalogue knows only their spells) and a throwaway PostgreSQL container of its own (`postgres:17-alpine` on a
  private network, no volume; the image refuses the in-memory storage), so the boot migrates, seeds and imports
  into an empty database. It checks health with the commit and the version, the login gate without a session (`/` and
  a deep link redirect to `/login`, which is `index.html`; every script and stylesheet `/login` names
  is served; `/api/auth/options` offers the dev login; `/api/character` and the catalogue answer 401), then through the dev login `/` and a deep link returning `index.html`, a data read,
  the JSON 404 on `/api/nope`, a non-root user and the `HEALTHCHECK`. Then it runs
  `scripts/restore-point.sh` in a second container of the image started without its entrypoint (as the
  promotion's console machine is): a restore point of that database comes back whole (checksum, size, the
  dump and its table counts), creates no database, restores into a scratch database with the same counts, and
  is taken across a few seconds of lost connection; the server running beside it, a release older than the
  schema and the name of another database are refused. The CI `image` job builds
  the image with the fixture and runs it on every PR and on main; nothing is pushed.
- **Release build**: on green CI on `main`, `.github/workflows/release.yml` computes the release version (or
  none), fetches the real catalogue with the read-only deploy key and runs `scripts/release-image.sh` (also
  behind `just release-image`): the image is built with the version, labelled with the commit, the version
  and the rulebooks commit and pushed to the private fly registry tagged with the full commit sha and
  `v<version>` (docs/deploy.md, "Release build").
- **On fly.io**: configuration in `deploy/fly.*.toml`, operator commands in [deploy.md](deploy.md).
- **Locally**: `docker compose up --build` builds the image with the real `.catalogue/` (fetch it with
  `just catalogue` first) and serves the app on http://localhost:8888, on the compose PostgreSQL (database
  `critical`, the one `just dev` uses with the real catalogue).

## Rule tables

The AD&D 2e Player's Handbook lookup tables: Table 1 (Strength: hit and damage adjustments, including
18/01-18/00), Table 2 (Dexterity: reaction, missile attack and defensive adjustments), Table 32 (Bard
spells per day by level 1-20) and Table 25 (`bard_experience`: the XP that reaches each level 1-20,
Thief/Bard column). They come from the catalogue artifact's `rule_tables`
([ADR 0021](adr/0021-spell-catalogue.md) step 9); `seeds/` holds none (format 1, which took them from
`seeds/rule_tables.json`, is retired, step 9.6.3). The fixture `api/testdata/catalogue.json` carries the
same numbers.

They are not loaded into the database: at every start (whatever catalogue version the database recorded) the
`ruletablesadpt` parser reads them (a gap or overlap in a table is a startup error naming the table) into
`ruletablesvc.Tables`, which implements `ruletablesvc.Lookup`:
`StrengthMods(str, exceptional) (hit, dmg)`, `DexMods(dex) (missile, ac)` and
`BardSpellsPerDay(level) map[spellLevel]count`; `Tables` also has `BardXPThreshold(level)` (the
XP ledger's port). `exceptional` is the 18/xx percentile (1-99, 100 for
18/00). Values are returned as printed in the tables. The tests check
values against the PHB pages, not against the data; a test keeps the fixture's numbers equal to the real
artifact's (by lookup; skipped without `.catalogue/catalogue.json`).

A format-4 artifact also carries the **class and race tables** of [ADR 0030](adr/0030-classes-and-races.md)
step 23 (ADR 0021 step 10 lists the keys and rows): XP per class, hit dice per group, Constitution,
Intelligence and Wisdom, the Constitution save bonus, racial requirements and adjustments, THAC0, saving
throws, warrior and specialist attacks, rates of fire, wizard and priest spells (with the Wisdom a priest
needs for 6th and 7th level), the thieving and bard skill tables, backstab and turning undead. The same
parser reads them at every start into `ruletablesvc.ClassTables` (`Tables.Classes()`; false for a format-3
artifact), validated by `ruletablesvc.NewClassTables`: a missing table, a gap in a level band or in the
scores, XP not increasing or a code twice is a startup error naming the table; an artifact without them
(format 3) refuses the start (`ErrNoClassTables`). `Tables` looks them up for the derived values
(`derive.go`): `LevelFor(class, xp)`, `XPThreshold(class, level)`, `THAC0For(group, level)`,
`SavesFor(group, level)`, `ConSaveBonusFor(con)` and `WizardSpellsPerDay(level)`, plus `ClassGroup(class)`.
`charactersvc.Derive` combines them into the character's derived block (ADR 0030 steps 2 to 8, 22.1 and
24), from the XP total that `xpsvc.Totals` reads from the ledger; the skills (steps 17 and 18) are a later
item.

A format-5 artifact adds the **racial level limits** of DMG Table 7 (ADR 0021 step 11, `race_level_limits`):
one row per race and class, read into `ruletablesvc.LevelLimits` (`Tables.LevelLimits()`; false for a format-3
or format-4 artifact) and validated by `ruletablesvc.NewLevelLimits` (each race and class once, a level within
0 to 20). `LevelLimits.Limit(race, class)` gives the cell: `MaxLevel`, `Unlimited` (the book's U) or neither
(the book's dash: the race cannot belong to the class); a human has no row. Shown on the race card, never
enforced (ADR 0030 step 21).

## In-memory storage (e2e mode)

[ADR 0025](adr/0025-postgresql-storage.md) step 8: the e2e suite runs the API with no database, file
or server.

- **Config**: `CRITICAL_STORAGE=memory` selects it (unset: the database at `CRITICAL_DSN`, which is
  then never opened). Any other value is refused at boot. `web/e2e/fixtures.ts` starts every API this
  way; nothing else should.
- **Never in staging or production** (step 8.3), by two guards: the boot refuses the mode when
  `FLY_APP_NAME` is set (fly.io sets it on every machine, so a stray `fly secrets set` cannot enable
  it) or when `CRITICAL_WEB_DIR` is set (only the production image serves the built web app); and
  `internal/flycheck` fails when `deploy/fly.*.toml` or the `Dockerfile` names `CRITICAL_STORAGE`.
  A build tag was not used: e2e would then test another binary than the one deployed.
- **What it is**: `api/internal/storage/memory`, one `Store` implementing every domain storage port
  (the ports of `composition.Storage`, not the sqlc `Querier`s), wired by `composition.New` when
  `Config.InMemory` is set. It starts empty and is seeded like a new database: the seed files through
  `api/internal/storage/seedfile` (the same parsing and checks as the database seed), the catalogue by
  `ImportCatalogue` (one import per process: another version is refused). Data dies with the process.
- **Contracts kept** (step 8.2), tested in `memory_test.go`: `InTx` holds one write lock for the whole
  transaction (writers serialised; a read outside a transaction waits for the writer and never sees an
  uncommitted write) and puts every table back when `fn` fails or panics; a nested `InTx` joins the
  outer one; a write outside a transaction is atomic on its own. The action log only appends, and the
  popped entries, the redo target and the HP total come from the domain rules
  (`actionlog.StandingVoids`, `LatestPopped`, `HPTotal`), never re-implemented. Ids come from a
  sequence per table (AUTOINCREMENT-like, rolled back with the transaction); an id a table holds is
  refused on insert. Every value stored or returned is a deep copy.
- **A new storage port** gets its in-memory implementation in `storage/memory` and its line in
  `composition.memoryStorage` (`TestMemoryStorage_EveryPortOnTheOneStore` fails on a port left out).

## Seeding the character

`seeds/character.json` (the campaign Bard) only loads into an empty `characters` table. To pick up
an edited seed in the dev database: stop `just dev`, run `just dev-reset` (deletes the compose
databases `critical` and `critical_fixture`), and start `just dev` again. This discards all dev state (HP ledger, buffs, action log).

## Testing Conventions

- Go tests never open a database: ports and sqlc `Querier`s are mocked, and tests assert the
  operations called and their arguments, never SQL text ([ADR 0025](adr/0025-postgresql-storage.md)
  steps 6 and 7). The storage's own seed and import logic is tested over `fakeStore` ("DAL code
  generation" above), its transactions and migration run on recording fakes of the connection.
- The composition root is tested through `composition.Build` over a `composition.Storage` of mocks
  (`api/internal/composition/harness_test.go`); `composition.New` is the production path that opens
  PostgreSQL and builds the real adapters (or wires the in-memory storage, see above). Rule and flow tests belong in the service packages, on mocks of
  their DAL ports.
- Use table-driven tests
- Dice engine accepts a seeded `rand.Source` for deterministic tests
- Test the domain/engine layer directly — do not test HTTP handlers for game logic
- Test breakdown struct accuracy: every modifier must appear in the result
- Use `testify/assert` for assertions

## Panicking Helpers

Use `pkg/must` (`must.Do(formula.Parse("3+level"))`) instead of per-package
`MustX` functions. Panicking is allowed only in tests and in package-level
initialisation from literals; anything fed by users, files or the network returns
its error.
