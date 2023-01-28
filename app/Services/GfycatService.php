<?php


namespace App\Services;

use GuzzleHttp\Client;

class GfycatService
{
    protected $client;

    public function __construct(Client $client)
    {
        $this->client = $client;
    }

    public function getId(string $url)
    {
       return url_basename($url);
    }

    public function getInfo(string $id)
    {
        $res = $this->client->get("https://api.gfycat.com/v1/gfycats/$id");

        return json_decode($res->getBody()->getContents());
    }

}
