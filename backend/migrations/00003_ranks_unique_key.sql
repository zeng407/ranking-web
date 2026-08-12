-- Add the unique index that makes a rank row's identity enforceable.
--
-- Without it, writing a rank means SELECT-then-INSERT-or-UPDATE, which is not
-- atomic and has already produced duplicate rows (see 00002_dedupe_ranks.sql).
-- With it, the Go worker uses a single
-- INSERT ... ON DUPLICATE KEY UPDATE and the race cannot occur at all, no matter
-- how many replicas run concurrently.
--
-- 00002 must run first: this statement fails if any duplicates remain.
--
-- PRODUCTION: DO NOT RUN THIS STATEMENT DIRECTLY.
--
-- `ranks` is roughly 14.2M rows and 5.2 GiB. MySQL 8 can add a secondary unique
-- index with ALGORITHM=INPLACE, LOCK=NONE, which permits concurrent reads and
-- writes, but it still needs a full index build plus temp space, and it takes a
-- brief metadata lock at the start and end that a long-running transaction can
-- block, queueing every subsequent query behind it.
--
-- For production, run the equivalent change with gh-ost or
-- pt-online-schema-change so it can be throttled and paused, then mark this
-- version applied with `migrate baseline`-style stamping rather than executing
-- it. The local timing measured when this migration was written is recorded in
-- refactor-plans/06-go-backend-week1.md.
--
-- LOCK=NONE is stated explicitly so the statement fails loudly if the server
-- cannot do it online, rather than silently taking an exclusive lock.

-- +goose Up
-- +goose NO TRANSACTION
-- +goose StatementBegin
ALTER TABLE ranks
    ADD UNIQUE INDEX ranks_post_element_type_date_unique (post_id, element_id, rank_type, record_date),
    ALGORITHM = INPLACE,
    LOCK = NONE;
-- +goose StatementEnd

-- +goose Down
-- +goose NO TRANSACTION
-- +goose StatementBegin
ALTER TABLE ranks
    DROP INDEX ranks_post_element_type_date_unique,
    ALGORITHM = INPLACE,
    LOCK = NONE;
-- +goose StatementEnd
