-- 0008_donate_shop — donate currency web shop (issue #34).
--
-- Two cold-config tables the web-api owns (donate_shop_item catalog + audit) and
-- the delivery_queue mailbox that carries GRANTS into the game without the web
-- ever writing live state (web-platform-plan.md §"entrega assíncrona"). The
-- account.donate_balance wallet column already exists (0001_init); crediting it
-- is a partial UPDATE that no tmServer save clobbers (SaveCharacter never touches
-- account; SaveCargo writes only cargo_coin). The account.role gate from 0005 is
-- reused to authorize the moderator CRUD — no new role column here.

-- Moderator-managed shop catalog. One row = one purchasable offer: an item
-- (item_index + up to three effect/value pairs) sold for `price` donate. Unlike
-- npc_shop_item the price lives HERE (donate is the shop's own currency, not the
-- global gold catalog). expires_days > 0 makes the delivered item timed.
CREATE TABLE donate_shop_item (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    item_index   INTEGER NOT NULL,
    eff1         SMALLINT NOT NULL DEFAULT 0,
    effv1        SMALLINT NOT NULL DEFAULT 0,
    eff2         SMALLINT NOT NULL DEFAULT 0,
    effv2        SMALLINT NOT NULL DEFAULT 0,
    eff3         SMALLINT NOT NULL DEFAULT 0,
    effv3        SMALLINT NOT NULL DEFAULT 0,
    price        INTEGER NOT NULL CHECK (price > 0),   -- cost in donate currency
    title        TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,        -- shown in the web vitrine
    expires_days INTEGER NOT NULL DEFAULT 0 CHECK (expires_days >= 0), -- 0 = permanent
    updated_by   BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Audit trail for privileged donate actions: shop-item CRUD and manual balance
-- credits. account_id is the acting account (moderator for CRUD, buyer for a
-- purchase). No config_meta version table — the tmServer never reads the catalog,
-- so there is nothing to hot-reload.
CREATE TABLE donate_shop_audit (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    shop_item_id BIGINT,               -- not an FK: survives donate_shop_item deletes
    account_id   BIGINT NOT NULL,      -- the moderator (CRUD/credit) or buyer (purchase)
    action       TEXT NOT NULL,        -- 'create'|'update'|'delete'|'set_enabled'|'credit_balance'|'purchase'
    before       JSONB,
    after        JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The async grant mailbox (web-platform-plan.md §"entrega assíncrona (mailbox)").
-- Anything that GRANTS item/cash/coin INSERTs here; the tmServer drains it inside
-- its single-owner loop and is the only writer of live state. Issue #34 uses only
-- kind='item' (a donate-shop purchase); cash/coin/daily-reward kinds are reserved
-- for later. payload for kind='item' = {item_index, eff1..effv3, expires_at}.
CREATE TABLE delivery_queue (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id   BIGINT NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    character_id BIGINT,               -- NULL = any character of the account
    kind         TEXT NOT NULL,        -- 'item' | 'cash' | 'coin'
    payload      JSONB NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending', -- 'pending' | 'delivered' | 'lost'
    source       TEXT,                 -- provenance, e.g. 'donate_shop:<item_id>'
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The drain query filters by (account_id, status='pending'); the audit index
-- mirrors npc_audit's.
CREATE INDEX delivery_queue_account_status_idx ON delivery_queue(account_id, status);
CREATE INDEX donate_shop_audit_shop_item_id_idx ON donate_shop_audit(shop_item_id);
