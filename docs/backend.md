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
id, action_id, kind ('amend'|'void'), hp_delta, summary, payload JSON, note, created_at
-- (each amend field, when set, replaces the entry's: latest amend that set it wins; the log's
--  amend now sets only note, post-roll adjustments set summary + payload)

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

**HP cap (critical-qki.38).** User, 2026-10-13: *"a player cannot go above its max hp. period. max hp
grows when leveling up only"*. There are no temporary HP. Current HP ≤ `hp_max` means the ledger
total (SUM of effective `hp_delta`) is ≤ 0. The rule is enforced when an entry is written, never
by rewriting the ledger (`actionlog.CapHeal`):
- HP-1: `actionlogsvc.Record` caps every positive `hp_delta`, whatever the service, at the gap to
  max inside the recording transaction. The entry stores `min(heal, -total)`, and its payload gets
  `hpRequested` / `hpApplied`. The Combat Log shows "+12 requested, capped at max HP".
- HP-2: at full HP the heal is still logged, as 0, with the same breakdown.
- HP-5: `hpsvc.GetState` shows `min(hp_max, hp_max + total)`, a guard for rows logged before the cap.

Damage is unchanged. Today the only positive deltas come from `POST /api/hp/event` (manual heal:
cure spells and potions are entered there); casts, item uses and rests change no HP. No amend or
void can push HP over max either: amends change only the note, and a pop removes the newest entry,
which returns the total to what it was before that entry (≤ 0). ADR 0007 (HP ledger,
critical-qki.37) will take this rule over.

**The log is a stack (critical-qki.38).** User, 2026-10-14: *"no amending values. just the note..
can only pop elements"*, for every entry kind:
- `POST /api/log/{id}/amend {"note"}` sets the entry's note (any kind, trimmed, ≤ 500 characters,
  `""` clears it). It is an `amend` correction with only `note` set (migration 014); a body with any
  other field (`delta`, `source`, …) is a 400. A note changes no value, and the entry stays
  `active` (only a value amend makes it `amended`). The log exposes `note`.
- `POST /api/log/{id}/void` pops: only the newest non-voided entry, any kind, else 409
  `ErrNotLatest` naming the entry on top. Its snapshots are restored as before, so popping in order
  walks the state back one step at a time. `POST /api/hp/undo` pops the top when it is an HP change
  (409 when another kind is on top, a no-op on an empty log).
- Value amends written before qki.38 (`hp_delta`/`summary` on an `hp_change`) stay valid and still
  count. Each effective value (HP delta, summary, payload) comes from the latest amend that set it.
  The post-roll adjustments (save adjustment, attack target AC, below) are value amends on the
  latest roll and are unchanged.

**Recording actions.** A service that changes state records an `actionlog.NewEntry` through the
action log's `Record` inside the same `domain.TxRunner.InTx` as its state write, with
`Snapshots` of the state it changed. It registers an `actionlog.Restorer` for its state key on the
action log in the composition root, so voiding (popping) the entry restores the "before" state.
buffsvc records every
buff change (activate, edit, absorb, dismiss) under the `buffs` key and Next Round under `buffs`
+ `round`, stamped with the round it ends; `buffSvc.Restorers()` is registered in composition.
Time passes (`POST /api/round/pass-time {hours}`, `buffsvc.PassTime`) is one `time_passed` entry
under `buffs` + `round`: `buffsvc.RoundsForHours` converts hours to rounds (1 round = 1 minute,
rounded down; rejects ≤ 0, > 720 h and under one round), timed buffs lose that many rounds and
expire at ≤ 0, charge-only and untimed buffs are untouched, the counter goes to 0. Daily items are
not refilled (long rest only).
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
`POST /api/roll/attack/{id}/adjust` `{targetAc}` corrects the target AC of the attack when it is the
latest roll (same rules, AC within ±20). Every attack rolls its damage dice, even a miss, and its
payload keeps them with every damage term (`rolled`), the multiplier d100 (`d100MultiplierRoll`)
and the crit-table row it fell in (`critRow`); combatsvc `settle` recomputes hit, margin, crit and damage from those and the new AC
(no die rolled, no ammo spent, a later crit-table edit changes nothing) and the amend carries
`rolledTargetAc`/`rolledHit`/`rolledCrit` (the AC and outcome as rolled, kept across corrections, so the line
reads rolled → now; dropped when the AC is back to the rolled one, which reads as rolled). Voiding still restores the ammo (snapshots are the
original entry's). An attack logged without `rolled` (before this existed) → 400, void and reroll.
A state key `family:id` (for example `items:arrows_plus3`) is restored by the Restorer of its family.

**Items: one model, three count behaviours (critical-qki.13).** User, 2026-10-13: *"these are all
consumables... it's just their type"*. An item is a name, a free-text `type` label (arrow, flask,
wand, gem…), a count, a note and optional mechanics: effect dice and save, `magic_bonus` (the
"Magical bonus" +N), `ammo_kind`. Only `behaviour` (migration 015, replacing `kind`/`per_day`)
changes how the count moves:

| Behaviour | Example | − (use) | + (restock) | At 0 | Long rest |
|---|---|---|---|---|---|
| `stock` | arrows, fire flasks | yes | yes | stays listed | no |
| `charges` | wand of lightning | yes | no (409 `ErrNotRestockable`) | removed | no |
| `daily` | Power Gem dispel 1/day | yes | no (409) | stays listed | refilled to `max_count` |

- `itemsvc.Use` spends one of any item (the attack's arrow, the inventory's −); the use that takes
  a `charges` item to 0 deletes it in the same transaction and entry. `UseItem` (`POST
  /api/items/{id}/use`) adds the effect roll and the `item_use` entry (payload `removed`).
- `itemsvc.Restock` (`POST /api/items/{id}/restock {count}`, 1-999) adds to a `stock` item: one
  `item_restock` entry (not a roll kind).
- Every count change is one entry with the `items:<id>` snapshot. A use that removed the item
  carries the whole item in its "before" (`snapCount.item`), so popping it re-inserts the item, in
  its list position, with 1 charge. A long rest refills `daily` items only; time fast-forward none.
- Editor (critical-qki.31): `POST /api/items`, `PATCH /api/items/{id}` (only the fields sent),
  `DELETE /api/items/{id}`; each is one entry (`item_add` / `item_edit` / `item_remove`, not roll
  kinds) with the `items:<id>` snapshot and answers `{item, logEntryId}`. The log is pop-only: popping
  the newest entry deletes a created item (snapshot `absent`), overwrites an edited one with every
  "before" field, or re-inserts a removed one with its id, position and count (`replace`).
  Validation (`Item.Validate`) fails with a `domain.FieldError`, rendered `400 {error, field}` so the
  UI shows the message under that field; effect dice are `NdM` or `NdM+K`. A new item's id is the slug
  of its name (suffix `_2`, `_3` when taken) and goes last in the list.
- Ammo is any item whose `ammo_kind` matches the weapon's; the attack picker offers those with
  count > 0.

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
                         // str_bonus + str_max, ac, to_hit, damage
                         // (pointers: nil = no contribution)
}
// weapon_buff makes the weapon count as +N (Power Gem: "the weapon becomes +5"), to hit and
// damage; it replaces the weapon's own magic when higher and does not add to it. to_hit adds
// to attack rolls; damage adds to damage after the multipliers.
// A buff may have both counters (Mirror Image) or neither (until dismissed).
// Effective STR: see "Effective STR" under Attack Modifiers. min_str gates the buff's
// other effects: they apply while effective STR >= min_str.

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
- Weapon magic, or a buff's "weapon becomes +N" when higher (Power Gem, at effective STR 18 or
  more: rule 3 below).
- Ammo magic: DMG 2e "Bow +1": with magic arrows, total the bow's and the arrow's bonuses (to hit
  and damage).
- Buff `to_hit` effects, one per buff; the situational modifier entered at the table.

### Effective STR

The STR score used for the Strength adjustments and for buff gates (critical-qki.33). STR
effects themselves are never gated, and no rule lowers the score below the sheet's.

User rulings, 2026-10-13: "strength level 2 says WItH OTHER. it's find with ITSELF"; "improved
replaces. level 2 can stack with itself. this is why kelsen often equips 2 str lvl 2 to be sure
to get to 18"; "simple: all str spells stack up to 18. unless it's improved strength that can go
to 23."

1. **Strength bonuses stack, up to 18.** With no Improved Strength active, effective STR = base +
   the **sum** of every active `str_bonus` (Strength cast or received, or entered by hand, with
   or without a `str_max`), total capped at 18 (PHB: Strength "cannot confer a Strength of 19 or
   more"). A buff's own `str_max` of 18 or less is not used. Casting or receiving Strength again
   adds a second buff instead of renewing the first.
2. **Improved Strength replaces them.** A `str_bonus` whose `str_max` is above 18 is Improved
   Strength (seeded 1d6+4, cap 23, for a bard). While one is active, every other Strength bonus
   contributes nothing (its badge shows "replaced"), and the best single Improved wins: base +
   bonus capped at its `str_max`. Casting it again renews it.
3. **Gate.** A buff's other effects apply while effective STR ≥ its `min_str`. The Power Gem
   forms set `min_str` 18: +5 and ×2 at effective STR 18 **or more** (19-23 with Improved
   Strength included). User, 2026-10-13: "not exactly 18 >= 18".

Exceptional Strength (18/xx) is not modelled: PHB, warriors only; Kelsen is a Bard. There is no
"set STR to X" effect (user, 2026-10-13: "remove that buff type"); a `str_set` key in a stored
buff is ignored when read, and in a request body it is ignored like any unknown key.

A weapon that fires ammunition (`ammo_kind`) needs an ammo item of that kind; the attack spends one.
Dice order: d20, crit-check d100, multiplier d100, then the damage dice, all rolled on **every** attack (a miss shows no
damage, but the target AC may be corrected after the roll).

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
