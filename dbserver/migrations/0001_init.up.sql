-- 0001_init — initial schema for migrated WYD accounts (data-formats.md §4).
-- Fixed-size C arrays are normalized; empty item slots are not stored.
-- Secrets are stored ONLY as argon2id hashes (never plaintext).

CREATE TABLE account (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,          -- canonical lowercase login
    pass_hash       TEXT NOT NULL,
    pin_hash        TEXT NOT NULL DEFAULT '',
    block_pass_hash TEXT NOT NULL DEFAULT '',
    real_name       TEXT NOT NULL DEFAULT '',
    email           TEXT NOT NULL DEFAULT '',
    telephone       TEXT NOT NULL DEFAULT '',
    address         TEXT NOT NULL DEFAULT '',
    ssn1            INTEGER NOT NULL DEFAULT 0,
    ssn2            INTEGER NOT NULL DEFAULT 0,
    donate_balance  INTEGER NOT NULL DEFAULT 0,
    cargo_coin      INTEGER NOT NULL DEFAULT 0,
    is_blocked      BOOLEAN NOT NULL DEFAULT FALSE,
    year            INTEGER NOT NULL DEFAULT 0,
    year_day        INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE character (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id     BIGINT NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    slot           SMALLINT NOT NULL CHECK (slot BETWEEN 0 AND 3),
    name           TEXT NOT NULL UNIQUE,
    class          SMALLINT NOT NULL DEFAULT 0,
    clan           SMALLINT NOT NULL DEFAULT 0,
    guild_id       INTEGER NOT NULL DEFAULT 0,
    guild_level    SMALLINT NOT NULL DEFAULT 0,
    level          INTEGER NOT NULL DEFAULT 0,
    exp            BIGINT NOT NULL DEFAULT 0,
    coin           INTEGER NOT NULL DEFAULT 0,
    str            SMALLINT NOT NULL DEFAULT 0,
    int            SMALLINT NOT NULL DEFAULT 0,
    dex            SMALLINT NOT NULL DEFAULT 0,
    con            SMALLINT NOT NULL DEFAULT 0,
    score_bonus    INTEGER NOT NULL DEFAULT 0,
    special_bonus  INTEGER NOT NULL DEFAULT 0,
    skill_bonus    INTEGER NOT NULL DEFAULT 0,
    max_hp         INTEGER NOT NULL DEFAULT 0,
    max_mp         INTEGER NOT NULL DEFAULT 0,
    hp             INTEGER NOT NULL DEFAULT 0,
    mp             INTEGER NOT NULL DEFAULT 0,
    critical       SMALLINT NOT NULL DEFAULT 0,
    regen_hp       INTEGER NOT NULL DEFAULT 0,
    regen_mp       INTEGER NOT NULL DEFAULT 0,
    resist_fire    SMALLINT NOT NULL DEFAULT 0,
    resist_ice     SMALLINT NOT NULL DEFAULT 0,
    resist_thunder SMALLINT NOT NULL DEFAULT 0,
    resist_magic   SMALLINT NOT NULL DEFAULT 0,
    learned_skill  INTEGER NOT NULL DEFAULT 0,
    magic          BIGINT NOT NULL DEFAULT 0,
    save_x         INTEGER NOT NULL DEFAULT 0,
    save_y         INTEGER NOT NULL DEFAULT 0,
    citizen        SMALLINT NOT NULL DEFAULT 0,
    class_master   SMALLINT NOT NULL DEFAULT 0,
    skill_bar      SMALLINT[] NOT NULL DEFAULT '{}',  -- SkillBar[4]
    short_skill    SMALLINT[] NOT NULL DEFAULT '{}',  -- ShortSkill[16]
    UNIQUE (account_id, slot)
);

CREATE INDEX character_account_id_idx ON character(account_id);

-- Normalizes Equip[16] + Carry[64] (per character) and Cargo[128] (per account).
CREATE TYPE item_owner_kind AS ENUM ('char_equip', 'char_carry', 'account_cargo');

CREATE TABLE item (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_kind   item_owner_kind NOT NULL,
    account_id   BIGINT REFERENCES account(id) ON DELETE CASCADE,
    character_id BIGINT REFERENCES character(id) ON DELETE CASCADE,
    slot         SMALLINT NOT NULL,
    item_index   SMALLINT NOT NULL,
    eff1         SMALLINT NOT NULL DEFAULT 0,
    effv1        SMALLINT NOT NULL DEFAULT 0,
    eff2         SMALLINT NOT NULL DEFAULT 0,
    effv2        SMALLINT NOT NULL DEFAULT 0,
    eff3         SMALLINT NOT NULL DEFAULT 0,
    effv3        SMALLINT NOT NULL DEFAULT 0,
    -- exactly one owner per kind
    CHECK ((owner_kind = 'account_cargo') = (account_id IS NOT NULL AND character_id IS NULL)),
    CHECK ((owner_kind IN ('char_equip','char_carry')) = (character_id IS NOT NULL))
);

CREATE INDEX item_character_id_idx ON item(character_id);
CREATE INDEX item_account_id_idx ON item(account_id);

-- Persisted buffs (affect[char][32]).
CREATE TABLE affect (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    character_id BIGINT NOT NULL REFERENCES character(id) ON DELETE CASCADE,
    type         SMALLINT NOT NULL,
    value        SMALLINT NOT NULL,
    level        INTEGER NOT NULL,
    time         BIGINT NOT NULL
);

CREATE INDEX affect_character_id_idx ON affect(character_id);
