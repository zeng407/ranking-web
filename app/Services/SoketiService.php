<?php

namespace App\Services;

use Http;
use Pusher\Pusher;

class SoketiService
{
    public function getSubscriptionCount($channel)
    {
        try{
            $pusher = $this->getPusher();
            $res = $pusher->get("/channels/{$channel}");

            if(isset($res->occupied)) {
                return (int)$res->subscription_count;
            }else{
                return 0;
            }
        }catch(\Exception $e){
            report($e);
            return -1;
        }
    }

    protected function getPusher()
    {
        $pusher = new Pusher(
            config('broadcasting.connections.pusher.key'),
            config('broadcasting.connections.pusher.secret'),
            config('broadcasting.connections.pusher.app_id'),
            [
                'cluster' => config('broadcasting.connections.pusher.options.cluster'),
                'useTLS' => config('broadcasting.connections.pusher.options.useTLS'),
                'host' => config('broadcasting.connections.pusher.options.host'),
                'port' => config('broadcasting.connections.pusher.options.port'),

            ]
        );

        return $pusher;
    }
}
