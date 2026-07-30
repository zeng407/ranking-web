<?php

namespace Tests\Unit;

use App\Exceptions\BatchVoteConflictException;
use App\Helper\Locker;
use App\Services\GameService;
use Illuminate\Support\Facades\DB;
use Tests\TestCase;
use Tests\TestHelper;

class BatchVoteTransactionTest extends TestCase
{
    use TestHelper;

    public function test_batch_vote_commits_all_related_changes(): void
    {
        [$game, $winnerId, $loserId] = $this->createTwoElementGame();

        $result = app(GameService::class)->batchUpdateGameRounds($game, [
            ['winner_id' => $winnerId, 'loser_id' => $loserId],
        ], 0);
        $lastRound = $result->lastRound();

        $this->assertNotNull($lastRound);
        $this->assertSame(1, $result->serverVoteCount());
        $this->assertCount(1, $result->acceptedVotes());
        $this->assertDatabaseHas('game_1v1_rounds', [
            'game_id' => $game->id,
            'winner_id' => $winnerId,
            'loser_id' => $loserId,
            'remain_elements' => 1,
        ]);
        $this->assertDatabaseHas('game_elements', [
            'game_id' => $game->id,
            'element_id' => $winnerId,
            'win_count' => 1,
            'is_eliminated' => false,
        ]);
        $this->assertDatabaseHas('game_elements', [
            'game_id' => $game->id,
            'element_id' => $loserId,
            'is_eliminated' => true,
        ]);
        $this->assertDatabaseHas('games', [
            'id' => $game->id,
            'vote_count' => 1,
        ]);
    }

    public function test_batch_vote_rolls_back_every_change_and_releases_lock_on_failure(): void
    {
        [$game, $winnerId, $loserId] = $this->createTwoElementGame();
        $beforeElements = $this->gameElementState($game->id);

        DB::unprepared(<<<'SQL'
CREATE TRIGGER fail_batch_round_insert
BEFORE INSERT ON game_1v1_rounds
BEGIN
    SELECT RAISE(ABORT, 'forced batch round failure');
END
SQL);

        $caughtException = null;
        try {
            app(GameService::class)->batchUpdateGameRounds($game, [
                ['winner_id' => $winnerId, 'loser_id' => $loserId],
            ]);
        } catch (\Throwable $exception) {
            $caughtException = $exception;
        } finally {
            DB::unprepared('DROP TRIGGER IF EXISTS fail_batch_round_insert');
        }

        $this->assertNotNull($caughtException, 'The forced insert failure should escape the service.');
        $this->assertSame($beforeElements, $this->gameElementState($game->id));
        $this->assertDatabaseCount('game_1v1_rounds', 0);
        $this->assertDatabaseHas('games', [
            'id' => $game->id,
            'vote_count' => 0,
        ]);

        $lock = Locker::lockUpdateGameElement($game);
        $this->assertTrue($lock->get(), 'The game lock must be released after rollback.');
        $lock->release();
    }

    public function test_exact_retry_is_idempotent_inside_the_game_lock(): void
    {
        [$game, $winnerId, $loserId] = $this->createTwoElementGame();
        $vote = ['winner_id' => $winnerId, 'loser_id' => $loserId];

        $first = app(GameService::class)->batchUpdateGameRounds($game, [$vote], 0);
        $retry = app(GameService::class)->batchUpdateGameRounds($game, [$vote], 0);

        $this->assertSame(1, $first->serverVoteCount());
        $this->assertSame(1, $retry->serverVoteCount());
        $this->assertCount(0, $retry->acceptedVotes());
        $this->assertSame($first->lastRound()->id, $retry->lastRound()->id);
        $this->assertDatabaseCount('game_1v1_rounds', 1);
        $this->assertDatabaseHas('game_elements', [
            'game_id' => $game->id,
            'element_id' => $winnerId,
            'win_count' => 1,
        ]);
    }

    public function test_retry_can_append_new_votes_after_its_committed_prefix(): void
    {
        [$game, $elementIds] = $this->createGameWithElements(4);
        $firstVote = [
            'winner_id' => $elementIds[0],
            'loser_id' => $elementIds[1],
        ];
        $nextVote = [
            'winner_id' => $elementIds[2],
            'loser_id' => $elementIds[3],
        ];

        app(GameService::class)->batchUpdateGameRounds($game, [$firstVote], 0);
        $retry = app(GameService::class)->batchUpdateGameRounds(
            $game,
            [$firstVote, $nextVote],
            0
        );

        $this->assertSame(2, $retry->serverVoteCount());
        $this->assertSame([$nextVote], $retry->acceptedVotes());
        $this->assertDatabaseCount('game_1v1_rounds', 2);
    }

    public function test_historical_pair_cannot_hide_a_stale_revision(): void
    {
        [$game, $elementIds] = $this->createGameWithElements(6);
        $historicalVote = [
            'winner_id' => $elementIds[0],
            'loser_id' => $elementIds[1],
        ];

        app(GameService::class)->batchUpdateGameRounds($game, [$historicalVote], 0);
        app(GameService::class)->batchUpdateGameRounds($game, [[
            'winner_id' => $elementIds[2],
            'loser_id' => $elementIds[3],
        ]], 1);

        try {
            app(GameService::class)->batchUpdateGameRounds($game, [
                $historicalVote,
                [
                    'winner_id' => $elementIds[4],
                    'loser_id' => $elementIds[5],
                ],
            ], 1);
            $this->fail('An unrelated historical pair must not acknowledge a newer round.');
        } catch (BatchVoteConflictException $exception) {
            $this->assertSame(BatchVoteConflictException::REVISION_MISMATCH, $exception->reason());
            $this->assertSame(2, $exception->serverVoteCount());
        }

        $this->assertDatabaseCount('game_1v1_rounds', 2);
    }

    public function test_stale_revision_is_rejected_before_a_different_tab_changes_the_bracket(): void
    {
        [$game, $elementIds] = $this->createGameWithElements(4);

        app(GameService::class)->batchUpdateGameRounds($game, [[
            'winner_id' => $elementIds[0],
            'loser_id' => $elementIds[1],
        ]], 0);

        try {
            app(GameService::class)->batchUpdateGameRounds($game, [[
                'winner_id' => $elementIds[2],
                'loser_id' => $elementIds[3],
            ]], 0);
            $this->fail('A stale client revision must not be accepted.');
        } catch (BatchVoteConflictException $exception) {
            $this->assertSame(BatchVoteConflictException::REVISION_MISMATCH, $exception->reason());
            $this->assertSame(1, $exception->serverVoteCount());
        }

        $this->assertDatabaseCount('game_1v1_rounds', 1);
    }

    public function test_eliminated_winner_is_rejected_without_changing_the_server_bracket(): void
    {
        [$game, $elementIds] = $this->createGameWithElements(4);

        app(GameService::class)->batchUpdateGameRounds($game, [[
            'winner_id' => $elementIds[0],
            'loser_id' => $elementIds[1],
        ]], 0);

        try {
            app(GameService::class)->batchUpdateGameRounds($game, [[
                'winner_id' => $elementIds[1],
                'loser_id' => $elementIds[2],
            ]], 1);
            $this->fail('An eliminated winner must not be accepted.');
        } catch (BatchVoteConflictException $exception) {
            $this->assertSame(BatchVoteConflictException::WINNER_ELIMINATED, $exception->reason());
            $this->assertSame($elementIds[1], $exception->elementId());
            $this->assertSame(1, $exception->serverVoteCount());
        }

        $this->assertDatabaseCount('game_1v1_rounds', 1);
    }

    public function test_batch_vote_endpoint_returns_structured_conflict_response(): void
    {
        [$game, $elementIds] = $this->createGameWithElements(4);

        app(GameService::class)->batchUpdateGameRounds($game, [[
            'winner_id' => $elementIds[0],
            'loser_id' => $elementIds[1],
        ]], 0);

        $response = $this->postJson(route('api.game.batch-vote'), [
            'game_serial' => $game->serial,
            'expected_vote_count' => 0,
            'votes' => [[
                'winner_id' => $elementIds[2],
                'loser_id' => $elementIds[3],
            ]],
        ]);

        $response->assertStatus(409)->assertExactJson([
            'code' => 'game_state_conflict',
            'reason' => BatchVoteConflictException::REVISION_MISMATCH,
            'server_vote_count' => 1,
        ]);
        $this->assertDatabaseCount('game_1v1_rounds', 1);
    }

    public function test_game_elements_endpoint_returns_state_with_its_authoritative_revision(): void
    {
        [$game, $elementIds] = $this->createGameWithElements(4);

        app(GameService::class)->batchUpdateGameRounds($game, [[
            'winner_id' => $elementIds[0],
            'loser_id' => $elementIds[1],
        ]], 0);

        $response = $this->getJson(route('api.game.elements', [
            'game' => $game->serial,
            'limit' => 4,
        ]));

        $response->assertOk()
            ->assertJsonPath('server_vote_count', 1)
            ->assertJsonCount(4, 'data');

        $loser = collect($response->json('data'))->firstWhere('id', $elementIds[1]);
        $this->assertNotNull($loser);
        $this->assertTrue((bool) $loser['is_eliminated']);
    }

    private function createTwoElementGame(): array
    {
        [$game, $elementIds] = $this->createGameWithElements(2);

        return [$game, $elementIds[0], $elementIds[1]];
    }

    private function createGameWithElements(int $count): array
    {
        $post = $this->seedPost($count);
        $game = $this->createGame($post, $count);
        $elementIds = DB::table('game_elements')
            ->where('game_id', $game->id)
            ->orderBy('id')
            ->pluck('element_id')
            ->all();

        $this->assertCount($count, $elementIds);

        return [$game, $elementIds];
    }

    private function gameElementState(int $gameId): array
    {
        return DB::table('game_elements')
            ->where('game_id', $gameId)
            ->orderBy('id')
            ->get(['id', 'win_count', 'is_eliminated', 'is_ready'])
            ->map(static fn ($row) => (array) $row)
            ->all();
    }
}
