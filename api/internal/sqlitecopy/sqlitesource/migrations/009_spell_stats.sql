-- Typed spell stat block (casting time, range, duration, area, components,
-- save) as JSON, parsed from the raw PHB columns once at seed time. Nullable:
-- rows that predate this migration are filled by the seeder's backfill.
ALTER TABLE spells ADD COLUMN stats TEXT;
