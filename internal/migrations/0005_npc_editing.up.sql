-- 0005_npc_editing — moderator-editable NPC configuration (npc-editing-plan.md).
--
-- Postgres is the source of truth for the NPC *definition* (a one-way flow: the
-- web writes cold config; tmServer materializes the live entity). This is NOT
-- live player state, so there is no clobber hazard like inventory/donate_balance.
-- The tmServer only reads these tables (via dbServer); it never writes them.

-- Moderator role gate. 'player' is the default; the web-api authorizes NPC-admin
-- RPCs on role in ('moderator','admin'). Kept as a column (not a separate table)
-- to match the flat `account` model.
ALTER TABLE account ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'player';

-- Editable definition of one NPC/spawn block. `slug` is the stable human id
-- (seeded from NPCGener.txt by `dbserver import-npcs`); `template_name` points at
-- the 816-byte STRUCT_MOB in Release/TMsrv/run/npc/. Position is the spawn point
-- (single global grid — map_id is carried for clarity but the world overlays by
-- x/y only for now, npc-editing-plan.md §9).
CREATE TABLE npc_definition (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug          TEXT NOT NULL UNIQUE,
    template_name TEXT NOT NULL,
    display_name  TEXT NOT NULL DEFAULT '',
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,   -- "aparece ou não"
    map_id        INTEGER NOT NULL DEFAULT 0,
    pos_x         INTEGER NOT NULL DEFAULT 0,      -- "onde fica"
    pos_y         INTEGER NOT NULL DEFAULT 0,
    route_type    SMALLINT NOT NULL DEFAULT 0,
    merchant      SMALLINT NOT NULL DEFAULT 0,     -- 0=none, 1=shop, 2=cargo guard, 19=shop type 3
    updated_by    BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Shop stock of a merchant NPC — overlays the template's Carry[]. Prices are NOT
-- here: the moderator edits the GLOBAL catalog price (item_price), so the same
-- item costs the same at every NPC (npc-editing-plan decision).
CREATE TABLE npc_shop_item (
    npc_id     BIGINT NOT NULL REFERENCES npc_definition(id) ON DELETE CASCADE,
    slot       SMALLINT NOT NULL CHECK (slot BETWEEN 0 AND 26),  -- MSG_ShopList has 27 slots
    item_index INTEGER NOT NULL,
    eff1       SMALLINT NOT NULL DEFAULT 0,
    effv1      SMALLINT NOT NULL DEFAULT 0,
    eff2       SMALLINT NOT NULL DEFAULT 0,
    effv2      SMALLINT NOT NULL DEFAULT 0,
    eff3       SMALLINT NOT NULL DEFAULT 0,
    effv3      SMALLINT NOT NULL DEFAULT 0,
    PRIMARY KEY (npc_id, slot)
);

-- Global per-item price override. NULL/absent = use the content catalog price
-- (itemPrices). The tmServer merges these over its base price map on reload.
CREATE TABLE item_price (
    item_index INTEGER PRIMARY KEY,
    price      BIGINT NOT NULL,
    updated_by BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Single-row hot-reload signal. Bumped in the SAME transaction as every write so
-- the tmServer's cheap version poll detects a change without diffing full state.
CREATE TABLE npc_config_meta (
    id      BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    version BIGINT NOT NULL DEFAULT 0
);
INSERT INTO npc_config_meta (id, version) VALUES (TRUE, 0);

-- Audit trail: moderation is a privileged action, so every mutation records who
-- did what (before/after snapshots as JSON).
CREATE TABLE npc_audit (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    npc_id     BIGINT,                 -- not an FK: survives npc_definition deletes
    account_id BIGINT NOT NULL,        -- the moderator
    action     TEXT NOT NULL,          -- 'create' | 'update' | 'delete' | 'set_shop' | 'set_price' | 'set_visibility'
    before     JSONB,
    after      JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX npc_shop_item_npc_id_idx ON npc_shop_item(npc_id);
CREATE INDEX npc_audit_npc_id_idx ON npc_audit(npc_id);
