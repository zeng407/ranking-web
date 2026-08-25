-- +goose Up
-- Majority-rule rooms: the room decides the round instead of the host.
--
-- Three columns rather than a separate settings table, because a room has exactly one
-- setting for its whole life and a table would only add a join to every read of it.
--
-- `round_seconds` carries BOTH settings the mode offers: a positive value is a countdown,
-- and zero means the host ends each round by hand. A separate boolean would be able to
-- disagree with the number beside it, and there is no third state to represent.
--
-- `round_ends_at` is the one piece of this the server has to own. The bracket is played in
-- the host's browser, so the server cannot settle a round itself — but the host and every
-- participant must be counting down to the SAME instant, and their device clocks are not
-- comparable. So the deadline is stored here, and clients are told how many seconds are
-- left rather than when it falls due.
--
-- game_rooms holds 15,233 rows, so a plain ALTER is a few milliseconds. Every existing room
-- keeps today's behaviour: vote_mode defaults to 'host', which is the host clicking a
-- candidate and nothing else changing.
ALTER TABLE game_rooms
    ADD COLUMN vote_mode varchar(16) NOT NULL DEFAULT 'host' AFTER serial,
    ADD COLUMN round_seconds smallint unsigned NOT NULL DEFAULT 0 AFTER vote_mode,
    ADD COLUMN round_ends_at datetime(3) NULL DEFAULT NULL AFTER round_seconds;

-- +goose Down
ALTER TABLE game_rooms
    DROP COLUMN round_ends_at,
    DROP COLUMN round_seconds,
    DROP COLUMN vote_mode;
