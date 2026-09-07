-- 0036_quest_reward — a recompensa dos cinco troféus de quest.
--
-- Cemitério, Jardim dos Deuses, Coração do Kaizen, Hidras e Elfos deixam cair um
-- troféu (itens 4117..4121, EF_VOLATILE 191). Usá-lo dá XP e ouro, e é a única
-- recompensa dessas quests. Até aqui os números vinham de
-- Common/Settings/QuestsRate.txt sobre os padrões compilados — ou seja, mudar um
-- deles pedia editar arquivo no servidor e reiniciar, sem registro de quem
-- mudou o quê.
--
-- Lido no BOOT, como a Mesa de XP e pelo mesmo motivo: é número de
-- balanceamento, não chave de operação. Trocar a recompensa com gente jogando
-- faria dois jogadores usarem o mesmo troféu com minutos de diferença e
-- receberem valores diferentes. A porta das masmorras (0035) é o contraste
-- deliberado — aquilo é operação e vale na hora.
--
-- A ausência de linha é "usa o valor do conteúdo", e a tabela nasce vazia: nada
-- aqui muda o jogo até alguém gravar.
--
-- As faixas de nível são meia-abertas, como o legado as usa: min inclusivo, max
-- exclusivo. Elas ficam editáveis junto porque são a outra metade da mesma
-- decisão — de que serve dobrar a XP do Cemitério se a faixa continua acabando
-- no 115.

CREATE TABLE quest_reward (
    tier        SMALLINT PRIMARY KEY CHECK (tier BETWEEN 0 AND 4),
    mortal_exp  BIGINT  NOT NULL DEFAULT 0 CHECK (mortal_exp  >= 0),
    arch_exp    BIGINT  NOT NULL DEFAULT 0 CHECK (arch_exp    >= 0),
    coin        INTEGER NOT NULL DEFAULT 0 CHECK (coin        >= 0),
    mortal_min  INTEGER NOT NULL DEFAULT 0 CHECK (mortal_min  >= 0),
    mortal_max  INTEGER NOT NULL DEFAULT 0 CHECK (mortal_max  >= 0),
    arch_min    INTEGER NOT NULL DEFAULT 0 CHECK (arch_min    >= 0),
    arch_max    INTEGER NOT NULL DEFAULT 0 CHECK (arch_max    >= 0),
    updated_by  BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Uma faixa invertida não recusa ninguém: ela recusa TODO MUNDO, calada, e
    -- o jogador só vê o troféu não fazer nada. Barrar aqui é mais barato que
    -- descobrir depois.
    CONSTRAINT quest_reward_faixa_mortal CHECK (mortal_max > mortal_min),
    CONSTRAINT quest_reward_faixa_arch   CHECK (arch_max   > arch_min)
);

CREATE TABLE quest_reward_meta (
    id      BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    version BIGINT NOT NULL DEFAULT 0
);
INSERT INTO quest_reward_meta (id, version) VALUES (TRUE, 0);
