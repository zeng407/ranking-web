<?php

namespace App\Jobs;

use App\Models\Element;
use App\Models\Game;
use App\Services\RankService;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;


class UpdateElementRank implements ShouldQueue, ShouldBeUnique
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;
    
    protected Game $game;
    protected Element $element;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(Game $game, Element $element)
    {
        $this->game = $game;
        $this->element = $element;
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle(RankService $rankService)
    {
        logger('UpdateElementRank job fired');
        $rankService->createElementRank($this->game, $this->element);
    }


    public function uniqueId()
    {
        return $this->game->post->serial . $this->element->id;
    }
}
