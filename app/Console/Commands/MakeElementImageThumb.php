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
        Element::where(function ($query) {
            $query->where('type', ElementType::IMAGE)
                ->orWhere('video_source', VideoSource::IMGUR)
                ->orWhere('video_source', VideoSource::URL);
        })
            ->whereHas('posts')
            ->chunkById(100, function ($elements) use (&$counter) {
                foreach ($elements as $element) {
                    try {
                        if ($element->video_source == VideoSource::URL) {
                            //create a path
                            $post = $element->posts()->first();
                            if (!$post) {
                                $this->warn('Cannot find post, element_id: ' . $element->id);
                                continue;
                            }

                            if ($element->path) {
                                continue;
                            }

                            try {
                                $fileInfo = get_headers($element->source_url, true);
                                if (isset($fileInfo[0]) 
                                    && ($fileInfo[0] == 'HTTP/1.1 404 Not Found' || $fileInfo[0] == 'HTTP/1.1 302 Found') ) {
                                    $this->warn('Cannot get file info: ' . $element->id);
                                    continue;
                                }

                                if (!isset ($fileInfo['Content-Length'])) {
                                    $this->warn('Cannot get Content-Length: ' . $element->id);
                                    continue;
                                }
                                $size = $fileInfo['Content-Length'];
                                $this->info($element->title . ' size: ' . round($size / 1024 / 1024, 2) . 'MB');
                                if ($size / 1024 / 1024 >= 10) {
                                    $this->warn('File size is too big: ' . $element->id . ' ' . ($size / 1024 / 1024) . 'MB');
                                    continue;
                                }

                                $tempFile = $this->elementService->downloadImage($element->source_url, $post->serial);
                                $element->update([
                                    'path' => $tempFile->getPath(),
                                    'thumb_url' => \Storage::url($tempFile->getPath())
                                ]);
                                $this->info('Element: ' . $element->id . ' ' . $element->thumb_url);
                                $counter++;
                            } catch (\Exception $exception) {
                                report($exception);
                                continue;
                            }
                        }

                        if (!\Storage::exists($element->path)) {
                            $path = dirname($element->path);
                            $file = $this->elementService->downloadImage($element->source_url, $path)
                                ?? $this->elementService->downloadImage($element->thumb_url, $path);
                            if (!$file) {
                                $this->warn('Cannot download image: ' . $element->id);
                                continue;
                            }
                            $path = $file->getPath();
                        } else {
                            $path = $element->path;
                        }

                        $url = \Storage::url($path);
                        if ($url != $element->thumb_url) {
                            $element->path = $path;
                            $element->thumb_url = $url;
                            $element->save();
                            $counter++;
                            $this->info('Element: ' . $element->id . ' ' . $element->thumb_url);
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
