-- Add the unique index that makes a post trend row's identity enforceable.
--
-- Without it, writing a trend position means SELECT-then-INSERT-or-UPDATE, which is
-- not atomic and has already produced 36 duplicate groups (see
-- 00004_dedupe_post_trends.sql). With it, the Go worker uses a single
-- INSERT ... ON DUPLICATE KEY UPDATE and the race cannot occur at all.
--
-- 00004 must run first: this statement fails if any duplicates remain.
--
-- WHY A GENERATED COLUMN
--
-- start_date is nullable and holds NULL for the `all` range — 3,980 rows. MySQL
-- treats NULLs in a unique index as distinct from each other, so a plain
--
--   UNIQUE (post_id, trend_type, time_range, start_date)
--
-- would enforce nothing for `all`: any number of rows could share the same post and
-- range, which is exactly one of the duplicate groups found. Indexing
-- COALESCE(start_date, sentinel) through a STORED generated column closes that hole
-- while leaving start_date itself untouched, so Laravel's
-- `where('start_date', null)` reads and its updateOrCreate keep working unchanged
-- through the cutover.
--
-- '1000-01-01' is the sentinel: it is the lowest value a MySQL DATE can hold, and
-- the earliest real start_date in the data is 2024-01-01, so it cannot collide with
-- a legitimate row.
--
-- PRODUCTION: DO NOT RUN THIS STATEMENT DIRECTLY.
--
-- Adding a STORED generated column cannot be done with ALGORITHM=INPLACE, so this
-- rebuilds the table. `post_trends` is around 974k rows, which is small enough for
-- that locally, but on production run the equivalent change with gh-ost or
-- pt-online-schema-change so it can be throttled and paused, then stamp this
-- version applied rather than executing it.
--
-- The column and the index are added in one ALTER because the rebuild is the
-- expensive part and doing it twice would double the cost.

-- +goose Up
-- +goose NO TRANSACTION
-- +goose StatementBegin
ALTER TABLE post_trends
    ADD COLUMN start_date_key DATE
        GENERATED ALWAYS AS (COALESCE(start_date, '1000-01-01')) STORED
        COMMENT 'Index-only mirror of start_date so the unique key covers the NULL all-range rows',
    ADD UNIQUE INDEX post_trends_post_type_range_date_unique
        (post_id, trend_type, time_range, start_date_key);
-- +goose StatementEnd

-- +goose Down
-- +goose NO TRANSACTION
-- +goose StatementBegin
ALTER TABLE post_trends
    DROP INDEX post_trends_post_type_range_date_unique,
    DROP COLUMN start_date_key;
-- +goose StatementEnd
