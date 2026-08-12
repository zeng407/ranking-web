ARG GO_VERSION=1.26.5
FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src
ARG VERSION=dev
ARG COMMIT=unknown
# CMD selects which entrypoint under ./cmd to build: api, worker or scheduler.
ARG CMD=api
RUN apk add --no-cache ca-certificates
# go.sum must be present for `go mod download` to verify module checksums and for
# this layer to keep caching correctly as dependencies are added.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The Redis-backed tests skip themselves unless REDIS_TEST_ADDR is set, so this
# build stays hermetic while the same suite covers integration locally and in CI.
RUN go test ./... && \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /out/entrypoint ./cmd/${CMD}

# The api and scheduler are pure Go with no external process dependencies, so
# they ship on scratch.
FROM scratch AS minimal
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/entrypoint /entrypoint
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/entrypoint"]

# The media worker shells out to ffmpeg for video thumbnails, which needs both a
# libc and the binary itself, so it cannot use scratch. Build it with
# --target media.
FROM alpine:3.22 AS media
RUN apk add --no-cache ca-certificates ffmpeg tzdata
USER 65532:65532
COPY --from=build /out/entrypoint /entrypoint
EXPOSE 8081
ENTRYPOINT ["/entrypoint"]

# Keeps `docker build ./backend` producing the api image exactly as before.
FROM minimal
