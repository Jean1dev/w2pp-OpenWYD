-- 0055_erion_lojas_npcs — a fileira de lojas de Erion sai: AcessoriosErion e os
-- quatro Sets (TK, BM, HT, FM). Pedido da equipe em 2026-09-11.
--
-- Mesmo caminho da 0047: os blocos NÃO saem do NPCGener.txt (o índice de um
-- bloco é a posição dele no arquivo, e apagar um desloca todos os seguintes).
-- Por serem mercadores, eles são do npc_definition, e desligar ali é o mesmo
-- "enabled" que o painel de NPCs mostra: volta com um clique. O seed do
-- dbserver não mexe em "enabled" num slug que já existe (store.seedNPCRows),
-- então a escolha sobrevive aos boots.
--
-- Os rótulos "#[n]" do arquivo dizem 6098-6102 para estes blocos. Estão velhos
-- (blocos acima deles saíram ou foram comentados) e NÃO são o índice: os
-- números abaixo saem do parser, e TestBlocosDesligadosPorIndice os prende aos
-- NPCs certos.
--
--   6084 AcessoriosErion (2457,1987)   6085 Set_TK_Erion (2459,1987)
--   6086 Set_BM_Erion    (2459,1990)   6087 Set_HT_Erion (2462,1990)
--   6088 Set_FM_Erion    (2464,1990)
--
-- As lojas iguais de Armia (Set_BM/FM/HT/TK 6079-6082, Acessorios5 6083) ficam.

UPDATE npc_definition SET enabled = FALSE
 WHERE origin = 'content' AND generator_index IN (6084, 6085, 6086, 6087, 6088);

UPDATE npc_config_meta SET version = version + 1 WHERE id = TRUE;
