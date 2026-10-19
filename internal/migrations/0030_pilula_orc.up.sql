-- Pilula do Poder Magico gate (MobExtra.QuestInfo.Mortal.PilulaOrc, Basedef.h:640):
-- set once a Mortal picks up or uses the Pilula do Poder Magico (item 470, Vol 6)
-- for +9 SkillBonus (_MSG_GetItem.cpp:94-126, _MSG_UseItem.cpp:981-998,
-- BASE_GetBonusSkillPoint Basedef.cpp:865-868; issue #404). 0 = not used.
ALTER TABLE character ADD COLUMN IF NOT EXISTS pilula_orc SMALLINT NOT NULL DEFAULT 0;
