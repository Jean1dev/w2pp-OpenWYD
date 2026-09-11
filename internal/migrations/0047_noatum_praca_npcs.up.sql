-- 0047_noatum_praca_npcs — a praça de Noatum fica só com o GodGovernment e a
-- Perzen; o Curandeiro de Armia sai também. Pedido da equipe em 2026-09-11.
--
-- Todos os NPCs retirados da praça são blocos do NPCGener com MinuteGenerate
-- -1, e o legado NUNCA gera um bloco assim sozinho: o laço do minuto pula
-- MinuteGenerate <= 0 (ProcessSecMinTimer.cpp:2727), e só um evento os chama.
-- É o port que povoa esses blocos no boot, e foi isso que encheu a praça. A
-- Perzen e o GodGovernment são MinuteGenerate 1 — o legado os tinha.
--
-- Os blocos NÃO saem do NPCGener.txt: o índice de um bloco é a posição dele no
-- arquivo, e apagar um desloca todos os seguintes — os índices fixos no código
-- (Kefra 396, salas secretas, água) e os slugs do banco (o que duplicou os NPCs
-- de Armia em dc41a14f). Por serem mercadores, eles são do npc_definition, e
-- desligar ali é o mesmo "enabled" que o painel de NPCs mostra: volta com um
-- clique. O seed do dbserver não mexe em "enabled" num slug que já existe
-- (store.seedNPCRows), então a escolha sobrevive aos boots.
--
-- As torres de Thor (23-26) não são mercadores e ficam no código
-- (world.eventOwnedGenerators): são da Guerra de Noatum.
--
--   3903 Camponesa_   4511 Merc_Fantasma   4849/4852 Mercador (montados)
--   5995-5998 Torcedor, Torcedor_, Torcedor__, Torcedor___   5999 Pescador
--   6058 Cap_Rowena   6077 Curandeiro (Armia, 2126,2114)
--
-- Cap_Rowena-6062 é a linha antiga do seed curado (0006) para o mesmo NPC, de
-- quando o bloco era o 6062; ela nasce pela posição e era o segundo Cap Rowena.

UPDATE npc_definition SET enabled = FALSE
 WHERE (origin = 'content' AND generator_index IN (3903, 4511, 4849, 4852, 5995, 5996, 5997, 5998, 5999, 6058, 6077))
    OR slug IN ('Cap_Rowena-6062', 'Merc_Fantasma-4511', 'Pescador-5999', 'Torcedor-5995');

UPDATE npc_config_meta SET version = version + 1 WHERE id = TRUE;
