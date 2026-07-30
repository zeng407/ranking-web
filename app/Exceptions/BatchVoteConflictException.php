<?php

namespace App\Exceptions;

use RuntimeException;

class BatchVoteConflictException extends RuntimeException
{
    public const REVISION_MISMATCH = 'revision_mismatch';
    public const WINNER_ELIMINATED = 'winner_eliminated';
    public const LOSER_ELIMINATED = 'loser_eliminated';

    private string $reason;
    private int $serverVoteCount;
    private ?int $elementId;

    public function __construct(
        string $reason,
        int $serverVoteCount,
        ?int $elementId = null
    ) {
        $this->reason = $reason;
        $this->serverVoteCount = $serverVoteCount;
        $this->elementId = $elementId;

        parent::__construct(sprintf(
            'Batch vote conflict (%s) at server revision %d%s.',
            $reason,
            $serverVoteCount,
            $elementId === null ? '' : " for element {$elementId}"
        ));
    }

    public function reason(): string
    {
        return $this->reason;
    }

    public function serverVoteCount(): int
    {
        return $this->serverVoteCount;
    }

    public function elementId(): ?int
    {
        return $this->elementId;
    }
}
