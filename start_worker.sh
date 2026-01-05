pkill -f 'php artisan queue:work'
php artisan queue:work --tries=3 --timeout=60 --sleep=3 --queue=default &
php artisan queue:work --tries=3 --timeout=60 --sleep=3 --queue=high,low &
php artisan queue:work --tries=3 --timeout=60 --sleep=3 --queue=rank_report_history &
php artisan queue:work --tries=3 --timeout=60 --sleep=3 --queue=game_room &
