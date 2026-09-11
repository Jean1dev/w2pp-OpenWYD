-- 0056_boss_respawn — o chefe sozinho volta em horas, não em 15 segundos.
--
-- Um bloco do NPCGener.txt sem período de minuto (MinuteGenerate <= 0) nunca é
-- regenerado pelo original fora de evento: o servidor sobe vazio e o timer de
-- minuto pula esses blocos (ProcessSecMinTimer.cpp:2721-2728). O rewrite popula
-- tudo no boot e devolve o monstro morto em 15 s (world/api.go, divergência
-- deliberada). Para os blocos grandes isso é o mundo cheio que o jogo tem hoje;
-- para os de até três monstros que valem 1 milhão de XP ou mais — Verid, Sombra
-- Negra, Golem Ancião, os Frenzy do Castelo Zakun, 165 blocos — é uma fazenda de
-- chefe a cada 15 segundos.
--
-- Decidido (11/09/2026): esses voltam em horas, 24 por padrão, e o número mora
-- aqui, na linha que o tmServer relê AO VIVO, para a equipe mudar pelo painel.
-- O teto de 168 (uma semana) cabe com folga no relógio de 32 bits do mundo.
-- O Kefra e os guardas não entram: são semanais, com regra própria no código.

ALTER TABLE world_event_config
    ADD COLUMN boss_respawn_hours SMALLINT NOT NULL DEFAULT 24 CHECK (boss_respawn_hours BETWEEN 1 AND 168);
