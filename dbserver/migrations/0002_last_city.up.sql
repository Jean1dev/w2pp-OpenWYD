-- last_city: the city (0..3) the character was last in. On login the player
-- spawns at that city's default area (not an exact saved position).
ALTER TABLE character ADD COLUMN last_city SMALLINT NOT NULL DEFAULT 0;
