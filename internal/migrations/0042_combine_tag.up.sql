-- 0042_combine_tag — a operação que os jogadores dão a cada máquina ou receita
-- (ADD, ABS), marcada pela equipe no painel.
--
-- Qual das receitas de Ankh do Ehre é a de absorção é vocabulário do servidor,
-- não algo que o código saiba — por isso fica numa tabela, e não fixo no painel.
-- É só rótulo: o jogo não lê esta tabela, e ela não mexe na versão da Mesa das
-- Máquinas. Sem linha, a receita não tem operação.

CREATE TABLE combine_tag (
    family      TEXT NOT NULL CHECK (family <> ''),
    rate_key    TEXT NOT NULL CHECK (rate_key <> ''),
    tag         TEXT NOT NULL CHECK (tag IN ('ADD', 'ABS')),
    updated_by  BIGINT REFERENCES account(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (family, rate_key)
);

-- A Agatha é o ADD — decidido pela equipe e já dito pela tela desde 0040.
INSERT INTO combine_tag (family, rate_key, tag) VALUES ('Agatha', 'ChanceBase', 'ADD');
