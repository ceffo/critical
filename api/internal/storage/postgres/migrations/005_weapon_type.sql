-- A weapon's type: a language-neutral code from a fixed list (ADR 0030 step 23), read by the class rules
-- (attacks per round, specialization, thief and priest weapon limits, backstab). '' is a weapon with no
-- type. Additive: existing rows keep '' and the seeds type the seeded weapons. The list is also held by
-- weaponsvc.Types; the CHECK keeps a stray code out of the table.
ALTER TABLE weapons ADD COLUMN weapon_type TEXT NOT NULL DEFAULT '' CHECK (weapon_type IN (
    '', 'long_bow', 'short_bow', 'light_crossbow', 'heavy_crossbow', 'hand_crossbow', 'dagger', 'dart',
    'sling', 'short_sword', 'long_sword', 'other_melee', 'other_thrown', 'other_missile'));
