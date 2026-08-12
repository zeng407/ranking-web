-- One game room per game, enforced by the database.
--
-- The fourth missing unique index in this schema, after post_trends, public_posts and
-- user_socialities. GameService::createGameRoom is
--
--     $game->game_room()->firstOrCreate([], ['serial' => ...])
--
-- over a plain KEY on game_id, so two callers can both find nothing and both insert.
--
-- THIS ONE HAS ALREADY FIRED, TWICE. 15,209 rooms over 15,207 games (2026-08-06, on the
-- restore of production): games 51695 and 83035 each have two. Both extra rooms were
-- created in the same second as their twin and are completely empty — 0 users, 0 bets —
-- so whoever created them got a room nobody could find, because every later lookup goes
-- through the game and returns the first row.
--
-- Deleting the higher id therefore loses nothing. The lower id is kept because that is
-- the row firstOrCreate would return from then on.

-- +goose Up
-- +goose StatementBegin
DELETE loser
  FROM game_rooms AS loser
  JOIN game_rooms AS keeper
    ON keeper.game_id = loser.game_id
   AND keeper.id < loser.id;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE game_rooms
    DROP INDEX game_rooms_game_id_foreign,
    ADD UNIQUE KEY game_rooms_game_id_unique (game_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- The deleted rooms are not restored: they were empty, and their serials were never
-- reachable. Only the index shape is reversed.
ALTER TABLE game_rooms
    DROP INDEX game_rooms_game_id_unique,
    ADD KEY game_rooms_game_id_foreign (game_id);
-- +goose StatementEnd
