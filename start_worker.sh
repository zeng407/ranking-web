pkill -f 'php artisan queue:listen'
php artisan queue:listen --tries=3 --timeout=60 --sleep=3 --queue=default &
php artisan queue:listen --tries=3 --timeout=60 --sleep=3 --queue=high,low &
php artisan queue:listen --tries=3 --timeout=60 --sleep=3 --queue=rank_report_history &
php artisan queue:listen --tries=3 --timeout=60 --sleep=3 --queue=game_room &
