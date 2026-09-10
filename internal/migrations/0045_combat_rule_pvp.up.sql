-- 0045_combat_rule_pvp — o dano em jogador entra na regra de combate.
--
-- Dois botões novos na mesma linha de combat_rule (0044):
--
--   pvp_skill_pct  quanto sobra do golpe de SKILL em outro jogador.
--   pvp_melee_pct  quanto sobra do golpe FÍSICO em outro jogador.
--
-- No legado todo golpe em jogador já é dividido por 4 — a Perfuração,
-- _MSG_Attack.cpp:1300-1307. Estes valem por cima disso: 100 é o legado
-- exato, 50 dá metade, 200 o dobro. A divisão por 4 foi balanceada para a
-- escala de dano original; o dano deste servidor cresceu mais que as barras de
-- vida, e estes são os botões de quantos golpes uma luta entre iguais deve
-- levar. Skill e golpe físico ficam separados porque não cresceram juntos.
--
-- DEFAULT 100 NOT NULL de propósito: uma linha gravada antes desta migração
-- ganha o legado nos dois campos e continua sendo uma regra válida. Sem o
-- DEFAULT ela voltaria com zero, fora da faixa, e o tmServer descartaria a
-- regra inteira — a equipe veria a tela certa e o jogo rodando outra coisa.
--
-- A faixa é 1..200: zero apagaria o dano em jogador sem ninguém perceber pelo
-- nome do botão, e acima do dobro nunca foi jogado. combat_rule_range_test.go
-- confere que ela bate com internal/combatrule.
--
-- Lido AO VIVO, como o resto da linha: nada muda no jogo até alguém gravar.

ALTER TABLE combat_rule
    ADD COLUMN pvp_skill_pct SMALLINT NOT NULL DEFAULT 100 CHECK (pvp_skill_pct BETWEEN 1 AND 200),
    ADD COLUMN pvp_melee_pct SMALLINT NOT NULL DEFAULT 100 CHECK (pvp_melee_pct BETWEEN 1 AND 200);
