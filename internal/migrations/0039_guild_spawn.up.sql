-- Onde renasce quem é da guild dona da cidade.
--
-- O legado tem os campos GuildSpawnX/Y na struct da zona (Basedef.h:956) e os
-- LÊ no respawn (Server.cpp:8514) e no login (ProcessDBMessage.cpp:911) — mas
-- nunca os escreve: não há uma única atribuição em todo o Source/, e o arquivo
-- de configuração da zona não traz o campo. Ficavam sempre zero, então a guild
-- dominante renasceria em (0,0), no canto do mapa. Portar aquilo como está
-- seria copiar o defeito.
--
-- Aqui o ponto é dado, e por isso fica no banco em vez de constante: cada
-- servidor escolhe onde é a base da guild em cada cidade, e muda sem recompilar.
--
-- O padrão é o próprio spawn da cidade (world.cities), de modo que ligar a
-- migração não move ninguém: até alguém editar, a guild dona renasce onde
-- renascia. Zero em qualquer um dos eixos significa "não configurado" e cai no
-- spawn da cidade — é o que torna a coluna segura de existir antes da tela.
ALTER TABLE guild_zone ADD COLUMN IF NOT EXISTS guild_spawn_x INTEGER NOT NULL DEFAULT 0;
ALTER TABLE guild_zone ADD COLUMN IF NOT EXISTS guild_spawn_y INTEGER NOT NULL DEFAULT 0;

ALTER TABLE guild_zone DROP CONSTRAINT IF EXISTS guild_zone_spawn_check;
ALTER TABLE guild_zone ADD CONSTRAINT guild_zone_spawn_check
    CHECK (guild_spawn_x BETWEEN 0 AND 4095 AND guild_spawn_y BETWEEN 0 AND 4095);

-- Os spawns de cidade de tmserver/internal/world/city.go, na mesma ordem de
-- zona: Armia, Azran, Erion, Nippleheim, Noatum.
UPDATE guild_zone SET guild_spawn_x = 2086, guild_spawn_y = 2093 WHERE zone = 0;
UPDATE guild_zone SET guild_spawn_x = 2494, guild_spawn_y = 1707 WHERE zone = 1;
UPDATE guild_zone SET guild_spawn_x = 2453, guild_spawn_y = 2000 WHERE zone = 2;
UPDATE guild_zone SET guild_spawn_x = 3652, guild_spawn_y = 3122 WHERE zone = 3;
UPDATE guild_zone SET guild_spawn_x = 1050, guild_spawn_y = 1706 WHERE zone = 4;
