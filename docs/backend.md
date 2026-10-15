# Critical — Backend Context (Go Agents)

## Stack

- **Language**: Go (latest stable)
- **HTTP**: standard `net/http` or `chi` router
- **Database**: SQLite via `modernc.org/sqlite` (pure Go, no CGO). `sqlite.Open` sets `busy_timeout=5000` on every pooled connection and `journal_mode=WAL` for file-backed DBs (WAL adds `-wal`/`-shm` files beside the DB).
- **Migrations**: simple sequential SQL files, applied at startup
- **Seed loader**: reads `seeds/*.json` at startup if corresponding table is empty

## Project Structure (api/)

```
api/
  main.go               ← HTTP server, router, startup
  internal/
    db/
      db.go             ← SQLite open, migrate, seed
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

## SQLite Data Model

**Hybrid model:** core scalars as columns, complex variable-length state as JSON columns.

### Normalized tables (core scalars)
```sql
-- characters: one row for the Bard
id, name, str, dex, con, int, wis, cha,
hp_max, thac0_base, level, xp_total

-- action_log: THE append-only log of table actions (HP ledger + combat log)
id, character_id, round, kind TEXT, summary TEXT, hp_delta INTEGER, payload JSON, created_at
-- action_snapshots: before/after state an entry changed, one row per state key
id, action_id, state_key, before_state JSON, after_state JSON
-- action_corrections: audited amend/void events pointing at an entry (never edit an entry)
id, action_id, kind ('amend'|'void'), hp_delta, summary, payload JSON, note, created_at
-- (each amend field, when set, replaces the entry's: latest amend that set it wins; the log's
--  amend now sets only note, post-roll adjustments set the payload)
-- summary holds only text the player typed (an HP change's source); every other entry is a
-- structured payload the UI words in the player's language (older rows keep an English summary)

-- hp_events: legacy HP ledger, carried into action_log by migration 007; no longer written

-- xp_events: append-only ledger (same pattern as hp_events)
id, character_id, delta INTEGER, source TEXT, session_date
```

### JSON column tables (complex state)
```sql
-- active_buffs: JSON array of BuffEffect objects
id, character_id, buffs JSON

-- spell_slots: JSON object of slots by level
id, character_id, slots JSON

-- round_state: current round counter and its initiative (d10, the log entry that rolled it; NULL: none)
id, character_id, round_number INTEGER
```

**Recording actions.** A service that changes state records an `actionlog.NewEntry` through the
action log's `Record` inside the same `domain.TxRunner.InTx` as its state write, with
`Snapshots` of the state it changed (a `before` and an `after` JSON per state key). It registers an
`actionlog.Restorer` for its state key on the action log in the composition root, so voiding
(popping) the entry restores the "before" state; a service exposes its restorers (for example
`buffSvc.Restorers()`) for the composition root to register. A state key `family:id` (for example
`items:arrows_plus3`) is restored by the Restorer of its family. A snapshot of state that other
services also write (the slot list, the buffs) is produced by one helper of the owning service
(`prepsvc.SlotsSnapshot`, `buffsvc.Add`) so every writer encodes it the same way.

What each service records, and the rules it follows, are in the ADRs ([index](adr/README.md)).

## DAL code generation (bob)

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

At startup, for each seed file:
```go
// Check if table is empty, if so load from seeds/
var count int
db.QueryRow("SELECT COUNT(*) FROM spells").Scan(&count)
if count == 0 {
    loadSeedsFromFile("seeds/spells.json", db)
}
```

## Rule-table seeds

`seeds/rule_tables.json` holds the AD&D 2e Player's Handbook lookup tables: Table 1 (Strength: hit and
damage adjustments, including 18/01-18/00), Table 2 (Dexterity: reaction, missile attack and defensive
adjustments) and Table 32 (Bard spells per day by level 1-20). It is not loaded into SQLite: the
`ruletablesadpt` adapter parses it at startup (a gap or overlap in a table is a startup error) into
`ruletablesvc.Tables`, which implements `ruletablesvc.Lookup`:
`StrengthMods(str, exceptional) (hit, dmg)`, `DexMods(dex) (missile, ac)` and
`BardSpellsPerDay(level) map[spellLevel]count`. `exceptional` is the 18/xx percentile (1-99, 100 for
18/00). Values are returned as printed in the tables. The tests check
values against the PHB pages, not against the seed.

## Seeding the character

`seeds/character.json` (the campaign Bard) only loads into an empty `characters` table. To pick up
an edited seed in the dev database: stop `just dev`, run `just dev-reset` (deletes `.dev/critical.db`),
and start `just dev` again. This discards all dev state (HP ledger, buffs, action log).

## Testing Conventions

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
