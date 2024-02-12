<?php

namespace App\Events;

use App\Models\Element;
use Illuminate\Broadcasting\Channel;
use Illuminate\Broadcasting\InteractsWithSockets;
use Illuminate\Broadcasting\PresenceChannel;
use Illuminate\Broadcasting\PrivateChannel;
use Illuminate\Contracts\Broadcasting\ShouldBroadcast;
use Illuminate\Foundation\Events\Dispatchable;
use Illuminate\Queue\SerializesModels;

class ElementDeleted
{
    use Dispatchable, InteractsWithSockets, SerializesModels;

    protected Element $element;
    /**
     * Create a new event instance.
     *
     * @return void
     */
    public function __construct(Element $element)
    {
        logger('[ImageElementDeleted] event fired');
        $this->element = $element;
    }

    public function getElement(): Element
    {
        return $this->element;
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