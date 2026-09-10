-- 0046_combat_rule_precisao — a precisão da magia entra na regra de combate.
--
-- Dois botões novos na mesma linha de combat_rule (0044, 0045):
--
--   spell_int_accuracy_pct  quanto da INT conta como DES na precisão de uma
--                           SKILL contra a esquiva. O legado só olha a DES de
--                           quem ataca (o attackerdex de GetParryRate,
--                           _MSG_Attack.cpp:1410), então um mago de INT cheia
--                           acerta como um personagem de DES 12 — uma FM com
--                           INT 3.148 errava ~43% das magias numa TK de DES 700.
--                           O jogo usa o maior entre DES e INT×pct/100; 0 é o
--                           legado.
--   max_miss_streak         quantas esquivas seguidas de skill no mesmo alvo se
--                           aceitam antes de a próxima ser forçada a acertar.
--                           0 desliga (legado: cada sorteio vale sozinho).
--
-- ATENÇÃO aos DEFAULTs: são o padrão DECIDIDO para este servidor (50% e 2), e
-- não o legado — ao contrário da 0045, cujo DEFAULT 100 era o legado exato. É
-- de propósito: uma linha já gravada no painel passa a ter o comportamento novo
-- nos dois campos, como um servidor sem linha nenhuma (combatrule.Default). Isso
-- vale até para uma linha gravada com o atalho do Kersef: ela vira o Kersef com
-- a precisão nova. Quem quiser o Kersef puro de novo clica o atalho outra vez,
-- que agora manda 0 e 0.
--
-- As faixas repetem internal/combatrule, e combat_rule_range_test.go confere
-- que as faixas e os DEFAULTs continuam batendo com o código.
--
-- Lido AO VIVO, como o resto da linha.

ALTER TABLE combat_rule
    ADD COLUMN spell_int_accuracy_pct SMALLINT NOT NULL DEFAULT 50 CHECK (spell_int_accuracy_pct BETWEEN 0 AND 100),
    ADD COLUMN max_miss_streak        SMALLINT NOT NULL DEFAULT 2  CHECK (max_miss_streak BETWEEN 0 AND 10);
