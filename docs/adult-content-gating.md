# Adult content (18+) — deployment notes

A post flagged `posts.is_censored = 1` is always shown blurred and always carries no ads.
Whether it *also* requires an account is a deployment setting, and its default is **off**:
out of the box an 18+ post is played, voted on and ranked by a visitor exactly like any
other post.

## The setting

```sh
# Only the API acts on it, but every Go binary parses it — a bad value stops all three.
ADULT_CONTENT_REQUIRE_SIGN_IN=true   # accepted: true/1/on, false/0/off. Default: false.
```

Set it and the rule below applies: preview stays public — the home feed card and the game
page's two blurred thumbnails — while starting a game, resuming one, voting, reading the
result and reading the ranking all need a signed-in caller.

Nothing else to do: no migration, no image build argument, no per-post column. Any value
the parser cannot read (`yes`, `1.0`, `enabled`) **fails startup** rather than defaulting,
so a typo cannot silently leave the gate open.

Changing it takes a restart of the API containers, and only the API: the flag is read once
at startup so that two statements serving the same request cannot disagree.

To confirm which way a running deployment is configured, read the startup log line:

```sh
docker compose logs backend | grep public_content_database_enabled
# ... "adult_sign_in_required":true
```

or ask the API about a censored post — `requires_sign_in` is the answer the browser uses:

```sh
curl -s "https://<host>/api/v1/game-posts/<censored serial>" | jq '.is_censored, .requires_sign_in'
```

## Where the rule lives

`postaccess.AdultPolicy` carries the setting and owns the two questions asked of it:
`RequireSignIn(isCensored, caller)` refuses a request, and `GateApplies(isCensored)` reports
whether a post is gated at all, whoever is asking. The policy is handed to
`gameplay.NewMySQLRepository` and `publiccontent.NewMySQLRepository` in `cmd/api/main.go`,
so every statement of one request shares one answer.

It is deliberately *not* folded into `postaccess.VisibilityClause`, because that clause is
spliced into every post read — including the ones that must keep answering anonymously for
the blurred preview, and the comment endpoints, which this rule does not touch.

Enforced at:

- `gameplay.MySQLRepository` — `Create`, `Resume`, `SubmitVotes`, `Result`.
- `publiccontent.MySQLRepository.visiblePostID` — the choke point for `Ranks`,
  `SearchRanks`, `Rank` and the trend chart that rides on the rank read.

Left open on purpose:

- `GET /api/v1/game-posts/{serial}` (`gameplay.Definition`) — the vote page's preview. It
  stays public and edge-cacheable, and carries `is_censored` so the client knows to blur,
  plus `requires_sign_in` — `GateApplies`, i.e. the post AND the setting — so the client
  knows whether to offer a game or a sign-in link. `requires_sign_in` does not depend on
  who asked, which is what keeps the response cacheable by serial.
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

With the setting off, an 18+ post behaves like any other post apart from the blur and the
missing ad slots. Everything below describes the setting turned on.

`GameView.vue` waits for the session before it decides. `refreshAuthState` is single-flight,
so the game page joins the header's boot-time `/auth/refresh` rather than issuing a second
one (a second call would rotate the refresh token mid-load).

On a gated post (`requires_sign_in: true`) with no account:

- `/g/:serial` shows the blurred preview, the count picker is replaced by a sign-in link
  carrying `?redirect=` back to the same page, and no game is created.
- `/r/:serial` shows a locked card with the "sign in to see the ranking" prompt and never
  calls `ranks`.
- A game saved in `localStorage` before signing out is *not* resumed: the gate returns
  before the snapshot is read.

An older frontend, which predates `requires_sign_in`, falls back to gating every 18+ post.
That is the safe direction, but it means the page shows the sign-in prompt while the API
happily lets visitors play — ship the two together.

## Rollback

Unset `ADULT_CONTENT_REQUIRE_SIGN_IN` (or set it to `false`) and restart the API: the gate
is off again, with no data to migrate either way. To go back further, redeploy the previous
backend image — that build has no setting and gates every 18+ post unconditionally. Rolling
back only the frontend is safe but pointless: the API would still answer 401 and the SPA
would show a generic error instead of the sign-in prompt.
