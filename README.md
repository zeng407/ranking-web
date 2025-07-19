![laravel build](https://github.com/zeng407/ranking-web/actions/workflows/laravel.yml/badge.svg)
![laravel build](https://github.com/zeng407/ranking-web/actions/workflows/node.js.yml/badge.svg)

# Explore the AI-Generated Wiki

Check out the amazing AI-generated wiki for more insights:  
[DeepWiki - Ranking Web Overview](https://deepwiki.com/zeng407/ranking-web/1-overview)

# Prerequisite
- [Docker](https://www.docker.com/)

# Install with Docker

1. Start all services:
   ```
   docker compose up -d --build
   ```
2. Enter the Laravel container and install PHP dependencies:
   ```
   docker compose exec laravel.test composer install
   ```
3. Copy the example environment file:
   ```
   docker compose exec laravel.test cp .env.example .env
   ```
   > **Recommended:** Edit `.env` and change `DB_PASSWORD` to a secure password before starting the application.
4. Generate the Laravel application key:
   ```
   docker compose exec laravel.test php artisan key:generate --ansi
   ```
5. Run database migrations:
   ```
   docker compose exec laravel.test php artisan migrate
   ```
6. Install frontend dependencies and build assets:
   ```
   docker compose exec laravel.test npm install && npm run dev
   ```

# Install PHP (Optional)

> **Note:** If you use Docker installation above, you do **not** need to install PHP and Composer manually on your host system. This section is only for manual/local installation.

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

### Build for Development

1. Run `composer install`. If you encounter missing dependencies, use the `--ignore-platform-reqs` flag.
2. Run `php artisan sail:install` and select (1)mysql, (2)minio, (3)redis, (4)selenium (5)soketi.
3. Run `./vendor/bin/sail up -d` to set up services.
4. Run `./vendor/bin/sail migrate` to set up the database.
5. Run `./vendor/bin/sail npm install & npm run dev` to set up static HTML/CSS/JS files.
6. Start your project at `http://localhost`.

### Minio Configuration

Visit `localhost:9000` to log in to your Minio account (see [docker-compose.yml](docker-compose.yml)). You need to create a bucket and generate an access key. Then, put the access key and other Minio configurations in your .env file. (See the following example .env)

```
# .env

...

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


## Run Background Workers and Scheduler

After entering the `sail shell`, run the following scripts to start the queue workers and scheduler in the background:

```sh
chmod +x ./start_worker.sh
chmod +x ./start_schedule.sh
./start_worker.sh &
./start_schedule.sh &
```

This will start all queue workers and the Laravel scheduler as background processes.


# Troubleshooting

 - You may encounter permission issues when using Docker. Make sure your user is in the Docker group.
`sudo usermod -aG docker $USER`

- If you encounter file or directory permission issues, you can fix them with:
```
sudo find . -type d -exec chmod 775 {} \;
sudo find . -type f -exec chmod 664 {} \;
```
