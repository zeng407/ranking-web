# pre-install
- [docker](https://www.docker.com/) - Docker is an open platform for developing, shipping, and running applications. It allows you to package your application and its dependencies into a standardized unit called a container, which can be deployed on any operating system.
- [laravel hard](https://herd.laravel.com/) - One click PHP development environment.

# Build for development
1. `composer install` (you may encounter missing dependencies, in that case, use the `--ignore-platform-reqs` flag)
2. `php artisan sail:install` for mysql, minio, redis, selenium
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


# Troubleshooting

## You may encounter permission issues when using Docker. Make sure your user is in the Docker group.
`sudo usermod -aG docker $USER`