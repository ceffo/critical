# Critical — Backend Context (Go Agents)

## Stack

- **Language**: Go (latest stable)
- **HTTP**: standard `net/http` or `chi` router
- **Database**: SQLite via `modernc.org/sqlite` (pure Go, no CGO)
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
      combat.go         ← THAC0 resolution, crit check, damage formula
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
id, action_id, kind ('amend'|'void'), hp_delta, summary, created_at

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

-- round_state: current round counter
id, character_id, round_number INTEGER
```

**Never derive current HP in the application layer.** Current HP = `hp_max` + the SQL SUM of
effective `hp_delta` over `action_log` (latest amend wins, voided entries count 0), see
`actionlogdal.HPTotal`.

**Recording actions.** A service that changes state records an `actionlog.NewEntry` through the
action log's `Record` inside the same `domain.TxRunner.InTx` as its state write, with
`Snapshots` of the state it changed. It registers an `actionlog.Restorer` for its state key on the
action log in the composition root, so voiding the entry restores the "before" state. A void is
refused (409) while a later non-voided entry changed the same state key.

## Key Go Structs

```go
type HpEvent struct {
    ID          int64
    CharacterID int64
    Delta       int    // negative for damage, positive for healing
    Source      string // "goblin arrow", "cure light wounds", etc.
    CreatedAt   time.Time
}

type BuffEffect struct {
    ID       string // uuid
    Name     string
    Type     string // "charge" or "duration"
    Charges  int    // for charge-based buffs (0 = inactive)
    Duration int    // rounds remaining for duration-based buffs
    Effects  map[string]interface{} // modifier map: {"spells_buff": 1.5, "weapon_buff": 5}
}

type RollResult struct {
    D20Roll       int
    D100CritRoll  int
    IsCrit        bool
    CritMultiplier int
    BaseThac0     int
    Modifiers     []Modifier // {source, value} pairs
    EffectiveThac0 int
    TargetACNeeded int
    Hit           bool
    HitMargin     int
    // Damage breakdown (populated if hit)
    BaseDamage    int
    SpellsBuff    float64
    WeaponBuff    int
    Multipliers   float64
    StrengthBonus int
    TotalDamage   int
    DamageBreakdown string // human-readable formula trace
}
```

## Damage Formula

```
Damage = ((BaseDamage × SpellsBuff) + WeaponBuff) × Multipliers + StrengthBonus
```

- `BaseDamage`: weapon die roll, selected by target size (S/M vs Large)
- `SpellsBuff`: multiplier from spells like Enlarge (default 1.0)
- `WeaponBuff`: weapon magic bonus (Power Gem makes weapon +5 while active)
- `Multipliers`: **multiplicative** product of all active multipliers (Power Gem ×2, crit ×N)
- `StrengthBonus`: flat STR modifier (+2 for STR 18)

## Critical Hit System

1. Roll D20 (attack) and D100 (crit check) simultaneously
2. If `(D20 - D100) ≥ required_to_hit` → critical hit confirmed
3. Roll second D100 → look up `seeds/crit_table.json` → get crit multiplier (×2, ×3, ×4...)
4. Multipliers stack multiplicatively with other active multipliers

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
18/00). The defensive adjustment is added to AC as printed (-3 improves AC by 3). The tests check
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
