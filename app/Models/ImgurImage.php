<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\SoftDeletes;

class ImgurImage extends Model
{
    use HasFactory;
    use SoftDeletes;

    protected $fillable = [
        'image_id',
        'imgur_album_id',
        'title',
        'description',
        'delete_hash',
        'link',
    ];

    public function imageable()
    {
        return $this->morphTo();
    }
    
}
