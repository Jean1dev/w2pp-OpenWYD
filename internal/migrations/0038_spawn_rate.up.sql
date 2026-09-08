-- 0038_spawn_rate — o ritmo de renascimento dos mobs por área.
--
-- O deserto nasce de 253 blocos do NPCGener.txt, e eles não têm um período só:
-- 2, 3 ou 4 minutos conforme o grupo, mais uma dúzia de blocos sem período
-- nenhum, que caem na fila individual de 15s do tmServer. Guardar um número
-- absoluto aqui achataria essa diferença e apagaria a intenção de quem escreveu
-- o conteúdo — o chefe que demora mais que o lixo em volta viraria lixo. Por
-- isso o que se guarda é uma PORCENTAGEM sobre o período de cada bloco: 100 é o
-- que o arquivo diz, 200 é o dobro do tempo (up mais lento), 50 é metade.
--
-- Isto é lido AO VIVO, como dungeon_gate e diferente da Mesa de XP. A distinção
-- que a migração 0035 escreve continua valendo: uma tabela de XP trocada com
-- gente jogando pagaria valores diferentes pela mesma morte conforme a hora. Um
-- tempo de spawn não tem esse problema — o mob só aparece mais cedo ou mais
-- tarde, e ninguém recebe nada diferente por isso — e é exatamente o tipo de
-- botão que se quer girar e ver acontecendo.
--
-- area é o índice de internal/spawnrate.Area. Números só são acrescentados.
-- A tabela nasce VAZIA: ausência de linha é 100%, e nada aqui muda o jogo até
-- alguém clicar.

CREATE TABLE spawn_rate (
    area       SMALLINT PRIMARY KEY CHECK (area BETWEEN 0 AND 63),
    percent    INTEGER NOT NULL CHECK (percent BETWEEN 10 AND 1000),
    updated_by BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE spawn_rate_meta (
    id      BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    version BIGINT NOT NULL DEFAULT 0
);
INSERT INTO spawn_rate_meta (id, version) VALUES (TRUE, 0);
