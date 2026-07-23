-- 0015_world_event_config -- portal-managed global world events (issue #116).
--
-- The portal writes this cold configuration through web-api. tmServer reads it
-- through dbServer, applies it inside the single-owner loop, and only writes
-- back event progress (current_index) after successful event drops.

CREATE TABLE world_event_config (
    id                    BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    enabled               BOOLEAN NOT NULL DEFAULT FALSE,
    item_index            INTEGER NOT NULL DEFAULT 0 CHECK (item_index >= 0),
    rate                  INTEGER NOT NULL DEFAULT 0 CHECK (rate >= 0),
    start_index           INTEGER NOT NULL DEFAULT 0 CHECK (start_index >= 0),
    current_index         INTEGER NOT NULL DEFAULT 0 CHECK (current_index >= 0),
    end_index             INTEGER NOT NULL DEFAULT 0 CHECK (end_index >= 0),
    indexed               BOOLEAN NOT NULL DEFAULT FALSE,
    notice_enabled        BOOLEAN NOT NULL DEFAULT TRUE,
    double_exp_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    newbie_event_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
    updated_by            BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO world_event_config (id) VALUES (TRUE);

CREATE TABLE world_event_meta (
    id      BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    version BIGINT NOT NULL DEFAULT 0
);
INSERT INTO world_event_meta (id, version) VALUES (TRUE, 0);

CREATE TABLE world_event_audit (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL,
    action     TEXT NOT NULL,
    before     JSONB,
    after      JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
