<?php


namespace App\Services;


use App\Enums\ElementType;
use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Models\User;
use App\Repositories\ElementRepository;
use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\Auth;
use Illuminate\Support\Facades\Storage;

class ElementService
{
    protected $repo;

    public function __construct(ElementRepository $elementRepository)
    {
        $this->repo = $elementRepository;
    }

    public function getLists(array $conditions, array $paginationOptions = [])
    {
        $query = $this->repo->filter($conditions);

        $perPage = 15;

        if(isset($paginationOptions['per_page'])){
            $perPage = $paginationOptions['per_page'];
        }

        return $query->paginate($perPage);
    }

    public function storePublic(UploadedFile $file, string $path, Post $post)
    {
        $saveDir = $path;
        $path = $file->store($saveDir);
        Storage::setVisibility($path, 'public');

        //todo sign path
        $url = Storage::url($path);

        //todo make thumb
        $thumb = $url;

        $element = $post->elements()->create([
            'path' => $path,
            'source_url' => $url,
            'thumb_url' => $thumb,
            'type' => ElementType::IMAGE,
            'title' => substr(pathinfo($file->getClientOriginalName(), PATHINFO_FILENAME), 0, config('post.title_size'))
        ]);

        return $element;
    }

    public function storePublicFromUrl(string $sourceUrl, string $path, Post $post)
    {
        try {
            //try check image validation
            if(!@getimagesize($sourceUrl)){
                return null;
            };

            $fileInfo = pathinfo($sourceUrl);
            $basename = $fileInfo['basename'];
            $content = file_get_contents($sourceUrl);

            $path = rtrim($path,'/').'/'.$basename;
            $isSuccess = Storage::put($path, $content, 'public');

            if(!$isSuccess){
                return null;
            }
        }catch (\Exception $exception){
            report($exception);
            return null;
        }

        //todo sign path
        $url = Storage::url($path);

        //todo make thumb
        $thumb = $url;

        $element = $post->elements()->create([
            'path' => $path,
            'source_url' => $url,
            'thumb_url' => $thumb,
            'type' => ElementType::IMAGE,
            'title' => $fileInfo['filename']
        ]);

        return $element;
    }

}
