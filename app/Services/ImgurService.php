<?php

namespace App\Services;

use App\Models\ImgurAlbum;
use App\Models\ImgurImage;
use Http;
use Cache;

class ImgurService implements InterfaceOauthService
{

    protected function getClient()
    {
        return Http::baseUrl('https://api.imgur.com/3/');
    }

    public function getAccountInfo(string $username)
    {
        $client = $this->getClient();
        $client->withHeaders([
            'Authorization' => 'Client-ID ' . config('services.imgur.client_id'),
        ]);
        $res = $client->get('account/' . $username);
        return json_decode($res->getBody()->getContents(), true);
    }

    public function createAlbum(string $title, string $description)
    {
        $client = $this->getClient();
        $client->withHeaders([
            'Authorization' => 'Bearer ' . $this->getAccessToken(),
        ]);
        $res = $client->post('album', [
            'title' => $title,
            'description' => $description,
        ]);

        return json_decode($res->getBody()->getContents(), true);
    }

    public function deleteAlbum(string $albumId)
    {
        $client = $this->getClient();
        $client->withHeaders([
            'Authorization' => 'Bearer ' . $this->getAccessToken(),
        ]);
        $res = $client->delete('album/' . $albumId);
        $res = json_decode($res->getBody()->getContents(), true);
        if ($res['success']) {
            ImgurAlbum::where('album_id', $albumId)->delete();
        }
        return $res;
    }

    public function uploadImage(string $imgUrl, ?string $title, ?string $description, string $albumId)
    {
        $client = $this->getClient();
        $client->withHeaders([
            // 'Authorization' => 'Bearer ' . $this->getAccessToken(),
            'Authorization' => 'Client-ID ' . config('services.imgur.client_id'),
        ]);
        $res = $client->post('image', [
            'image' => $imgUrl,
            'title' => $title,
            'description' => $description,
            // 'album' => $albumId,
            'type' => 'URL',
        ]);

        return json_decode($res->getBody()->getContents(), true);
    }

    public function deleteImage(string $imageId)
    {
        $client = $this->getClient();
        $client->withHeaders([
            'Authorization' => 'Bearer ' . $this->getAccessToken(),
        ]);
        $res = $client->delete('image/' . $imageId);
        $res = json_decode($res->getBody()->getContents(), true);
        if ($res['success']) {
            ImgurImage::where('image_id', $imageId)->delete();
        }
        return $res;
    }

    public function parseGalleryAlbumId(string $url)
    {
        $matches = [];
        preg_match('/^https?:\/\/(?:www\.)?imgur\.com\/(?:gallery|a|t\/[a-zA-Z0-9]+)\/([a-zA-Z0-9]+)$/i', $url, $matches);
        if(isset($matches[1])){
            return $matches[1];
        }

        preg_match('/^https?:\/\/(?:www\.)?imgur\.com\/(?:gallery|a)?\/?([a-zA-Z0-9]+)$/i', $url, $matches);
        return $matches[1] ?? null;
    }

    public function getGalleryAlbumImages(string $albumId)
    {
        $client = $this->getClient();
        $client->withHeaders([
            'Authorization' => 'Client-ID ' . config('services.imgur.client_id'),
        ]);
        $res = $client->get("gallery/album/$albumId");
        $res = json_decode($res->getBody()->getContents(), true);

        return $res;
    }

    public function getImage(string $imageId)
    {
        $client = $this->getClient();
        $client->withHeaders([
            'Authorization' => 'Client-ID ' . config('services.imgur.client_id'),
        ]);
        $res = $client->get("image/$imageId");
        $res = json_decode($res->getBody()->getContents(), true);

        return $res;
    }

    public function getAlbumImages(string $albumId)
    {
        $client = $this->getClient();
        $client->withHeaders([
            'Authorization' => 'Client-ID ' . config('services.imgur.client_id'),
        ]);
        $res = $client->get("album/$albumId/images");
        $res = json_decode($res->getBody()->getContents(), true);

        return $res;
    }

    public function refreshAccessToken()
    {
        $client = Http::withHeaders([
            'Authorization' => 'Client-ID ' . config('services.imgur.client_id'),
        ]);
        $res = $client->post('https://api.imgur.com/oauth2/token', [
            'refresh_token' => $this->getRefreshToken(),
            'client_id' => config('services.imgur.client_id'),
            'client_secret' => config('services.imgur.client_secret'),
            'grant_type' => 'refresh_token',
        ]);
        $res = json_decode($res->getBody()->getContents(), true);
        if (isset($res['access_token'])) {
            Cache::put('imgur_access_token', $res['access_token'], $res['expires_in'] / 60);
        }
        if (isset($res['refresh_token'])) {
            Cache::put('imgur_refresh_token', $res['refresh_token'], 60 * 24 * 30);
        }

        return $res;
    }

    protected function getRefreshToken()
    {
        return Cache::get('imgur_refresh_token') ?? config('services.imgur.refresh_token');
    }

    protected function getAccessToken()
    {
        return Cache::get('imgur_access_token') ?? config('services.imgur.access_token');
    }
}
