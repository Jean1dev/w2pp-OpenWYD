ALTER TABLE character ADD COLUMN arch_crystal_stage SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE character ADD CONSTRAINT character_arch_crystal_stage_check CHECK (arch_crystal_stage BETWEEN 0 AND 4);
