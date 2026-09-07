-- Volta ao intervalo das sete zonas do legado.
--
-- As linhas fora dele precisam sair antes, senão o ALTER falha: o Postgres
-- valida o CHECK novo contra o que já está gravado. Isso descarta as tabelas
-- do Deserto, que é o que reverter esta migração significa — sem elas o
-- Deserto volta a pagar taxa de campo, como no legado.
DELETE FROM xp_rule WHERE zone > 6;
ALTER TABLE xp_rule DROP CONSTRAINT IF EXISTS xp_rule_zone_check;
ALTER TABLE xp_rule ADD CONSTRAINT xp_rule_zone_check CHECK (zone BETWEEN 0 AND 6);
