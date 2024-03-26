<?php

namespace App\Console\Commands;

use App\Enums\ElementType;
use App\Services\ElementService;
use Illuminate\Console\Command;
use App\Models\Element;

class BackupElementImage extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'backup:element-image';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Backup element image to local storage';

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
        $this->info('Start backup element image');
        $counter = 0;
        Element::where('type', 'image')
            ->whereNotNull('thumb2_url')
            ->whereHas('posts')
            ->chunkById(100, function ($elements) use (&$counter) {
            foreach ($elements as $element) {
                try {
                    if(!\Storage::exists($element->path)){
                        $path = dirname($element->path);
                        $file = $this->elementService->downloadImage($element->thumb_url, $path)
                            ?? $this->elementService->downloadImage($element->source_url, $path);
                        if(!$file){
                            $this->warn('Cannot download image: ' . $element->id);
                            continue;
                        }
                        $path = $file->getPath();
                    }else{
                        $path = $element->path;
                    }
                    
                    $url = \Storage::url($path);
                    if($url != $element->thumb2_url){
                        $element->path = $path;
                        $element->thumb2_url = $url;
                        $element->save();
                        $counter++;
                    }
                    $this->info('Backup element image: ' . $element->id . ' - ' . $element->thumb2_url);
                } catch (\Exception $e) {
                    \Log::error($e->getMessage());
                }
            }
            });

        $this->info('Backup element image done. Total: ' . $counter);
        return 0;
    }

    
}
