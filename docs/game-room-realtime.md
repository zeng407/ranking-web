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

The host's own panel in the game view polls its leaderboard every 15 seconds and the
vote tally every 5 seconds — the tally whenever the counts are on screen, which is a
majority room always and a host-mode room only while the black box is open.

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

**A restart from the result page is a remount, and the room has to survive that too.**
Finishing a bracket navigates to `/r/…` and 再玩一次 navigates back to `/g/…`; the router view
is keyed on the path, so the second navigation destroys the view and builds a new one on the
new game. The rebind is fired by the instance being torn down and writes its
`gameroom_host_<gameSerial>` key only when the request comes back, so the fresh instance would
read that key, find nothing, and tell the host to set multiplayer mode up again — with their
room open and their participants still in it.

So the room is also written under `gameroom_host_post_<postSerial>`, with the game it was on
and a timestamp. A mount that has no room for its own game reads that pointer, adopts the room
before any request, and fires the rebind itself; rebinding a room that is already on the target
game returns it rather than failing, so it does not matter how far the torn-down instance got.
The pointer is ignored when it names another post, when it names this same game, and when it is
more than six hours old — a restart happens seconds later, and a room left open this morning
must not seat its old participants in a game started tonight. A refused rebind clears it, so a
room this page cannot drive is not offered again on the next restart.

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

**The host votes as an ordinary player.** Their click is a wager, not a verdict: it is the
same `POST .../bets` every participant sends, it joins them to the room and puts them on the
leaderboard, and the round still ends on the clock or on 「結束回合」. So there is still one
way to settle a round — the tally — and a host with taste no longer has to sit the game out.
「結束回合」 is offered in both settings, since cutting a round short is useful whether or not
one was going to end on its own.

**The black box is a host-mode device and is not offered here.** It exists so guessers cannot
copy each other in 猜喜好; in a majority room the counts are the rule the round is decided by,
every participant already sees them on their own cards, and hiding them would hide the room
from the person running it. `showVotes = blackBox || majority` is the single answer to "do the
counts belong on screen", and the toggle button is not rendered in majority mode.

**The round settings sit beside the clock**, behind a gear in the panel that shows the
remaining time, and open the same settings step the mode was first chosen in. A host who wants
to move from a 20-second round to manual is not looking for the invite link they already
handed out, so 確定 there writes the settings and closes; the step only continues to the invite
when it was reached by first choosing the mode, which happens once per room. For the same
reason the invite step carries no settings button — the link is handed out from there and
nothing else is.

### The vote history

`GET /api/v1/game-rooms/{serial}/history?anonymous_id=&game_serial=&limit=` answers the rounds
the room has already decided, newest first: both elements, both counts, and `your_pick` — the
element the caller wagered on, or 0 if they did not. Both screens show it; the host's own match
history gains a line on the cards it can match, and the participant's page lists it under the
standings.

It is an **aggregate over `game_room_user_bets`, not a new table** — no migration. A settled
round leaves one row per voter: winners carry `won_at` and their own `(winner_id, loser_id)`,
losers carry `lost_at` and the pair flipped, so the two counts are `SUM(won_at IS NOT NULL)` and
`SUM(lost_at IS NOT NULL)` over rows grouped by the round key *and the pairing*
(`current_round, of_round, remain_elements` plus the ordered pair). The pairing is in the key
because a restart replays the bracket into the same room, and two different matches numbered
"round 1 of 16" must not merge into one row.

The winning element reads as `COALESCE(MAX(CASE WHEN won_at IS NOT NULL THEN winner_id END),
MAX(CASE WHEN lost_at IS NOT NULL THEN loser_id END))`. The fallback is not defensive: in a
**host** room every voter can be wrong, and then no winning row exists to read the winner from.

Because the rows are the room's, the history spans its whole life — including the bracket it
was playing before a restart, which the invite link and the room serial also survived. Reading
it never seats anyone: `anonymous_id` is matched against `game_room_users`, never inserted, so
a lurker stays off the leaderboard. Limit defaults to 20 and is refused above 50 (422).

The host does not wait for that read to see the round they just settled. Their wagers reach
the server through the vote outbox, which flushes over a second after the round is over, so an
aggregate read at settlement time answers with every round *but* this one. The tally that
decided the round is the same count, so the row is written locally from it — winner, loser, both
counts, the round numbers, and the host's own pick — and dropped again as soon as a server read
returns that same round (keyed on `current_round`, `of_round` and the unordered pair). A local
row is a stand-in for exactly one server row, never an extra one.

The host re-reads it when the room opens, on the 15-second board poll, when `GameBetRank`
arrives, when a round is settled and when the room follows a restart. A participant re-reads it
whenever the pairing on screen changes, which is what a round ending looks like from their side
no matter who or what ended it.

### The two taste boards

A round the room decided pays by **how lopsided it was**, the same magnitude to both sides:

```
magnitude = round(bet_won_score * (1 + winner_votes / total_votes))
```

`bet_won_score` is 10, so the rule reads "the usual 10, plus up to another 10 for how one-sided
the round was". A 50/50 round — which is what a coin-tossed tie is — pays ±15; 70/30 pays ±17;
a unanimous round pays ±20. **No combo bonus**, in either direction: agreeing with the room six
times running does not make a player six times more mainstream than agreeing once, and a flat
±10 would score a 51/49 round and a 99/1 round identically. The 連勝 stat is hidden in a
majority room because the column is zero there by construction.

Host mode is untouched — there the score measures agreement with one person, the streak is part
of that game, and the vote counts are not a rule at all.

**The row's `score` is authoritative in this mode and derived in the other.** The magnitude is a
fact about the *round*: it needs both sides' counts, which no single player's wagers can show,
so `SettleBets` resolves it once and writes `+m` / `−m` into `game_room_user_bets.score`, and
the tally sums that column instead of re-deriving it. Host mode does the opposite on purpose —
it refuses to trust `last_combo` because that column is written when the wager is *placed*, from
whatever had settled by then, which makes it a function of how fast the player clicked. The
majority payout is written by the settlement itself, from the rows it is settling. Computing it
in Go rather than in SQL keeps one rounding decision instead of two: the number reaches both
statements as a parameter (see `MajorityPayout`, and `AccuracyHundredths` for what the
alternative costs). Redelivery is still idempotent — the counts do not filter on `won_at`, so a
second settle recomputes the same magnitude and rewrites the same numbers.

No migration: `score` already exists and was already written on every settlement.

That makes one score column readable from both ends, which is the two rankings the mode is
named for:

| Board | Read from | Rank shown |
| --- | --- | --- |
| 大眾品味排行榜 | `top_10`, highest score first | the row's own `rank` |
| 獨特品味排行榜 | `bottom_10`, lowest score first | its position in that list |

`bottom_10` arrives worst first, and worst by score is exactly most unique, so the first row
of it is unique rank 1. The `rank` those rows carry is their place on the popular board and
would read here as a large number, so `uniqueRows` replaces it. It cannot be inverted from
`total_users` either: that count includes players no refresh has ranked yet, and each one
would shift the whole board by one.

A room the host decides keeps the single merged list it has always had — there the score
measures agreement with one person's taste, which has only one direction worth reading.

**Switching a room's mode re-scores its whole history.** Both paths are full recomputes from the
wager rows, so a room moved to host mode re-derives every round under host rules, including
rounds that were decided by vote — and one moved the other way sums a `score` column those
earlier rounds wrote under the streak rule. That is the honest reading of "the room's rules
changed", and nothing in the UI offers the switch today.

### Deploying it

- **Run migration `00016_game_room_voting.sql`.** It adds `vote_mode`, `round_seconds` and
  `round_ends_at` to `game_rooms`. Existing rooms default to `vote_mode = 'host'`, which is
  the behaviour they already had, so the migration is safe to run ahead of the deploy.
- Nothing new is needed from Soketi, the worker, or `app-config.js`: the countdown travels on
  the `GameRoomRound` frame that already exists, and on the poll behind it.
- **The proportional payout needs no migration**, but it does re-score majority rooms that were
  already played: the next refresh re-derives their totals under the new rule, so a room played
  before the deploy will move (its streak bonuses disappear and each round is re-paid by its
  split). Deploy between games rather than mid-bracket if that matters.
- The host's votes poll runs whenever the room is in majority mode, not only while the black
  box is open — the same call carries the clock, and in this mode the counts are on screen
  anyway.
- **The history endpoint needs no migration and no new table**, and no cache: it reads
  `game_room_user_bets` by `room_id` on the index it already has. It is polled once per settled
  round per client, not per poll interval.

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

   `frontend/public/app-config.js` — the copy baked into the image — keeps `key: ''`, and
   that is deliberate: the repository must not decide that an environment has Soketi, and a
   deployment without one has to stay playable on the poll. A local stack turns the socket
   on by mounting its own copy over the built one instead of editing the repo file:

   ```yaml
   # compose.local.yml, alongside compose.separated.yml and not committed
   services:
     frontend:
       volumes:
         - ./deploy/local/app-config.js:/usr/share/nginx/html/app-config.js:ro
   ```

   ```
   docker compose -f compose.separated.yml -f compose.local.yml up -d frontend
   ```

   Without it the host's screen falls back to a 15-second board poll and a 5-second votes
   poll, which is playable but makes the leaderboard and the counts look slow.

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
- In host mode a participant sees per-candidate wager counts but no room total; the
  total and the full tally are the host's black box. In majority mode there is no box:
  the counts decide the round, so everybody — the host included — sees the same ones.
- The host's own pick is marked from memory, not from the server: a reload mid-round
  loses the highlight. The wager itself stands, and the history shows which side it
  was on once the round settles.
