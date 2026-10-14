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
id, action_id, kind ('amend'|'void'), hp_delta, summary, payload JSON, created_at
-- (an amend's payload, when set, replaces the entry's: latest amend with a payload wins)

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
refused (409) while a later non-voided entry changed the same state key. buffsvc records every
buff change (activate, edit, absorb, dismiss) under the `buffs` key and Next Round under `buffs`
+ `round`, stamped with the round it ends; `buffSvc.Restorers()` is registered in composition.
combatsvc records every attack (kind `attack`, payload = the full roll breakdown, camelCase like the
`/api/roll/attack` response); a missile attack spends one ammo item through `itemsvc.Use` in the same
transaction and snapshots it under the per-item key `items:<id>` (`itemsvc.UseSnapshot`); the
Restorer registered for the `items` family (`itemSvc.Restorers()`) writes the count back on void.
castsvc records every cast and received spell (kind `cast`) as one entry with the `spell_slots`
snapshot (`prepsvc.SlotsSnapshot`, a cast only) and the `buffs` snapshot (`buffsvc.Add`, which
writes the buff inside the caller's transaction and replaces a same-name buff), so voiding gives the
slot back and removes the buff. Effects come from `seeds/spell_effects.json` at the caster level (the
Bard's level for a cast, the caster's for a received spell; by-group dice use the Bard's group,
rogue); low-confidence, utility and uncatalogued spells return their text only.
Buffs carry a free-text `Note` (in the active-buffs JSON and the `buffs` snapshot, so void restores it;
editable through `BuffEdit.Note`). Saved buff templates (`saved_buffs` table, migration 013, nothing seeded)
are listed/saved/deleted through `/api/buffs/saved` (`GET`, `POST` replaces a same-name template,
`DELETE /{id}`); each save/delete is a `buff_change` entry (`save_template` / `delete_template`) with a
`saved_buffs` snapshot of the whole list (the restorer writes it back with the same ids). Activating from
a template is the normal `POST /api/buffs` with the filled values; a delete never touches active buffs.
combatsvc records every saving throw (kind `save`, no snapshots: it changes no state; payload =
category, d20, buff modifiers (one per buff, `SaveBonusFrom`), target, adjustment + reason, total, passed, margin, `rolledPassed`).
`POST /api/roll/save/{id}/adjust` `{adjustment, reason?}` sets the adjustment of the save when it is
the **latest roll** (newest entry of `actionlog.RollKinds`: attack, save, cast, item use, the ones
that replace the result card; voided ones included; ±20 max): combatsvc recomputes verdict and margin from
the stored d20 and modifiers (the d20 never changes) and `actionlogsvc.AmendLatest` records an
audited amend carrying the new summary and payload (plus `previousPassed`/`previousAdjustment`).
Not a save → 400, voided or not the latest → 409. The log exposes `effectivePayload`.
A state key `family:id` is restored by the Restorer of its family, and a void is only blocked by
later entries with the same full key (a +1 arrow does not block voiding a +3 shot).

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
    Charges  int    // left to absorb (Stoneskin layers, Mirror Images); 0 = none; ends at 0
    Duration int    // rounds left, counted down by Next Round; 0 = not timed; expires at 0
    Effects  BuffEffects // min_str, spells_buff, weapon_buff, multiplier, save_bonus,
                         // str_set, str_bonus + str_max, ac, to_hit, damage
                         // (pointers: nil = no contribution)
}
// weapon_buff makes the weapon count as +N (Power Gem: "the weapon becomes +5"), to hit and
// damage; it replaces the weapon's own magic when higher and does not add to it. to_hit adds
// to attack rolls; damage adds to damage after the multipliers.
// A buff may have both counters (Mirror Image) or neither (until dismissed).
// Effective STR = best single STR effect (set, or base + bonus capped at str_max; never
// lower than base; Strength magic is not cumulative). min_str gates against effective STR.

type RollResult struct {
    D20Roll         int
    D100CritRoll    int
    IsCrit          bool
    CritMultiplier  float64 // decimal allowed (x1.5)
    CritPlaceholder bool       // x2 until the crit table lands (critical-qki.12)
    BaseThac0       int
    Modifiers       []Modifier // {kind, source, detail, value}: every to-hit modifier
    EffectiveThac0  int
    TargetACNeeded  int
    Hit             bool
    HitMargin       int
    // Damage breakdown (populated if hit)
    BaseDamage      int
    DamageDice      string       // dice rolled for the target size
    SpellsBuff      float64
    WeaponBuff      int          // weapon (or buff "+N") magic + ammo magic
    Multipliers     float64
    StrengthBonus   int
    BonusDamage     int          // buff damage
    DamageTerms     []DamageTerm // {term, kind, source, detail, value}: every damage contribution
    TotalDamage     int
    DamageBreakdown string       // human-readable formula trace
    // Attack context: weapon, target AC/size, ammo spent + remaining, log entry id
}
```

## Attack Modifiers

Hit when `d20 + modifiers ≥ THAC0 − target AC`. A natural 1 or 20 is plain arithmetic: no
auto-miss, no auto-hit. To-hit modifiers, each listed in `Modifiers`:

- Strength hit adjustment, PHB Table 1, from the **effective** STR (buffs) — melee and hurled
  weapons; never for bows (PHB "Ability Modifiers in Missile Combat": only with a strength bow).
- Dexterity missile adjustment, PHB Table 2 — every missile weapon (bow or hurled).
- Weapon magic, or a buff's "weapon becomes +N" when higher (Power Gem, only at effective STR 18).
- Ammo magic: DMG 2e "Bow +1": with magic arrows, total the bow's and the arrow's bonuses (to hit
  and damage).
- Buff `to_hit` effects, one per buff; the situational modifier entered at the table.

A weapon that fires ammunition (`ammo_kind`) needs an ammo item of that kind; the attack spends one.

## Damage Formula

```
Damage = floor(((BaseDamage × SpellsBuff) + WeaponBuff) × Multipliers) + StrengthBonus + BuffDamage
```

Never below 0. Every contribution is listed in `DamageTerms`.

- `BaseDamage`: weapon die roll, selected by target size (S/M vs Large)
- `SpellsBuff`: multiplier from spells like Enlarge (default 1.0)
- `WeaponBuff`: weapon magic damage (or the buff's "+N", Power Gem +5) plus ammo magic damage
- `Multipliers`: **multiplicative** product of all active multipliers (Power Gem ×2, crit ×N)
- `StrengthBonus`: PHB Table 1 damage adjustment for the effective STR (+2 at 18); melee and
  hurled weapons only, none for bows
- `BuffDamage`: buff `damage` effects, flat

## Critical Hit System

1. Roll D20 (attack) and D100 (crit check) simultaneously
2. If `(D20 - D100) ≥ required_to_hit` → critical hit confirmed
3. The second D100 is looked up in the house crit table (`crit_table_rows`, edited in the app).
   Until it is edited the multiplier is ×2, flagged `CritPlaceholder`. Table multipliers may be
   decimals (×1.5)
4. Multipliers stack multiplicatively with other active multipliers at full precision. The damage
   is floored **once**, on the final value after the strength and buff damage terms
   (`RollResult.UnroundedDamage` → `TotalDamage`); whole multipliers give the same totals as
   before. The breakdown string shows `… = 10.5 → 10` when the result is fractional

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
