-- Baseline of the schema, 37 tables.
--
-- SOURCE: `mysqldump --no-data --single-transaction` of
-- `rk_db_restore_20260729` on 2026-08-05. That is the primary database; `rk-db`
-- is legacy and must NOT be used to regenerate this file.
--
-- An earlier revision of this baseline was taken from `rk-db`. The two databases
-- have identical columns, but NOT identical indexes: the restore carries eight
-- indexes `rk-db` lacks, two of them UNIQUE constraints.
--
--   game_1v1_rounds  idx_rounds_game_loser        (game_id, loser_id)
--   game_1v1_rounds  idx_rounds_game_winner       (game_id, winner_id)
--   game_elements    ge_game_element              (game_id, element_id)
--   games            idx_games_post_completed_id  (post_id, completed_at, id)
--   post_statistics  post_statistics_unique_index (post_id, start_date, time_range)  UNIQUE
--   rank_reports     rank_reports_hidden_index    (hidden)
--   rank_reports     unique_post_element          (post_id, element_id)              UNIQUE
--   ranks            ranks_post_record_date_idx   (post_id, record_date)
--
-- A baseline built from `rk-db` therefore silently dropped two uniqueness
-- guarantees and the composite indexes the ranking queries depend on. Compare
-- indexes, not just columns, when validating a schema source.
--
-- Purpose: give a fresh database (CI, a new RDS instance) the same starting point
-- an established database already has. An established database must NOT run this;
-- stamp it with `migrate baseline`, which records the version as applied without
-- executing it.
--
-- Excluded on purpose: `go_schema_migrations` (goose creates it) and
-- `rank_dedupe_staging` plus `ranks_post_element_type_date_unique`, which
-- migrations 00002 and 00003 create. Including them here would make 00003 fail on
-- a fresh database.
--
-- Goose records applied versions in `go_schema_migrations`. Laravel keeps its own
-- `migrations` table, which this tool never reads or writes: that table disagrees
-- with the repository in both environments measured (68 rows against 66 files in
-- the restore, 61 against 66 locally after removing one orphan), so it cannot be
-- a source of truth.
--
-- Down is intentionally empty. Unwinding a baseline means restoring a backup, not
-- dropping 37 tables in production.

-- +goose Up
-- +goose NO TRANSACTION
-- DDL in MySQL is not transactional, so a partial failure leaves the database
-- partly built. That is acceptable only because this runs against an empty one.
SET FOREIGN_KEY_CHECKS = 0;

CREATE TABLE `comments` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned DEFAULT NULL,
  `anonymous_id` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `nickname` varchar(30) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `content` varchar(512) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `label` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `ip` varchar(45) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `anonymous_mode` tinyint(1) NOT NULL DEFAULT '0',
  `delete_hash` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `edited_at` timestamp NULL DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `comments_user_id_foreign` (`user_id`),
  KEY `comments_anonymous_id_index` (`anonymous_id`),
  CONSTRAINT `comments_user_id_foreign` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=21447 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `element_issues` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `element_id` bigint unsigned NOT NULL,
  `type` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `resolved_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `element_issues_element_id_foreign` (`element_id`),
  CONSTRAINT `element_issues_element_id_foreign` FOREIGN KEY (`element_id`) REFERENCES `elements` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=572 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `elements` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `path` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `source_url` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `thumb_url` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `mediumthumb_url` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `lowthumb_url` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `title` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `type` enum('image','video') CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `video_source` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `video_id` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `video_duration_second` int unsigned DEFAULT NULL,
  `video_start_second` int unsigned DEFAULT NULL,
  `video_end_second` int unsigned DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=427812 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `failed_jobs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `uuid` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `connection` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `queue` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `payload` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `exception` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `failed_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `failed_jobs_uuid_unique` (`uuid`)
) ENGINE=InnoDB AUTO_INCREMENT=3045 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `game_1v1_rounds` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `game_id` bigint unsigned NOT NULL,
  `current_round` int unsigned NOT NULL,
  `of_round` int unsigned NOT NULL,
  `remain_elements` int unsigned NOT NULL,
  `winner_id` bigint unsigned DEFAULT NULL,
  `loser_id` bigint unsigned DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `game_1v1_rounds_game_id_foreign` (`game_id`),
  KEY `game_1v1_rounds_winner_id_foreign` (`winner_id`),
  KEY `game_1v1_rounds_loser_id_foreign` (`loser_id`),
  KEY `idx_rounds_game_winner` (`game_id`,`winner_id`),
  KEY `idx_rounds_game_loser` (`game_id`,`loser_id`),
  CONSTRAINT `game_1v1_rounds_game_id_foreign` FOREIGN KEY (`game_id`) REFERENCES `games` (`id`),
  CONSTRAINT `game_1v1_rounds_loser_id_foreign` FOREIGN KEY (`loser_id`) REFERENCES `elements` (`id`),
  CONSTRAINT `game_1v1_rounds_winner_id_foreign` FOREIGN KEY (`winner_id`) REFERENCES `elements` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=50701786 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `game_elements` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `game_id` bigint unsigned NOT NULL,
  `element_id` bigint unsigned NOT NULL,
  `win_count` int unsigned NOT NULL DEFAULT '0',
  `is_eliminated` tinyint(1) NOT NULL DEFAULT '0',
  `is_ready` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `game_elements_game_id_foreign` (`game_id`),
  KEY `game_elements_element_id_foreign` (`element_id`),
  KEY `ge_game_element` (`game_id`,`element_id`),
  CONSTRAINT `game_elements_element_id_foreign` FOREIGN KEY (`element_id`) REFERENCES `elements` (`id`),
  CONSTRAINT `game_elements_game_id_foreign` FOREIGN KEY (`game_id`) REFERENCES `games` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=126818492 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `game_room_user_bets` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `game_room_id` bigint unsigned NOT NULL,
  `game_room_user_id` bigint unsigned NOT NULL,
  `current_round` int unsigned NOT NULL,
  `of_round` int unsigned NOT NULL,
  `remain_elements` int unsigned NOT NULL,
  `winner_id` bigint unsigned NOT NULL,
  `loser_id` bigint unsigned NOT NULL,
  `won_at` timestamp NULL DEFAULT NULL,
  `lost_at` timestamp NULL DEFAULT NULL,
  `last_combo` smallint unsigned NOT NULL DEFAULT '0',
  `score` smallint NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `game_room_user_bets_game_room_id_foreign` (`game_room_id`),
  KEY `game_room_user_bets_game_room_user_id_foreign` (`game_room_user_id`),
  KEY `game_room_user_bets_winner_id_foreign` (`winner_id`),
  KEY `game_room_user_bets_loser_id_foreign` (`loser_id`),
  CONSTRAINT `game_room_user_bets_game_room_id_foreign` FOREIGN KEY (`game_room_id`) REFERENCES `game_rooms` (`id`) ON DELETE CASCADE,
  CONSTRAINT `game_room_user_bets_game_room_user_id_foreign` FOREIGN KEY (`game_room_user_id`) REFERENCES `game_room_users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `game_room_user_bets_loser_id_foreign` FOREIGN KEY (`loser_id`) REFERENCES `elements` (`id`) ON DELETE CASCADE,
  CONSTRAINT `game_room_user_bets_winner_id_foreign` FOREIGN KEY (`winner_id`) REFERENCES `elements` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=1027569 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `game_room_users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `game_room_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned DEFAULT NULL,
  `anonymous_id` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `nickname` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `score` int NOT NULL DEFAULT '0',
  `rank` int unsigned NOT NULL DEFAULT '0',
  `accuracy` decimal(5,2) NOT NULL DEFAULT '0.00',
  `combo` smallint NOT NULL DEFAULT '0',
  `total_played` int unsigned NOT NULL DEFAULT '0',
  `total_correct` int unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `game_room_users_game_room_id_foreign` (`game_room_id`),
  KEY `game_room_users_user_id_foreign` (`user_id`),
  KEY `game_room_users_anonymous_id_index` (`anonymous_id`),
  CONSTRAINT `game_room_users_game_room_id_foreign` FOREIGN KEY (`game_room_id`) REFERENCES `game_rooms` (`id`) ON DELETE CASCADE,
  CONSTRAINT `game_room_users_user_id_foreign` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=30055 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `game_rooms` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `game_id` bigint unsigned NOT NULL,
  `serial` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `game_rooms_serial_unique` (`serial`),
  KEY `game_rooms_game_id_foreign` (`game_id`),
  CONSTRAINT `game_rooms_game_id_foreign` FOREIGN KEY (`game_id`) REFERENCES `games` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=15210 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `games` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `serial` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `user_id` bigint unsigned DEFAULT NULL,
  `post_id` bigint unsigned NOT NULL,
  `element_count` int unsigned NOT NULL,
  `vote_count` int NOT NULL DEFAULT '0',
  `candidates` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `completed_at` timestamp NULL DEFAULT NULL,
  `ip` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `ip_country` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `games_serial_unique` (`serial`),
  KEY `games_post_id_foreign` (`post_id`),
  KEY `games_user_id_foreign` (`user_id`),
  KEY `idx_games_post_completed_id` (`post_id`,`completed_at`,`id`),
  CONSTRAINT `games_post_id_foreign` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`),
  CONSTRAINT `games_user_id_foreign` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB AUTO_INCREMENT=959023 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `home_carousel_items` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `position` int unsigned NOT NULL,
  `type` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `title` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `description` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `image_url` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `video_url` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `video_source` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `video_id` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `video_start_second` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `video_end_second` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `is_active` tinyint(1) NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=52 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `imgur_albums` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `albumable_type` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `albumable_id` bigint unsigned NOT NULL,
  `album_id` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `title` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `description` varchar(300) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `delete_hash` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `imgur_albums_albumable_type_albumable_id_index` (`albumable_type`,`albumable_id`)
) ENGINE=InnoDB AUTO_INCREMENT=6200 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `imgur_images` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `imageable_type` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `imageable_id` bigint unsigned NOT NULL,
  `image_id` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `imgur_album_id` bigint unsigned DEFAULT NULL,
  `title` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `description` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `delete_hash` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `link` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `imgur_images_imageable_type_imageable_id_index` (`imageable_type`,`imageable_id`),
  KEY `imgur_images_imgur_album_id_foreign` (`imgur_album_id`),
  CONSTRAINT `imgur_images_imgur_album_id_foreign` FOREIGN KEY (`imgur_album_id`) REFERENCES `imgur_albums` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=144586 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `migrations` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `migration` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `batch` int NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=83 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `password_resets` (
  `email` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `token` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  KEY `password_resets_email_index` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `personal_access_tokens` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tokenable_type` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `tokenable_id` bigint unsigned NOT NULL,
  `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `token` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `abilities` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `last_used_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `personal_access_tokens_token_unique` (`token`),
  KEY `personal_access_tokens_tokenable_type_tokenable_id_index` (`tokenable_type`,`tokenable_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `post_comments` (
  `post_id` bigint unsigned NOT NULL,
  `comment_id` bigint unsigned NOT NULL,
  PRIMARY KEY (`post_id`,`comment_id`),
  KEY `post_comments_comment_id_foreign` (`comment_id`),
  CONSTRAINT `post_comments_comment_id_foreign` FOREIGN KEY (`comment_id`) REFERENCES `comments` (`id`) ON DELETE CASCADE,
  CONSTRAINT `post_comments_post_id_foreign` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `post_elements` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `post_id` bigint unsigned NOT NULL,
  `element_id` bigint unsigned NOT NULL,
  PRIMARY KEY (`id`),
  KEY `post_elements_post_id_foreign` (`post_id`),
  KEY `post_elements_element_id_foreign` (`element_id`),
  CONSTRAINT `post_elements_element_id_foreign` FOREIGN KEY (`element_id`) REFERENCES `elements` (`id`),
  CONSTRAINT `post_elements_post_id_foreign` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=427813 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `post_policies` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `post_id` bigint unsigned NOT NULL,
  `access_policy` enum('private','public','password') CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'private',
  `password` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `post_policies_post_id_foreign` (`post_id`),
  CONSTRAINT `post_policies_post_id_foreign` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=6202 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `post_statistics` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `post_id` bigint unsigned NOT NULL,
  `time_range` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `start_date` date NOT NULL,
  `play_count` int unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `post_statistics_unique_index` (`post_id`,`start_date`,`time_range`),
  KEY `post_statistics_post_id_foreign` (`post_id`),
  KEY `post_statistics_start_date_index` (`start_date`),
  KEY `post_statistics_play_count_index` (`play_count`),
  CONSTRAINT `post_statistics_post_id_foreign` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=2422657 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `post_tags` (
  `post_id` bigint unsigned NOT NULL,
  `tag_id` bigint unsigned NOT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`post_id`,`tag_id`),
  KEY `post_tags_tag_id_foreign` (`tag_id`),
  CONSTRAINT `post_tags_post_id_foreign` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`) ON DELETE CASCADE,
  CONSTRAINT `post_tags_tag_id_foreign` FOREIGN KEY (`tag_id`) REFERENCES `tags` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `post_trends` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `post_id` bigint unsigned NOT NULL,
  `trend_type` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `time_range` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `start_date` date DEFAULT NULL,
  `position` int unsigned NOT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `post_trends_post_id_foreign` (`post_id`),
  KEY `post_trends_trend_type_index` (`trend_type`),
  KEY `post_trends_time_range_index` (`time_range`),
  KEY `post_trends_start_date_index` (`start_date`),
  KEY `post_trends_position_index` (`position`),
  CONSTRAINT `post_trends_post_id_foreign` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=974029 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `posts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned DEFAULT NULL,
  `serial` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `title` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `description` varchar(300) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `is_censored` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `posts_serial_unique` (`serial`),
  KEY `posts_user_id_foreign` (`user_id`),
  KEY `posts_title_index` (`title`),
  KEY `posts_description_index` (`description`),
  CONSTRAINT `posts_user_id_foreign` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=6202 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `public_posts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `post_id` bigint unsigned NOT NULL,
  `new_position` int NOT NULL DEFAULT '9999',
  `day_position` int NOT NULL DEFAULT '9999',
  `week_position` int NOT NULL DEFAULT '9999',
  `month_position` int NOT NULL DEFAULT '9999',
  `title` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `description` varchar(300) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `tags` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `data` json DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  `is_dirty` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `public_posts_post_id_foreign` (`post_id`),
  KEY `public_posts_new_position_index` (`new_position`),
  KEY `public_posts_day_position_index` (`day_position`),
  KEY `public_posts_week_position_index` (`week_position`),
  KEY `public_posts_month_position_index` (`month_position`),
  KEY `public_posts_title_index` (`title`),
  KEY `public_posts_description_index` (`description`),
  KEY `public_posts_tags_index` (`tags`),
  CONSTRAINT `public_posts_post_id_foreign` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=3257 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `rank_report_histories` (
  `id` int NOT NULL AUTO_INCREMENT,
  `element_id` bigint unsigned NOT NULL,
  `post_id` bigint unsigned NOT NULL,
  `rank_report_id` bigint unsigned NOT NULL,
  `time_range` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `start_date` date NOT NULL,
  `rank` int unsigned NOT NULL,
  `win_count` int unsigned NOT NULL,
  `lose_count` int unsigned NOT NULL,
  `win_rate` decimal(5,2) NOT NULL,
  `champion_count` int unsigned NOT NULL DEFAULT '0',
  `game_complete_count` int unsigned NOT NULL DEFAULT '0',
  `champion_rate` decimal(8,2) NOT NULL DEFAULT '0.00',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `rank_report_histories_element_id_foreign` (`element_id`),
  KEY `rank_report_histories_post_id_foreign` (`post_id`),
  KEY `rank_report_histories_rank_report_id_foreign` (`rank_report_id`),
  KEY `rank_report_histories_start_date_index` (`start_date`)
) ENGINE=InnoDB AUTO_INCREMENT=67626490 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `rank_reports` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `post_id` bigint unsigned NOT NULL,
  `element_id` bigint unsigned NOT NULL,
  `rank` int unsigned DEFAULT NULL,
  `final_win_position` bigint unsigned DEFAULT NULL,
  `final_win_rate` decimal(5,2) DEFAULT NULL,
  `win_position` bigint unsigned DEFAULT NULL,
  `win_rate` decimal(5,2) DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  `hidden` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `unique_post_element` (`post_id`,`element_id`),
  KEY `rank_reports_post_id_foreign` (`post_id`),
  KEY `rank_reports_element_id_foreign` (`element_id`),
  KEY `rank_reports_hidden_index` (`hidden`),
  CONSTRAINT `rank_reports_element_id_foreign` FOREIGN KEY (`element_id`) REFERENCES `elements` (`id`),
  CONSTRAINT `rank_reports_post_id_foreign` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=65260499 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `ranks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `post_id` bigint unsigned NOT NULL,
  `element_id` bigint unsigned NOT NULL,
  `rank_type` enum('pk_king','champion') CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `record_date` date NOT NULL,
  `win_count` bigint unsigned NOT NULL DEFAULT '0',
  `round_count` bigint unsigned NOT NULL DEFAULT '0',
  `win_rate` decimal(5,2) NOT NULL DEFAULT '0.00',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ranks_post_id_foreign` (`post_id`),
  KEY `ranks_element_id_foreign` (`element_id`),
  KEY `ranks_post_record_date_idx` (`post_id`,`record_date`),
  CONSTRAINT `ranks_element_id_foreign` FOREIGN KEY (`element_id`) REFERENCES `elements` (`id`),
  CONSTRAINT `ranks_post_id_foreign` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=31412657 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `reported_comments` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `comment_id` bigint unsigned NOT NULL,
  `reporter_id` bigint unsigned DEFAULT NULL,
  `reporter_ip` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `comment_content` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `reason` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `comment_abuse_reports_comment_id_foreign` (`comment_id`),
  KEY `comment_abuse_reports_reporter_id_foreign` (`reporter_id`),
  CONSTRAINT `comment_abuse_reports_comment_id_foreign` FOREIGN KEY (`comment_id`) REFERENCES `comments` (`id`) ON DELETE CASCADE,
  CONSTRAINT `comment_abuse_reports_reporter_id_foreign` FOREIGN KEY (`reporter_id`) REFERENCES `users` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB AUTO_INCREMENT=1465 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `roles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `slug` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `description` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `roles_slug_unique` (`slug`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `tags` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(15) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `tags_name_unique` (`name`)
) ENGINE=InnoDB AUTO_INCREMENT=1524 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `telescope_entries` (
  `sequence` bigint unsigned NOT NULL AUTO_INCREMENT,
  `uuid` char(36) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `batch_id` char(36) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `family_hash` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `should_display_on_index` tinyint(1) NOT NULL DEFAULT '1',
  `type` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `content` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`sequence`),
  UNIQUE KEY `telescope_entries_uuid_unique` (`uuid`),
  KEY `telescope_entries_batch_id_index` (`batch_id`),
  KEY `telescope_entries_family_hash_index` (`family_hash`),
  KEY `telescope_entries_created_at_index` (`created_at`),
  KEY `telescope_entries_type_should_display_on_index_index` (`type`,`should_display_on_index`)
) ENGINE=InnoDB AUTO_INCREMENT=176788 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `telescope_entries_tags` (
  `entry_uuid` char(36) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `tag` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (`entry_uuid`,`tag`),
  KEY `telescope_entries_tags_tag_index` (`tag`),
  CONSTRAINT `telescope_entries_tags_entry_uuid_foreign` FOREIGN KEY (`entry_uuid`) REFERENCES `telescope_entries` (`uuid`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `telescope_monitoring` (
  `tag` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (`tag`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `user_game_results` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned DEFAULT NULL,
  `anonymous_id` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `game_id` bigint unsigned NOT NULL,
  `champion_id` bigint unsigned NOT NULL,
  `loser_id` bigint unsigned DEFAULT NULL,
  `loser_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `champion_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `candidates` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `user_game_results_user_id_foreign` (`user_id`),
  KEY `user_game_results_game_id_foreign` (`game_id`),
  KEY `user_game_results_champion_id_foreign` (`champion_id`),
  KEY `user_game_results_anonymous_id_index` (`anonymous_id`),
  KEY `user_game_results_loser_id_foreign` (`loser_id`),
  CONSTRAINT `user_game_results_champion_id_foreign` FOREIGN KEY (`champion_id`) REFERENCES `elements` (`id`) ON DELETE CASCADE,
  CONSTRAINT `user_game_results_game_id_foreign` FOREIGN KEY (`game_id`) REFERENCES `games` (`id`) ON DELETE CASCADE,
  CONSTRAINT `user_game_results_loser_id_foreign` FOREIGN KEY (`loser_id`) REFERENCES `elements` (`id`) ON DELETE SET NULL,
  CONSTRAINT `user_game_results_user_id_foreign` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=990366 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `user_roles` (
  `user_id` bigint unsigned NOT NULL,
  `role_id` bigint unsigned NOT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`user_id`,`role_id`),
  UNIQUE KEY `user_roles_user_id_role_id_unique` (`user_id`,`role_id`),
  KEY `user_roles_role_id_foreign` (`role_id`),
  CONSTRAINT `user_roles_role_id_foreign` FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`) ON DELETE CASCADE,
  CONSTRAINT `user_roles_user_id_foreign` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `user_socialities` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `google_email` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `google_id` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `google_token` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `google_refresh_token` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `user_socialities_user_id_foreign` (`user_id`),
  KEY `user_socialities_google_id_index` (`google_id`),
  CONSTRAINT `user_socialities_user_id_foreign` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=11305 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `name_updated_at` timestamp NULL DEFAULT NULL,
  `email` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `email_verified_at` timestamp NULL DEFAULT NULL,
  `avatar_url` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `password` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `remember_token` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `users_email_unique` (`email`)
) ENGINE=InnoDB AUTO_INCREMENT=13399 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET FOREIGN_KEY_CHECKS = 1;

-- +goose Down
-- Deliberately empty. See the note above.
