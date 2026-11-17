-- Drops characters.skill_points, the flat skill points of a character (ADR 0040 step 3.5):
-- migration 034 added characters.skill_sets (points per skill set) and copied each character's
-- points there, and no release since reads or writes skill_points, so nothing relies on its legacy
-- read any more (the one that carried 034 is deployed everywhere).
--
-- Contract step of the expand/contract that 034 began (ADR 0024 step 5.2, ADR 0025 step 2.2).
-- Schema only (step 2.1), so the proof is a guard: the drop is refused, and the run rolls back,
-- while a character still has flat points and no skill sets, points only the copy 034 ran (and
-- this release no longer holds) could have moved. The data step that did the copy is gone with
-- the column it read; a database that never ran 034 is upgraded through the release that carries
-- it first.
--
-- Re-run safe: the file is applied once (schema_migrations); a database that already lost the
-- column, such as a fresh one built from the migrations in order, never reaches the guard twice.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM characters WHERE skill_points <> '{}' AND skill_sets = '{}') THEN
        RAISE EXCEPTION 'characters.skill_points still holds points of characters without skill sets: upgrade through the release that carries migration 034 first (its copy moves them), then retry';
    END IF;
END
$$;

ALTER TABLE characters DROP COLUMN skill_points;
