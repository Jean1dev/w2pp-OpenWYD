ALTER TABLE character DROP CONSTRAINT IF EXISTS character_sub_celestial_coerente;
ALTER TABLE character
    DROP COLUMN IF EXISTS sub_celestial_guardada,
    DROP COLUMN IF EXISTS sub_celestial_level,
    DROP COLUMN IF EXISTS sub_celestial_ativo,
    DROP COLUMN IF EXISTS celestial_reset;
