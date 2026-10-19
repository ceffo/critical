-- The thief's and the bard's skill armour is the worn body armour (ADR 0033 step 14, amending ADR
-- 0030 steps 1, 17 and 18): characters.skill_armor, the armour entered for the skills, is no longer
-- written or read. It stays (a later cleanup migration drops it) because the skill_armor entries
-- already logged still pop and redo by writing it back.
--
-- Schema only (ADR 0025 step 2.1): the carry-over is a data step run just after this file
-- (carrySkillArmour), in the same transaction: a thief or a bard that wears no body armour gets
-- the armour it entered, worn, so every skill percentage is the same before and after.
--
-- Additive (ADR 0024 step 5.2): a comment. The release before this one keeps reading and writing
-- the column for the minutes both run; what it writes is not read by this one.
COMMENT ON COLUMN characters.skill_armor IS
    'The skill armour entered before ADR 0033 step 14; no longer read (the worn body armour is), written only by the undo of an old skill_armor entry.';
