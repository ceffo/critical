-- A bow's Strength rating (ADR 0033 step 3, ADR 0002 step 4): the Strength a strength bow is built
-- for, a score (1 to 25) and, at 18, its exceptional percentile (1 to 100, 0 none). NULL is an
-- ordinary bow, and every other piece.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): every stored piece takes NULL, so every bow
-- already stored is an ordinary one; the release before this one never reads either column. No
-- CHECK: the service validates every write (itemsvc.validate), as for the weapon type.
ALTER TABLE items ADD COLUMN strength_rating             BIGINT;
ALTER TABLE items ADD COLUMN strength_rating_exceptional BIGINT NOT NULL DEFAULT 0;
