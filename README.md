# pre-install
- [docker](https://www.docker.com/) - Docker is an open platform for developing, shipping, and running applications. It allows you to package your application and its dependencies into a standardized unit called a container, which can be deployed on any operating system.
- [laravel hard](https://herd.laravel.com/) - One click PHP development environment.

# Build for development
1. `composer install` (you may encounter missing dependencies, in that case, use the `--ignore-platform-reqs` flag)
2. `php artisan sail:install` for mysql, minio, redis
3. visit `localhost:9000` to login to your Minio account (see [docker-compose.yml](docker-compose.yml)). You need to create a bucket and generate an access key. Then, put the access key and other Minio configurations in your .env file.
4. run `./vendor/bin/sail up -d` to set up services
5. run `./vendor/bin/sail migrate` to set up database
6. run `./vendor/bin/sail npm install & npm run dev` to set up static html/css/js file

### sample .env
```
# minio configuration example
AWS_ACCESS_KEY_ID=xxxxxxxxxxxxx
AWS_SECRET_ACCESS_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxx
AWS_DEFAULT_REGION=us-east-1
AWS_BUCKET=xxxxxx
AWS_URL=http://localhost:9000/xxxxxx
AWS_ENDPOINT=http://ranking-web-minio-1:9000
AWS_USE_PATH_STYLE_ENDPOINT=true
```

# Build for production
1. `composer install --no-dev` (you may encounter missing dependencies, in that case, use the `--ignore-platform-reqs` flag)
2. `git submodule init` and `git submodule update` for fetching laradock repository
3. go to the laradock directory and `cp .env.example .env`
4. modify the configurations of the services you need to use, such as `mysql`, `php`, `redis`, `supervisor`, `scheduler`, `minio`, `caddy`
5. (optional) modify caddy/caddy/Caddyfile
6. (optional) modify php-worker/supervisor.d/laravel-worker.conf

- Caddyfile
```
2pick.app {
        root * /var/www/public
        php_fastcgi php-fpm:9000
        file_server

        encode zstd gzip
        log /var/log/caddy/access.log

        tls 2pick.app@gmail.com
}

file.2pick.app {
         reverse_proxy minio:9000
}
```

- laravel-worker.conf
```
[program:laravel-worker]
process_name=%(program_name)s_%(process_num)02d
command=php /var/www/artisan queue:work redis --sleep=3 --tries=3
autostart=true
autorestart=true
numprocs=4
user=laradock
redirect_stderr=true
```

# Troubleshooting

## You may encounter permission issues when using Docker. Make sure your user is in the Docker group.
`sudo usermod -aG docker $USER`