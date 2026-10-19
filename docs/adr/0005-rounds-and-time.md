# ADR 0005: Rounds and time

**Status**: accepted, 2026-10-15 (critical-qki.36; collects the rules shipped in critical-qki.8,
critical-qki.29 and critical-qki.30, until then in `docs/backend.md`)

_UI word: the French UI says tour for a round ([ADR 0036](0036-vocabulary.md) step 3)._

## Context

The round counter is the combat clock: the player advances it by hand and it counts the timed
buffs down ([ADR 0004](0004-buffs.md) step 2). Outside combat the table jumps ahead by hours
("two hours of travel"), which must expire the buffs without a hundred taps. A mistake at the
table must always be undoable.

## Decision

Tests live in `api/internal/domain/service/buffsvc/` unless named otherwise.

1. **Next Round.** The counter goes up by 1 and every timed buff loses 1 round (ADR 0004
   steps 2 and 4); the buffs that reach 0 expire and are named in the entry. One entry
   (`round_advance`) holds both snapshots, buffs and round, and is stamped with the round it
   **ends** (recorded before the counter moves).
   Tests: `TestAdvanceRound` (the three cases, the order `record`, `save buffs`, `save round`,
   one transaction, the snapshots).
2. **End combat.** The counter goes to 0 and the buffs are left alone (they keep their rounds:
   a spell outlives the fight). It is logged even at round 0.
   Tests: `TestRoundMoves` ("End combat resets to 0", "End combat at round 0 still logs": no
   write of the buffs, snapshot of the round only).
3. **Set round.** The counter is set to any whole number from 0 up, the buffs are left alone and
   nothing counts down; a negative round is refused and nothing is written.
   Tests: `TestRoundMoves` ("Set round up", "Set round down to 0", "Set round rejects a negative
   round without touching anything").
4. **Voidable.** Each of the three is one log entry and voiding it restores the previous round
   (and, for Next Round, the buffs, expired ones included).
   Tests: `TestRoundMoves` ("voiding restores the previous round"), `TestRestorers_WriteTheSnapshotBack`
   ("round", the refusals), `TestRestorers_CoverEverySnapshotKey`.
5. **Hours to rounds.** Fast-forwarding takes a time in decimal hours; one round is one minute
   (AD&D), so a round count is `floor(hours × 60)` (1.5 h = 90, 0.02 h = 1). A tiny epsilon absorbs
   float noise (1.15 h is 69 rounds, not 68). The time must be above 0, at most 720 h (30 days:
   a typo like 8000 for 8 is refused rather than wiping every buff) and at least one round.
   Nothing is written for an invalid time.
   Tests: `TestRoundsForHours` (the values, "rejects zero / negative / under one round / over the
   cap / NaN / infinite", the message), `TestPassTime` ("invalid hours write nothing").
6. **Passing time.** Every timed buff loses that many rounds and expires when it reaches 0 or
   below (exactly the rounds left expires: 30 rounds of a 30-round buff); buffs with charges only
   and buffs until dismissed are untouched; a timed buff with charges expires with its rounds
   (ADR 0004 step 4). The round counter goes to 0 (out of combat).
   Tests: `TestPassTime` ("1.5 h: 100 rounds keeps 10, 60 expires, charge-only and untimed
   untouched", "exactly the rounds left expires (edge at 0)", "a timed buff with charges…",
   "no buffs").
7. **Buffs and the round only.** Passing time is one `time_passed` entry, with buffs and round
   snapshots, stamped with the round it leaves; voiding it brings back the buffs and the round.
   It changes no other state: daily items and spell slots are refilled by a long rest only.
   Tests: `TestPassTime` (the entry, its snapshots, payload; the service holds no slot or item port, so it
   cannot refill either).
8. **Preview.** Before confirming, the player sees what passing a time would do: the server applies
   steps 5 and 6 to the current buffs and returns the rounds and the buffs that would expire,
   writing and logging nothing (`GET /api/round/pass-time/preview?hours=`). Invalid hours are
   refused with step 5's message. The client never decides which buffs end.
   Tests: `TestRounds_Step8_PreviewPassTime` ("1.5 h: 90 rounds, the buffs at 90 or less would
   expire, nothing is written", "invalid hours read nothing"); `router` `TestRoundPassTimePreview`.

## Consequences

- The round counter has no meaning for a buff's age beyond Next Round and passing time: a Set
  round or End combat never expires anything, a deliberate choice so a miskeyed round is harmless.
- The conversion is whole rounds, rounded down: a fraction of a minute is not tracked.
- Next Round is followed by the new round's initiative, its own entry; End combat, Set round to
  0 and passing time end the initiative ([ADR 0013](0013-initiative.md)).
- Not covered: the long rest (daily items, spell slots; see the slots ADR), turn order inside a
  round.
