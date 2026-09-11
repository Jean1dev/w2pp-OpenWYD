-- 0059_combat_rule_ataque_fisico — o Ataque físico do jogador ganha escala.
--
-- Medido em jogo com /gm dano (2026-09-12): uma TK +11 marcava 10.941 de
-- Ataque e uma HT +11, 10.755. A conta fecha e o grosso não é mais o bônus de
-- arma da classe (2.079, já contado uma vez pela 0052): são os buffs, que
-- multiplicam por ~1,63 (136% do DAMAGEMULTI mais 20% da Divina), a montaria
-- (812) e o dano da arma (1.168).
--
-- Alvo pedido: ~6.800 na TK e ~6.500 na HT. 61% leva a 6.674 e 6.560.
--
--   physical_damage_pct  escala do Ataque físico do JOGADOR, 1 a 200.
--                        100 deixa como calculado; monstro e evocação nunca
--                        são escalados.
--
-- Aplicada no FIM de effectiveDamage, depois dos buffs e do dano da arma: o
-- número que sai é o da janela do personagem e o do golpe, sem divergir.
--
-- DEFAULT 61 é o padrão DECIDIDO, como nas 0046, 0052 e 0054: a linha já
-- gravada no painel passa a usar a escala assim que a migração roda. O atalho
-- do Kersef manda 100.
--
-- A faixa repete internal/combatrule; combat_rule_range_test.go confere faixa e
-- DEFAULT contra o código. Lido AO VIVO, como o resto da linha.

ALTER TABLE combat_rule
    ADD COLUMN physical_damage_pct SMALLINT NOT NULL DEFAULT 61 CHECK (physical_damage_pct BETWEEN 1 AND 200);
