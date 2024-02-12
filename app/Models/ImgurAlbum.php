<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\SoftDeletes;

/**
 * @mixin IdeHelperImgurAlbum
 */
class ImgurAlbum extends Model
{
    use HasFactory;
    use SoftDeletes;

    protected $fillable = [
        'album_id',
        'title',
        'description',
        'delete_hash',
    ];

    public function albumable()
    {
        return $this->morphTo();
    }
}
