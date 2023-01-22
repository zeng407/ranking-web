# pre-required
- [docker](https://www.docker.com/)  

# Build your docker image
```
# cp Dockerfile
cp Dockerfile.example Dockerfile

# cp docker-compose
cp docker-compose.yaml.example docker-compose.yaml

# cp .env
cp .env.example .env
```

# first deploy
```
# enter container
docker exec -ti bash rk-web

# install package
compsoer install

# init project
php artisan key:generate

# migrate table (seed for testing)
php artisan migrate --seed

```

# s3 bucket

```
# 1. go to localhost:9001
# 2. the account&password is written on docker-composer.yaml
# 3. login & craete a bucket
# 4. create access key
# 5. edit .env 

AWS_ACCESS_KEY_ID=
AWS_SECRET_ACCESS_KEY=
AWS_DEFAULT_REGION=us-east-1
AWS_BUCKET=bucket-name
AWS_URL=https://127.0.0.1:9000/bucket-name
AWS_ENDPOINT=https://127.0.0.1:9000
AWS_USE_PATH_STYLE_ENDPOINT=true
```

# develop
```
# build frontend
npm run dev
```
