# Game rooms (multiplayer) — realtime deployment notes

A game room has three moving parts on a participant's screen: the **pairing** the
host currently has up, the **vote tally** for it, and the **leaderboard**. All
three are broadcast, and all three are also re-read by a poll.

| What moves | Event on `game-room.{serial}` | Published when |
| --- | --- | --- |
| Leaderboard | `GameBetRank` | The worker finishes settling a batch of decided rounds. |
| Pairing + its vote tally + the room's game serial | `GameRoomRound` | The host settles a round, and when the host restarts and the room follows them onto the new game. |

The tally inside `GameRoomRound` comes from the same `CurrentVotes` query that
`GET /api/v1/game-rooms/{serial}` answers with, so the push and the poll cannot
disagree about what is on screen.

`useGameRoom` still polls the whole room state, but the cadence depends on the
socket: every 5 seconds while it is down or unavailable, every 20 seconds while it
is connected. It also re-reads the state once on every fresh connect, to recover
the frames that were missed while the socket was away.

The host's own panel in the game view polls its leaderboard every 15 seconds and,
while the black box is open, the vote tally every 5 seconds.

## What a restart does to the room

Restarting mints a new game serial. The room is **rebound** to it rather than
reopened — `PUT /api/v1/game-rooms/{serial}/game` with `from_game_serial`,
`game_serial` and the pairing now on screen — so the invite link and the QR code
already handed out keep working, and the seated participants are moved onto the
new game by the `GameRoomRound` broadcast the rebind publishes.

Two rules make that safe without a host identity (there is none: `game_rooms` has
no owner column and every room route is `optionalAuth`):

- **Compare-and-swap on the current game serial.** Only a client that can name the
  game the room is on right now may move it, which in practice is the host's own
  tab. A stale source serial is refused with 403.
- **Same post.** The new game must belong to the same post as the old one. Anything
  else is refused with 409, as is a game that already has a room of its own.

Wagers still open on the game the room left never settle — the host abandoned that
bracket, so nothing will ever decide those rounds. Participants keep the score
they had.

## What a reload does to the room

A reload used to move the host's pairing and nobody else's. The resumed game re-picked the
match on screen — legal, since it had not been voted on — while the server still held the
pair reported by the last vote sync, so everybody seated kept looking at the pre-reload
match and their poll kept confirming it.

Two things stop that now:

- **While a room is open for the game, a resume keeps the pairing it had.** Re-picking would
  strand every wager already placed on that match, since it would never be played.
- **Picking the room back up reports the pairing anyway.** `POST /api/v1/game-rooms` is
  idempotent and the host's page calls it on load; with `current_candidates` it also
  broadcasts `GameRoomRound` to a room that already existed. That covers the pair the server
  never heard about — a vote sync that failed before the reload, for instance. A room created
  by that same call announces nothing: nobody has joined it yet.

## Turning the websocket on

The room is fully playable with no websocket at all: it falls back to the poll and
the live indicator says so (「定期更新中」 instead of 「即時連線」). Enabling it makes
the pairing and the leaderboard move the instant the host acts, instead of on the
next poll.

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
- The poll is not a fallback that can be removed once the websocket works, which is
  why it slows to 20 seconds rather than stopping. A socket that dies without
  closing reports nothing — no error, no close event — and the poll is the only
  thing that turns a frozen room into a slightly stale one.
- A participant sees per-candidate wager counts but no room total; the total and
  the full tally are the host's black box.
