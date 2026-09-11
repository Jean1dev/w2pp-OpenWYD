-- 0052_combat_rule_bonus_arma — o bônus de arma da classe conta uma vez só.
--
-- O ataque físico recebe, por classe e tipo de arma, DES×a + FOR×b
-- (score_derive.go classWeaponDamage). O legado soma esse termo UMA VEZ POR
-- SKILL DE EVOLUÇÃO aprendida (Basedef.cpp:3252/3309/3378 no TK: Confiança,
-- Trans, Espada Mágica; o mesmo em FM e BM), então quem tem as três leva o
-- termo três vezes. Numa TK +11 com FOR 2.802 e Espada de 2 mãos eram ~6.200
-- de um Ataque de 12.630 — metade do golpe vinha só da repetição.
--
-- Na magia o mesmo termo já contava uma vez (issue #280). Decidido
-- (2026-09-11): o físico segue a mesma regra.
--
--   weapon_damage_grants  quantas evoluções aprendidas somam o bônus de arma
--                         (1 a 3). 3 é o legado.
--
-- O DEFAULT 1 é o padrão DECIDIDO, como na 0046: a linha que já existe passa a
-- contar o bônus uma vez assim que a migração roda. O atalho do Kersef no painel
-- manda 3.
--
-- A faixa repete internal/combatrule; combat_rule_range_test.go confere a faixa
-- e o DEFAULT contra o código. Lido AO VIVO, como o resto da linha.

ALTER TABLE combat_rule
    ADD COLUMN weapon_damage_grants SMALLINT NOT NULL DEFAULT 1 CHECK (weapon_damage_grants BETWEEN 1 AND 3);
