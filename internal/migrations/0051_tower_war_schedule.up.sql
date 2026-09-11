-- 0051_tower_war_schedule — a Guerra de Torres ganha interruptor e horário próprios.
--
-- A guerra diária de torres só rodava com o "evento de novato" ligado
-- (newbie_event_enabled, 0015), e esse mesmo interruptor também muda a XP e a
-- vida dos monstros para os níveis baixos. Eram duas decisões amarradas num
-- botão só: quem quisesse a guerra levava o bônus de novato junto, e quem
-- desligasse o bônus perdia a guerra.
--
-- Decidido: a guerra roda TODO DIA às 20h. Então os DEFAULTs são essa decisão
-- — ligada, 20h —, e a linha que já existe em world_event_config passa a ter a
-- guerra ligada às 20h assim que esta migração roda, sem ninguém precisar
-- entrar no painel. O horário é do relógio do servidor.
--
--   tower_war_enabled  liga ou desliga a guerra diária.
--   tower_war_hour     a hora (0 a 23) em que ela começa: aviso nos primeiros
--                      5 minutos, a torre aparece aos 6 e a guerra termina aos
--                      30. A guilda com a torre no fim ganha +100 de fama.
--
-- Lido AO VIVO, como o resto da linha: o tmServer relê world_event_config
-- quando a versão muda.

ALTER TABLE world_event_config
    ADD COLUMN tower_war_enabled BOOLEAN  NOT NULL DEFAULT TRUE,
    ADD COLUMN tower_war_hour    SMALLINT NOT NULL DEFAULT 20 CHECK (tower_war_hour BETWEEN 0 AND 23);
