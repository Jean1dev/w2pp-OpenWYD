-- A Mesa de XP deixou de ter só as sete zonas do legado.
--
-- 0030 gravou `CHECK (zone BETWEEN 0 AND 6)`, que era exatamente o conjunto de
-- branches do MobKilled.cpp na época. O Deserto foi separado em cinco zonas
-- próprias (7..11, internal/level.ZoneDeserto*) e o CHECK passou a recusar a
-- gravação com um erro que o painel só conseguia relatar como "erro ao gravar
-- a zona Deserto Pilar".
--
-- O teto novo é folgado de propósito. Amarrar o banco à contagem exata de
-- zonas do código significa uma migração a cada zona nova, e foi justamente
-- isso que quebrou aqui. O CHECK continua servindo para o que importa —
-- barrar lixo e valor negativo — enquanto quem decide quais zonas existem é
-- internal/level.Zones(), que o painel já consulta para validar o formulário.
-- Uma linha órfã de uma zona que o código não conhece é inofensiva: Zone.rule()
-- cai no campo para índice desconhecido.
ALTER TABLE xp_rule DROP CONSTRAINT IF EXISTS xp_rule_zone_check;
ALTER TABLE xp_rule ADD CONSTRAINT xp_rule_zone_check CHECK (zone BETWEEN 0 AND 63);
