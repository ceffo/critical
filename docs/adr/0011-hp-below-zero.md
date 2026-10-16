# ADR 0011: HP below zero

**Status**: accepted, 2026-10-15 (critical-kiv.10)

## Context

[ADR 0007](0007-hp-ledger.md) keeps HP as an append-only ledger capped at max HP, and its step 3
logs damage as typed, so HP could sink without limit below 0. At the table a character at -10 is
dead and further damage means nothing, and the player needs to see at a glance whether the
character is still standing, down or dead. Triage (2026-10-15), confirmed by the user: HP stops at
-10, and the server returns the condition: ok, badly wounded (at or below 25% of max), unconscious
(at or below 0), dead (at or below -10). The heal cap (ADR 0007 steps 4 to 6) is unchanged.

This ADR is the bottom end of the ledger rules; the ledger itself (entries, SUM, amend, pop, undo)
is ADR 0007's. It may later be folded into it.

## Decision

Tests live in `api/internal/domain/` unless named otherwise.

1. **HP never below -10 (the floor).** When a negative delta is recorded (whatever the service
   that records it), it is clamped inside the recording transaction so that current HP
   (`hp_max + total + delta`, `total` being the SUM before the entry) stops at -10: the entry
   stores `max(delta, min(0, -10 - (hp_max + total)))`. Damage down to -10 is logged as typed
   (ADR 0007 step 3 still holds above the floor); damage past it is logged clamped; at or below
   the floor it is logged as 0, the event kept. A clamped entry carries `hpRequested` and
   `hpApplied` in its payload, like a capped heal, and the Combat Log shows both. It is the mirror
   of the heal cap (ADR 0007 step 4): applied when writing, the ledger never rewritten, so
   `SUM(delta)` stays the truth, a later heal counts from -10 and popping a clamped entry (the global undo of
   [ADR 0010](0010-action-stack-and-undo.md), from the journal) puts the total back
   exactly where it was. As in ADR 0007 step 6, the HP shown is never below -10
   either (`max(-10, min(hp_max, hp_max + total))`), a guard for rows logged before the floor.
   `GET /api/hp` returns the floor as `floor: -10`.
   Tests: `TestFloorDamage` (actionlog), `TestRecord_FloorsDamage` (actionlogsvc: "damage past
   the floor is clamped", "damage at the floor logs 0", boundaries, errors),
   `TestGetState_NeverBelowFloor` (hpsvc).
2. **Condition, evaluated on the server.** From the HP shown (step 1 and ADR 0007 step 6), first
   match wins: `dead` at or below -10; `unconscious` at or below 0; `badlyWounded` at or below a
   quarter of max HP, compared as `4 × HP ≤ hp_max` so no rounding is involved (10 of 40 is badly
   wounded, 11 of 41 is not); `ok` otherwise. `GET /api/hp` (and every HP response) returns it as
   `condition`. The client only maps the code to a label and a style.
   Tests: `TestConditionOf` (hpsvc: each boundary), `TestGetState_Condition` (hpsvc).

## Consequences

- Damage past -10 is lost for good, like a heal's excess past max: popping and re-entering is
  the way to correct it (ADR 0007 step 8).
- The ledger total is now bounded at both ends: `-10 - hp_max ≤ total ≤ 0` for every row written
  since these rules.
- The HP block draws its scale from the floor to max with a flat-tinted dying zone below 0
  (no gradient, [ADR 0009](0009-dashboard-layout-and-theme-system.md)); that is display only.
- Not covered: bleeding a point per round below 0, stabilising, raising the dead, and any game
  effect of the condition (the app only shows it).
