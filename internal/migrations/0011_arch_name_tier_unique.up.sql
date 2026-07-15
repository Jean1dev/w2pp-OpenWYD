-- Arch characters inherit the Mortal name in the legacy account file. Keep
-- names unique per tier so the Mortal/Arch pair can coexist without allowing two
-- Mortals with the same name.
ALTER TABLE character DROP CONSTRAINT IF EXISTS character_name_key;
UPDATE character SET class_master = 2 WHERE class_master = 0;
CREATE UNIQUE INDEX IF NOT EXISTS character_name_class_master_idx
    ON character (name, class_master);
