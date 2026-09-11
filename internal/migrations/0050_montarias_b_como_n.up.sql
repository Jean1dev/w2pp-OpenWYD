-- 0050_montarias_b_como_n — as cinco montarias B recebem o que a equipe
-- configurou nas N correspondentes (pedido de 2026-09-11, a partir da tela
-- /rates/montarias):
--
--   2371 Cavalo s/Sela B    ← 2366 Cavalo s/Sela N
--   2372 Cavalo Fantasma B  ← 2367 Cavalo Fantasma N
--   2373 Cavalo Leve B      ← 2368 Cavalo Leve N
--   2374 Cavalo Equipado B  ← 2369 Cavalo Equipado N
--   2375 Andaluz B          ← 2370 Andaluz N
--
-- Copia o crescimento (chance do âmago nas seis faixas), a absorção PvP/PvE e o
-- dano e a magia. NÃO copia a evasão nem a imunidade:
--   - imunidade: as B mantêm a do padrão delas (16, 20, 24, 28, 32 — a tabela
--     do cliente em internal/mountbonus), que é o que as distingue das N;
--   - evasão: as B não têm (0 no padrão), e a equipe vai pôr à mão no painel.
-- Uma B que já tenha linha em mount_bonus fica com a evasão e a imunidade que
-- tem; só dano e magia mudam.
--
-- Os valores são os da tela no momento do pedido. O dano e a magia estão
-- guardados como COEFICIENTES; a tela mostra o que eles dão no nível 120
-- ((120+20)*dano/100 e (120+15)*magia/100), e cada número da tela corresponde a
-- um único coeficiente inteiro: 392/67 → 280/50, 462/87 → 330/65,
-- 532/94 → 380/70, 630/103 → 450/77, 812/128 → 580/95
-- (montarias_b_test.go confere a conta).
--
-- Vale quando o jogo reiniciar, como toda a tela de montarias.

INSERT INTO mount_growth_rate (mount_index, band, rate, updated_by)
SELECT v.mount_index, v.band, v.rate, 'migração 0050 (B como N)'
  FROM (VALUES
    (2371, 0, 50), (2371, 1, 40), (2371, 2, 33), (2371, 3, 30), (2371, 4, 28), (2371, 5, 27),
    (2372, 0, 44), (2372, 1, 40), (2372, 2, 33), (2372, 3, 30), (2372, 4, 25), (2372, 5, 25),
    (2373, 0, 43), (2373, 1, 38), (2373, 2, 32), (2373, 3, 26), (2373, 4, 25), (2373, 5, 24),
    (2374, 0, 42), (2374, 1, 38), (2374, 2, 30), (2374, 3, 25), (2374, 4, 23), (2374, 5, 20),
    (2375, 0, 40), (2375, 1, 34), (2375, 2, 33), (2375, 3, 22), (2375, 4, 23), (2375, 5, 20)
  ) AS v(mount_index, band, rate)
ON CONFLICT (mount_index, band) DO UPDATE SET
    rate = EXCLUDED.rate, updated_by = EXCLUDED.updated_by, updated_at = now();

INSERT INTO mount_absorb (mount_index, absorb_pvp, absorb_pve, updated_by) VALUES
    (2371, 15, 25, 'migração 0050 (B como N)'),
    (2372, 17, 25, 'migração 0050 (B como N)'),
    (2373, 19, 25, 'migração 0050 (B como N)'),
    (2374, 23, 25, 'migração 0050 (B como N)'),
    (2375, 27, 25, 'migração 0050 (B como N)')
ON CONFLICT (mount_index) DO UPDATE SET
    absorb_pvp = EXCLUDED.absorb_pvp, absorb_pve = EXCLUDED.absorb_pve,
    updated_by = EXCLUDED.updated_by, updated_at = now();

INSERT INTO mount_bonus (mount_index, attack, magic, evasion, resist, updated_by) VALUES
    (2371, 280, 50, 0, 16, 'migração 0050 (B como N)'),
    (2372, 330, 65, 0, 20, 'migração 0050 (B como N)'),
    (2373, 380, 70, 0, 24, 'migração 0050 (B como N)'),
    (2374, 450, 77, 0, 28, 'migração 0050 (B como N)'),
    (2375, 580, 95, 0, 32, 'migração 0050 (B como N)')
ON CONFLICT (mount_index) DO UPDATE SET
    attack = EXCLUDED.attack, magic = EXCLUDED.magic,
    updated_by = EXCLUDED.updated_by, updated_at = now();
