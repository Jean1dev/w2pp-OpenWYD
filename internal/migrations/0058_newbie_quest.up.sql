-- 0058_newbie_quest — o passo da quest dos Treinadores do campo de treino.
--
-- MobExtra.QuestInfo.Mortal.Newbie (_MSG_Quest.cpp:1896-2100): 0 = nada feito,
-- 1/2/3 = Treinador 1/2/3 atendidos, 4 = Chefe de Treino. Cada passo exige o
-- anterior e a chave do portão correspondente, então sem guardar isto o jogador
-- refaria o passo 1 para sempre e nunca chegaria ao 2.
--
-- SMALLINT como os outros portões de quest (0013 terra_mistica): o valor cabe em
-- um byte e o proto o carrega como int32.

ALTER TABLE character ADD COLUMN IF NOT EXISTS newbie_quest SMALLINT NOT NULL DEFAULT 0;
