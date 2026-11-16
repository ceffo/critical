-- The skills' descriptions (ADR 0021 step 33.3, ADR 0037 step 19.6).
--
-- One row per skill of a skill set of the catalogue, keyed by the pair (skill set id, skill id): a
-- set has its own text for a skill (the bard's Pick Pockets is the bard's paragraph, not the
-- thief's). description is the key of its localized_texts rows (ADR 0023; `en` and `fr`, one row
-- per language), written by the catalogue import (format 25) in the import's transaction. The
-- pair is a skill_sets id and one of its skills' ids in the artifact; there is no foreign key to
-- them, as localized_texts has none and the skill sets are read from the artifact at every start,
-- not stored. Nothing in game state refers to a description (a character's skill points name the
-- set and the skill, ADR 0040 step 12), so the import upserts by pair and deletes a row the
-- artifact no longer lists with its texts.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): a new table the release before this one never
-- reads. Schema only (step 2.1): the rows come from the next catalogue import, which the new
-- artifact's format and version trigger at start.
CREATE TABLE skill_descriptions (
    skill_set   TEXT   NOT NULL,
    skill       TEXT   NOT NULL,
    description BIGINT NOT NULL,
    PRIMARY KEY (skill_set, skill)
);
