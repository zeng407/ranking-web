<?php

namespace App\Events;

use App\Models\Element;
use App\Models\Post;
use Illuminate\Broadcasting\Channel;
use Illuminate\Broadcasting\InteractsWithSockets;
use Illuminate\Broadcasting\PresenceChannel;
use Illuminate\Broadcasting\PrivateChannel;
use Illuminate\Contracts\Broadcasting\ShouldBroadcast;
use Illuminate\Foundation\Events\Dispatchable;
use Illuminate\Queue\SerializesModels;

class VideoElementCreated
{
    use Dispatchable, InteractsWithSockets, SerializesModels;


    public Element $element;

    public Post $post;

    /**
     * Create a new event instance.
     *
     * @return void
     */
    public function __construct(Element $element, Post $post)
    {
        logger('Event [ElementCreated] fired!', ['element' => $element->id, 'post' => $post->id]);
        $this->element = $element;
        $this->post = $post;
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
