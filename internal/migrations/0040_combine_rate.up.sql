-- 0040_combine_rate — a Mesa das Máquinas: as taxas de combinação por máquina e,
-- para o refino +10, por faixa de item.
--
-- Substitui a edição à mão de Release/Common/Settings/CompRate.txt. A ausência de
-- linha significa "usa o arquivo", que por sua vez cai no padrão compilado — o
-- servidor roda igual quando o dbServer não responde.
--
-- Duas tabelas porque são duas perguntas diferentes:
--
-- combine_rate guarda a taxa de UMA chave (família + chave), que é a unidade que
-- o CompRate.txt já usa: "Ailyn ChanceBase 10", "Ehre Espiritual 40". É o que a
-- taxa fixa de ADD (Agatha) e ABS (Ehre) precisa, e o que as sete receitas do
-- Ehre precisam para serem ajustadas uma a uma.
--
-- combine_band guarda a curva por faixa de item do refino +10, que o legado não
-- tem: um multiplicador sobre a taxa da máquina, escolhido pelo ReqLvl do item.
--
-- Por que ReqLvl e não o grau: o grau NÃO separa conjunto. Levantamento de
-- 09/09/2026 sobre o ItemList.csv — 470 das 654 armas dividem o mesmo grau 6,
-- enquanto o ReqLvl as distribui em faixas de 55 a 158 itens. O grau também
-- mente entre variantes: Chapéu_de_Mytril (N)/(M)/(A) têm os três grau 4 e
-- ReqLvl 137/150/164.
--
-- Por que a faixa é por TIPO e não uma régua só: armas e armaduras se concentram
-- em lugares opostos. Entre ReqLvl 200 e 249 há 112 armas e UMA armadura; entre
-- 250 e 299 há 49 armaduras e 4 armas. Uma régua única faria o moderador ajustar
-- faixas quase vazias e achar que não fez nada.
--
-- O nome da faixa é livre e fica guardado: "Armas C" e "Armas D" são vocabulário
-- do servidor, não do código, e é assim que o moderador raciocina sobre elas.
--
-- O histórico não mora aqui: quem edita é o painel, que já tem o admin_audit_log
-- append-only (migração 0022), e é lá que a taxa antiga e a nova ficam.

CREATE TABLE combine_rate (
    family        TEXT     NOT NULL CHECK (family <> ''),
    rate_key      TEXT     NOT NULL CHECK (rate_key <> ''),
    -- 0..100 é a faixa que o parser do legado aceita (CReadFiles.cpp valida o
    -- valor lido), e manter o mesmo teto evita que o painel grave algo que o
    -- arquivo não poderia expressar.
    rate          SMALLINT NOT NULL CHECK (rate BETWEEN 0 AND 100),
    updated_by    BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (family, rate_key)
);

CREATE TABLE combine_band (
    -- 1 arma, 2 armadura. O tipo sai do nPos do item: 64/128/192 são as armas,
    -- 2/4/8/16/32 as cinco peças de armadura.
    slot_kind     SMALLINT NOT NULL CHECK (slot_kind IN (1, 2)),
    req_lvl_min   INTEGER  NOT NULL CHECK (req_lvl_min >= 0),
    req_lvl_max   INTEGER  NOT NULL CHECK (req_lvl_max >= 0),
    label         TEXT     NOT NULL CHECK (label <> ''),
    -- Multiplicador em centésimos para não guardar float: 100 é neutro, 180 é
    -- 1,8×, 40 é 0,4×. O teto de 1000 (10×) existe porque a taxa resultante é
    -- limitada a 100 de qualquer forma, e um número maior só esconderia um erro
    -- de digitação.
    mult_pct      INTEGER  NOT NULL DEFAULT 100 CHECK (mult_pct BETWEEN 0 AND 1000),
    updated_by    BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (slot_kind, req_lvl_min),
    CONSTRAINT combine_band_range CHECK (req_lvl_max >= req_lvl_min)
);

-- A versão é o que o tmServer compara para saber se precisa reler, no mesmo
-- formato de xp_rule_meta e world_event_meta.
CREATE TABLE combine_rate_meta (
    id      BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    version BIGINT NOT NULL DEFAULT 0
);
INSERT INTO combine_rate_meta (id, version) VALUES (TRUE, 0);
