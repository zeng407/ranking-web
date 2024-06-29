<?php

namespace App\Jobs;

use App\Models\GameRoom;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class ReNotifyGameBet implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    protected GameRoom $gameRoom;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(GameRoom $gameRoom)
    {
        $this->gameRoom = $gameRoom;
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle()
    {
        logger('ReNotifyGameBet', [$this->gameRoom->serial]);
        NotifyGameBet::dispatch($this->gameRoom);
    }
}
