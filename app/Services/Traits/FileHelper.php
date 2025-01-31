<?php

namespace App\Services\Traits;

use App\Services\Models\StoragedImage;
use Illuminate\Http\UploadedFile;
use Ramsey\Uuid\Uuid;
use Storage;

trait FileHelper
{
    protected function downloadImage(string $url, string $directory): ?StoragedImage
    {
        $content = $this->getContent($url);
        $fileInfo = pathinfo($url);
        $basename = $this->generateFileName();
        if (isset ($fileInfo['extension'])) {
            // trim query string
            $fileInfo['extension'] = explode('?', $fileInfo['extension'])[0];

            $basename .= '.' . $fileInfo['extension'];
        }

        $path = rtrim($directory, '/') . '/' . $basename;
        $isSuccess = Storage::put($path, $content, 'public');
        if (!$isSuccess) {
            return null;
        }
        return new StoragedImage($url, $path, $fileInfo);
    }

    protected function moveUploadedFile(UploadedFile $file, string $directory): string|bool
    {
        if($this->isImageFile($file->getMimeType())){
            try{
                $image = new \Imagick($file->getRealPath());
                $mineType = $image->getImageMimeType();
            }catch (\Exception $exception){
                report($exception);
                $mineType = null;
            }
            if($mineType){
                $extension = '.' . explode('/', $mineType)[1];
                // x-webp to webp
                if($extension === '.x-webp'){
                    $extension = '.webp';
                }
            } else {
                $extension = null;
            }
        }else{
            $extension = $file->getMimeType();
            $extension = $extension ? ('.' . explode('/', $extension)[1]) : '';
        }
        $path = $file->storeAs($directory, $this->generateFileName(). $extension);
        Storage::setVisibility($path, 'public');
        return $path;
    }

    protected function generateFileName()
    {
        return Uuid::uuid4()->toString();
    }

    protected function parseTitle(UploadedFile $file)
    {
        $title = mb_substr(pathinfo($file->getClientOriginalName(), PATHINFO_FILENAME), 0, config('setting.element_title_size'));
        logger("title: {$title}");
        $title = preg_replace('/[\n\r\t]/', '', $title);
        return $title;
    }

    protected function getContent(string $sourceUrl)
    {
        $content = null;
        logger("getContent: {$sourceUrl}");
        try {
            $ch = curl_init();
            curl_setopt($ch, CURLOPT_URL, $sourceUrl);
            curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
            curl_setopt($ch, CURLOPT_USERAGENT, 'Mozilla/5.0 (Windows; U; Windows NT 5.1; en-US; rv:1.8.1.13) Gecko/20080311 Firefox/2.0.0.13');
            $content = curl_exec($ch);
            curl_close($ch);
        } catch (\Exception $exception) {
        }

        try {
            $content = file_get_contents($sourceUrl);
        } catch (\Exception $exception) {
            logger($exception->getMessage());
        }

        return $content;
    }

    protected function isImageFile($mimeType)
    {
        if(!$mimeType){
            return false;
        }
        return strpos($mimeType, 'image') !== false;
    }

    protected function deleteElemntFile($path)
    {
        // trim path
        $path = str_replace(Storage::url(''), '', $path);
        if (Storage::exists($path)) {
            Storage::delete($path);
            \Log::info("Deleted file: {$path}");
        }
    }
}
