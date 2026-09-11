-- 0057_campo_de_treino_drops — o saque do campo de treino do novato.
--
-- Pedido do Marco em 11/09/2026: "um pouco de gold e Repletion A. Pouca chance
-- de Ori e de Lac". Vale para os três bichos que só nascem no campo — Porco,
-- Aguia e Serpente — e para nenhum outro: Krill, Gremlin, Rei_Gremlin e
-- Chefe_Krill também nascem em volta de Armia, sem o limite de nível do campo, e
-- uma regra neles valeria lá também.
--
-- Chance em centésimos de por cento, por morte e exata (a Mesa de Drops não
-- passa pelo bônus de drop):
--
--   Repletion A (4016, Classe_A)    500 = 5%    — 1 a cada 20 mortes
--   Poeira de Oriharucon (412)       50 = 0,5%  — 1 a cada 200
--   Poeira de Lactolerium (413)      20 = 0,2%  — 1 a cada 500
--
-- A Bolsa da Sorte (4104) sai do Porco e da Águia: os dois soltavam ~27 a cada
-- 100 mortes, uma por espaço, e com a bolsa cheia o personagem novo perdia o
-- Repletion e as poeiras por falta de lugar.
--
-- O ouro não cabe aqui, porque a Mesa não dá moeda: é o Coin dos templates Aguia
-- e Porco, mudado no mesmo commit.
--
-- As regras da Águia só pegam porque ela deixou de ser NPC no mesmo deploy
-- (internal/campotreino): como linha de npc_definition ela renascia sem nome, e
-- sem nome fica fora da Mesa.
--
-- ON CONFLICT DO NOTHING: uma linha que alguém já gravou pelo painel vale mais
-- que a proposta.

INSERT INTO drop_rule (mob, item, chance) VALUES
    ('Porco',    4016, 500), ('Aguia',    4016, 500), ('Serpente', 4016, 500),
    ('Porco',     412,  50), ('Aguia',     412,  50), ('Serpente',  412,  50),
    ('Porco',     413,  20), ('Aguia',     413,  20), ('Serpente',  413,  20),
    ('Porco',    4104,   0), ('Aguia',    4104,   0)
ON CONFLICT (mob, item) DO NOTHING;

UPDATE drop_rule_meta SET version = version + 1 WHERE id = TRUE;
