<?php

namespace App\Jobs;

use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
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

    protected Post $post;
    protected Element $element;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(Post $post, Element $element)
    {
        $this->post = $post;
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
        $rankService->createElementRank($this->post, $this->element);
    }


    public function uniqueId()
    {
        return $this->post->serial . $this->element->id;
    }
}
