<?php

namespace App\Services;

use App\Models\ImgurAlbum;
use App\Models\ImgurImage;
use Http;

class ImgurService
{
    protected $client;

    public function __construct()
    {
        $this->client = Http::baseUrl('https://api.imgur.com/3/');
    }
    public function getAccountInfo(string $username)
    {
        $this->client->withHeaders([
            'Authorization' => 'Client-ID ' . config('services.imgur.client_id'),
        ]);
        $res = $this->client->get('account/' . $username);
        return json_decode($res->getBody()->getContents(), true);
    }

    public function createAlubm(string $title, string $description)
    {
        $this->client->withHeaders([
            'Authorization' => 'Bearer ' . config('services.imgur.access_token'),
        ]);
        $res = $this->client->post('album', [
            'title' => $title,
            'description' => $description,
        ]);

        return json_decode($res->getBody()->getContents(), true);
    }

    public function deleteAlbum(string $albumId)
    {
        $this->client->withHeaders([
            'Authorization' => 'Bearer ' . config('services.imgur.access_token'),
        ]);
        $res = $this->client->delete('album/' . $albumId);
        $res = json_decode($res->getBody()->getContents(), true);
        if($res['success']) {
            ImgurAlbum::where('album_id', $albumId)->delete();
        }
        return $res;
    }

    public function uploadImage(string $imgUrl, ?string $title, ?string $description, string $albumId)
    {
        $this->client->withHeaders([
            'Authorization' => 'Bearer ' . config('services.imgur.access_token'),
        ]);
        $res = $this->client->post('image', [
            'image' => $imgUrl,
            'title' => $title,
            'description' => $description,
            'album' => $albumId,
            'type' => 'URL',
        ]);
        return json_decode($res->getBody()->getContents(), true);
    }

    public function deleteImage(string $imageId)
    {
        $this->client->withHeaders([
            'Authorization' => 'Bearer ' . config('services.imgur.access_token'),
        ]);
        $res = $this->client->delete('image/' . $imageId);
        $res = json_decode($res->getBody()->getContents(), true);
        if($res['success']) {
            ImgurImage::where('image_id', $imageId)->delete();
        }
        return $res;
    }

    public function parseGalleryAlbumId(string $url)
    {
        $matches = [];
        preg_match('/^https?:\/\/imgur\.com\/(gallery|a)\/([a-zA-Z0-9]+)$/', $url, $matches);
        return $matches[2] ?? null;
    }

    public function getGalleryAlbumImages(string $albumId)
    {
        $this->client->withHeaders([
            'Authorization' => 'Client-ID ' . config('services.imgur.client_id'),
        ]);
        $res = $this->client->get("gallery/album/$albumId");
        $res = json_decode($res->getBody()->getContents(), true);

        return $res;

    }

    public function getImage(string $imageId)
    {
        $this->client->withHeaders([
            'Authorization' => 'Client-ID ' . config('services.imgur.client_id'),
        ]);
        $res = $this->client->get("image/$imageId");
        $res = json_decode($res->getBody()->getContents(), true);

        return $res;

    }
}