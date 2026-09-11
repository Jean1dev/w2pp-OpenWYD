-- 0049_drop_rule — a Mesa de Drops: a chance exata de um item cair de um monstro.
--
-- O loot de cada monstro mora no arquivo do template (Carry, 64 slots), cada
-- slot numa chance fixa da g_pDropRate dobrada pelo nível do monstro. Essa
-- tabela não tem 10% para um chefe de nível alto — um slot ali cai a 100%, 25%,
-- 2,86% ou 0,2% para baixo —, e mudar um slot era editar o arquivo. Uma linha
-- aqui diz direto: "o Ovo de Fenrir cai do chefe do Pesadelo N a 8%"
-- (internal/droprule).
--
-- Uma linha para (monstro, item) SUBSTITUI o que o template faz com aquele
-- item: os slots do Carry que o têm são pulados, e a linha rola uma vez, na sua
-- chance. 0% tira o item daquele monstro. mob = '*' é "todos os monstros" e só
-- aceita 0% — tira um item do mapa inteiro —, e uma linha de monstro nomeado
-- passa por cima dela.
--
-- A chance é em centésimos de por cento (800 = 8%) e é exata: o bônus de drop
-- de quem mata não mexe nela.
--
-- Lido AO VIVO, como a regra de combate: o tmServer pergunta a versão a cada
-- ~15 s e relê a tabela quando ela muda. Montar a mesa antes do lançamento
-- olhando o jogo não pode custar um reinício por linha.
--
-- A tabela nasce VAZIA, e ausência de linha é o template valendo — como sempre
-- foi. Os CHECKs repetem as faixas de internal/droprule, e
-- drop_rule_range_test.go confere que os dois continuam batendo.

CREATE TABLE drop_rule (
    mob        TEXT     NOT NULL CHECK (length(mob) BETWEEN 1 AND 64),
    item       SMALLINT NOT NULL CHECK (item BETWEEN 391 AND 6499),
    chance     INTEGER  NOT NULL CHECK (chance BETWEEN 0 AND 10000),
    updated_by BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (mob, item),
    CHECK (mob <> '*' OR chance = 0)
);

CREATE TABLE drop_rule_meta (
    id      BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    version BIGINT NOT NULL DEFAULT 0
);
INSERT INTO drop_rule_meta (id, version) VALUES (TRUE, 0);
