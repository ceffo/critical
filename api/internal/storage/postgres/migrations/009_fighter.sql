-- The fighter's entries (ADR 0030 steps 9 and 12).
--
-- characters.str_exceptional is the exceptional strength percentile a fighter with Strength 18
-- enters (step 9): 1 to 100, 100 being 18/00; 0 is none (every other character). charactersvc
-- validates every write.
-- weapons.specialized is the per-weapon specialization flag (step 12), 0/1 like two_handed;
-- weaponsvc refuses it for a character whose class does not specialize.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): two columns with a default the release before
-- this one never reads. Schema only, no data step: every stored character and weapon has none.
ALTER TABLE characters ADD COLUMN str_exceptional BIGINT NOT NULL DEFAULT 0
    CHECK (str_exceptional BETWEEN 0 AND 100);
ALTER TABLE weapons ADD COLUMN specialized BIGINT NOT NULL DEFAULT 0 CHECK (specialized IN (0, 1));
