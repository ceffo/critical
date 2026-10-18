-- The bard's entries (ADR 0030 steps 1 and 18).
--
-- A bard enters skill points and a skill armour as a thief does, in the same two columns
-- (migration 011): characters.skill_points holds the points of its four skills (PHB Table 33), and
-- characters.skill_armor gains 'metal', the bard's "metal armour up to chain mail" (chain, ring,
-- scale, hide or brigandine), which only a bard may wear for its skills (charactersvc checks the
-- class). The percentages are derived on read, never stored; the bard_skills rows stay as the
-- imported sheet's figures and are no longer read.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): a wider CHECK; the release before this one
-- never writes 'metal' and reads a bard's points as none. Schema only (step 2.1): the seed
-- character already stored (Kelsen) gets the points its seed names from a data step run just after
-- this file (pointSeedCharacter), the points that give his sheet's percentages.
ALTER TABLE characters DROP CONSTRAINT characters_skill_armor_check;
ALTER TABLE characters ADD CONSTRAINT characters_skill_armor_check
    CHECK (skill_armor IN ('', 'none', 'leather', 'padded_studded_leather', 'elven_chain', 'metal'));
