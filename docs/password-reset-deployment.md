# Password reset (Go) — deployment notes

Password reset was the last feature that still required Laravel: the Go API could
not send mail, so the SPA handed the user to the PHP site. Both halves live here
now — `POST /api/v1/auth/password/forgot` mails a link, and
`POST /api/v1/auth/password/reset` spends it — and the pages behind them are SPA
routes (`/{locale}/password/forgot`, `/{locale}/password/reset/{token}`).

This is the only outbound mail in the Go stack.

## Turning it on

Mail is off by default. The switch is `GO_MAIL_TRANSPORT`:

| Value | Effect |
| --- | --- |
| empty (default) | Both endpoints answer `503 account_not_configured`. The api logs `password_reset_disabled` at startup. |
| `log` | Nothing is sent. The recipient, subject and the full body — **reset link included, in clear** — go to the api's log. Local verification only. |
| `smtp` | Real mail through `MAIL_HOST`. |

`MAIL_MAILER` is deliberately *not* the switch. It is already `smtp` in every
`.env` because Laravel needs it, so reading it as intent would demand a working
relay from every environment, including CI and a developer's laptop. The same
reasoning governs `GO_OAUTH_GOOGLE_REDIRECT_URL`.

Environment, all read by the `backend` service in `compose.separated.yml`:

```
GO_MAIL_TRANSPORT=smtp
MAIL_HOST=smtp.gmail.com
MAIL_PORT=587
MAIL_ENCRYPTION=tls          # tls = STARTTLS (587), ssl = implicit TLS (465), empty = plaintext
MAIL_USERNAME=...            # empty means no AUTH at all
MAIL_PASSWORD=...
MAIL_FROM_ADDRESS=no-reply@example.com
MAIL_FROM_NAME=殘酷二選一
APP_URL=https://2pick.example.com
```

Rules the code enforces:

- `GO_MAIL_TRANSPORT=smtp` requires `MAIL_HOST`, `MAIL_PORT` and
  `MAIL_FROM_ADDRESS`. Any transport requires `APP_URL`. A missing value fails
  startup rather than starting with an endpoint that cannot work.
- `MAIL_ENCRYPTION=tls` means STARTTLS **must** succeed. A relay that refuses it
  is an error; the sender never falls back to plaintext with the password on the
  wire.
- `APP_URL` is the origin of the mailed link, and it is where the user lands. If
  it does not match the site the SPA is served from, every link 404s.
- `MAIL_FROM_NAME` is written in Compose as `${APP_NAME:-殘酷二選一}`, not
  `${MAIL_FROM_NAME}`. Laravel's `.env` holds the literal string `"${APP_NAME}"`
  and expands it itself; Compose does not re-expand a value it read from that
  file, so `${MAIL_FROM_NAME}` would put `${APP_NAME}` in every inbox. The api
  also drops a display name that still contains `${` as a last defence.
- The subject and the sender name are RFC 2047 encoded and the body is sent
  base64, so non-ASCII copy survives relays that would otherwise mangle it.

`MAIL_PASSWORD` is a credential — a Gmail account needs an **app password**, not
the account password. It must not reach Git, a Docker image, CI output or the
frontend bundle: it belongs in the deployment's `.env` and nowhere else.

## Migration

The tokens live in `go_password_resets`, created by
`backend/migrations/00014_go_password_resets.sql`. It must be applied before the
endpoints are used, or a request fails on a missing table:

The migrator is `backend/cmd/migrate`, a separate entrypoint — the api image is
built with `CMD=api` and contains only the api, so it cannot run migrations
itself. Locally, against the Compose stack:

```bash
docker run --rm --network ranking-web_sail \
  -v "$PWD/backend":/src -w /src \
  -e DB_HOST=mysql -e DB_DATABASE=rk_db_restore_20260729 \
  -e DB_USERNAME=root -e DB_PASSWORD= \
  golang:1.26.5-alpine go run ./cmd/migrate up

docker compose -p ranking-web exec mysql \
  mysql -uroot rk_db_restore_20260729 \
  -e "SELECT version_id FROM go_schema_migrations ORDER BY id DESC LIMIT 3"
```

In a deployment, build the same image with `--build-arg CMD=migrate` and run
`up` from it before rolling out the api.

Laravel's own `password_resets` table is untouched and unused. The two are
separate on purpose: two migration histories must not manage one table, Laravel
bcrypts its tokens (so a lookup needs the address first, then a bcrypt compare,
where a sha256 `token_hash` unique index hits directly), and "usable once" is a
`used_at` column here rather than a deleted row.

**Cutover cost:** reset links Laravel had already mailed and nobody had used yet
stop working the moment PHP stops serving `/password/reset`. They live 60
minutes, so the window is small, but it is not empty.

## Limits

- A token lives **60 minutes** (`ResetTokenTTL`), matching
  `config/auth.php passwords.users.expire`.
- One account gets at most one mail a **minute** (`ResetThrottle`, matching
  `passwords.users.throttle`).
- One source address gets at most **5 mails an hour**
  (`ResetRequestsPerWindow` / `ResetRequestWindow`), counted in Redis under
  `go:password-reset:ip:`. Without Redis this limit is absent: the api logs
  `password_reset_rate_limit_disabled` and keeps working. The per-account
  throttle does not replace it — it does not stop one source from asking for
  mail to a thousand different addresses, and the sending account is a shared
  relay that gets locked when that happens.
- A limiter that errors **fails open**: a Redis outage must not take password
  recovery down with it.
- A token is claimed atomically
  (`UPDATE … SET used_at = ? WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`),
  so two clicks on the same link cannot both set a password.
- A successful reset issues a session immediately and revokes the account's
  other refresh tokens: whoever knew the old password is signed out.

## Why forgot always answers 200

`POST /auth/password/forgot` answers `200 {"status":"sent"}` for an address that
has an account, for one that does not, for a request the throttle stopped, and
for a mail the relay refused. Anything else turns the endpoint into a way to test
which addresses are registered.

The consequence is a contract on the copy: the page after submitting must **not**
say a mail was sent. It says *if this address is registered*, a link has been
sent — the conditional is load-bearing, and
`frontend/src/views/ForgotPasswordView.test.ts` asserts it.

A mail the relay refused is logged (`password_reset_mail_failed`) and still
answers 200, so a broken relay is visible in the log and nowhere else. Watch that
log after changing relay settings; a silent failure looks exactly like success
from outside.

## Verifying without a relay

```bash
# 1. Ask for a link. Answers 200 whatever the address.
curl -sS -X POST http://localhost/api/v1/auth/password/forgot \
  -H 'content-type: application/json' \
  -d '{"email":"player@example.test","locale":"zh_TW"}'

# 2. With GO_MAIL_TRANSPORT=log, the link is in the api log.
docker compose -p ranking-web logs backend | grep mail_not_sent_logged_instead
```

Open the link in a browser, set a password, and the reset signs the account in.
Checks worth making after that: `/api/v1/auth/me` reports the account, the old
refresh cookie no longer refreshes, the same link answers `token: ["invalid"]` on
a second use, and a second forgot request for the same address within a minute
still answers 200 with no second mail in the log.

Remember to unset `GO_MAIL_TRANSPORT=log` afterwards. It writes reset links to
the log in clear, which is a way into any account whose address you know.

## Mail copy

`backend/internal/auth/password_reset_mail.go` holds the subject and body for
`zh_TW`, `en` and `ja`, chosen by the `locale` the SPA sends. An unknown or
missing locale falls back to `zh_TW`. This is the only user-facing prose in the
Go API — a mail is read outside the browser, so it cannot be translated there.
Validation errors are still machine codes the frontend translates.
