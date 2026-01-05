<?php

namespace App\Jobs;

use App\Http\Resources\Game\ChampionResource;
use App\Models\UserGameResult;
use Illuminate\Broadcasting\InteractsWithSockets;
use Illuminate\Contracts\Broadcasting\ShouldBroadcast;
use Illuminate\Contracts\Broadcasting\ShouldBroadcastNow;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Broadcasting\Channel;
use Illuminate\Bus\Queueable;


class BroadcastNewChampion implements ShouldBroadcast
{
    use Dispatchable, InteractsWithSockets, Queueable;

    protected UserGameResult $userGameResult;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(UserGameResult $userGameResult)
    {
        $this->userGameResult = $userGameResult;
        $this->onQueue('low');
    }

    public function broadcastWith()
    {
        return ChampionResource::make($this->userGameResult)->toArray(request());
    }

    public function broadcastOn()
    {
        return new Channel('home.champion');
    }

    public function broadcastAs()
    {
        return 'new-champion';
    }
}
