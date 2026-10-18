-- A weapon's type: a language-neutral code (ADR 0030 step 23), read by the class rules (attacks per
-- round, specialization, thief and priest weapon limits, backstab). '' is a weapon with no type.
-- Additive. The list of codes is weaponsvc.Types, which validates every write; there is no CHECK,
-- so the list lives in one place. The weapons already stored are typed by a data step run just after
-- this file (typeSeedWeapons: migrations hold no data, ADR 0025 step 2.1.1).
ALTER TABLE weapons ADD COLUMN weapon_type TEXT NOT NULL DEFAULT '';
