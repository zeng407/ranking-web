-- One participant row per browser per room.
--
-- GameService::getGameRoomUser is another SELECT-then-INSERT over a non-unique index:
-- it looks for a row matching the caller and creates one when there is none. The Go
-- find-or-create needs this index to be safe under two simultaneous first requests,
-- which is exactly what happens when a room link is opened and the page fires its
-- initial calls together.
--
-- 0 duplicates on (game_room_id, anonymous_id) across 30,054 rows (2026-08-06), so
-- unlike the wagers this race has never been lost. The index costs nothing at that size.
--
-- WHY NOT ALSO (game_room_id, user_id)
--
-- Because the data says it would be wrong. Three logged-in users hold two rows each in
-- one room, from two different browsers, and BOTH rows have real play history — 47 rounds
-- and 4, 13 and 0, 74 and 15. They are two genuine participants, not a duplicate, and a
-- unique index there would have to merge or delete one of them.
--
-- That also exposes a defect being deliberately left behind rather than ported. Laravel
-- matches with
--
--     where(user_id = X OR anonymous_id = Y)->first()
--
-- with no ORDER BY. When a logged-in user has rows under two anonymous ids, both sides of
-- the OR match different rows and which one comes back is up to the query plan — so which
-- participant you resume as can change between requests. The Go port matches on
-- (game_room_id, anonymous_id) alone, which is deterministic and is what the data shows
-- actually happened: each browser kept its own row. user_id is still recorded on the row,
-- for audit and for the profile lookups, but it is not part of the identity.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE game_room_users
    DROP INDEX game_room_users_anonymous_id_index,
    ADD UNIQUE KEY game_room_users_participant_unique (game_room_id, anonymous_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE game_room_users
    DROP INDEX game_room_users_participant_unique,
    ADD KEY game_room_users_anonymous_id_index (anonymous_id);
-- +goose StatementEnd
