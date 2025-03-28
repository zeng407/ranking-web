<?php

namespace App\Jobs;

use App\Models\Element;
use App\Models\Game;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Database\Eloquent\Collection;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;
use DB;

class AttachGameElements implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    protected $elements;

    protected Game $game;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(Collection $elements, Game $game)
    {
        logger('AttachGameElements', ['game' => $game]);
        $this->onQueue('high');
        $this->elements = $elements;
        $this->game = $game;
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle()
    {
        logger('handle AttachGameElements', ['game' => $this->game]);
        $this->elements->each(function (Element $element){
            if (!$this->game->elements()->where('element_id', $element->id)->exists()) {
                $this->game->elements()->attach($element, [
                    'is_ready' => true
                ]);
            }
        });
    }
}
