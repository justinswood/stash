-- Performance: index the created_at / updated_at date sorts.
--
-- Scenes are sorted with `ORDER BY datetime(scenes.created_at)` (and updated_at)
-- to normalize mixed timezone suffixes (Z vs +offset) for correct ordering.
-- Wrapping the column in datetime() means a plain column index can't satisfy the
-- sort, so the query fell back to a full scan + temp b-tree sort of every row.
--
-- SQLite supports indexes on deterministic expressions, and datetime() is
-- deterministic, so an expression index matching the ORDER BY term lets the
-- optimizer read rows in date order (EXPLAIN QUERY PLAN: "SCAN scenes USING
-- INDEX index_scenes_created_at_dt" instead of "USE TEMP B-TREE FOR ORDER BY").
-- Only the title/id tie-break within equal timestamps still needs sorting.

CREATE INDEX IF NOT EXISTS `index_scenes_created_at_dt` ON `scenes`(datetime(`created_at`));
CREATE INDEX IF NOT EXISTS `index_scenes_updated_at_dt` ON `scenes`(datetime(`updated_at`));
