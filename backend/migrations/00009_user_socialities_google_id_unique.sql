-- The missing unique index on user_socialities.google_id.
--
-- Same defect class as post_trends and public_posts, and the third time it appears in
-- this schema: application code does SELECT ... WHERE google_id = ? and then INSERTs
-- when nothing came back, with only a non-unique index underneath. Two OAuth
-- callbacks for the same Google account arriving together — a double-clicked consent
-- screen is enough — both read nothing and both insert, and that account now has two
-- rows. Which one wins the login afterwards depends on row order.
--
-- WHY THIS ONE HAS NOT FIRED YET
--
-- 11,304 rows, 11,304 distinct google_id values, 0 duplicate groups (2026-08-06, on
-- the restore of production). Unlike the other two, this race has never actually been
-- lost — a second callback needs to land inside the few milliseconds of the insert,
-- and sign-ups are rare enough that it has not happened in the table's lifetime. That
-- is luck, not a guarantee, and the index costs nothing on 11k rows.
--
-- No generated column is needed here, unlike post_trends: google_id has no NULLs, and
-- MySQL treating NULLs as distinct is the behaviour we want for a socialite row that
-- has no Google link.
--
-- The old non-unique index is dropped in the same statement. A unique index serves
-- every lookup the plain one served, so keeping both would only cost writes.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_socialities
    DROP INDEX user_socialities_google_id_index,
    ADD UNIQUE KEY user_socialities_google_id_unique (google_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_socialities
    DROP INDEX user_socialities_google_id_unique,
    ADD KEY user_socialities_google_id_index (google_id);
-- +goose StatementEnd
