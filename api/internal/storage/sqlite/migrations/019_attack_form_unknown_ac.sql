-- The attack form's target AC may be unknown (docs/adr/0012), the default for
-- a new form. target_ac_known = 0 means unknown, target_ac is then
-- ignored. Rows saved before keep the AC they had: it was set or left at
-- 5 knowingly, so they are marked known. (The migrator splits on semicolons: none in comments.)
ALTER TABLE attack_forms ADD COLUMN target_ac_known INTEGER NOT NULL DEFAULT 0 CHECK (target_ac_known IN (0, 1));
UPDATE attack_forms SET target_ac_known = 1;
