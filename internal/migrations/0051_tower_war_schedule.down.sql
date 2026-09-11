ALTER TABLE world_event_config
    DROP COLUMN IF EXISTS tower_war_hour,
    DROP COLUMN IF EXISTS tower_war_enabled;
