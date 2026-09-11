DELETE FROM drop_rule
 WHERE mob IN ('Porco', 'Aguia', 'Serpente') AND item IN (4016, 412, 413, 4104);
UPDATE drop_rule_meta SET version = version + 1 WHERE id = TRUE;
