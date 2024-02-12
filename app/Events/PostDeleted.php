<?php

namespace App\Events;

use App\Models\Post;
use Illuminate\Broadcasting\Channel;
use Illuminate\Broadcasting\InteractsWithSockets;
use Illuminate\Broadcasting\PresenceChannel;
use Illuminate\Broadcasting\PrivateChannel;
use Illuminate\Contracts\Broadcasting\ShouldBroadcast;
use Illuminate\Foundation\Events\Dispatchable;
use Illuminate\Queue\SerializesModels;

class PostDeleted
{
    use Dispatchable, InteractsWithSockets, SerializesModels;

    protected Post $post;

    /**
     * Create a new event instance.
     *
     * @return void
     */
    public function __construct(Post $post)
    {
        logger('[PostDeleted] event fired', ['post' => $post->id]);
        $this->post = $post;
    }

    public function getPost(): Post
    {
        return $this->post;
    }

    /**
     * Get the channels the event should broadcast on.
     *
     * @return \Illuminate\Broadcasting\Channel|array
     */
    public function broadcastOn()
    {
        return new PrivateChannel('channel-name');
    }
}