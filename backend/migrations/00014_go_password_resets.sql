-- Password reset tokens for the Go API's own forgot-password flow.
--
-- Password reset was the last thing only Laravel could do: it needs to send mail, and the
-- Go API had no sender. With mailer.Sender in place the flow moves here, and this is the
-- table it needs.
--
-- WHY NOT REUSE Laravel's password_resets
--
-- Two reasons, either of them sufficient.
--
-- The first is ownership. That table belongs to Laravel's migration history, and the two
-- histories must never manage the same table — a change on one side would be invisible to
-- the other, and a rollback on one side could drop a table the other depends on.
--
-- The second is shape. Laravel stored the token bcrypt-hashed and keyed the row by e-mail
-- address, so verifying a link meant selecting every row for that address and running
-- bcrypt against each. The token is 32 random bytes, so there is no dictionary to defend
-- against and no reason to pay bcrypt: SHA-256 with a unique index is one index lookup.
-- That is the same argument the refresh tokens make in 00008.
--
-- Links Laravel had already mailed and nobody had used yet stop working when traffic
-- moves here. They expire in an hour, so the window is small and the cost is one more
-- click on "forgot my password".
--
-- WHY used_at RATHER THAN DELETING THE ROW
--
-- A reset link must work once. Deleting the row on use would do that, but it would also
-- erase the evidence that it happened, and the distinction between "never existed",
-- "expired" and "already used" is what makes an abuse report answerable. The API still
-- answers all three the same way to the caller.
--
-- ONE ROW PER REQUEST, NOT ONE PER ACCOUNT
--
-- Laravel replaced the account's row on every request. Here every request inserts, so
-- requested_at is a real history: it is what the sixty-second throttle reads, and what
-- would show a pattern of requests against one account.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE go_password_resets (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    -- SHA-256 of the opaque token, hex encoded. The token itself is never stored, so a
    -- copy of this table is not a set of usable reset links.
    token_hash CHAR(64) NOT NULL,
    requested_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    -- Set when the link is used. A second use finds it non-null and is refused.
    used_at TIMESTAMP NULL DEFAULT NULL,
    -- Audit only, like go_refresh_tokens.created_ip. Never used to authorise.
    requested_ip VARCHAR(45) NULL DEFAULT NULL,
    PRIMARY KEY (id),
    -- The lookup every reset does. Unique because a collision would point one link at
    -- another account.
    UNIQUE KEY go_password_resets_token_hash_unique (token_hash),
    -- The throttle reads the newest request for one account.
    KEY go_password_resets_user_id_requested_at_index (user_id, requested_at),
    CONSTRAINT go_password_resets_user_id_foreign
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE go_password_resets;
-- +goose StatementEnd
