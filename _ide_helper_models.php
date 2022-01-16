<?php

// @formatter:off
/**
 * A helper file for your Eloquent Models
 * Copy the phpDocs from this file to the correct Model,
 * And remove them from this file, to prevent double declarations.
 *
 * @author Barry vd. Heuvel <barryvdh@gmail.com>
 */


namespace App\Models{
/**
 * App\Models\Element
 *
 * @mixin IdeHelperElement
 * @property int $id
 * @property string|null $path
 * @property string|null $source_url
 * @property string|null $thumb_url
 * @property string|null $title
 * @property string $type
 * @property string|null $video_source
 * @property string|null $video_id
 * @property int|null $video_duration_second
 * @property int|null $video_start_second
 * @property int|null $video_end_second
 * @property \Illuminate\Support\Carbon|null $created_at
 * @property \Illuminate\Support\Carbon|null $updated_at
 * @property \Illuminate\Support\Carbon|null $deleted_at
 * @property-read \Illuminate\Database\Eloquent\Collection|\App\Models\Post[] $posts
 * @property-read int|null $posts_count
 * @method static \Database\Factories\ElementFactory factory(...$parameters)
 * @method static \Illuminate\Database\Eloquent\Builder|Element newModelQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|Element newQuery()
 * @method static \Illuminate\Database\Query\Builder|Element onlyTrashed()
 * @method static \Illuminate\Database\Eloquent\Builder|Element query()
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereCreatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereDeletedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element wherePath($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereSourceUrl($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereThumbUrl($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereTitle($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereType($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereUpdatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereVideoDurationSecond($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereVideoEndSecond($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereVideoId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereVideoSource($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Element whereVideoStartSecond($value)
 * @method static \Illuminate\Database\Query\Builder|Element withTrashed()
 * @method static \Illuminate\Database\Query\Builder|Element withoutTrashed()
 */
	class IdeHelperElement extends \Eloquent {}
}

namespace App\Models{
/**
 * App\Models\Game
 *
 * @mixin IdeHelperGame
 * @property int $id
 * @property string $serial
 * @property int $post_id
 * @property int $element_count
 * @property \Illuminate\Support\Carbon|null $created_at
 * @property \Illuminate\Support\Carbon|null $updated_at
 * @property-read \Illuminate\Database\Eloquent\Collection|\App\Models\Element[] $elements
 * @property-read int|null $elements_count
 * @property-read \Illuminate\Database\Eloquent\Collection|\App\Models\Game1V1Round[] $game_1v1_rounds
 * @property-read int|null $game_1v1_rounds_count
 * @property-read \App\Models\Post $post
 * @method static \Database\Factories\GameFactory factory(...$parameters)
 * @method static \Illuminate\Database\Eloquent\Builder|Game newModelQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|Game newQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|Game query()
 * @method static \Illuminate\Database\Eloquent\Builder|Game whereCreatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game whereElementCount($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game whereId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game wherePostId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game whereSerial($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game whereUpdatedAt($value)
 */
	class IdeHelperGame extends \Eloquent {}
}

namespace App\Models{
/**
 * App\Models\Game1V1Round
 *
 * @mixin IdeHelperGame1V1Round
 * @property int $id
 * @property int $game_id
 * @property int $current_round
 * @property int $of_round
 * @property int $remain_elements
 * @property int|null $winner_id
 * @property int|null $loser_id
 * @property string|null $complete_at
 * @property \Illuminate\Support\Carbon|null $created_at
 * @property \Illuminate\Support\Carbon|null $updated_at
 * @property-read \App\Models\Game $game
 * @property-read \App\Models\Element|null $loser
 * @property-read \App\Models\Element|null $winner
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round newModelQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round newQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round query()
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round whereCompleteAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round whereCreatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round whereCurrentRound($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round whereGameId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round whereId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round whereLoserId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round whereOfRound($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round whereRemainElements($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round whereUpdatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Game1V1Round whereWinnerId($value)
 */
	class IdeHelperGame1V1Round extends \Eloquent {}
}

namespace App\Models{
/**
 * App\Models\GameElement
 *
 * @mixin IdeHelperGameElement
 * @property int $id
 * @property int $game_id
 * @property int $element_id
 * @property int $win_count
 * @property int $is_eliminated
 * @property-read \App\Models\Element $element
 * @property-read \App\Models\Game $game
 * @method static \Illuminate\Database\Eloquent\Builder|GameElement newModelQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|GameElement newQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|GameElement query()
 * @method static \Illuminate\Database\Eloquent\Builder|GameElement whereElementId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|GameElement whereGameId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|GameElement whereId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|GameElement whereIsEliminated($value)
 * @method static \Illuminate\Database\Eloquent\Builder|GameElement whereWinCount($value)
 */
	class IdeHelperGameElement extends \Eloquent {}
}

namespace App\Models{
/**
 * App\Models\Post
 *
 * @mixin IdeHelperPost
 * @property int $id
 * @property int|null $user_id
 * @property string $serial
 * @property string|null $title
 * @property string|null $description
 * @property \Illuminate\Support\Carbon|null $created_at
 * @property \Illuminate\Support\Carbon|null $updated_at
 * @property \Illuminate\Support\Carbon|null $deleted_at
 * @property-read \Illuminate\Database\Eloquent\Collection|\App\Models\Element[] $elements
 * @property-read int|null $elements_count
 * @property-read \Illuminate\Database\Eloquent\Collection|\App\Models\Game[] $games
 * @property-read int|null $games_count
 * @property-read \App\Models\PostPolicy|null $post_policy
 * @property-read \Illuminate\Database\Eloquent\Collection|\App\Models\PostTrend[] $post_trends
 * @property-read int|null $post_trends_count
 * @property-read \Illuminate\Database\Eloquent\Collection|\App\Models\RankReport[] $rank_reports
 * @property-read int|null $rank_reports_count
 * @property-read \Illuminate\Database\Eloquent\Collection|\App\Models\Rank[] $ranks
 * @property-read int|null $ranks_count
 * @property-read \App\Models\User|null $user
 * @method static \Database\Factories\PostFactory factory(...$parameters)
 * @method static \Illuminate\Database\Eloquent\Builder|Post newModelQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|Post newQuery()
 * @method static \Illuminate\Database\Query\Builder|Post onlyTrashed()
 * @method static \Illuminate\Database\Eloquent\Builder|Post public()
 * @method static \Illuminate\Database\Eloquent\Builder|Post query()
 * @method static \Illuminate\Database\Eloquent\Builder|Post whereCreatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Post whereDeletedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Post whereDescription($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Post whereId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Post whereSerial($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Post whereTitle($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Post whereUpdatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Post whereUserId($value)
 * @method static \Illuminate\Database\Query\Builder|Post withTrashed()
 * @method static \Illuminate\Database\Query\Builder|Post withoutTrashed()
 */
	class IdeHelperPost extends \Eloquent {}
}

namespace App\Models{
/**
 * App\Models\PostPolicy
 *
 * @mixin IdeHelperPostPolicy
 * @property int $id
 * @property int $post_id
 * @property string $access_policy
 * @property string|null $password
 * @property \Illuminate\Support\Carbon|null $created_at
 * @property \Illuminate\Support\Carbon|null $updated_at
 * @property-read \App\Models\Post $post
 * @method static \Database\Factories\PostPolicyFactory factory(...$parameters)
 * @method static \Illuminate\Database\Eloquent\Builder|PostPolicy newModelQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|PostPolicy newQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|PostPolicy query()
 * @method static \Illuminate\Database\Eloquent\Builder|PostPolicy whereAccessPolicy($value)
 * @method static \Illuminate\Database\Eloquent\Builder|PostPolicy whereCreatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|PostPolicy whereId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|PostPolicy wherePassword($value)
 * @method static \Illuminate\Database\Eloquent\Builder|PostPolicy wherePostId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|PostPolicy whereUpdatedAt($value)
 */
	class IdeHelperPostPolicy extends \Eloquent {}
}

namespace App\Models{
/**
 * App\Models\PostTrend
 *
 * @property int $id
 * @property int $post_id
 * @property string $trend_type
 * @property string $time_range
 * @property string|null $start_date
 * @property int $position
 * @property \Illuminate\Support\Carbon|null $created_at
 * @property \Illuminate\Support\Carbon|null $updated_at
 * @property-read \App\Models\Post $post
 * @method static \Illuminate\Database\Eloquent\Builder|PostTrend newModelQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|PostTrend newQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|PostTrend query()
 * @method static \Illuminate\Database\Eloquent\Builder|PostTrend whereCreatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|PostTrend whereId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|PostTrend wherePosition($value)
 * @method static \Illuminate\Database\Eloquent\Builder|PostTrend wherePostId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|PostTrend whereStartDate($value)
 * @method static \Illuminate\Database\Eloquent\Builder|PostTrend whereTimeRange($value)
 * @method static \Illuminate\Database\Eloquent\Builder|PostTrend whereTrendType($value)
 * @method static \Illuminate\Database\Eloquent\Builder|PostTrend whereUpdatedAt($value)
 * @mixin \Eloquent
 */
	class IdeHelperPostTrend extends \Eloquent {}
}

namespace App\Models{
/**
 * App\Models\Rank
 *
 * @mixin IdeHelperRank
 * @property int $id
 * @property int $post_id
 * @property int $element_id
 * @property string $rank_type
 * @property string $record_date
 * @property int|null $position
 * @property int $win_count
 * @property int $round_count
 * @property string $win_rate
 * @property \Illuminate\Support\Carbon|null $created_at
 * @property \Illuminate\Support\Carbon|null $updated_at
 * @property-read \App\Models\Element $element
 * @property-read \App\Models\Post $post
 * @method static \Illuminate\Database\Eloquent\Builder|Rank newModelQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|Rank newQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|Rank query()
 * @method static \Illuminate\Database\Eloquent\Builder|Rank whereCreatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Rank whereElementId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Rank whereId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Rank wherePosition($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Rank wherePostId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Rank whereRankType($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Rank whereRecordDate($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Rank whereRoundCount($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Rank whereUpdatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Rank whereWinCount($value)
 * @method static \Illuminate\Database\Eloquent\Builder|Rank whereWinRate($value)
 */
	class IdeHelperRank extends \Eloquent {}
}

namespace App\Models{
/**
 * App\Models\RankReport
 *
 * @mixin IdeHelperRankReport
 * @property int $id
 * @property int $post_id
 * @property int $element_id
 * @property int|null $rank
 * @property int|null $final_win_position
 * @property string|null $final_win_rate
 * @property int|null $win_position
 * @property string|null $win_rate
 * @property \Illuminate\Support\Carbon|null $created_at
 * @property \Illuminate\Support\Carbon|null $updated_at
 * @property-read \App\Models\Element $element
 * @property-read \App\Models\Post $post
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport newModelQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport newQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport query()
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport whereCreatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport whereElementId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport whereFinalWinPosition($value)
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport whereFinalWinRate($value)
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport whereId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport wherePostId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport whereRank($value)
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport whereUpdatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport whereWinPosition($value)
 * @method static \Illuminate\Database\Eloquent\Builder|RankReport whereWinRate($value)
 */
	class IdeHelperRankReport extends \Eloquent {}
}

namespace App\Models{
/**
 * App\Models\User
 *
 * @mixin IdeHelperUser
 * @property int $id
 * @property string $name
 * @property string $email
 * @property \Illuminate\Support\Carbon|null $email_verified_at
 * @property string $password
 * @property string|null $remember_token
 * @property \Illuminate\Support\Carbon|null $created_at
 * @property \Illuminate\Support\Carbon|null $updated_at
 * @property-read \Illuminate\Notifications\DatabaseNotificationCollection|\Illuminate\Notifications\DatabaseNotification[] $notifications
 * @property-read int|null $notifications_count
 * @property-read \Illuminate\Database\Eloquent\Collection|\App\Models\Post[] $posts
 * @property-read int|null $posts_count
 * @property-read \Illuminate\Database\Eloquent\Collection|\Laravel\Sanctum\PersonalAccessToken[] $tokens
 * @property-read int|null $tokens_count
 * @method static \Database\Factories\UserFactory factory(...$parameters)
 * @method static \Illuminate\Database\Eloquent\Builder|User newModelQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|User newQuery()
 * @method static \Illuminate\Database\Eloquent\Builder|User query()
 * @method static \Illuminate\Database\Eloquent\Builder|User whereCreatedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|User whereEmail($value)
 * @method static \Illuminate\Database\Eloquent\Builder|User whereEmailVerifiedAt($value)
 * @method static \Illuminate\Database\Eloquent\Builder|User whereId($value)
 * @method static \Illuminate\Database\Eloquent\Builder|User whereName($value)
 * @method static \Illuminate\Database\Eloquent\Builder|User wherePassword($value)
 * @method static \Illuminate\Database\Eloquent\Builder|User whereRememberToken($value)
 * @method static \Illuminate\Database\Eloquent\Builder|User whereUpdatedAt($value)
 */
	class IdeHelperUser extends \Eloquent {}
}

