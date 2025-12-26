<?php

namespace App\Jobs;

use App\Models\Element;
use App\Models\Post;
use App\Services\RankService;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class UpdateBatchElementRanks implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    protected Post $post;
    protected array $elementIds;

    /**
     * Create a new job instance.
     *
     * @param Post $post
     * @param array $elementIds
     */
    public function __construct(Post $post, array $elementIds)
    {
        // 設定到低優先級佇列，以免阻擋即時性任務
        $this->post = $post;
        $this->elementIds = $elementIds;
    }

    /**
     * Execute the job.
     *
     * @param RankService $rankService
     * @return void
     */
    public function handle(RankService $rankService)
    {
        if (empty($this->elementIds)) {
            return;
        }

        // 為了避免一次撈取過多資料，使用 chunk 處理
        // 雖然 createElementRank 裡面是個別 query，但這裡先撈出 Model 傳進去
        Element::whereIn('id', $this->elementIds)
            ->chunk(50, function ($elements) use ($rankService) {
                foreach ($elements as $element) {
                    try {
                        // 執行排名更新邏輯
                        $rankService->createElementRank($this->post, $element);
                    } catch (\Exception $e) {
                        \Log::error('Update rank failed in batch job', [
                            'post_id' => $this->post->id,
                            'element_id' => $element->id,
                            'error' => $e->getMessage()
                        ]);
                    }
                }
            });
    }
}
