<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

/**
 * @mixin IdeHelperUserRole
 */
class UserRole extends Model
{
    use HasFactory;

    protected $fillable = ['is_active'];
}
