# Adult content (18+) — deployment notes

A post flagged `posts.is_censored = 1` may be *previewed* by anyone and *used* only by an
account. Preview means the home feed card and the game page's two thumbnails, both blurred
by CSS. Everything past the preview — starting a game, resuming one, voting, reading the
result, and the ranking page — requires a signed-in caller.

There is nothing to configure: **no new environment variable, no migration, no image
build argument.** The rule reads a column that already exists and a caller identity the
API already resolves. Deploying is just shipping the new backend and frontend images.

## Where the rule lives

`postaccess.RequireSignIn(isCensored, caller)` is the single predicate. It is deliberately
*not* folded into `postaccess.VisibilityClause`, because that clause is spliced into every
post read — including the ones that must keep answering anonymously for the blurred
preview, and the comment endpoints, which this rule does not touch.

Enforced at:

- `gameplay.MySQLRepository` — `Create`, `Resume`, `SubmitVotes`, `Result`.
- `publiccontent.MySQLRepository.visiblePostID` — the choke point for `Ranks`,
  `SearchRanks`, `Rank` and the trend chart that rides on the rank read.

Left open on purpose:

- `GET /api/v1/game-posts/{serial}` (`gameplay.Definition`) — the vote page's preview. It
  stays public and edge-cacheable, and carries `is_censored` so the client knows to blur
  and to ask for a sign-in instead of a game.
- `GET /api/v1/posts` — the home feed reads precomputed `public_posts` rows and is
  unchanged. The champion rail already filtered `is_censored = 0` before this work.

## Cache behaviour — the part worth checking after a deploy

A refusal answers **401 `unauthenticated`**, not 404. 404 is already meaningful on the game
page (it is the private-post door-code path), and the SPA has to tell the two apart.

The 401 is per-caller, so it must never reach a shared cache. `writeError` already sets
`Cache-Control: no-store` and emits no `Cloudflare-CDN-Cache-Control`, which is what keeps
it out of the CDN. Verify with:

```sh
# 401, Cache-Control: no-store, no Cloudflare-CDN-Cache-Control header
curl -sD - -o /dev/null "https://<host>/api/v1/ranks?post_serial=<censored serial>"

# still 200 and still edge-cacheable — this is the blurred preview
curl -sD - -o /dev/null "https://<host>/api/v1/game-posts/<censored serial>"
```

If a `ranks` 401 ever comes back with a CDN cache-control header, stop and fix that before
anything else: a cached 401 would lock out signed-in users, and a cached 200 would leak a
gated ranking to visitors.

## Client behaviour

`GameView.vue` waits for the session before it decides. `refreshAuthState` is single-flight,
so the game page joins the header's boot-time `/auth/refresh` rather than issuing a second
one (a second call would rotate the refresh token mid-load).

On an 18+ post with no account:

- `/g/:serial` shows the blurred preview, the count picker is replaced by a sign-in link
  carrying `?redirect=` back to the same page, and no game is created.
- `/r/:serial` shows a locked card with the "sign in to see the ranking" prompt and never
  calls `ranks`.
- A game saved in `localStorage` before signing out is *not* resumed: the gate returns
  before the snapshot is read.

## Rollback

Redeploy the previous backend image. Nothing persisted changed, so there is no data to
migrate back. Rolling back only the frontend is safe but pointless: the API would still
answer 401 and the SPA would show a generic error instead of the sign-in prompt.
