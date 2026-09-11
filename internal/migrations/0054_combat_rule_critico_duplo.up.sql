-- 0054_combat_rule_critico_duplo — o crítico duplo volta, com teto.
--
-- O crítico duplo (golpe ×2) sai quando a posição da tabela de acerto (0 a 999)
-- fica abaixo de 100×(velocidade de ataque − 5) (Basedef.cpp:6149). A
-- velocidade de ataque do legado é 50 + velocidade dos itens + buffs +
-- transformação + DES/5, com teto 150 (Basedef.cpp:3200, 4675, 4704-4711).
-- Este servidor montava só 50 + buffs, então ninguém passava de 5 e o crítico
-- duplo nunca saía. Com a fórmula do legado ele sai 10% por ponto acima de 5:
-- DES 100 → 20%, DES 250 → 50%, DES 500 ou mais → TODO golpe ×2.
--
-- Decidido (2026-09-11): a fórmula do legado, com teto no painel.
--
--   double_critical_max_pct  chance máxima do crítico duplo, 0 a 100.
--                            100 é o legado; 0 desliga.
--
-- O DEFAULT 25 é o padrão DECIDIDO, como na 0046 e na 0052: a linha já gravada
-- no painel passa a usar o teto assim que a migração roda. O atalho do Kersef
-- manda 100.
--
-- A faixa repete internal/combatrule; combat_rule_range_test.go confere a faixa
-- e o DEFAULT contra o código. Lido AO VIVO, como o resto da linha.

ALTER TABLE combat_rule
    ADD COLUMN double_critical_max_pct SMALLINT NOT NULL DEFAULT 25 CHECK (double_critical_max_pct BETWEEN 0 AND 100);
