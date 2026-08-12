-- Remove duplicate rows from `ranks` so a unique index can be added.
--
-- WHY THEY EXIST
--
-- `ranks` has no unique index on (post_id, element_id, rank_type, record_date),
-- and Laravel's Rank::updateOrCreate is a SELECT followed by an INSERT or UPDATE
-- with nothing making the pair atomic. Two concurrent runs for the same element
-- both find no row and both insert.
--
-- The local database showed exactly this: in both affected groups the two rows
-- share a created_at to the second, and only one of them has a later
-- updated_at, because every later updateOrCreate found that row and left the
-- other frozen at its insert values.
--
--   id      post element  type      date        wins rounds rate  updated_at
--   23448   46   2759     champion  2024-04-05  6    627    0.96  23:57:05
--   23449   46   2759     champion  2024-04-05  6    480    1.25  00:02:01
--
-- RETENTION RULE
--
-- Keep the row with the newest updated_at, breaking ties on the lowest id: that
-- is the row the application has actually been maintaining. COALESCE gives a
-- NULL updated_at a floor so the ordering is a strict total order and exactly one
-- row per group survives.
--
-- WHY A STAGING TABLE
--
-- `ranks` holds roughly 14.2M rows with no index covering the four key columns,
-- so the duplicate groups are found with one GROUP BY pass and recorded; the
-- delete then works from primary keys. A TEMPORARY table cannot be used here:
-- goose runs each statement through a pooled *sql.DB, so a later statement may
-- land on a different connection, and a temporary table is visible only to the
-- connection that created it.
--
-- Idempotent: on a database with no duplicates the staging table comes out empty
-- and nothing is deleted.

-- +goose Up
-- +goose NO TRANSACTION
-- +goose StatementBegin
DROP TABLE IF EXISTS rank_dedupe_staging;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE rank_dedupe_staging (
    delete_id BIGINT UNSIGNED NOT NULL PRIMARY KEY
) ENGINE = InnoDB;
-- +goose StatementEnd

-- Group sizes here are tiny (two rows), but the limit is raised anyway so a
-- larger group on another database cannot silently truncate the ordered id list
-- and pick the wrong survivor.
-- +goose StatementBegin
SET SESSION group_concat_max_len = 1048576;
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO rank_dedupe_staging (delete_id)
SELECT loser.id
FROM ranks AS loser
JOIN (
    SELECT
        post_id,
        element_id,
        rank_type,
        record_date,
        CAST(
            SUBSTRING_INDEX(
                GROUP_CONCAT(
                    id
                    ORDER BY COALESCE(updated_at, '1970-01-01 00:00:00') DESC, id ASC
                ),
                ',', 1
            ) AS UNSIGNED
        ) AS keep_id
    FROM ranks
    GROUP BY post_id, element_id, rank_type, record_date
    HAVING COUNT(*) > 1
) AS duplicated
  ON duplicated.post_id = loser.post_id
 AND duplicated.element_id = loser.element_id
 AND duplicated.rank_type = loser.rank_type
 AND duplicated.record_date = loser.record_date
WHERE loser.id <> duplicated.keep_id;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE ranks
FROM ranks
JOIN rank_dedupe_staging ON rank_dedupe_staging.delete_id = ranks.id;
-- +goose StatementEnd

-- The staging table is kept deliberately. It is the only record of which rows
-- were removed, and it is small. Drop it once the change has been verified.

-- +goose Down
-- Deliberately empty. The deleted rows were unreachable duplicates the
-- application had stopped updating; restoring them would only reintroduce the
-- ambiguity the unique index exists to prevent. Recover from a backup if needed.
