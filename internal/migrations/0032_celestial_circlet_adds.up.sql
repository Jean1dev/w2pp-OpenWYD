-- Sub-Celestial diadem adds (MobExtra.QuestInfo.Celestial.Add120/150/180/200,
-- Basedef.h:667-670): SetCircletSubGod (Server.cpp:9794-9851) writes EF_AC/EF_HP into
-- Equip[1] when a SCELESTIAL reaches stored level 120, 150, 180 and 199, and
-- each flag makes sure that add is granted only once (issue #409). 0 = not yet.
ALTER TABLE character ADD COLUMN IF NOT EXISTS celestial_add120 SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE character ADD COLUMN IF NOT EXISTS celestial_add150 SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE character ADD COLUMN IF NOT EXISTS celestial_add180 SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE character ADD COLUMN IF NOT EXISTS celestial_add200 SMALLINT NOT NULL DEFAULT 0;
