-- Refresh tokens for the Go API's own sessions.
--
-- The first table this migration history creates rather than repairs. Laravel has
-- no equivalent: it keeps sessions in a cookie and only mints short-lived access
-- tokens for the Go API through GoAccessTokenService. Taking over authentication
-- means Go has to own the long-lived half itself.
--
-- WHY A TABLE AND NOT A SIGNED COOKIE
--
-- A self-contained signed refresh token cannot be revoked before it expires, and
-- revocation is the whole point of the long-lived half: logout, a stolen token, a
-- banned account. Server-side rows make revocation a write.
--
-- WHAT IS STORED
--
-- token_hash, never the token. The token is 32 random bytes, so a single SHA-256
-- is the right choice — bcrypt exists to slow down guessing a low-entropy secret,
-- and against 256 bits of entropy it would only slow down the legitimate refresh.
--
-- ROTATION AND THEFT DETECTION
--
-- Each refresh consumes one row (used_at) and issues a new one in the same family.
-- Presenting a row that is already used means the token was captured and replayed:
-- either the attacker or the real client is using a copy. There is no way to tell
-- which, so the whole family is revoked and both are forced to log in again. That
-- is the standard response, and it is the reason family_id exists rather than a
-- flat list of tokens.
--
-- csrf_hash pairs with the httpOnly cookie. The cookie is sent automatically by the
-- browser, so possession of it cannot authorise a state change on its own; the
-- client must also echo a value it can only read from a response body or a
-- non-httpOnly cookie. Stored hashed for the same reason as the token.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE go_refresh_tokens (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    -- Groups one login's rotation chain. Revoking a family logs out that device.
    family_id CHAR(36) NOT NULL,
    -- SHA-256 of the opaque token, hex encoded.
    token_hash CHAR(64) NOT NULL,
    -- SHA-256 of the CSRF value bound to this session.
    csrf_hash CHAR(64) NOT NULL,
    issued_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    -- Set when this token is exchanged. A second presentation of a used token is
    -- the theft signal.
    used_at TIMESTAMP NULL DEFAULT NULL,
    revoked_at TIMESTAMP NULL DEFAULT NULL,
    -- Kept for audit only. Never used to authorise: both are client-controlled and
    -- change legitimately on mobile networks.
    created_ip VARCHAR(45) NULL DEFAULT NULL,
    user_agent VARCHAR(255) NULL DEFAULT NULL,
    created_at TIMESTAMP NULL DEFAULT NULL,
    updated_at TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (id),
    -- The lookup every refresh does. Unique because a hash collision would let one
    -- token authenticate as another session.
    UNIQUE KEY go_refresh_tokens_token_hash_unique (token_hash),
    -- Revoking a family, and listing a user's sessions.
    KEY go_refresh_tokens_family_id_index (family_id),
    KEY go_refresh_tokens_user_id_index (user_id),
    -- Purging expired rows without scanning the table.
    KEY go_refresh_tokens_expires_at_index (expires_at),
    CONSTRAINT go_refresh_tokens_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE go_refresh_tokens;
-- +goose StatementEnd
