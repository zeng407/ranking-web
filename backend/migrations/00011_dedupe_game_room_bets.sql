-- One wager per player per round, and a repair of the scores the missing index cost.
--
-- GameService::bet is an updateOrCreate keyed on
-- (game_room_id, game_room_user_id, current_round, of_round, remain_elements) with no
-- unique index underneath, so a double-clicked vote inserts twice.
--
-- THIS ONE HAS FIRED THIRTEEN TIMES, AND IT CHANGED PEOPLE'S SCORES.
--
-- All 13 groups (2026-08-06, on the restore of production) have exactly two copies, the
-- same winner_id in both, and created_at equal to the second — double submissions, not
-- changed votes. Both copies were then settled: won_at set on both, or lost_at on both.
-- Because the tally sums every settled wager, those rounds were counted twice, and each
-- group's contribution is exactly double what one round pays: 20 for a 10, 80 for a 40,
-- 100 for a 50. Eleven players are affected; two of them lost the race twice.
--
-- WHY THE KEY IS FOUR COLUMNS AND NOT FIVE
--
-- Laravel's key includes game_room_id, but a game_room_user belongs to exactly one room,
-- so it adds nothing to the uniqueness and only widens the index.
--
-- WHY THE REPAIR IS SCOPED THE WAY IT IS
--
-- 513 players currently have stored totals that differ from a settled-only tally. It
-- would be wrong to "fix" all of them: 512 of those differences are explained by
-- unsettled wagers, which the OLD rule counted and the new rule deliberately does not.
-- Rewriting them here would quietly apply a rule change to five hundred rows under the
-- heading of a de-duplication.
--
-- So the repair covers exactly two groups, and no others:
--
--   (a) the players who held duplicate wagers, captured BEFORE the delete — their
--       totals were computed over rows that are about to disappear;
--   (b) players whose totals disagree with a tally while EVERY one of their wagers is
--       settled — for those the old rule and the new rule give the same answer, so the
--       difference cannot be the rule change and can only be stale data.
--
-- Group (b) is not padding. It contains player 289, whose stored totals sit two rounds
-- behind its own 233 settled wagers: the old four-mechanism coalescing dropped the last
-- two settlements and nothing ever recomputed the room. That is the defect the Go
-- version's version counter replaces.

-- +goose Up
-- +goose StatementBegin
-- Captured before the delete, because afterwards there is no way to tell which players
-- were affected. A real table rather than a temporary one so the set is auditable if the
-- migration is interrupted.
CREATE TABLE go_migration_00011_affected (
    game_room_user_id BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (game_room_user_id)
) ENGINE = InnoDB;
-- +goose StatementEnd

-- +goose StatementBegin
-- IGNORE because a player can appear in more than one duplicate group: two of the eleven
-- lost the race in two different rounds.
INSERT IGNORE INTO go_migration_00011_affected (game_room_user_id)
SELECT game_room_user_id
  FROM game_room_user_bets
 GROUP BY game_room_user_id, current_round, of_round, remain_elements
HAVING COUNT(*) > 1;
-- +goose StatementEnd

-- +goose StatementBegin
-- The lowest id is kept: that is the row the create won, and the one an updateOrCreate
-- would have gone on updating.
DELETE loser
  FROM game_room_user_bets AS loser
  JOIN game_room_user_bets AS keeper
    ON keeper.game_room_user_id = loser.game_room_user_id
   AND keeper.current_round = loser.current_round
   AND keeper.of_round = loser.of_round
   AND keeper.remain_elements = loser.remain_elements
   AND keeper.id < loser.id;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE game_room_user_bets
    ADD UNIQUE KEY game_room_user_bets_round_unique
        (game_room_user_id, current_round, of_round, remain_elements);
-- +goose StatementEnd

-- +goose StatementBegin
-- Group (b), added to the same staging table: totals that disagree with a tally while
-- every wager is settled.
-- The tally is a derived table rather than a GROUP BY with the comparison in HAVING:
-- HAVING can only see grouped columns, aggregates and select aliases, so referring to
-- player.total_played there is an error rather than a subtle wrong answer.
--
-- LEFT JOIN so a player with no wagers at all is still considered: their totals should be
-- 0/0/1000, and anything else is stale.
INSERT IGNORE INTO go_migration_00011_affected (game_room_user_id)
SELECT player.id
  FROM game_room_users AS player
  LEFT JOIN (
        SELECT game_room_user_id AS id,
               SUM(won_at IS NULL AND lost_at IS NULL)                        AS unsettled,
               SUM(won_at IS NOT NULL OR lost_at IS NOT NULL)                 AS settled,
               SUM(won_at IS NOT NULL)                                        AS correct,
               SUM(IF(won_at IS NOT NULL OR lost_at IS NOT NULL, score, 0))   AS score_sum
          FROM game_room_user_bets
         GROUP BY game_room_user_id
       ) AS tally ON tally.id = player.id
 WHERE COALESCE(tally.unsettled, 0) = 0
   AND (player.total_played <> COALESCE(tally.settled, 0)
        OR player.total_correct <> COALESCE(tally.correct, 0)
        OR player.score <> 1000 + COALESCE(tally.score_sum, 0));
-- +goose StatementEnd

-- +goose StatementBegin
-- The tally counts SETTLED wagers only, matching internal/gameroom.Tally. 1000 is
-- config('setting.default_bet_score'). accuracy mirrors the same ROUND() the Go
-- recompute uses, so a room repaired here and a room recomputed by the worker agree.
UPDATE game_room_users AS target
  JOIN (
        SELECT affected.game_room_user_id AS id,
               COUNT(bet.id)                            AS played,
               COALESCE(SUM(bet.won_at IS NOT NULL), 0) AS correct,
               1000 + COALESCE(SUM(bet.score), 0)       AS score
          FROM go_migration_00011_affected AS affected
          LEFT JOIN game_room_user_bets AS bet
                 ON bet.game_room_user_id = affected.game_room_user_id
                AND (bet.won_at IS NOT NULL OR bet.lost_at IS NOT NULL)
         GROUP BY affected.game_room_user_id
       ) AS tally ON tally.id = target.id
   SET target.total_played = tally.played,
       target.total_correct = tally.correct,
       target.score = tally.score,
       target.accuracy = IF(tally.played = 0, 0,
                            ROUND(tally.correct * 10000 / tally.played) / 100),
       target.updated_at = NOW();
-- +goose StatementEnd

-- +goose StatementBegin
-- Ranks follow the scores that just changed, room by room. Without this the leaderboard
-- would still be ordered by the doubled totals.
UPDATE game_room_users AS target
  JOIN (
        SELECT player.id,
               ROW_NUMBER() OVER (PARTITION BY player.game_room_id
                                      ORDER BY player.score DESC, player.id) AS position
          FROM game_room_users AS player
         WHERE player.game_room_id IN (
                   SELECT DISTINCT room_user.game_room_id
                     FROM game_room_users AS room_user
                     JOIN go_migration_00011_affected AS affected
                       ON affected.game_room_user_id = room_user.id)
       ) AS ordered ON ordered.id = target.id
   SET target.`rank` = ordered.position,
       target.updated_at = NOW();
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE go_migration_00011_affected;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- The removed copies are not restored, and neither are the doubled totals: both were
-- the defect. Only the index is reversed.
ALTER TABLE game_room_user_bets DROP INDEX game_room_user_bets_round_unique;
-- +goose StatementEnd
