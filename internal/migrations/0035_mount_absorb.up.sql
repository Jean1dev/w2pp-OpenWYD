-- 0035_mount_absorb — quanto do dano a montaria adulta come no lugar do dono,
-- separado por quem está batendo: outro jogador ou um monstro.
--
-- O legado já absorve. Em _MSG_Attack.cpp:1520-1533 o dano vira (dam*3)>>2 se o
-- alvo tem montaria adulta viva equipada — 25% fixos — e metade do que foi
-- absorvido é descontado do HP da própria montaria (ProcessAdultMount,
-- Server.cpp:4718). O mesmo bloco está repetido nas duas cadeias de ataque
-- (Server.cpp:10024, ProcessSecMinTimer.cpp:2294), então vale igual em PvP e em
-- PvE. Este rewrite nunca portou nada disso: até aqui montaria não reduzia dano
-- nenhum.
--
-- O que esta tabela acrescenta ao legado é UM eixo: o 25 único vira dois
-- números por linhagem. É o que deixa existir montaria de PvE e montaria de PvP
-- — a mesma peça de conteúdo passando a ter uma escolha, em vez de trinta
-- linhagens idênticas na defesa.
--
-- Por que POR LINHAGEM e não por faixa de nível, como a curva de crescimento
-- (0030): a curva descreve uma JORNADA, e só uma sequência consegue dizer "fácil
-- de começar, difícil de terminar". A absorção descreve o que a montaria É. Duas
-- linhas por montaria seria a mesma informação com seis vezes mais campos para
-- alguém preencher errado.
--
-- Por que atacante e não elemento/tipo de dano: é a única distinção que o motor
-- de dano já sabe fazer sozinho. O legado a espalha como aritmética de índice
-- (`< MAX_USER` em uns dezesseis pontos do cálculo); aqui ela é um campo,
-- TargetIsPlayer, e quem chama já sabe se quem bateu é jogador ou monstro.
--
-- A AUSÊNCIA de linha significa "usa o padrão do código" (25/25, o legado), não
-- "absorção zero" — a mesma regra de 0023, 0029 e 0030. Um servidor que nunca
-- abriu esta tela fica exatamente com o comportamento do legado, e ninguém
-- descobre por acidente que deixou uma linhagem sem defesa nenhuma.
--
-- Sem hot-reload, como todo o overlay de conteúdo: o tmServer lê no boot.

CREATE TABLE IF NOT EXISTS mount_absorb (
    mount_index SMALLINT NOT NULL CHECK (mount_index BETWEEN 2360 AND 2389),
    -- 0..100, percentual do golpe que a montaria come. 0 é uma configuração
    -- legítima ("esta linhagem não defende disso"), e é por isso que a ausência
    -- de linha precisa ser diferente de zero.
    --
    -- O teto é 100 e não menos porque quem opera tem direito de fazer uma
    -- montaria de evento absurda; o que o servidor garante é que o golpe nunca
    -- fica negativo, não que ele continue doendo.
    absorb_pvp  SMALLINT NOT NULL CHECK (absorb_pvp BETWEEN 0 AND 100),
    absorb_pve  SMALLINT NOT NULL CHECK (absorb_pve BETWEEN 0 AND 100),
    updated_by  TEXT     NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (mount_index)
);
