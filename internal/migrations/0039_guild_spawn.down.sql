-- Volta ao estado sem ponto de renascimento por guild: a guild dona passa a
-- renascer no spawn da cidade, como todo mundo.
ALTER TABLE guild_zone DROP CONSTRAINT IF EXISTS guild_zone_spawn_check;
ALTER TABLE guild_zone DROP COLUMN IF EXISTS guild_spawn_x;
ALTER TABLE guild_zone DROP COLUMN IF EXISTS guild_spawn_y;
