-- Terra Mistica quest gate (MobExtra.QuestInfo.Mortal.TerraMistica,
-- _MSG_Quest.cpp AMU_MISTICO, issue #139): set once the Mortal party quest is
-- completed, so the NPC won't hand it out twice.
ALTER TABLE character ADD COLUMN IF NOT EXISTS terra_mistica SMALLINT NOT NULL DEFAULT 0;
