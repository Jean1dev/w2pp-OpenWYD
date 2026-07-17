-- 0012_guild_system — durable guild lifecycle, relations, city zones, tower,
-- and Castle/Zakum state (issue #114).
--
-- Legacy WYD spread this across Guilds.txt, GuildInfo, Guild_*/Chall_* files
-- and DBSrv relay messages. The rewrite keeps the same ushort guild ids and
-- member levels, but makes dbServer the transactional authority.

CREATE TABLE guild (
    id         INTEGER PRIMARY KEY CHECK (id > 0 AND id < 65536),
    name       TEXT NOT NULL UNIQUE,
    clan       SMALLINT NOT NULL DEFAULT 0,
    fame       INTEGER NOT NULL DEFAULT 0,
    citizen    SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE guild_member (
    guild_id     INTEGER NOT NULL REFERENCES guild(id) ON DELETE CASCADE,
    character_id BIGINT NOT NULL REFERENCES character(id) ON DELETE CASCADE,
    account_id   BIGINT NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    slot         SMALLINT NOT NULL CHECK (slot BETWEEN 0 AND 3),
    name         TEXT NOT NULL,
    guild_level  SMALLINT NOT NULL CHECK (guild_level BETWEEN 0 AND 9),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (guild_id, character_id),
    UNIQUE (character_id)
);

CREATE INDEX guild_member_account_slot_idx ON guild_member(account_id, slot);
CREATE INDEX guild_member_level_idx ON guild_member(guild_id, guild_level);

CREATE TABLE guild_relation (
    guild_id        INTEGER NOT NULL REFERENCES guild(id) ON DELETE CASCADE,
    target_guild_id INTEGER NOT NULL REFERENCES guild(id) ON DELETE CASCADE,
    kind            SMALLINT NOT NULL CHECK (kind IN (1, 2)), -- 1 ally, 2 war
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (guild_id, target_guild_id),
    CHECK (guild_id <> target_guild_id)
);

CREATE TABLE guild_zone (
    zone             SMALLINT PRIMARY KEY CHECK (zone BETWEEN 0 AND 4),
    charge_guild     INTEGER NOT NULL DEFAULT 0,
    challenge_guild  INTEGER NOT NULL DEFAULT 0,
    clan             SMALLINT NOT NULL DEFAULT 0,
    victory          SMALLINT NOT NULL DEFAULT 0,
    city_tax         SMALLINT NOT NULL DEFAULT 0 CHECK (city_tax BETWEEN 0 AND 30),
    challenge_money  BIGINT NOT NULL DEFAULT 0,
    tax_vault        BIGINT NOT NULL DEFAULT 0,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO guild_zone(zone) VALUES (0), (1), (2), (3), (4);

CREATE TABLE guild_tower_state (
    id              SMALLINT PRIMARY KEY CHECK (id = 1),
    owner_guild     INTEGER NOT NULL DEFAULT 0,
    updated_at_unix BIGINT NOT NULL DEFAULT 0
);

INSERT INTO guild_tower_state(id) VALUES (1);

CREATE TABLE castle_quest_state (
    id          SMALLINT PRIMARY KEY CHECK (id = 1),
    level       INTEGER NOT NULL DEFAULT 0,
    time_left   INTEGER NOT NULL DEFAULT 0,
    clear       BOOLEAN NOT NULL DEFAULT FALSE,
    leader_name TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO castle_quest_state(id) VALUES (1);
