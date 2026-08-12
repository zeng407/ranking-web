-- +goose Up
-- One access policy per post.
--
-- PostService::create calls post_policy()->updateOrCreate() and PostService::update calls
-- post_policy()->update(), both of which read then write. The column has only the
-- non-unique foreign-key index, so two requests that create a post's policy at the same
-- instant can both find nothing and both insert — and then the post has two policies and
-- which one applies is whichever the query happens to read first. For a `password` post
-- that decides whether a stranger gets in.
--
-- It has not happened yet: all 6,201 posts hold exactly one policy row
-- (2,814 public + 2,352 private + 1,035 password = 6,201), so this index can be added
-- without repairing anything first. It is the guarantee the read-then-write never had.
--
-- The FK index is left alone: MySQL needs an index on the referencing column for the
-- foreign key, and the new unique index would serve that, but dropping the old one and
-- letting the constraint fall back is a change this migration does not need to make.
ALTER TABLE post_policies
    ADD UNIQUE INDEX post_policies_post_id_unique (post_id);

-- +goose Down
ALTER TABLE post_policies
    DROP INDEX post_policies_post_id_unique;
