-- Remove duplicate rows from `post_trends` so a unique index can be added.
--
-- WHY THEY EXIST
--
-- The same defect as 00002_dedupe_ranks.sql, in a second table. `post_trends` has
-- no unique index on (post_id, trend_type, time_range, start_date), and
-- UpdatePostTrendsPosition writes with PostTrend::updateOrCreate on exactly those
-- four columns — a SELECT followed by an INSERT or UPDATE with nothing making the
-- pair atomic. Two concurrent runs for the same post both find no row and both
-- insert.
--
-- The local database shows 36 such groups, 36 surplus rows, and every one carries
-- the signature of that race:
--
--   id      post trend time_range start_date  position created_at           updated_at
--   487530  160  hot   today      2025-06-12  342      2025-06-13 00:05:11  2025-06-14 08:37:57
--   487531  160  hot   today      2025-06-12  9999     2025-06-13 00:05:11  2025-06-14 08:37:38
--
-- Both rows were inserted in the same second. The job resets the whole group to
-- 9999 and then assigns real positions, so the row its SELECT kept finding got the
-- position while the other stayed frozen at 9999 forever — invisible in the
-- rankings but still counted by anything that reads the table.
--
-- RETENTION RULE
--
-- Keep the newest updated_at, breaking ties on the lowest id: that is the row the
-- application has actually been maintaining, and in every observed group it is
-- also the one holding a real position rather than 9999. COALESCE floors a NULL
-- updated_at so the ordering is a strict total order and exactly one row per group
-- survives.
--
-- start_date IS NULL for the `all` range, 3,980 rows, one of which is a duplicate
-- group. NULL <=> NULL is used to join those groups, because `=` would not match
-- them and they would be left behind for the unique index to trip over.
--
-- Idempotent: on a database with no duplicates the staging table comes out empty
-- and nothing is deleted.

-- +goose Up
-- +goose NO TRANSACTION
-- +goose StatementBegin
DROP TABLE IF EXISTS post_trend_dedupe_staging;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE post_trend_dedupe_staging (
    delete_id BIGINT UNSIGNED NOT NULL PRIMARY KEY
) ENGINE = InnoDB;
-- +goose StatementEnd

-- Observed groups hold two rows, but the limit is raised anyway so a larger group
-- on another database cannot silently truncate the ordered id list and pick the
-- wrong survivor.
-- +goose StatementBegin
SET SESSION group_concat_max_len = 1048576;
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO post_trend_dedupe_staging (delete_id)
SELECT loser.id
FROM post_trends AS loser
JOIN (
    SELECT
        post_id,
        trend_type,
        time_range,
        start_date,
        CAST(
            SUBSTRING_INDEX(
                GROUP_CONCAT(
                    id
                    ORDER BY COALESCE(updated_at, '1970-01-01 00:00:00') DESC, id ASC
                ),
                ',', 1
            ) AS UNSIGNED
        ) AS keep_id
    FROM post_trends
    GROUP BY post_id, trend_type, time_range, start_date
    HAVING COUNT(*) > 1
) AS duplicated
  ON duplicated.post_id = loser.post_id
 AND duplicated.trend_type = loser.trend_type
 AND duplicated.time_range = loser.time_range
 AND duplicated.start_date <=> loser.start_date
WHERE loser.id <> duplicated.keep_id;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE post_trends
FROM post_trends
JOIN post_trend_dedupe_staging ON post_trend_dedupe_staging.delete_id = post_trends.id;
-- +goose StatementEnd

-- The staging table is kept deliberately. It is the only record of which rows were
-- removed, and it is small. Drop it once the change has been verified.

-- +goose Down
-- Deliberately empty. The deleted rows were unreachable duplicates the application
-- had stopped updating, all of them frozen at position 9999; restoring them would
-- only reintroduce the ambiguity the unique index exists to prevent. Recover from a
-- backup if needed.
