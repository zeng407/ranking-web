<?php

namespace App\Console\Commands;

use App\Enums\ElementType;
use App\Enums\VideoSource;
use App\Services\ElementService;
use Illuminate\Console\Command;
use App\Models\Element;

class MakeElementImageThumb extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'make:element-image-thumb';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Make element image thumb';

    protected ElementService $elementService;

    /**
     * Create a new command instance.
     *
     * @return void
     */
    public function __construct(ElementService $elementService)
    {
        $this->elementService = $elementService;
        parent::__construct();
    }

    /**
     * Execute the console command.
     *
     * @return int
     */
    public function handle()
    {
        $this->info('Make element image thumb');
        $counter = 0;
        Element::where(function($query){
            $query->where('type', ElementType::IMAGE)
                ->orWhere('video_type', VideoSource::IMGUR);
            })
            ->whereHas('posts')
            ->chunkById(100, function ($elements) use (&$counter) {
            foreach ($elements as $element) {
                try {
                    if(!\Storage::exists($element->path)){
                        $path = dirname($element->path);
                        $file = $this->elementService->downloadImage($element->source_url, $path)
                            ?? $this->elementService->downloadImage($element->thumb_url, $path);
                        if(!$file){
                            $this->warn('Cannot download image: ' . $element->id);
                            continue;
                        }
                        $path = $file->getPath();
                    }else{
                        $path = $element->path;
                    }
                    
                    $url = \Storage::url($path);
                    if($url != $element->thumb_url){
                        $element->path = $path;
                        $element->thumb_url = $url;
                        $element->save();
                        $counter++;
                        $this->info('Element: ' . $element->id. ' ' . $element->thumb_url);
                    }
                } catch (\Exception $e) {
                    \Log::error($e->getMessage());
                }
            }
            });

        $this->info('Total: ' . $counter);
        return 0;
    }

    
}
