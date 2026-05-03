# PRD: Critical — AD&D 2.5 Digital Character Engine

## Problem Statement

Playing AD&D 2.5 remotely via video call (Microsoft Teams) generates significant mental overhead: the player must simultaneously track HP events, manage active buff durations across rounds, compute multi-factor damage equations, verify THAC0 attack chains, and monitor remaining spell slots — all while staying engaged in the narrative. This math overhead slows the game and introduces errors that require back-tracking. No existing digital tool models AD&D 2.5 mechanics accurately enough to be trusted at the table.

## Solution

A personal, browser-based character sheet engine that acts as the authoritative rules engine for a single character (a Bard). The app automates all mechanical computation — attack resolution, damage calculation, buff expiry, spell slot tracking — and presents a persistent dashboard so that HP, active buffs, and combat state are always visible simultaneously. The player interacts with intent ("cast spell", "attack", "take damage") and the app returns fully-explained results, removing the need to perform any arithmetic manually.

## Guiding Principles

- **Vertical slice first.** A fully working Bard sheet before any other class is supported.
- **Frontend is presentation only.** All game logic lives in the Go backend. The React frontend sends intent and displays results.
- **Full transparency.** Every roll and calculation shows its complete breakdown. The DM can challenge any result.
- **Personal tool, not a product.** No authentication, no multi-tenancy, no SaaS infrastructure.

## User Stories

### HP & Damage Tracking

1. As a player, I want to log a damage event with a source label and delta, so that my HP updates immediately and I can trace what hit me.
2. As a player, I want to log a healing event with a source label and delta, so that my HP reflects recovery from spells or rest.
3. As a player, I want to see my current HP derived from a full append-only ledger of all events, so that I can scroll back through the session's damage and healing history.
4. As a player, I want to undo the last HP ledger entry (via a correction row, not deletion), so that I can recover from input errors without losing history.
5. As a player, I want to see my max HP alongside current HP at all times, so that I immediately know how close I am to death.

### Attack Resolution

6. As a player, I want to roll a melee attack by selecting a weapon, so that the app rolls a D20 and resolves whether I hit.
7. As a player, I want the attack result to show the full breakdown (roll, base THAC0, STR/DEX modifier, weapon modifier, effective THAC0, target AC needed, hit/miss margin), so that I can explain my roll to the DM.
8. As a player, I want to roll a ranged attack by selecting a ranged weapon, so that DEX modifier is applied instead of STR.
9. As a player, I want the app to simultaneously roll a D100 critical check alongside the D20 attack roll, so that I know immediately whether a critical hit occurred.
10. As a player, I want a critical hit confirmed when (D20 roll − D100 roll) still hits the target AC, so that the house crit rule is enforced automatically.
11. As a player, I want a confirmed critical hit to trigger a second D100 roll looked up against the critical multiplier table, so that the crit severity is determined correctly.

### Damage Resolution

12. As a player, I want to roll damage for a hit using the formula `((BaseDamage × SpellsBuff) + WeaponBuff) × Multipliers + StrengthBonus`, so that all active modifiers are correctly applied.
13. As a player, I want the damage result to show every factor in the equation with its source (weapon die, spell buff name, weapon magic bonus, multiplier sources, STR bonus), so that the full calculation is auditable.
14. As a player, I want the app to select the correct weapon die based on target size (S/M vs Large), so that Bastard Sword, Short Bow, and Dart damage is correct.
15. As a player, I want multiple active multipliers (Power Gem ×2, critical hit ×N) to stack multiplicatively, so that combined multipliers are computed correctly.

### Buff & Effect Management

16. As a player, I want to activate a buff (spell or item effect) with a label, duration in rounds, and effect type, so that the app tracks it automatically.
17. As a player, I want charge-based buffs (e.g. Stoneskin images) to decrement their counter when a triggering event occurs, so that remaining charges are always accurate.
18. As a player, I want duration-based buffs to decrement their round counter when I advance the round, so that I never need to track rounds manually.
19. As a player, I want an alert when a buff expires (reaches zero duration or charges), so that I immediately notice that a protection has ended.
20. As a player, I want to manually dismiss any active buff at any time regardless of remaining duration, so that I can handle edge cases (dispel, DM ruling).
21. As a player, I want the active buffs panel to always be visible in the left dashboard column, so that I never lose track of my current protections during combat.
22. As a player, I want the Power Gem buff to automatically set my weapon's magic bonus to +5 and add ×2 to the Multipliers factor, so that its effect is integrated into damage rolls transparently.
23. As a player, I want the Power Gem buff to only be activatable when my STR score is 18, so that the conditional requirement is enforced.

### Round Counter

24. As a player, I want a visible round counter in the right dashboard panel, so that I always know the current combat round.
25. As a player, I want a one-tap "Next Round" action that advances the round counter and decrements all duration-based buff timers simultaneously, so that round tracking is a single action.
26. As a player, I want to reset the round counter between combats, so that a new encounter starts cleanly.

### Spell Slot Management

27. As a player, I want to see my daily spell slots by level (prepared count and remaining count), so that I know what I can cast.
28. As a player, I want to mark a spell slot as used with one tap, so that the remaining count decrements immediately.
29. As a player, I want to see which specific spell is prepared in each slot, so that I can quickly identify my options during a turn.
30. As a player, I want to restore all spell slots (long rest reset), so that I can start a new session with a full complement.
31. As a player, I want the spell slot panel to be part of the switchable center work area, so that it is accessible without leaving the main dashboard.

### Spell Database

32. As a player, I want to look up a spell by name and see its casting time, range, area of effect, duration, and components, so that I can answer rules questions without leaving the app.
33. As a player, I want spell metadata sourced from a pre-seeded database (scraped from vodabois.fi/2eSpells), so that all Wizard and Priest spell data is available without manual entry.
34. As a player, I want spell lookup available in the center work area, so that I can check a spell without losing combat visibility.

### Character Stats & Saving Throws

35. As a player, I want to see my six core attributes (STR, DEX, CON, INT, WIS, CHA) displayed persistently, so that I can reference them without navigating away.
36. As a player, I want to roll a saving throw by category (Paralyze/Poison, Rod/Staff/Wand, Petrify/Polymorph, Breath Weapon, Spells) with full breakdown (roll, target number, active modifiers, pass/fail margin), so that saves are resolved with the same transparency as attacks.
37. As a player, I want active buff modifiers to saving throws (e.g. Protection from Evil bonus) to be automatically included in save resolution, so that I never forget a modifier.

---

## Implementation Decisions

### Repository & Deployment

- **Monorepo structure:** `api/` (Go backend), `web/` (React frontend), `seeds/` (JSON data files), `docker-compose.yml` at root.
- **Docker Compose:** Two services — `nginx` (serves compiled React static files, proxies `/api/*` to Go) and `api` (Go binary, minimal alpine image). SQLite on a named Docker volume mounted to the api container.
- **Hosting:** Proxmox homelab LXC container. Remote access via Tailscale (WireGuard mesh) — no port-forwarding or TLS certificate management required.
- **Auth:** None. Tailscale is the security boundary. App opens directly.

### Frontend

- **Stack:** React + Vite + TypeScript + Tailwind CSS + shadcn/ui.
- **Layout:** Persistent dashboard — left panel (vitals: HP bar, AC, active buffs), center panel (switchable work area: spells / combat / character stats / spell lookup), right panel (round counter, recent events).
- **Rule:** Zero game logic in the frontend. The React app sends intent to the API and renders structured responses. All computation happens in Go.
- **Target:** Desktop browser (1080p+). Mobile responsiveness is out of scope for Phase 1.

### Backend

- **Language:** Go. Strongly typed structs for Character, Spell, Item, BuffEffect, HpEvent, RollResult.
- **API:** REST. Future gRPC if inter-service complexity warrants it.
- **Dice engine:** Centralized in Go. All roll types (D20 attack, D100 crit check, D100 crit multiplier lookup, weapon damage, saving throw) are resolved server-side and returned with full breakdown structs.

### Data Model

- **Hybrid SQLite schema:** Core character scalars (STR, DEX, CON, INT, WIS, CHA, HP max, THAC0 base) stored as normalized columns. Complex variable-length state (active buffs array, spell slots object, prepared spells list) stored as JSON columns. SQLite's native JSON support used for querying when needed.
- **HP ledger:** Append-only table `(id, character_id, delta, source, timestamp)`. Current HP = `SELECT SUM(delta)`. Undo = insert a correction row (no deletes).
- **XP ledger:** Same append-only pattern as HP ledger `(id, character_id, delta, source, session_date)`.

### Seeded Data

- **Spell database:** One-time scrape of `vodabois.fi/2eSpells` committed as `seeds/spells.json`. Loaded into SQLite at startup if the spells table is empty. Covers Wizard and Priest spells with full metadata (casting time, range, AOE, duration, components). Adding/correcting a spell = edit JSON + restart.
- **Character data:** `seeds/character.json` — Bard's full sheet (attributes, HP max, THAC0, spell slots by level, prepared spells, equipment). Loaded at first startup. No character editor UI in Phase 1.
- **Critical multiplier table:** D100 result → multiplier value (×2, ×3, ×4, etc.). Stored as `seeds/crit_table.json`. **This data is not yet available and must be provided before the combat engine is complete.**

### Game Mechanics

- **Damage formula:** `((BaseDamage × SpellsBuff) + WeaponBuff) × Multipliers + StrengthBonus`
  - `BaseDamage`: weapon die roll, selected by target size (S/M vs Large)
  - `SpellsBuff`: multiplier from spells like Enlarge (default 1.0)
  - `WeaponBuff`: weapon magic bonus (+1, +2, etc.)
  - `Multipliers`: product of all active multiplier sources (Power Gem ×2, crit ×N). Stacking is **multiplicative**.
  - `StrengthBonus`: flat STR modifier
- **Critical hit flow:** Roll D20 + D100 simultaneously. If `(D20 − D100) ≥ required to hit` → critical confirmed. Roll second D100 → look up crit multiplier table → apply to Multipliers field.
- **Buff system:** Two types — charge-based (decrement on trigger event) and duration-based (decrement on "Next Round" action). Both can be manually dismissed. Expiry triggers an alert notification.
- **Round counter:** Manual. One-tap "Next Round" decrements all duration-based buff timers and increments the round counter.

### API Shape (representative endpoints)

- `GET /api/character` — full character state
- `POST /api/hp/event` — append HP ledger entry `{delta, source}`
- `POST /api/roll/attack` — resolve attack roll `{weapon_id, target_ac}`
- `POST /api/roll/damage` — resolve damage roll `{weapon_id, target_size, active_buffs}`
- `POST /api/round/advance` — advance round counter, return expired buffs
- `POST /api/buffs` — activate a buff
- `DELETE /api/buffs/:id` — dismiss a buff
- `POST /api/spells/:id/cast` — decrement spell slot
- `GET /api/spells` — spell database search/list

---

## Testing Decisions

**What makes a good test:** Tests should verify external behavior (inputs → outputs of a module's public interface), not implementation details (internal struct fields, helper functions). A test should break only when the observable contract changes, not when internal code is refactored.

**Modules to test:**

- **Dice engine / roll resolver** — pure functions with deterministic inputs (seeded RNG in tests). Test: correct formula application, modifier accumulation, breakdown struct accuracy, edge cases (natural 1, natural 20, crit threshold boundary).
- **Damage calculator** — test each factor in `((BaseDamage × SpellsBuff) + WeaponBuff) × Multipliers + StrengthBonus` in isolation and combined. Test multiplicative stacking with 2+ active multipliers.
- **HP ledger** — test append, SUM derivation, correction-row undo behavior.
- **Buff manager** — test charge decrement, duration decrement on round advance, expiry detection, manual dismiss.
- **Critical hit resolver** — test the two-roll sequence, the (D20 − D100) threshold check, and multiplier table lookup.

**Frontend:** No logic to test. Component rendering tests are out of scope.

---

## Phased Delivery

| Phase | Features |
|-------|----------|
| **1 — Combat Loop** | HP ledger, Attack roll (THAC0 full breakdown), Damage roll (full equation + crit), Active buffs + round counter, Spell slots (prep + cast) |
| **2 — Full Sheet** | Character stats display, Saving throws (full breakdown), Spell database metadata lookup |
| **3 — Progression** | Skill checks (Climb Wall, Hear Noise, Pickpocket, Ancient Language), XP ledger |
| **4 — Inventory** | Inventory management (equipped vs carried, finite resources, wands), Scroll tracking |

---

## Out of Scope

- **Authentication / user accounts:** Tailscale provides network-level access control.
- **Multi-character support:** Phase 2+ only. Phase 1 is a single hardcoded Bard.
- **DM view:** Future phase — a read-only view of player state for the Dungeon Master.
- **Priest / Psionic class mechanics:** Horizontal expansion after the Bard vertical slice is complete.
- **Mobile / tablet responsive layout:** Desktop browser is the primary target.
- **Character editor UI:** Character data is managed via `seeds/character.json`.
- **Offline resilience / PWA:** Tailscale keeps the homelab reachable; no offline mode needed.
- **Real-time clock for buff durations:** All duration tracking is round-based (manual).

---

## Outstanding Items

- **Critical multiplier table** (`seeds/crit_table.json`): The D100 → ×N lookup table must be provided by the user before the combat damage engine can be completed. This is a Phase 1 blocker.
- **Bard spell slot progression table:** The Bard's spell slots per level at current character level must be provided for `seeds/character.json`.
- **Bard character sheet:** Full attribute values, HP max, current THAC0, prepared spell list, and equipped items needed for initial seed.
