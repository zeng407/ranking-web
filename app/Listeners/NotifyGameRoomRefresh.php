<?php

namespace App\Listeners;

use App\Events\RefreshGameCandidates;
use App\Jobs\BroadcastGameRoomRefresh;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class NotifyGameRoomRefresh
{
    /**
     * Handle the event.
     *
     * @param  object  $event
     * @return void
     */
    public function handle(RefreshGameCandidates $event)
    {
        if($event->game->game_room){
            broadcast(new BroadcastGameRoomRefresh($event->game->game_room));
        }
    }
}
