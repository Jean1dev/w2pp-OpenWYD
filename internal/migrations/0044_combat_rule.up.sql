-- 0044_combat_rule — a regra de combate que a equipe gira pelo painel.
--
-- Três botões (internal/combatrule), numa linha só, porque são uma decisão só
-- sobre o que a magia é neste servidor:
--
--   weapon_int_magic_pct  quanto do termo de INT da arma entra na Magia. Com a
--                         oitava skill aprendida, o Kersef soma
--                         (DES×k + INT×k)/100 conforme a arma. 0 = a Magia vem
--                         só do equipamento, montaria e buffs; 100 = o Kersef.
--   spell_damage_multi    se os buffs de porcentagem de dano (poções, Assalto,
--                         Meditação, transformações) multiplicam também a magia,
--                         como no Kersef, ou só o golpe físico, como no legado.
--   mob_resist_base       a constante da escala de resistência da magia contra
--                         MONSTRO, (base − resist/2)%. O legado usa 150, o que dá
--                         +50% contra qualquer mob de resistência baixa. Contra
--                         jogador os 150 ficam, diga isto o que disser.
--
-- Isto é lido AO VIVO, como spawn_rate e dungeon_gate: o tmServer pergunta a
-- versão a cada ~15s e, quando ela muda, relê a linha e refaz o score de quem
-- está online, para a Magia nova chegar à janela sem ninguém relogar. É o tipo
-- de botão que se ajusta olhando uma luta, e reiniciar a cada volta custaria
-- uma queda para todo mundo que está jogando.
--
-- A tabela nasce VAZIA, e ausência de linha é a regra DECIDIDA para este
-- servidor (combatrule.Default: termo 0%, multiplicador fora da magia, base
-- 100), não o comportamento portado. O Kersef (combatrule.Kersef: 100%,
-- multiplicador na magia, base 150) fica no código para comparar e voltar.
-- Voltar ao padrão é apagar a linha.
--
-- Os CHECKs repetem as faixas de internal/combatrule, e
-- combat_rule_range_test.go confere que os dois continuam batendo.

CREATE TABLE combat_rule (
    id                   BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    weapon_int_magic_pct SMALLINT NOT NULL CHECK (weapon_int_magic_pct BETWEEN 0 AND 100),
    spell_damage_multi   BOOLEAN  NOT NULL,
    mob_resist_base      SMALLINT NOT NULL CHECK (mob_resist_base BETWEEN 50 AND 150),
    updated_by           BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE combat_rule_meta (
    id      BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    version BIGINT NOT NULL DEFAULT 0
);
INSERT INTO combat_rule_meta (id, version) VALUES (TRUE, 0);
