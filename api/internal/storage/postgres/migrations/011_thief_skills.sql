-- The thief's entries (ADR 0030 steps 1 and 17).
--
-- characters.skill_points are a thief's entered skill points: a JSON object from skill code (PHB
-- Table 26's eight skills) to points, the skills without points absent; '{}' is none (every other
-- character). characters.skill_armor is the armour the thief wears for its skills (none, leather,
-- padded_studded_leather or elven_chain); '' for a character without thieving skills.
-- charactersvc validates every write; the percentages are derived on read, never stored.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): two columns with a default the release before
-- this one never reads. Schema only, no data step: no stored character is a thief.
ALTER TABLE characters ADD COLUMN skill_points TEXT NOT NULL DEFAULT '{}';
ALTER TABLE characters ADD COLUMN skill_armor TEXT NOT NULL DEFAULT ''
    CHECK (skill_armor IN ('', 'none', 'leather', 'padded_studded_leather', 'elven_chain'));
