-- 0009_daily_reward — daily reward system (issue #35).
--
-- Mirrors 0008_donate_shop's shape: a moderator-managed catalog
-- (daily_reward_item) + its audit trail (daily_reward_audit), reusing the
-- existing delivery_queue mailbox for the actual grant (no schema change
-- there — see 0008_donate_shop.up.sql). The one genuinely new concept is
-- daily_reward_claim: the once-per-account-per-UTC-calendar-day gate. The
-- limit is account-wide (not per item), so the UNIQUE constraint is on
-- (account_id, claim_date) only; reward_item_id is stored for audit/UI
-- purposes ("what did they redeem today") but is not part of the key.

-- Moderator-managed reward catalog. One row = one redeemable offer: an item
-- (item_index + up to three effect/value pairs). Unlike donate_shop_item there
-- is no price column — daily rewards are free, gated only by the once-a-day
-- claim. expires_days > 0 makes the delivered item timed.
CREATE TABLE daily_reward_item (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    item_index   INTEGER NOT NULL,
    eff1         SMALLINT NOT NULL DEFAULT 0,
    effv1        SMALLINT NOT NULL DEFAULT 0,
    eff2         SMALLINT NOT NULL DEFAULT 0,
    effv2        SMALLINT NOT NULL DEFAULT 0,
    eff3         SMALLINT NOT NULL DEFAULT 0,
    effv3        SMALLINT NOT NULL DEFAULT 0,
    title        TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,        -- shown in the web vitrine
    expires_days INTEGER NOT NULL DEFAULT 0 CHECK (expires_days >= 0), -- 0 = permanent
    updated_by   BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Audit trail for reward-item CRUD and claims, mirroring donate_shop_audit.
CREATE TABLE daily_reward_audit (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reward_item_id BIGINT,             -- not an FK: survives daily_reward_item deletes
    account_id     BIGINT NOT NULL,    -- the moderator (CRUD) or claimant (claim)
    action         TEXT NOT NULL,      -- 'create'|'update'|'delete'|'set_enabled'|'claim'
    before         JSONB,
    after          JSONB,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The once-per-account-per-UTC-day gate. claim_date is the UTC calendar day
-- (computed server-side as (now() AT TIME ZONE 'UTC')::date), never a rolling
-- 24h window. reward_item_id records which offer was redeemed (informational;
-- ON DELETE SET NULL so deleting a catalog item never breaks history) but is
-- NOT part of the uniqueness key — the limit is one reward total per account
-- per day, regardless of which item was picked.
CREATE TABLE daily_reward_claim (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id     BIGINT NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    reward_item_id BIGINT REFERENCES daily_reward_item(id) ON DELETE SET NULL,
    claim_date     DATE NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, claim_date)
);

CREATE INDEX daily_reward_audit_reward_item_id_idx ON daily_reward_audit(reward_item_id);
