-- Remove duplicate rows from `public_posts` so a unique index can be added.
--
-- WHY THEY EXIST
--
-- The third table with this defect, after `ranks` (00002) and `post_trends` (00004).
-- `public_posts` has no unique index on post_id, and
-- PublicPostScheduleExecutor::updatePublicPosts writes with
-- PublicPost::updateOrCreate(['post_id' => ...]) — a SELECT followed by an INSERT or
-- UPDATE with nothing making the pair atomic. The four passes run back to back in one
-- job, so a redelivery or a second worker is enough for two of them to insert the
-- same post.
--
-- The local database shows 3 duplicated post ids, 3 surplus rows. Smaller than the
-- other two tables because the schedule holds a 60-minute overlap lock, which narrows
-- the window without closing it.
--
-- A duplicate here is worse than in the other tables: the listing reads
-- public_posts directly, so the post appears twice on the page, and the four position
-- columns of the two rows drift apart as later passes update whichever row their
-- SELECT happens to find.
--
-- RETENTION RULE
--
-- Keep the newest updated_at, breaking ties on the lowest id: the row the application
-- has been maintaining. COALESCE floors a NULL updated_at so the ordering is a strict
-- total order and exactly one row per post survives.
--
-- Idempotent: with no duplicates the staging table comes out empty and nothing is
-- deleted.

-- +goose Up
-- +goose NO TRANSACTION
-- +goose StatementBegin
DROP TABLE IF EXISTS public_post_dedupe_staging;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE public_post_dedupe_staging (
    delete_id BIGINT UNSIGNED NOT NULL PRIMARY KEY
) ENGINE = InnoDB;
-- +goose StatementEnd

-- +goose StatementBegin
SET SESSION group_concat_max_len = 1048576;
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO public_post_dedupe_staging (delete_id)
SELECT loser.id
FROM public_posts AS loser
JOIN (
    SELECT
        post_id,
        CAST(
            SUBSTRING_INDEX(
                GROUP_CONCAT(
                    id
                    ORDER BY COALESCE(updated_at, '1970-01-01 00:00:00') DESC, id ASC
                ),
                ',', 1
            ) AS UNSIGNED
        ) AS keep_id
    FROM public_posts
    GROUP BY post_id
    HAVING COUNT(*) > 1
) AS duplicated
  ON duplicated.post_id = loser.post_id
WHERE loser.id <> duplicated.keep_id;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE public_posts
FROM public_posts
JOIN public_post_dedupe_staging ON public_post_dedupe_staging.delete_id = public_posts.id;
-- +goose StatementEnd

-- The staging table is kept deliberately: it is the only record of which rows were
-- removed, and it is tiny. Drop it once the change has been verified.

-- +goose Down
-- Deliberately empty. The removed rows were duplicate listings of a post that already
-- has one; restoring them would put the post back on the page twice. Recover from a
-- backup if needed.
