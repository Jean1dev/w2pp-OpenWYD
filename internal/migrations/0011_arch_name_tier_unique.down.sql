DROP INDEX IF EXISTS character_name_class_master_idx;
ALTER TABLE character ADD CONSTRAINT character_name_key UNIQUE (name);
