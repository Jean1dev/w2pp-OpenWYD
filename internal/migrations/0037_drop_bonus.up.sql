-- 0037_drop_bonus — as escadas do sorteio de bônus de drop.
--
-- Todo equipamento que cai de monstro passa por um sorteio que escreve os três
-- espaços de efeito daquela cópia (SetItemBonus, portado em
-- tmserver/internal/refine/dropbonus.go). Duas escadas decidem como o drop se
-- sente: quanto vale o bônus, e com que frequência sai refino.
--
-- Até aqui esses números eram literais no código. Mudar a generosidade do drop
-- pedia um deploy, e não havia registro de quem mudou o quê.
--
-- Lido no BOOT, como a Mesa de XP e a recompensa das quests, e pelo mesmo
-- motivo: é número de balanceamento, não chave de operação. Trocar a escada com
-- gente jogando faria dois jogadores matarem o mesmo monstro com minutos de
-- diferença e receberem itens de gerações diferentes, sem nada na tela que
-- explicasse a diferença.
--
-- A ausência de linha é "usa o valor do legado", e a tabela nasce vazia: nada
-- aqui muda o jogo até alguém gravar.
--
-- O que NÃO fica editável, de propósito: qual efeito cada peça recebe. Isso é
-- conteúdo — a armadura sempre dá crítico, a bota sempre dá dano adicional —, e
-- mexer nisso muda o que o jogo é, não o quanto ele é generoso.

CREATE TABLE drop_bonus (
    distancia   SMALLINT PRIMARY KEY CHECK (distancia BETWEEN 0 AND 3),

    -- Escada da magnitude. Um sorteio de 0 a 99 é comparado com os quatro
    -- limites em ordem; o primeiro que ele não alcança escolhe o degrau. O
    -- valor que entra no item é o degrau vezes o multiplicador da peça, então
    -- degrau 0 quer dizer que o item não ganha nada.
    mag_limite1 SMALLINT NOT NULL CHECK (mag_limite1 BETWEEN 0 AND 100),
    mag_limite2 SMALLINT NOT NULL CHECK (mag_limite2 BETWEEN 0 AND 100),
    mag_limite3 SMALLINT NOT NULL CHECK (mag_limite3 BETWEEN 0 AND 100),
    mag_limite4 SMALLINT NOT NULL CHECK (mag_limite4 BETWEEN 0 AND 100),
    mag_degrau1 SMALLINT NOT NULL CHECK (mag_degrau1 BETWEEN 0 AND 20),
    mag_degrau2 SMALLINT NOT NULL CHECK (mag_degrau2 BETWEEN 0 AND 20),
    mag_degrau3 SMALLINT NOT NULL CHECK (mag_degrau3 BETWEEN 0 AND 20),
    mag_degrau4 SMALLINT NOT NULL CHECK (mag_degrau4 BETWEEN 0 AND 20),
    mag_degrau5 SMALLINT NOT NULL CHECK (mag_degrau5 BETWEEN 0 AND 20),

    -- Faixas do espaço do refino, na mesma leitura: abaixo da primeira sai +2,
    -- da segunda +1, da terceira +0, da quarta um bônus especial, e daí em
    -- diante nada. Um limite de 100 apaga tudo o que vem depois dele — é assim
    -- que o legado diz "nesta faixa todo drop leva alguma coisa".
    ref_dois     SMALLINT NOT NULL CHECK (ref_dois     BETWEEN 0 AND 100),
    ref_um       SMALLINT NOT NULL CHECK (ref_um       BETWEEN 0 AND 100),
    ref_zero     SMALLINT NOT NULL CHECK (ref_zero     BETWEEN 0 AND 100),
    ref_especial SMALLINT NOT NULL CHECK (ref_especial BETWEEN 0 AND 100),

    updated_by  BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Limite fora de ordem não recusa nada: ele apaga em silêncio um degrau
    -- inteiro da escada, e ninguém descobre olhando a tela. Barrar aqui custa
    -- menos que descobrir depois que o +1 sumiu do servidor por uma semana.
    CONSTRAINT drop_bonus_ordem_mag CHECK (
        mag_limite1 <= mag_limite2 AND mag_limite2 <= mag_limite3 AND mag_limite3 <= mag_limite4
    ),
    CONSTRAINT drop_bonus_ordem_ref CHECK (
        ref_dois <= ref_um AND ref_um <= ref_zero AND ref_zero <= ref_especial
    )
);

CREATE TABLE drop_bonus_meta (
    id      BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    version BIGINT NOT NULL DEFAULT 0,
    -- O sorteio faltou a vida inteira deste rewrite. Desligar tem que ser um
    -- clique e não um deploy, caso ele caia mal num servidor com gente dentro.
    ligado  BOOLEAN NOT NULL DEFAULT TRUE
);
INSERT INTO drop_bonus_meta (id, version, ligado) VALUES (TRUE, 0, TRUE);
