-- Molar de Gargula gate (MobExtra.QuestInfo.Mortal.MolarGargula, Basedef.h:639):
-- set once a Mortal uses the Molar_de_Gargula (item 4122, Vol 194) to raise
-- Equip[1..5] to +6 (_MSG_UseItem.cpp:2494-2546, issue #398). 0 = not used.
ALTER TABLE character ADD COLUMN IF NOT EXISTS molar_gargula SMALLINT NOT NULL DEFAULT 0;
