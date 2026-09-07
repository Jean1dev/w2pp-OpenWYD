-- 0035_dungeon_gate — a porta de cada masmorra instanciada.
--
-- Diferente de TUDO o que a Mesa de XP guarda, isto é lido AO VIVO. A distinção
-- é deliberada e vale a pena escrever: uma tabela de XP trocada com gente
-- jogando pagaria valores diferentes pela mesma morte conforme a hora, então ela
-- só entra no reinício. Uma porta é o contrário — "fecha o Místico agora" que só
-- vale depois do reinício não serve para nada. Por isso esta tabela tem versão
-- própria e o tmServer a repolla, como já faz com a configuração de eventos.
--
-- A ausência de linha é "aberta e avisando", e não o zero do tipo. É o
-- comportamento que o servidor tinha antes disto existir, e uma migração que
-- fechasse tudo calada seria o pior primeiro dia possível. Por isso a tabela
-- nasce VAZIA: nada aqui muda o jogo até alguém clicar.
--
-- gate é o índice de internal/dungeon.Gate. Números só são acrescentados, nunca
-- reordenados — reordenar moveria cada linha gravada para outra masmorra sem um
-- único erro em lugar nenhum. O Cubo da Maldade está de fora de propósito: ele
-- não tem handler de entrada, e uma porta para uma masmorra em que ninguém entra
-- é um controle que não faz nada.

CREATE TABLE dungeon_gate (
    gate       SMALLINT PRIMARY KEY CHECK (gate BETWEEN 0 AND 63),
    open       BOOLEAN NOT NULL DEFAULT TRUE,
    announce   BOOLEAN NOT NULL DEFAULT TRUE,
    updated_by BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE dungeon_gate_meta (
    id      BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    version BIGINT NOT NULL DEFAULT 0
);
INSERT INTO dungeon_gate_meta (id, version) VALUES (TRUE, 0);
