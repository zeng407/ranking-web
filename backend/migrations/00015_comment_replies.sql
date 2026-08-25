-- +goose Up
-- Replies to a comment, at most three levels deep.
--
-- Two columns rather than three: there is deliberately no `floor`. A floor number is the
-- position of a top-level comment within its post, and since a deleted comment now keeps
-- its row and is served as a tombstone, that position never moves. It is therefore read
-- with ROW_NUMBER() rather than stored, which also means the legacy PHP endpoint at
-- routes/api.php:28 keeps working: it knows nothing about these columns, and a comment it
-- inserts still lands on the next floor because floors are counted, not assigned.
--
-- `depth` is stored even though it is derivable from the parent chain, because the only
-- two things that ever ask for it — the depth limit on insert and the reply button on the
-- client — would otherwise both have to walk that chain. It is written once and never
-- changes: a comment cannot be re-parented.
--
-- comments holds 21,447 rows in about 6 MB, so a plain ALTER is a few milliseconds. It is
-- none of the four large tables the package comment warns about.
ALTER TABLE comments
    ADD COLUMN parent_id bigint unsigned NULL DEFAULT NULL AFTER user_id,
    ADD COLUMN depth tinyint unsigned NOT NULL DEFAULT 1 AFTER parent_id,
    ADD CONSTRAINT comments_parent_id_foreign FOREIGN KEY (parent_id) REFERENCES comments (id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE comments
    DROP FOREIGN KEY comments_parent_id_foreign,
    DROP COLUMN depth,
    DROP COLUMN parent_id;
