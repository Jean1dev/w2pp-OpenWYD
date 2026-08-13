-- Persist the complete NPCGener recipe for content-owned merchant blocks.
ALTER TABLE npc_definition ADD COLUMN origin TEXT NOT NULL DEFAULT 'custom'
    CHECK (origin IN ('content', 'custom'));
ALTER TABLE npc_definition ADD COLUMN generator_index INTEGER;
ALTER TABLE npc_definition ADD COLUMN follower_template TEXT NOT NULL DEFAULT '';
ALTER TABLE npc_definition ADD COLUMN minute_generate INTEGER NOT NULL DEFAULT 0;
ALTER TABLE npc_definition ADD COLUMN min_group INTEGER NOT NULL DEFAULT 0;
ALTER TABLE npc_definition ADD COLUMN max_group INTEGER NOT NULL DEFAULT 0;
ALTER TABLE npc_definition ADD COLUMN max_num_mob INTEGER NOT NULL DEFAULT 1;
ALTER TABLE npc_definition ADD COLUMN formation SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE npc_definition ADD COLUMN generator_data JSONB NOT NULL DEFAULT '{}';
CREATE UNIQUE INDEX npc_definition_generator_index_idx
    ON npc_definition(generator_index) WHERE generator_index IS NOT NULL;

-- Definitions shipped by the old curated seed are content, not moderator-created.
UPDATE npc_definition SET origin = 'content'
WHERE slug IN (SELECT slug FROM npc_definition WHERE updated_by IS NULL);
