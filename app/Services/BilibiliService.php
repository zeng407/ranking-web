<?php


namespace App\Services;

class BilibiliService
{
    private array $cache = [];

    /**
     * @param $url
     * @return mixed|string
     * @throws \Exception
     */
    public function parseVideoId($url): string
    {
        // try getting video_id format from bilibili url https://www.bilibili.com/video/xxxxx
        preg_match("/^((?:https?:)?\/\/)?((?:www|m)\.)?((?:bilibili(-nocookie)?\.com))(\/(?:[\w\-]+\?v=|video\/|v\/)?)([\w\-]+)(\S+)?$/", $url, $matches);
        logger("return  matches ");
        logger($matches);

        if ($matches && isset($matches[6]) && $matches[5] === '/video/') {
            return str_replace('/', '', $matches[6] ?? '');
        }

        return '';

    }

    public function getThumbnail($url)
    {
        logger("getThumbnail from url: $url");

        // create a new DOMDocument and load the HTML
        $html = $this->getHtml($url);
        $doc = new \DOMDocument();
        @$doc->loadHTML($html);

        // create a new DOMXPath and query the document
        $xpath = new \DOMXPath($doc);
        $metas = $xpath->query('//meta[@property="og:image"]');

        // check if a meta tag was found
        if ($metas->length > 0) {
            $meta = $metas->item(0);
            $thumbnail = $meta->getAttribute('content');
            logger($thumbnail);
            
            // if url starts with // then convert to https
            if(strpos($thumbnail, '//') === 0) {
                $thumbnail = 'https:' . $thumbnail;
            }

            // if url ends with @100w_100h_1c.png then remove it
            if(strpos($thumbnail, '@100w_100h_1c.png') !== false) {
                $thumbnail = str_replace('@100w_100h_1c.png', '', $thumbnail);
            }
            
            return $thumbnail;
        }

        return '';
    }

    public function getH1Title($url)
    {
        logger("getH1Title from url: $url");

        $html = $this->getHtml($url);

        // create a new DOMDocument and load the HTML
        $doc = new \DOMDocument();
        @$doc->loadHTML($html);

        // create a new DOMXPath and query the document
        $xpath = new \DOMXPath($doc);
        $h1s = $xpath->query('//h1');

        // check if a h1 tag was found
        if ($h1s->length > 0) {
            $h1 = $h1s->item(0);
            return $h1->nodeValue;
        }

        return '';
    }

    public function getHtml($url)
    {
        if(isset($this->cache[$url])) {
            return $this->cache[$url];
        }

        // parse html content
        $client = new \GuzzleHttp\Client();
        $response = $client->request('GET', $url);
        $html = $response->getBody()->getContents();

        $this->cache[$url] = $html;
        return $html;
    }

}
