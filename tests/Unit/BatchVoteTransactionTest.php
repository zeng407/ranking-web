<?php

namespace Tests\Unit;

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

        $lastRound = app(GameService::class)->batchUpdateGameRounds($game, [
            ['winner_id' => $winnerId, 'loser_id' => $loserId],
        ]);

        $this->assertNotNull($lastRound);
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

    private function createTwoElementGame(): array
    {
        $post = $this->seedPost(2);
        $game = $this->createGame($post, 2);
        $elementIds = DB::table('game_elements')
            ->where('game_id', $game->id)
            ->orderBy('id')
            ->pluck('element_id')
            ->all();

        $this->assertCount(2, $elementIds);

        return [$game, $elementIds[0], $elementIds[1]];
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
