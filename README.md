# Tools
- [Docker](https://www.docker.com/) - Docker is an open platform for developing, shipping, and running applications. It allows you to package your application and its dependencies into a standardized unit called a container, which can be deployed on any operating system.
- PHP 8.2
- Composer 2.7

# Install

1. Install PHP 8.2

```
# Update package lists
sudo apt update

# Install necessary packages
sudo apt install -y software-properties-common

# Add ondrej/php which has PHP 8.2
sudo add-apt-repository ppa:ondrej/php

# Update package lists
sudo apt update

# Install PHP 8.2
sudo apt install -y php8.2
```

2. Install Composer 2.7

```
# Download Composer installer
php -r "copy('https://getcomposer.org/installer', 'composer-setup.php');"

# Install Composer
php composer-setup.php --install-dir=/usr/local/bin --filename=composer --version=2.7.0

# Remove installer
php -r "unlink('composer-setup.php');"
```

# Build for Development
1. Run `composer install`. If you encounter missing dependencies, use the `--ignore-platform-reqs` flag.
2. Run `php artisan sail:install` and select (1)mysql, (2)minio, (3)redis, (4)selenium (5)soketi.
3. Visit `localhost:9000` to log in to your Minio account (see [docker-compose.yml](docker-compose.yml)). You need to create a bucket and generate an access key. Then, put the access key and other Minio configurations in your .env file. (See the following example .env)
4. Run `./vendor/bin/sail up -d` to set up services.
5. Run `./vendor/bin/sail migrate` to set up the database.
6. Run `./vendor/bin/sail npm install & npm run dev` to set up static HTML/CSS/JS files.
7. Start your project at `http://localhost`.

### Example .env

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

### Register Youtube API
- [YouTube Data API v3](https://console.cloud.google.com/apis/library/youtube.googleapis.com?hl=zh-TW&project=plasma-circle-334908)

After registering, put your API key in the .env file:

```
YOUTUBE_API_KEY=put_your_api_key_here
```

# Troubleshooting

## You may encounter permission issues when using Docker. Make sure your user is in the Docker group.
`sudo usermod -aG docker $USER`