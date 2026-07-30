<?php

namespace App\Services\Models;

use App\Models\Game1V1Round;

class BatchVoteResult
{
    private ?Game1V1Round $lastRound;
    private array $acceptedVotes;
    private int $serverVoteCount;

    public function __construct(
        ?Game1V1Round $lastRound,
        array $acceptedVotes,
        int $serverVoteCount
    ) {
        $this->lastRound = $lastRound;
        $this->acceptedVotes = $acceptedVotes;
        $this->serverVoteCount = $serverVoteCount;
    }

    public function lastRound(): ?Game1V1Round
    {
        return $this->lastRound;
    }

    public function acceptedVotes(): array
    {
        return $this->acceptedVotes;
    }

    public function serverVoteCount(): int
    {
        return $this->serverVoteCount;
    }
}
