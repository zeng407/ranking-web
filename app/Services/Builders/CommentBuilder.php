<?php

namespace App\Services\Builders;

use App\Models\Post;
use App\Models\User;
use Ramsey\Uuid\Uuid;

class CommentBuilder
{
    protected Post $post;
    protected string $content;
    protected string $nickname;
    protected ?User $user;
    protected string $anonymous_id;
    protected array $label;
    protected string $ip;
    protected bool $anonymous_mode = false;

    public function setContent(string $content): self
    {
        $this->content = $content;
        return $this;
    }

    public function setUser(?User $user): self
    {
        $this->user = $user;
        return $this;
    }

    public function setAnonymousId(string $anonymous_id): self
    {
        $this->anonymous_id = $anonymous_id;
        return $this;
    }

    public function setLabel(array $label): self
    {
        $this->label = $label;
        return $this;
    }

    public function setIp(string $ip): self
    {
        $this->ip = $ip;
        return $this;
    }

    public function setAnonymousMode(bool $anonymous_mode): self
    {
        $this->anonymous_mode = $anonymous_mode;
        return $this;
    }

    public function setPost(Post $post): self
    {
        $this->post = $post;
        return $this;
    }

    public function getNickname(): string
    {
        if ($this->user == null) {
            return config('setting.anonymous_nickname');
        }
        return $this->user->name;
    }

    public function getUserid(): ?int
    {
        if ($this->user == null) {
            return null;
        }
        return $this->user->id;
    }

    public function getIp(): string
    {
        return $this->ip ?? 'unknown';
    }

    public function getAnonymousId(): string
    {
        return $this->anonymous_id ?? 'unknown';
    }

    public function getLabel(): array
    {
        return $this->label;
    }

    public function getAnonymousMode(): bool
    {
        return $this->anonymous_mode;
    }

    public function build(): \App\Models\Comment
    {
        return $this->post->comments()->create([
            'nickname' => $this->getNickname(),
            'anonymous_id' => $this->getAnonymousId(),
            'user_id' => $this->getUserid(),
            'content' => $this->content,
            'label' => $this->getLabel(),
            'anonymous_mode' => $this->getAnonymousMode(),
            'delete_hash' => Uuid::uuid4()->toString(),
            'ip' => $this->getIp()
        ]);
    }


}