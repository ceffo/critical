-- The non-weapon proficiencies' descriptions (ADR 0021 step 16.3, ADR 0037 step 19).
--
-- One row per catalogue proficiency id: description is the key of its localized_texts rows (ADR
-- 0023; `en` and `fr`, one row per language), written by the catalogue import (format 9) in the
-- import's transaction. The id is a seeds/proficiencies.json id; there is no foreign key to the
-- seeds, as localized_texts has none, and nothing in game state refers to a description (a pick
-- stores the id, ADR 0037 step 12), so the import upserts by id and deletes a row the artifact no
-- longer lists with its texts.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): a new table the release before this one never
-- reads. Schema only (step 2.1): the rows come from the next catalogue import, which the new
-- artifact's format and version trigger at start.
CREATE TABLE proficiency_descriptions (
    proficiency_id TEXT   PRIMARY KEY,
    description    BIGINT NOT NULL
);
