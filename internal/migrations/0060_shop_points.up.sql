-- 0060_shop_points — os pontos que a lojinha aberta rende ao dono.
--
-- Moeda NOVA, deliberadamente separada de account.donate_balance. A carteira de
-- donate é dinheiro que alguém pagou, e é ela que o painel de faturamento soma
-- como receita; ponto de lojinha é tempo, não dinheiro. Misturar os dois faria o
-- relatório de receita contar como venda o que foi farm, e não haveria como
-- separar depois.
--
-- Não existe nada disso no legado. Lá a lojinha custava o personagem — o
-- vendedor ERA a barraca e ficava preso nela — e portanto se pagava sozinha. Aqui
-- a barraca é um clone e o dono sai andando, então a recompensa precisa de chão:
-- o tmServer só paga janela de quinze minutos com pelo menos um item à venda
-- (handler.shopStocked).
--
-- O saldo é somado pelo BANCO (balance = balance + delta), nunca escrito de volta
-- como total calculado pelo servidor: dois personagens da mesma conta podem
-- fechar janela ao mesmo tempo, e um read-modify-write perderia um dos dois.

CREATE TABLE IF NOT EXISTS shop_points (
    account_id BIGINT PRIMARY KEY REFERENCES account(id) ON DELETE CASCADE,
    balance    INTEGER NOT NULL DEFAULT 0 CHECK (balance >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- O extrato. Espelha donate_shop_audit: existe para responder "de onde veio esse
-- saldo" sem depender do log do processo, que rotaciona.
--
-- character_name é informativo e fica solto de propósito (sem FK): o ponto é da
-- CONTA, o personagem só registra quem estava com a barraca de pé, e nome de
-- personagem não é único neste servidor — Arch e Celestial herdam o nome do
-- Mortal.
CREATE TABLE IF NOT EXISTS shop_points_audit (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id     BIGINT NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    character_name TEXT NOT NULL DEFAULT '',
    delta          INTEGER NOT NULL,
    balance_after  INTEGER NOT NULL,
    reason         TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS shop_points_audit_account_id_idx
    ON shop_points_audit (account_id, created_at DESC);
