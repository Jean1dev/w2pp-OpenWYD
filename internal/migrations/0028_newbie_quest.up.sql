-- Newbie quest progress (MobExtra.QuestInfo.Mortal.Newbie, Basedef.h:637):
-- the Campo de Treino step a Mortal has completed with the four Treinadores
-- (_MSG_Quest.cpp:1895-2100, issue #395). 0 = not started, 4 = finished.
ALTER TABLE character ADD COLUMN IF NOT EXISTS newbie_quest SMALLINT NOT NULL DEFAULT 0;
