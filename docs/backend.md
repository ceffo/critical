# Critical — Backend Context (Go Agents)

## Stack

- **Language**: Go (latest stable)
- **HTTP**: standard `net/http` or `chi` router
- **Database**: PostgreSQL ([ADR 0025](adr/0025-postgresql-storage.md)) through `database/sql` and the `github.com/lib/pq` driver (sqlc's `database/sql` output binds arrays with `pq.Array`). `storage/postgres.Open` opens the pool from `CRITICAL_DSN` (a PostgreSQL URL or connection string); in development it is the docker compose service `postgres` ("Development database" below), on fly the cluster of [ADR 0024](adr/0024-deployment.md) step 2. The server module has no SQLite dependency (`just check-no-sqlite`, part of `just validate`).
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
-- characters: one row for the Bard
id, name, str, dex, con, int, wis, cha,
hp_max, thac0_base, level, xp_total

-- action_log: THE append-only log of table actions (HP ledger + combat log)
id, character_id, round, kind TEXT, summary TEXT, hp_delta INTEGER, payload JSON, created_at,
transaction_id
-- (transaction_id: the id of the first entry of the request that recorded it, ADR 0018 step 1;
--  undo pops a transaction whole. Rows older than the column are each their own transaction)
-- action_snapshots: before/after state an entry changed, one row per state key
id, action_id, state_key, before_state JSON, after_state JSON
-- action_corrections: audited amend/void events pointing at an entry (never edit an entry)
id, action_id, kind ('amend'|'void'), hp_delta, summary, payload JSON, note, step_id, created_at
-- (step_id: the 'correction' entry a post-roll correction belongs to, ADR 0010 step 6; popping
--  that entry puts the amend out of effect. NULL on older amends, which stay in effect)
-- (each amend field, when set, replaces the entry's: latest amend that set it wins; the log's
--  amend now sets only note, post-roll adjustments set the payload)
-- summary holds only text the player typed (an HP change's source); every other entry is a
-- structured payload the UI words in the player's language (older rows keep an English summary)

-- hp_events: legacy HP ledger, carried into action_log by migration 007; no longer written

-- xp_entries: the XP ledger, outside the action log (edited and deleted in place, ADR 0015);
-- the total is characters.xp_total plus every entry (base + 10% when bonus)
id, character_id, entry_date TEXT, session TEXT, reason TEXT, base INTEGER, bonus INTEGER (0/1)

-- proficiencies, bard_skills: the skill catalog (ADR 0014), reconciled by id with seeds/skills.json
-- at startup; a check is an action_log entry (skill_check), nothing is stored here
id TEXT, position, ability TEXT ('str'..'cha'), modifier INTEGER
id TEXT, position, percent INTEGER (0-100)

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

## Development database

`just dev` (through `just dev-db`) starts the docker compose service `postgres` (`docker compose up
--detach --wait postgres`; it keeps running after `just dev` stops) and runs the API on it:

- database `critical` with the real catalogue (`.catalogue/catalogue.json`) and the campaign seeds;
  `critical_fixture` with the fixture catalogue and seeds of `api/testdata` when there is no real
  catalogue (the init script `deploy/postgres-init/` creates it with the volume);
- DSN `postgres://critical:critical@localhost:5432/<database>?sslmode=disable`, also the default
  `CRITICAL_DSN`; `CRITICAL_PG_PORT` moves the host port (the service listens on 127.0.0.1 only);
- the data lives in the compose volume `critical-pg` of the checkout's compose project (a worktree
  has its own); `just dev-reset` deletes it (`docker compose down --volumes`), and the next start
  migrates, seeds and imports the catalogue again. Files in `.dev/` are never touched: the copy of a
  SQLite `.dev/critical.db` into PostgreSQL is the copy tool's ([ADR 0025](adr/0025-postgresql-storage.md) step 9).

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

### Layout

| Path | Contents |
|------|----------|
| `api/internal/adapters/*/queries/*.sql` | Hand-written SQL query sources |
| `api/sqlc.yaml` | sqlc config, one entry per adapter (section above) |
| `api/internal/adapters/*/queries/{db,models,querier}.go`, `*.sql.go` | sqlc output of an adapter |
| `api/internal/adapters/*/queries/mocks/` | mockery mock of an adapter's `Querier` |

<!-- br-agent-instructions-v1 -->

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

The seed files are read and checked by `api/internal/storage/seedfile`, shared by every storage engine
(the database and the in-memory storage). At startup, for each seed file (character, weapons, items:
only into an empty table; skills: reconciled by id):
```go
// Check if table is empty, if so load from seeds/
var count int
db.QueryRow("SELECT COUNT(*) FROM weapons").Scan(&count)
if count == 0 {
    loadSeedsFromFile("seeds/weapons.json", db)
}
```

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
- **Formats**: the API reads format 3 only (spells and the PHB rule tables, ADR 0021 step 9); any other,
  the retired format 1 included, is refused, naming it and `just catalogue`. See "Rule tables" below.
- **Without access**: `api/testdata/catalogue.json` is a small fixture, format 3, with invented text
  (every spell the `api/testdata/seeds` spellbook and spell effects use, some with French, one list, one
  table) and the PHB's real rule-table numbers (ADR 0021 step 9.7). Go tests and the e2e suite always use it; `just dev` uses it, with `api/testdata/seeds` and its
  own database `critical_fixture`, when `.catalogue/catalogue.json` is absent (the campaign seeds refer to
  spells only the real artifact holds). The database `critical` is only ever opened with the real catalogue.
- **Seed checks**: the spellbook and every `spell_effects.json` id are checked against the catalogue at
  startup; an id it does not hold refuses the start.
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
  depth), `.git`, `.beads`, `docs/`, `node_modules`, test results and `api/testdata` out of the default
  build context.
- **Build inputs**: the named build context `catalogue` (a directory holding `catalogue.json`) and the build
  argument `COMMIT`. Without the `catalogue` context the build stops with a message naming the flag. PR and CI
  builds and `just image` pass the fixture (`--build-context catalogue=api/testdata`); only the release
  build and the local `docker compose` pass the real catalogue, and it never enters the public repo, a PR
  build or a shared cache (step 7). `just image` also smoke-checks the result.
- **In the image**: `/app/critical-srv`, the web build in `/app/web`, the campaign `seeds/` in `/app/seeds`,
  and the catalogue at `/app/catalogue.json`. The `CRITICAL_*` variables point at those paths; any of them
  can be overridden at `docker run`. The image holds no data, has no volume and names no database: the
  database is PostgreSQL at `CRITICAL_DSN`, given at run time (on fly a secret). `deploy/fly.*.toml` still
  set the SQLite DSN and the `/data` volume until the fly PostgreSQL configuration replaces them
  (`internal/flycheck` compares every other `[env]` value with the image's).
- **`CRITICAL_WEB_DIR`** (optional): the web build to serve. Set, every path outside `/api` is the file of
  that path, or `index.html` when there is none (SPA fallback, so a deep link reloads into the app); files
  under `assets/` (content-hashed by Vite) are sent with `Cache-Control: public, max-age=31536000,
  immutable`, everything else with `no-cache`; only GET and HEAD are served. A directory without
  `index.html` stops the server at boot. Unset (as in `just dev`, where Vite serves the app), nothing is
  served outside `/api`. Either way an unknown path that is not served is the JSON 404
  (`{"error": "not found: /api/nope", "code": "not_found"}`).
- **`/api/health`**: `{"status": "ok", "commit": "<sha>"}`. The image build sets the commit with
  `-ldflags -X …/internal/cmd/api.commit=$COMMIT`; any other build reports `unknown`. The release name is not
  in the image (ADR 0024, consequences): the GitHub Release maps a tag to its commit.
- **Smoke check**: `scripts/image-smoke.sh <image> <commit>` runs the image on the fixture seeds (the fixture
  catalogue knows only their spells) and a throwaway PostgreSQL container of its own (`postgres:17-alpine` on a
  private network, no volume; the image refuses the in-memory storage), so the boot migrates, seeds and imports
  into an empty database. It checks health with the commit, `/` and a deep link returning
  `index.html`, the JSON 404 on `/api/nope`, a non-root user and the `HEALTHCHECK`. The CI `image` job builds
  the image with the fixture and runs it on every PR and on main; nothing is pushed.
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
