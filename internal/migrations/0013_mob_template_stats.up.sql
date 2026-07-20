-- 0013_mob_template_stats — moderator-editable mob/NPC template stats
-- (mob-template-editing-plan.md), the equivalent-tool successor to the legacy
-- EDITAPPMOB (Source/Code/EDITAPPMOB). Sibling of 0005_npc_editing: that
-- migration covers spawn position/visibility/shop for the DB-managed merchant
-- subset (npc_definition/npc_shop_item); this one covers the combat/attribute
-- stats of ANY npc/<template_name> file — the field surface EDITAPPMOB
-- actually edited (Level/HP/MP/attributes/EXP/skills/equip). Postgres is the
-- source of truth for the override; absence of a row means the raw 816-byte
-- STRUCT_MOB file is used unchanged (no behavior change for unedited
-- templates). The tmServer only ever reads these tables (via dbServer) and
-- applies them once at boot — never a hot-reload, matching EDITAPPMOB's own
-- restart-to-apply behavior.

CREATE TABLE mob_template_stat (
    template_name TEXT PRIMARY KEY,
    display_name  TEXT NOT NULL DEFAULT '',  -- '' = keep the template file's own name
    clan          SMALLINT NOT NULL DEFAULT 0,
    merchant      SMALLINT NOT NULL DEFAULT 0, -- STRUCT_MOB top-level Merchant; distinct from
                                                -- npc_definition.merchant (a different concern)
    class         SMALLINT NOT NULL DEFAULT 0,
    coin          INTEGER NOT NULL DEFAULT 0,
    exp           BIGINT NOT NULL DEFAULT 0,
    spx           INTEGER NOT NULL DEFAULT 0,
    spy           INTEGER NOT NULL DEFAULT 0,
    level         INTEGER NOT NULL DEFAULT 0,
    ac            INTEGER NOT NULL DEFAULT 0,
    damage        INTEGER NOT NULL DEFAULT 0,
    chaos_rate    SMALLINT NOT NULL DEFAULT 0,
    attack_run    SMALLINT NOT NULL DEFAULT 0,
    direction     SMALLINT NOT NULL DEFAULT 0,
    str           SMALLINT NOT NULL DEFAULT 0,
    intel         SMALLINT NOT NULL DEFAULT 0, -- "int" is a reserved word
    dex           SMALLINT NOT NULL DEFAULT 0,
    con           SMALLINT NOT NULL DEFAULT 0,
    special1      SMALLINT NOT NULL DEFAULT 0,
    special2      SMALLINT NOT NULL DEFAULT 0,
    special3      SMALLINT NOT NULL DEFAULT 0,
    special4      SMALLINT NOT NULL DEFAULT 0,
    max_hp        INTEGER NOT NULL DEFAULT 0,
    hp            INTEGER NOT NULL DEFAULT 0,
    max_mp        INTEGER NOT NULL DEFAULT 0,
    mp            INTEGER NOT NULL DEFAULT 0,
    learned_skill INTEGER NOT NULL DEFAULT 0,
    score_bonus   INTEGER NOT NULL DEFAULT 0,
    skill_bar1    SMALLINT NOT NULL DEFAULT 0,
    skill_bar2    SMALLINT NOT NULL DEFAULT 0,
    skill_bar3    SMALLINT NOT NULL DEFAULT 0,
    skill_bar4    SMALLINT NOT NULL DEFAULT 0,
    regen_hp      INTEGER NOT NULL DEFAULT 0,
    regen_mp      INTEGER NOT NULL DEFAULT 0,
    resist1       SMALLINT NOT NULL DEFAULT 0,
    resist2       SMALLINT NOT NULL DEFAULT 0,
    resist3       SMALLINT NOT NULL DEFAULT 0,
    resist4       SMALLINT NOT NULL DEFAULT 0,
    updated_by    BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Equip[] slot override for a mob template (0..15, MAX_EQUIP). Same shape as
-- npc_shop_item, minus quantity (equip slots don't stack).
CREATE TABLE mob_template_equip (
    template_name TEXT NOT NULL REFERENCES mob_template_stat(template_name) ON DELETE CASCADE,
    slot          SMALLINT NOT NULL CHECK (slot BETWEEN 0 AND 15),
    item_index    INTEGER NOT NULL,
    eff1          SMALLINT NOT NULL DEFAULT 0,
    effv1         SMALLINT NOT NULL DEFAULT 0,
    eff2          SMALLINT NOT NULL DEFAULT 0,
    effv2         SMALLINT NOT NULL DEFAULT 0,
    eff3          SMALLINT NOT NULL DEFAULT 0,
    effv3         SMALLINT NOT NULL DEFAULT 0,
    PRIMARY KEY (template_name, slot)
);

CREATE INDEX mob_template_equip_template_idx ON mob_template_equip(template_name);
