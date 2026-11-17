-- Drops characters.skill_points, the flat skill points of a character (ADR 0040 step 3.5):
-- migration 034 added characters.skill_sets (points per skill set) and copied each character's
-- points there; the release that carried 034 still read skill_points as a fallback (a release from
-- before 034 may run during a rolling deploy), and this release neither reads nor writes it. The
-- release before this one (034 to 047) still SELECTs the column, so it breaks the moment 048 is
-- applied: restart every instance on this release together, no rolling deploy across 048.
--
-- Contract step of the expand/contract that 034 began (ADR 0024 step 5.2, ADR 0025 step 2.2).
-- Schema only (step 2.1), so the proof is a guard: the drop is refused, and the run rolls back,
-- while a character still has flat points and no skill sets. Those points can be live (written by a
-- release from before 034 after 034's copy ran: the class's first set read them) or dead (034 copied
-- nothing for a class without a skill set, and no release could read them). Which one only the
-- class definition says, and a migration holds no data, so the guard never clears them: it names
-- the characters and the two remedies, and nothing is lost until a person chooses.
--
-- Re-run safe: the file is applied once (schema_migrations); a database that already lost the
-- column, such as a fresh one built from the migrations in order, never reaches the guard twice.
DO $$
DECLARE
    stuck TEXT;
BEGIN
    SELECT string_agg(id::text, ', ' ORDER BY id) INTO stuck
    FROM characters WHERE skill_points <> '{}' AND skill_sets = '{}';
    IF stuck IS NOT NULL THEN
        RAISE EXCEPTION 'characters.skill_points still holds points of characters without skill sets (ids: %). Fix each by hand in psql (a migration holds no data, ADR 0025 step 2.1), then restart. Dead points (the class has no skill set): set skill_points = ''{}'' on the character. Live points: set skill_sets = json_build_object(''<skill set code of the class>'', skill_points::json)::text on it', stuck;
    END IF;
END
$$;

ALTER TABLE characters DROP COLUMN skill_points;
