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

The re-pick itself stays — a host reloads precisely to get a different match. What changed
is that the room is told. `POST /api/v1/game-rooms` is idempotent and the host's page calls
it on every load, so with `current_candidates` it doubles as that report, and the handler
broadcasts `GameRoomRound` when the room already existed. A room created by the same call
announces nothing: nobody has joined it yet.

The report is sent twice on a reload, because two different pairings are on screen. Picking
the stored room back up reports whatever is up at that moment — the pre-reload match, since
the host is still answering the saved-game dialog — and the re-pick that follows their answer
reports itself (`resumeSnapshot` calls `reportPair`). Reporting only from the first would
broadcast the match the reload was meant to replace.

Wagers already placed on the pre-reload pairing are neither lost nor punished. A participant
can simply pick again — the wager row is keyed on the round, so a second pick replaces the
first — and a wager left on a pairing the round never presented is discarded by the
settlement rather than counted as a loss.

## Majority mode (多數決)

A room decides its own rounds when `game_rooms.vote_mode = 'majority'`. The side with more
wagers on the pairing wins it; a tie — 0-0 included, which is what an unwatched room produces
every round — is broken by a coin toss, so a quiet room still advances instead of stalling or
quietly favouring one side.

`round_seconds` carries both of the settings the host is offered:

| `round_seconds` | What the host chose | `round_ends_at` |
| --- | --- | --- |
| 5–300 | A countdown of that many seconds | armed on every round change |
| 0 | Manual end only | always `NULL` |

One column rather than a flag plus a duration, so the two can never disagree.

**The server holds the clock; the host's browser settles the round.** The bracket is played
locally in the host's page — the server never learns which pair comes next and so cannot
decide a round on its own. But the deadline has to be the server's, because the host and
every participant must count down to the same instant and their device clocks are not
comparable.

That is also why the API reports **`seconds_left`, never an absolute timestamp**. MySQL
computes it with `TIMESTAMPDIFF(MICROSECOND, NOW(3), round_ends_at)`, so the subtraction uses
the same clock that wrote the deadline, and each client counts that number down locally
between reads. Every read and every pushed frame re-seeds it, so local drift lasts at most
one poll interval and is corrected rather than accumulated.

The deadline is armed in three places, all of them best-effort and none of them able to fail
a request:

- `POST /api/v1/game-rooms` (`EnsureRoom`) and `PUT .../game` (`Rebind`) — room opened,
  reloaded, or followed onto a restarted game.
- `announceGameRoomRounds`, on every settled round. It runs **before** the broadcast is
  published, so the `GameRoomRound` frame carries the fresh deadline, and **before** the
  announcer nil-check, because a room is playable by polling alone and its clock has to run
  either way.

`voting` (`{mode, round_seconds, seconds_left}`) rides at the top level of the room state,
the votes response and the `GameRoomRound` frame — not inside the tally, because it must be
readable between rounds when there is no tally at all.

`PUT /api/v1/game-rooms/{serial}/voting` writes the settings. Like every other room route it
is `optionalAuth` and there is no host identity: naming the game serial the room is bound to
right now is the only proof of hosting, and a caller who names a different one is refused
with 403. Bad mode or out-of-range seconds is 422 `invalid_voting`.

While a room is in majority mode the host's own vote buttons are disabled. Two ways to settle
a round would be two sources of truth, and a host clicking through would leave the votes they
asked the room for unread. 「結束回合」 is offered in both settings, since cutting a round
short is useful whether or not one was going to end on its own.

### Deploying it

- **Run migration `00016_game_room_voting.sql`.** It adds `vote_mode`, `round_seconds` and
  `round_ends_at` to `game_rooms`. Existing rooms default to `vote_mode = 'host'`, which is
  the behaviour they already had, so the migration is safe to run ahead of the deploy.
- Nothing new is needed from Soketi, the worker, or `app-config.js`: the countdown travels on
  the `GameRoomRound` frame that already exists, and on the poll behind it.
- The host's votes poll now runs whenever the room is in majority mode, not only while the
  black box is open — the same call carries the clock. The counts still only reach the screen
  when the box is open.

## How fast the pairing travels

Measured end to end in a browser, from the host's click to the participant's DOM changing:
**0.19s**. It was 0.86s until the worker's reserve window was shortened, and that difference
is worth knowing about because it is not in this feature's code at all.

The worker consumes six queues in priority order. Only the last one blocks; the rest are
drained by a non-blocking pop once per loop. So a message on `game_room` — which is second,
by priority — waits for the blocking read on `low` to time out, and that window was two
seconds. `queue.ReserveBlockTimeout` is now 250ms, and the reserve issues BRPOPLPUSH itself
because the client's typed helper silently floors a sub-second timeout to one second
(`internal/queue/consumer.go`, `blockingReserve`). The cost is one non-blocking pop per queue
per window on an idle worker.

If the pairing ever feels slow again, that constant is the first thing to check, and
`worker_job_completed`'s `duration_ms` next to the publish time in the API log is how to tell
the queue hop from the handler.

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
