-- One-time login codes for the web client (web client ADR 017). The portal asks
-- the webserver for a code; the game login (dbserver AccountLogin) accepts it
-- once in place of the password. Only the SHA-256 of the random code is kept.
CREATE TABLE account_play_code (
    code_hash   BYTEA PRIMARY KEY,
    account_id  BIGINT NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX account_play_code_account_idx ON account_play_code (account_id);
