# Critical — Project Context (All Agents)

## What This Project Is

Critical is a hosted AD&D 2.5 character sheet engine for a gaming group. Invited players log in with Google and each keeps several characters of five classes (bard, fighter, mage, cleric, thief). It automates mechanical computation (attack resolution, damage calculation, buff expiry, spell slot tracking) and presents a persistent dashboard. The player sends intent; the app returns fully-explained results.

**It is a multi-user game tool for one table, not a SaaS product.** Access is by invitation only, and nothing but the login page is visible without a session ([ADR 0029](adr/0029-users-login-and-access.md)). A character has one owner and only the owner sees it. The class rules arrive with the classes ADR; the Bard is the first class implemented. A DM role and a shared table are a later epic.

## Monorepo Layout

```
critical/
  api/              ← Go backend (ALL game logic lives here)
  web/              ← React frontend (presentation only)
  seeds/            ← JSON seed data (spell_effects.json, spellbook.json, starting_spellbook.json, character.json, weapons.json, skills.json); spells and the PHB rule tables come from the catalogue artifact (ADR 0021, `just catalogue`)
  docs/             ← Agent context files (this folder)
  Dockerfile        ← production image: the API serving the web app (docs/backend.md, "Production image")
  docker-compose.yml
```

## Core Architectural Rule

**Zero game logic in the React frontend.** The frontend sends intent to the API and renders structured responses. All dice rolling, rule resolution, formula calculation, and state mutation happens exclusively in Go. If you find yourself writing a calculation in TypeScript, stop — move it to Go.

## REST API Conventions

- Base path: `/api/`
- JSON request/response bodies
- Errors: `{"error": "message", "code": "stable_code"}` (plus `"field"` for a field error) with appropriate HTTP status code. The code comes from the domain error through the one mapping table in `internal/render`; the web words known codes in en + fr (`apiError.*` i18n keys) and shows `error` as the fallback.
- A state-changing answer names the state keys it changed in the `X-State-Keys` header (`docs/backend.md`, "State keys on write answers"); the web reads again only what they name.
- All roll results return a complete breakdown struct (never just hit/miss or a bare number)
- Endpoints follow resource/action naming: `POST /api/hp/event`, `POST /api/roll/attack`, `POST /api/round/advance`

## Git & Commit Rules

- Commit after each meaningful unit of work (passing tests, working endpoint, etc.)
- Commit message format: `<type>: <short description>` (e.g. `feat: HP ledger append endpoint`)
- End the subject with the Linear issue, e.g. `feat: HP ledger append endpoint (CRI-42)`
- Never commit broken tests or failing builds

## Key Domain Vocabulary

The vocabulary lives in [`GLOSSARY.md`](../GLOSSARY.md) (English term, French UI word, the ADR
that holds the rules; [ADR 0036](adr/0036-vocabulary.md)). Read it before naming anything; how the terms
relate is in [`docs/domain-model.md`](domain-model.md). Two
process words that are not domain:

- **Vertical Slice**: one Linear issue covers schema + Go engine + REST endpoint + React panel for one feature
- **Kelsen**: the seed character (the table's bard), see the glossary

## Phase 1 Scope (MVP for the 2026-10-17 game)

The goal is a tool the player can use at the table on Saturday 2026-10-17, run locally on a laptop
(`just dev`, desktop layout, no hosting). Hard cut at Saturday. The epics are `critical-qki` and the
dashboard v3 redesign `critical-kiv`; items labelled `mvp-saturday` are the must-haves.

**In for Saturday:**

1. Real character data: the campaign Google Sheet is the source of truth (level 15 Bard, HP 85,
   THAC0 13, STR 12), plus the PHB Strength / Dexterity / Bard spell tables as seed data
2. HP Ledger and Combat Roll Resolution (attack + crit + damage) with real weapons, ammo by arrow
   type, and the full AD&D 2.5 modifiers (STR, DEX, buffs); no special natural 1 / natural 20
3. Active Buffs + Round Counter, including charge buffs and effective STR (Strength bonuses stack up
   to 18, Improved Strength replaces them up to 23: "all str spells stack up to 18. unless it's
   improved strength that can go to 23"; Power Gem effects apply at effective STR 18 or more,
   usually via Strength spells cast by others; user: "not exactly 18 >= 18")
4. Spells with effects (Wizard-list effects extracted into a seed) and spells received from the party
5. Daily spell preparation from the spellbook within the PHB Bard limits
6. One append-only action log (also the combat log) with before/after snapshots, used as a stack.
   User, 2026-10-14: "no amending values. just the note.. can only pop elements", for every entry
   kind: an amend only sets the entry's note, and only the newest non-voided entry can be voided
   (popped, restoring what it changed); both are audited corrections
7. Weapons and items in DB tables seeded from JSON; charged items and consumables
8. i18n: en/fr string keys for the UI; spell and item texts are localized-text rows served as language maps (ADR 0023)
9. A round-centric dashboard; Playwright e2e harness behind `just e2e`
10. Dashboard v3 (ADR 0009): responsive desktop layout from 1280 to 2560 CSS px (rail, workspaces,
    Fil and Moi columns), Nocturne themes with six ambiances, light / dark / system, and the per-device
    settings popover
11. Skill checks: bard skills and non-weapon proficiencies from the campaign sheet, rolled on the
    server
12. XP ledger: total, progress to the next level and the ledger (no level-up)

**After Saturday (not to be built now):** Gaia fire/lightning protections, mobile and other device
layouts (below 1024 CSS px), level-up mechanics (the multi-user model, with several characters per user and five classes, is now scoped: ADR 0029).

**Hosting and deployment** (`critical-8g1`) is no longer deferred (user, 2026-10-16): fly.io, staging
from main and production from version tags, designed in [ADR 0024](adr/0024-deployment.md). The apps
go public behind the Google login (`critical-hit`, `critical-hit-staging`) once ADR 0029's gate is live.

Do not implement anything from the "after" list even if it seems related.
