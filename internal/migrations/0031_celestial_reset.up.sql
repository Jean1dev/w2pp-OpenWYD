-- Celestial reset counter (MobExtra.QuestInfo.Celestial.Reset, Basedef.h:676):
-- bumped each time a level-199 Celestial with the Cythera Arcana done uses the
-- Pedra da Furia (item 3020) to trade 20M exp for +200 attribute points
-- (_MSG_UseItem.cpp:3579-3615, BASE_GetBonusScorePoint Basedef.cpp:989;
-- issue #408). Capped at 3 by the item. 0 = never reset.
ALTER TABLE character ADD COLUMN IF NOT EXISTS celestial_reset SMALLINT NOT NULL DEFAULT 0;
