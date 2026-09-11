-- 0048_npc_generator_off — desligar um bloco do NPCGener sem reiniciar.
--
-- O legado tinha "reloadnpc" (imple.cpp:842): relia o NPCGener.txt do disco. Aqui
-- isso não serve: em produção o Release/ vem dentro da imagem, e o arquivo só
-- muda com um deploy — que reinicia o servidor de qualquer jeito. O que a equipe
-- quer de fato ("desabilitar algum npc usar o comando in game") é uma chave por
-- bloco que vale na hora e sobrevive ao reinício. É esta tabela.
--
-- Uma linha = um bloco DESLIGADO; ligar de novo apaga a linha. O índice é a
-- posição do bloco no NPCGener.txt (Entity.GenIndex), o mesmo que /gm npc mostra.
-- Serve para qualquer bloco — monstro, boss ou mercador do painel.
--
-- Escrita pelo /gm npc off|on (tmServer → dbServer, SetGeneratorOff) e lida AO
-- VIVO: o tmServer pergunta a versão a cada ~15 s, como nas portas das masmorras.

CREATE TABLE npc_generator_off (
    generator_index INTEGER     PRIMARY KEY CHECK (generator_index >= 0),
    turned_off_by   TEXT        NOT NULL DEFAULT '',
    turned_off_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE npc_generator_off_meta (
    id      BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    version BIGINT  NOT NULL DEFAULT 0
);

INSERT INTO npc_generator_off_meta (id, version) VALUES (TRUE, 0);
