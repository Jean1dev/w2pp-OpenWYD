-- Celestial tier quest gates (MobExtra.QuestInfo.Celestial, Basedef.h:659-678).
-- celestial_lv40 / celestial_lv90 unlock the level 40/90 caps on the Celestial curve
-- (set by /destravar40 and /destravar90); celestial_circle marks the Cythera Arcana
-- quest done (set by /arcana). class_master (tier) already exists; it is now written
-- by the in-game save so tier transformations persist.
ALTER TABLE character ADD COLUMN IF NOT EXISTS celestial_lv40   SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE character ADD COLUMN IF NOT EXISTS celestial_lv90   SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE character ADD COLUMN IF NOT EXISTS celestial_circle SMALLINT NOT NULL DEFAULT 0;
