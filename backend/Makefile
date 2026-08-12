.PHONY: run run-worker run-scheduler test test-integration check build build-api build-worker build-scheduler \
	soketi-scaling-up soketi-scaling-down test-soketi-scaling test-soketi-scaling-control \
	test-gameroom-e2e

run:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

run-scheduler:
	go run ./cmd/scheduler

test:
	go test ./...

# The Redis-backed tests skip themselves unless REDIS_TEST_ADDR is set. Point it
# at the Sail Redis to exercise them.
test-integration:
	REDIS_TEST_ADDR=$${REDIS_TEST_ADDR:-127.0.0.1:6379} go test -count=1 ./...

check:
	go fmt ./...
	go vet ./...
	go test -race ./...

# The game room end-to-end test drives MySQL, Redis and the Sail Soketi together.
# Point it at whatever the local stack publishes; the test skips if any of these is
# missing, so it never fails a hermetic build.
test-gameroom-e2e:
	MYSQL_TEST_HOST=$${MYSQL_TEST_HOST:-127.0.0.1} \
	MYSQL_TEST_DATABASE=$${MYSQL_TEST_DATABASE:-rk_db_restore_20260729} \
	REDIS_TEST_ADDR=$${REDIS_TEST_ADDR:-127.0.0.1:6379} \
	SOKETI_WS=$${SOKETI_WS:-ws://127.0.0.1:6001} \
	SOKETI_HTTP=$${SOKETI_HTTP:-127.0.0.1:6001} \
	SOKETI_APP_ID=$${SOKETI_APP_ID:-$$PUSHER_APP_ID} \
	SOKETI_APP_KEY=$${SOKETI_APP_KEY:-$$PUSHER_APP_KEY} \
	SOKETI_APP_SECRET=$${SOKETI_APP_SECRET:-$$PUSHER_APP_SECRET} \
	go test -count=1 -v -run TestSettledRoundReachesASubscribedClient ./internal/gameroom/

SOKETI_SCALING_COMPOSE := docker compose -f ../compose.soketi-scaling.yml
SOKETI_SCALING_ENV := SOKETI_A_WS=ws://127.0.0.1:6011 \
	SOKETI_A_HTTP=127.0.0.1:6011 \
	SOKETI_B_HTTP=127.0.0.1:6012

# Two Soketi instances behind one Redis. Use 127.0.0.1 rather than localhost: in a
# container localhost resolves to ::1 first, and the published ports are IPv4 only.
soketi-scaling-up:
	$(SOKETI_SCALING_COMPOSE) up -d --wait

soketi-scaling-down:
	$(SOKETI_SCALING_COMPOSE) down -v

# Proves cross-instance delivery works with ADAPTER_DRIVER=redis.
test-soketi-scaling: soketi-scaling-up
	$(SOKETI_SCALING_ENV) go test -count=1 -v -run TestBroadcast ./internal/realtime/

# The control case: the same tests against the default `local` adapter, which must
# NOT deliver across instances. Run this whenever the adapter config changes —
# without it, a passing scaling test proves nothing about the adapter.
test-soketi-scaling-control:
	SOKETI_ADAPTER_DRIVER=local $(SOKETI_SCALING_COMPOSE) up -d --force-recreate --wait
	$(SOKETI_SCALING_ENV) SOKETI_EXPECT_CROSS_DELIVERY=false \
		go test -count=1 -v -run TestBroadcast ./internal/realtime/

build: build-api build-worker build-scheduler

build-api:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o bin/ranking-api ./cmd/api

build-worker:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o bin/ranking-worker ./cmd/worker

build-scheduler:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o bin/ranking-scheduler ./cmd/scheduler
