# Game rooms (multiplayer) — realtime deployment notes

A game room has two moving parts on a participant's screen: the **pairing** the
host currently has up, and the **leaderboard**. They arrive by different routes,
and only one of them is a websocket.

| What moves | How it reaches the participant |
| --- | --- |
| Leaderboard | Soketi broadcast (`GameBetRank` on `game-room.{serial}`), published by the Go worker after each settlement. Also re-read by the poll. |
| Pairing, vote counts, own score | The poll only. Nothing is broadcast for these. |

`useGameRoom` therefore polls the whole room state (`GET /api/v1/game-rooms/{serial}`)
every 5 seconds — the cadence the Laravel UI used for the same job. Without that
poll a participant sits on last round's pairing until they reload, **even when the
websocket is connected**, because the host advancing a round produces no event.

The host's own panel in the game view polls its leaderboard every 15 seconds and,
while the black box is open, the vote tally every 5 seconds.

## Turning the websocket on

The room is fully playable with no websocket at all: it falls back to the poll and
the live indicator says so (「定期更新中」 instead of 「即時連線」). Enabling it makes
the leaderboard move the instant a round settles.

1. Run Soketi. `compose.separated.yml` already defines the service on port 6001.
2. Give the Go backend/worker its publish credentials — `GO_PUSHER_APP_ID`,
   `GO_PUSHER_APP_KEY`, `GO_PUSHER_APP_SECRET`, `GO_PUSHER_HOST`, `GO_PUSHER_PORT`,
   `GO_PUSHER_SCHEME` in the environment. Compose maps these onto the backend's
   `PUSHER_*` variables.
3. Register the same app in Soketi. With the single-app defaults that means
   `SOKETI_DEFAULT_APP_ID/KEY/SECRET` must equal the `GO_PUSHER_*` triple, or the
   worker's publish is rejected while still looking like a success from the API side.
4. Point the frontend at it. Edit the deployed `app-config.js`
   (`/usr/share/nginx/html/app-config.js`):

   ```js
   realtime: {
     key: 'the app key',   // empty disables the websocket
     host: '',             // empty = the page's own hostname
     port: 6001,
     cluster: '',
   },
   ```

   `secure` defaults to whether the page itself is HTTPS, so a proxied `wss://`
   needs no extra setting.

The app **key** is a public client identifier and belongs in `app-config.js`. The
app **secret** is not: it stays in the backend's environment and must never reach
the frontend bundle, an image, or the repository.

## Caveats

- Soketi runs with no adapter here, so it is single-instance. A second replica
  would deliver broadcasts only to the clients connected to the instance that
  received them, while the publish still returns 200.
- The poll is not a fallback that can be removed once the websocket works. A socket
  that dies without closing reports nothing — no error, no close event — and the
  poll is the only thing that turns a frozen room into a slightly stale one.
- A participant sees per-candidate wager counts but no room total; the total and
  the full tally are the host's black box.
