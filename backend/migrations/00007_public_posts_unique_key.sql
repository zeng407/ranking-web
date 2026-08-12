-- Make a public post's identity enforceable: one row per post.
--
-- Without this, writing the listing means SELECT-then-INSERT-or-UPDATE, which is not
-- atomic and has already produced 3 duplicate listings (see
-- 00006_dedupe_public_posts.sql). With it, the Go refresh uses a single
-- INSERT ... ON DUPLICATE KEY UPDATE per chunk and the race cannot occur.
--
-- 00006 must run first: this statement fails if any duplicates remain.
--
-- post_id is NOT NULL, so unlike post_trends this needs no generated column — MySQL's
-- treating NULLs as distinct is not a factor here.
--
-- The table is small, around 2,200 rows, so the index build is quick and
-- ALGORITHM=INPLACE with LOCK=NONE applies. LOCK=NONE is stated explicitly so the
-- statement fails loudly rather than silently taking an exclusive lock if the server
-- cannot do it online.
--
-- The existing non-unique post_id index is left in place: it backs the foreign key,
-- and dropping it would force MySQL to rebuild the constraint.

-- +goose Up
-- +goose NO TRANSACTION
-- +goose StatementBegin
ALTER TABLE public_posts
    ADD UNIQUE INDEX public_posts_post_id_unique (post_id),
    ALGORITHM = INPLACE,
    LOCK = NONE;
-- +goose StatementEnd

-- +goose Down
-- +goose NO TRANSACTION
-- +goose StatementBegin
ALTER TABLE public_posts
    DROP INDEX public_posts_post_id_unique,
    ALGORITHM = INPLACE,
    LOCK = NONE;
-- +goose StatementEnd
